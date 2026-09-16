// SPDX-License-Identifier: MIT

// Spec rules 1–12, one describe per rule, exercised directly on the
// model rather than through the corpus.
//
// The corpus pins the rules it happens to reach: none of its fixtures
// carries a non-NFC value, a line long enough to fold, a non-ASCII UID,
// a binary ATTACH, or an X-VSTAR-HASH inside an .ics. These tests cover
// the rest of rules 1–12 so a regression surfaces here, at the rule it
// broke, rather than as a corpus byte offset several layers away.

import { describe, expect, it } from "vitest";

import {
  calendar as canonicalCalendar,
  component as canonicalComponent,
  componentInContext,
} from "../src/canonical/index.js";
import { X_VSTAR_HASH_PROPERTY, calendar as hashCalendar } from "../src/hashing/index.js";
import type { Calendar, Component, Param, Property } from "../src/types.js";

const utf8 = new TextDecoder("utf8");

/** The canonical bytes of `c` decoded as UTF-8, for readable assertions. */
function text(bytes: Uint8Array): string {
  return utf8.decode(bytes);
}

function prop(name: string, value: string, params: Param[] = []): Property {
  return { name, params, value };
}

function todo(props: Property[], sub: Component[] = []): Component {
  return { type: "VTODO", props, sub };
}

function cal(components: Component[], prodId = "-//V*//Test//EN"): Calendar {
  return { prodId, components };
}

const DTSTAMP = prop("DTSTAMP", "20260504T120000Z");

describe("rule 1 — CRLF line endings", () => {
  it("terminates every physical line with CRLF", () => {
    const got = text(canonicalComponent(todo([DTSTAMP, prop("UID", "u")])));
    expect(got).toBe("BEGIN:VTODO\r\nDTSTAMP:20260504T120000Z\r\nUID:u\r\nEND:VTODO\r\n");
  });
});

describe("rule 2 — property and parameter order", () => {
  it("sorts properties alphabetically by name", () => {
    const got = text(
      canonicalComponent(
        todo([prop("SUMMARY", "s"), prop("UID", "u"), prop("DTSTAMP", "20260504T120000Z")]),
      ),
    );
    expect(got.split("\r\n").slice(1, 4)).toEqual([
      "DTSTAMP:20260504T120000Z",
      "SUMMARY:s",
      "UID:u",
    ]);
  });

  it("sorts each property's parameters alphabetically by name", () => {
    const got = text(
      canonicalComponent(
        todo([
          prop("ATTENDEE", "mailto:a@example.com", [
            { name: "ROLE", value: "CHAIR" },
            { name: "CN", value: "Ada" },
            { name: "PARTSTAT", value: "ACCEPTED" },
          ]),
        ]),
      ),
    );
    expect(got).toContain("ATTENDEE;CN=Ada;PARTSTAT=ACCEPTED;ROLE=CHAIR:mailto:a@example.com");
  });

  it("is stable for equally-named properties", () => {
    const got = text(
      canonicalComponent(todo([prop("MEMBER", "b"), prop("MEMBER", "a"), prop("MEMBER", "c")])),
    );
    expect(got.split("\r\n").slice(1, 4)).toEqual(["MEMBER:b", "MEMBER:a", "MEMBER:c"]);
  });
});

/**
 * Split canonical bytes into physical lines, measured as BYTES.
 *
 * Decoding first and splitting the text would be wrong here: RFC 5545
 * folding breaks on an octet boundary, which can land mid-UTF-8
 * sequence, and a decoder replaces each orphaned fragment with U+FFFD —
 * three octets where there was one. That inflation is exactly what a
 * length assertion would then misread as an over-long line.
 */
function physicalLines(bytes: Uint8Array): Uint8Array[] {
  const out: Uint8Array[] = [];
  let start = 0;
  for (let i = 0; i + 1 < bytes.length; i++) {
    if (bytes[i] === 0x0d && bytes[i + 1] === 0x0a) {
      out.push(bytes.subarray(start, i));
      i++;
      start = i + 1;
    }
  }
  return out;
}

