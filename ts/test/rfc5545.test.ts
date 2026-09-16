// SPDX-License-Identifier: MIT

import { describe, expect, it } from "vitest";

import type { Calendar, Component, Property } from "../src/index.js";
import { propertyEqual } from "../src/index.js";
import { encode, encodeComponent, parse, parseContentLine } from "../src/codec/rfc5545/index.js";
import { assertBytesEqual, loadFixtures } from "./fixtures.js";

const fixtures = loadFixtures("rfc5545", ".ics");

describe("rfc5545 corpus", () => {
  it("finds the conformance fixtures", () => {
    expect(fixtures.length).toBeGreaterThan(0);
  });

  for (const fixture of fixtures) {
    describe(fixture.stem, () => {
      it("round-trips: parse -> encode -> parse is semantically equal", () => {
        const first = parse(fixture.input);
        const encoded = encode(first);
        const second = parse(encoded);
        expectCalendarsEqual(second, first);
      });

      it("encodes to CRLF-terminated bytes and is byte-stable on re-encode", () => {
        const once = encode(parse(fixture.input));
        const twice = encode(parse(once));
        assertBytesEqual(twice, once, fixture.stem);
        // Every physical line terminator is CRLF: no bare LF anywhere.
        for (let i = 0; i < once.length; i++) {
          if (once[i] === 0x0a) {
            expect(once[i - 1], `bare LF at offset ${i} in ${fixture.stem}`).toBe(0x0d);
          }
        }
      });

      it("folds every physical line at 75 octets", () => {
        for (const line of physicalLines(encode(parse(fixture.input)))) {
          expect(line.length, `over-long physical line in ${fixture.stem}`).toBeLessThanOrEqual(75);
        }
      });

      it("preserves property and component order", () => {
        const cal = parse(fixture.input);
        const wire = logicalLines(fixture.input);
        expect(componentOrder(cal)).toEqual(wireComponentOrder(wire));
      });
    });
  }
});

describe("rfc5545 parseContentLine", () => {
  it("parses a bare name and value", () => {
    const p = parseContentLine("SUMMARY:hello");
    expect(p.name).toBe("SUMMARY");
    expect(p.value).toBe("hello");
    expect(p.params).toEqual([]);
  });

  it("parses parameters in wire order", () => {
    const p = parseContentLine("ATTENDEE;ROLE=CHAIR;CN=Jad:mailto:jad@example.com");
    expect(p.params.map((x) => x.name)).toEqual(["ROLE", "CN"]);
    expect(p.value).toBe("mailto:jad@example.com");
  });

  it("keeps colons inside a quoted parameter value out of the value split", () => {
    const p = parseContentLine('X-THING;ALT="a:b;c":payload');
    expect(p.params).toEqual([{ name: "ALT", value: "a:b;c" }]);
    expect(p.value).toBe("payload");
  });
});

describe("rfc5545 encodeComponent", () => {
  it("emits BEGIN/END with no VCALENDAR wrapper", () => {
    const comp: Component = {
      type: "VTODO",
      props: [{ name: "UID", params: [], value: "x" }],
      sub: [],
    };
    const text = new TextDecoder().decode(encodeComponent(comp));
    expect(text).toBe("BEGIN:VTODO\r\nUID:x\r\nEND:VTODO\r\n");
  });
});

describe("rfc5545 folding is measured in UTF-8 octets", () => {
  it("folds a multi-byte value on byte count, not UTF-16 code units", () => {
    // 60 'é' characters: 60 UTF-16 code units, 120 UTF-8 bytes. With
    // "SUMMARY:" the logical line is 128 octets and MUST fold.
    const cal: Calendar = {
      prodId: "-//test//EN",
      components: [
        {
          type: "VTODO",
          props: [{ name: "SUMMARY", params: [], value: "é".repeat(60) }],
          sub: [],
        },
      ],
    };
    for (const line of physicalLines(encode(cal))) {
      expect(line.length).toBeLessThanOrEqual(75);
    }
  });
});

/** Split encoded bytes on CRLF, returning the physical lines' bytes. */
function physicalLines(bytes: Uint8Array): Uint8Array[] {
  const out: Uint8Array[] = [];
  let start = 0;
  for (let i = 0; i + 1 < bytes.length; i++) {
    if (bytes[i] === 0x0d && bytes[i + 1] === 0x0a) {
      out.push(bytes.subarray(start, i));
      start = i + 2;
      i++;
    }
  }
  if (start < bytes.length) out.push(bytes.subarray(start));
  return out;
}

/** Unfolded logical lines of an LF- or CRLF-terminated input. */
function logicalLines(bytes: Uint8Array): string[] {
  const text = new TextDecoder().decode(bytes);
  const out: string[] = [];
  for (const raw of text.split(/\r?\n/)) {
    if (raw === "") continue;
    if ((raw.startsWith(" ") || raw.startsWith("\t")) && out.length > 0) {
      out[out.length - 1] += raw.slice(1);
      continue;
    }
    out.push(raw);
  }
  return out;
}

/** Flattened `TYPE:prop,prop,…` list, depth-first, in tree order. */
function componentOrder(cal: Calendar): string[] {
  const out: string[] = [];
  const walk = (comps: readonly Component[]): void => {
    for (const c of comps) {
      out.push(`${c.type}:${c.props.map((p) => p.name.toUpperCase()).join(",")}`);
      walk(c.sub);
    }
  };
  walk(cal.components);
  return out;
}

/**
 * The same list, read straight off the wire lines. Slots are reserved
 * at BEGIN and filled at END so the result is pre-order, matching
 * `componentOrder`'s depth-first walk of the parsed model.
 */
function wireComponentOrder(lines: string[]): string[] {
  const out: string[] = [];
  const stack: { index: number; type: string; props: string[] }[] = [];
  for (const line of lines) {
    const colon = line.indexOf(":");
    const head = line.slice(0, colon);
    const value = line.slice(colon + 1);
    const name = head.split(";")[0]?.toUpperCase() ?? "";
    if (name === "BEGIN") {
      const type = value.toUpperCase();
      if (type === "VCALENDAR") continue;
      out.push("");
      stack.push({ index: out.length - 1, type, props: [] });
      continue;
    }
    if (name === "END") {
      const done = stack.pop();
      if (done) out[done.index] = `${done.type}:${done.props.join(",")}`;
      continue;
    }
    stack[stack.length - 1]?.props.push(name);
  }
  return out;
}

function expectCalendarsEqual(got: Calendar, want: Calendar): void {
  expect(got.prodId).toBe(want.prodId);
  expectComponentsEqual(got.components, want.components);
}

function expectComponentsEqual(got: readonly Component[], want: readonly Component[]): void {
  expect(got.length).toBe(want.length);
  for (let i = 0; i < want.length; i++) {
    const a = got[i] as Component;
    const b = want[i] as Component;
    expect(a.type).toBe(b.type);
    expectPropsEqual(a.props, b.props);
    expectComponentsEqual(a.sub, b.sub);
  }
}

function expectPropsEqual(got: readonly Property[], want: readonly Property[]): void {
  expect(got.length).toBe(want.length);
  for (let i = 0; i < want.length; i++) {
    expect(
      propertyEqual(got[i] as Property, want[i] as Property),
      `property ${i}: ${JSON.stringify(got[i])} != ${JSON.stringify(want[i])}`,
    ).toBe(true);
  }
}