describe("rule 3 — folding at 75 OCTETS, after assembly", () => {
  it("folds an ASCII line at exactly 75 octets", () => {
    const got = canonicalComponent(todo([prop("SUMMARY", "x".repeat(200))]));
    const lines = physicalLines(got);
    for (const line of lines) expect(line.length).toBeLessThanOrEqual(75);
    expect(lines[1]).toHaveLength(75);
  });

  it("measures UTF-8 bytes, not UTF-16 code units", () => {
    // 'é' is one UTF-16 code unit and two UTF-8 octets. Forty of them
    // plus `SUMMARY:` is 88 octets but only 48 code units, so a
    // code-unit-counting implementation would not fold this at all.
    const got = canonicalComponent(todo([prop("SUMMARY", "é".repeat(40))]));
    const lines = physicalLines(got);
    for (const line of lines) expect(line.length).toBeLessThanOrEqual(75);
    // A continuation line exists, and leads with the SP RFC 5545 §3.1
    // requires.
    expect(lines.length).toBeGreaterThan(3);
    expect(lines[2]?.[0]).toBe(0x20);
  });

  it("folds AFTER parameters are appended, not before", () => {
    // The parameters push the fold point into the value. Folding the
    // value alone would put the break 30 octets later and still unfold
    // to the same logical line, so only the byte positions catch it.
    const got = canonicalComponent(
      todo([
        prop("ATTENDEE", "mailto:someone-with-a-long-address@example.com", [
          { name: "CN", value: "A Person With A Fairly Long Common Name" },
        ]),
      ]),
    );
    const first = physicalLines(got)[1] as Uint8Array;
    expect(first).toHaveLength(75);
    expect(text(first).startsWith("ATTENDEE;CN=")).toBe(true);
  });
});

describe("rule 6 — top-level component order", () => {
  it("sorts by UID", () => {
    const got = text(
      canonicalCalendar(
        cal([
          todo([DTSTAMP, prop("UID", "charlie")]),
          todo([DTSTAMP, prop("UID", "alpha")]),
          todo([DTSTAMP, prop("UID", "bravo")]),
        ]),
      ),
    );
    expect(got.match(/UID:(\w+)/g)).toEqual(["UID:alpha", "UID:bravo", "UID:charlie"]);
  });

  it("sorts a VTIMEZONE by its TZID", () => {
    const tz = (id: string): Component => ({ type: "VTIMEZONE", props: [prop("TZID", id)], sub: [] });
    const got = text(canonicalCalendar(cal([tz("Zulu"), tz("Alpha"), tz("Mike")])));
    expect(got.match(/TZID:(\w+)/g)).toEqual(["TZID:Alpha", "TZID:Mike", "TZID:Zulu"]);
  });

  it("compares UIDs byte-wise on UTF-8, not by UTF-16 code unit", () => {
    // U+FF21 FULLWIDTH LATIN CAPITAL A encodes as EF BC A1 in UTF-8 and
    // sits at 0xFF21 in UTF-16; U+1D400 MATHEMATICAL BOLD CAPITAL A
    // encodes as F0 9D 90 80 but is the surrogate pair D835 DC00 in
    // UTF-16. Byte-wise the fullwidth A sorts FIRST (0xEF < 0xF0);
    // by UTF-16 code unit it sorts LAST (0xFF21 > 0xD835).
    const got = text(
      canonicalCalendar(
        cal([
          todo([DTSTAMP, prop("UID", "\u{1D400}")]),
          todo([DTSTAMP, prop("UID", "Ａ")]),
        ]),
      ),
    );
    expect(got.indexOf("UID:Ａ")).toBeLessThan(got.indexOf("UID:\u{1D400}"));
  });

  it("sorts key-less components last, in stable input order", () => {
    const keyless = (s: string): Component => todo([DTSTAMP, prop("SUMMARY", s)]);
    const got = text(
      canonicalCalendar(
        cal([keyless("second"), todo([DTSTAMP, prop("UID", "u")]), keyless("third")]),
      ),
    );
    expect(got.match(/UID:u|SUMMARY:(\w+)/g)).toEqual(["UID:u", "SUMMARY:second", "SUMMARY:third"]);
  });

  it("preserves sub-component input order", () => {
    const alarm = (uid: string): Component => ({
      type: "VALARM",
      props: [prop("UID", uid), prop("ACTION", "DISPLAY")],
      sub: [],
    });
    const got = text(
      canonicalComponent(todo([DTSTAMP, prop("UID", "u")], [alarm("zulu"), alarm("alpha")])),
    );
    expect(got.indexOf("UID:zulu")).toBeLessThan(got.indexOf("UID:alpha"));
  });
});

describe("rule 7 — X-VSTAR-HASH exclusion", () => {
  it("strips the property from the canonical bytes", () => {
    const got = text(
      canonicalComponent(
        todo([DTSTAMP, prop("UID", "u"), prop(X_VSTAR_HASH_PROPERTY, "sha256:deadbeef")]),
      ),
    );
    expect(got).not.toContain(X_VSTAR_HASH_PROPERTY);
  });

  it("hashes identically with and without a stored hash", () => {
    const bare = cal([todo([DTSTAMP, prop("UID", "u")])]);
    const stamped = cal([todo([DTSTAMP, prop("UID", "u"), prop(X_VSTAR_HASH_PROPERTY, "sha256:00")])]);
    expect(hashCalendar(stamped)).toBe(hashCalendar(bare));
  });

  it("strips it at nested depth too", () => {
    const alarm = (props: Property[]): Component => ({ type: "VALARM", props, sub: [] });
    const bare = cal([todo([DTSTAMP, prop("UID", "u")], [alarm([prop("ACTION", "DISPLAY")])])]);
    const stamped = cal([
      todo(
        [DTSTAMP, prop("UID", "u")],
        [alarm([prop("ACTION", "DISPLAY"), prop(X_VSTAR_HASH_PROPERTY, "sha256:ff")])],
      ),
    ]);
    expect(hashCalendar(stamped)).toBe(hashCalendar(bare));
  });
});

describe("rule 8 — RRULE preserved verbatim", () => {
  it("does not normalize rule-part order or elided defaults", () => {
    const a = canonicalComponent(todo([prop("RRULE", "FREQ=DAILY;INTERVAL=1")]));
    const b = canonicalComponent(todo([prop("RRULE", "FREQ=DAILY")]));
    expect(text(a)).toContain("RRULE:FREQ=DAILY;INTERVAL=1");
    expect(text(b)).toContain("RRULE:FREQ=DAILY");
    expect(text(a)).not.toBe(text(b));
  });

  it("does not reorder rule parts", () => {
    const got = text(canonicalComponent(todo([prop("RRULE", "BYDAY=MO,WE;FREQ=WEEKLY")])));
    expect(got).toContain("RRULE:BYDAY=MO,WE;FREQ=WEEKLY");
  });
});

describe("rule 9 — NFC normalization", () => {
  const decomposed = "é"; // 'e' + COMBINING ACUTE ACCENT
  const composed = "é"; // 'é'

  it("normalizes property values", () => {
    const got = text(canonicalComponent(todo([prop("SUMMARY", `caf${decomposed}`)])));
    expect(got).toContain(`SUMMARY:caf${composed}`);
    expect(got).not.toContain(decomposed);
  });

  it("normalizes parameter values", () => {
    const got = text(
      canonicalComponent(
        todo([prop("ATTENDEE", "mailto:a@b.c", [{ name: "CN", value: `Ren${decomposed}` }])]),
      ),
    );
    expect(got).toContain(`CN=Ren${composed}`);
  });

  it("does NOT normalize property names", () => {
    // A name is ASCII by construction; the assertion is that the name
    // path is the uppercase path and nothing else touches it.
    const got = text(canonicalComponent(todo([prop("x-lower", "v")])));
    expect(got).toContain("X-LOWER:v");
  });

  it("makes two spellings of the same text hash identically", () => {
    const a = cal([todo([DTSTAMP, prop("UID", "u"), prop("SUMMARY", `caf${decomposed}`)])]);
    const b = cal([todo([DTSTAMP, prop("UID", "u"), prop("SUMMARY", `caf${composed}`)])]);
    expect(hashCalendar(a)).toBe(hashCalendar(b));
  });

  it("normalizes BEFORE folding, so the fold lands on the composed length", () => {
    // 40 decomposed 'é' are 120 UTF-8 octets; composed they are 80.
    // Folding before normalizing would place the break inside the
    // composed string at a different octet.
    const decomposedValue = decomposed.repeat(40);
    const composedValue = composed.repeat(40);
    const a = canonicalComponent(todo([prop("SUMMARY", decomposedValue)]));
    const b = canonicalComponent(todo([prop("SUMMARY", composedValue)]));
    expect(Array.from(a)).toEqual(Array.from(b));
  });

  it("is idempotent — canonical form is a fixpoint", () => {
    const c = todo([DTSTAMP, prop("UID", "u"), prop("SUMMARY", `caf${decomposed}`)]);
    const once = canonicalComponent(c);
    const twice = canonicalComponent({
      ...c,
      props: c.props.map((p) => ({ ...p, value: p.value.normalize("NFC") })),
    });
    expect(Array.from(once)).toEqual(Array.from(twice));
  });
});

describe("rule 10 — ATTACH reduces to URI form", () => {
  it("strips VALUE=BINARY and ENCODING=BASE64", () => {
    const got = text(
      canonicalComponent(
        todo([
          prop("ATTACH", "aGVsbG8=", [
            { name: "VALUE", value: "BINARY" },
            { name: "ENCODING", value: "BASE64" },
            { name: "FMTTYPE", value: "text/plain" },
          ]),
        ]),
      ),
    );
    expect(got).toContain("ATTACH;FMTTYPE=text/plain:aGVsbG8=");
    expect(got).not.toContain("BINARY");
    expect(got).not.toContain("BASE64");
  });

  it("matches the parameter names and values case-insensitively", () => {
    const got = text(
      canonicalComponent(
        todo([
          prop("ATTACH", "x", [
            { name: "value", value: "binary" },
            { name: "encoding", value: "base64" },
          ]),
        ]),
      ),
    );
    expect(got).toContain("ATTACH:x");
  });

  it("leaves a non-ATTACH VALUE=BINARY alone", () => {
    const got = text(
      canonicalComponent(todo([prop("X-BLOB", "x", [{ name: "VALUE", value: "BINARY" }])])),
    );
    expect(got).toContain("X-BLOB;VALUE=BINARY:x");
  });
});

describe("rule 11 — DATE values canonicalize as themselves", () => {
  it("emits the eight-octet value verbatim and retains VALUE=DATE", () => {
    const got = text(
      canonicalComponent(todo([prop("DTSTART", "20260515", [{ name: "VALUE", value: "DATE" }])])),
    );
    expect(got).toContain("DTSTART;VALUE=DATE:20260515");
  });

  it("upper-cases the VALUE argument so case variants converge", () => {
    const got = text(
      canonicalComponent(todo([prop("DUE", "20260516", [{ name: "VALUE", value: "date" }])])),
    );
    expect(got).toContain("DUE;VALUE=DATE:20260516");
  });

  it("strips a stray TZID from a DATE", () => {
    const got = text(
      canonicalComponent(
        todo([
          prop("DUE", "20260516", [
            { name: "TZID", value: "America/Montreal" },
            { name: "VALUE", value: "DATE" },
          ]),
        ]),
      ),
    );
    expect(got).toContain("DUE;VALUE=DATE:20260516");
    expect(got).not.toContain("TZID");
  });

  it("never promotes a DATE to a DATE-TIME, even with a resolvable VTIMEZONE", () => {
    const zone: Component = {
      type: "VTIMEZONE",
      props: [prop("TZID", "Fixed/Plus05")],
      sub: [
        {
          // A sub-component type the CompType union does not name; the
          // RFC 5545 parser casts the same way.
          type: "STANDARD" as Component["type"],
          props: [
            prop("DTSTART", "19700101T000000"),
            prop("TZOFFSETFROM", "+0500"),
            prop("TZOFFSETTO", "+0500"),
          ],
          sub: [],
        },
      ],
    };
    const c = todo([
      DTSTAMP,
      prop("UID", "u"),
      prop("DTSTART", "20260515", [
        { name: "TZID", value: "Fixed/Plus05" },
        { name: "VALUE", value: "DATE" },
      ]),
    ]);
    const got = text(componentInContext(c, cal([zone, c])));
    expect(got).toContain("DTSTART;VALUE=DATE:20260515");
    expect(got).not.toContain("T000000Z");
    expect(got).not.toContain("20260514");
  });
});

describe("rule 12 — DURATION preserved verbatim", () => {
  it("does not normalize units", () => {
    expect(text(canonicalComponent(todo([prop("DURATION", "P1D")])))).toContain("DURATION:P1D");
    expect(text(canonicalComponent(todo([prop("DURATION", "PT24H")])))).toContain("DURATION:PT24H");
  });

  it("preserves a relative TRIGGER's sign and spelling", () => {
    const got = text(canonicalComponent(todo([prop("TRIGGER", "-PT15M")])));
    expect(got).toContain("TRIGGER:-PT15M");
  });

  it("distinguishes P0D from PT0S", () => {
    const a = canonicalComponent(todo([prop("DURATION", "P0D")]));
    const b = canonicalComponent(todo([prop("DURATION", "PT0S")]));
    expect(text(a)).not.toBe(text(b));
  });
});

describe("order irrelevance", () => {
  it("hashes semantically identical calendars in different orders identically", () => {
    const a = cal([
      todo([prop("UID", "b"), DTSTAMP, prop("SUMMARY", "two")]),
      todo([prop("SUMMARY", "one"), prop("UID", "a"), DTSTAMP]),
    ]);
    const b = cal([
      todo([DTSTAMP, prop("UID", "a"), prop("SUMMARY", "one")]),
      todo([prop("SUMMARY", "two"), DTSTAMP, prop("UID", "b")]),
    ]);
    expect(hashCalendar(a)).toBe(hashCalendar(b));
  });

  it("hashes identically when only parameter order differs", () => {
    const withParams = (params: Param[]): Calendar =>
      cal([todo([DTSTAMP, prop("UID", "u"), prop("ATTENDEE", "mailto:a@b.c", params)])]);
    const a = withParams([
      { name: "ROLE", value: "CHAIR" },
      { name: "CN", value: "Ada" },
    ]);
    const b = withParams([
      { name: "CN", value: "Ada" },
      { name: "ROLE", value: "CHAIR" },
    ]);
    expect(hashCalendar(a)).toBe(hashCalendar(b));
  });
});

describe("canonicalization does not mutate its input", () => {
  it("leaves the component untouched", () => {
    const c = todo([
      prop("SUMMARY", "s"),
      prop("UID", "u"),
      prop(X_VSTAR_HASH_PROPERTY, "sha256:00"),
    ]);
    const before = JSON.stringify(c);
    canonicalComponent(c);
    expect(JSON.stringify(c)).toBe(before);
  });

  it("leaves the calendar untouched", () => {
    const c = cal([todo([prop("SUMMARY", "s"), prop("UID", "z")]), todo([prop("UID", "a")])]);
    const before = JSON.stringify(c);
    canonicalCalendar(c);
    expect(JSON.stringify(c)).toBe(before);
  });
});
