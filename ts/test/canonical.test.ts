// SPDX-License-Identifier: MIT

// The layer-(b) gate: for every conformance fixture carrying a
// `.canonical` sibling, this port's canonical bytes MUST equal the
// file's, byte for byte; for every `.hash` sibling, the hash string
// MUST equal the file's contents.
//
// The comparison compares BYTES (`Uint8Array`). The single licensed
// transform is stripping `\r` before `\n` in OUR produced bytes — the
// corpus is LF on disk, the canonical form is CRLF (spec rule 1). The
// transform runs on our output, never on the file's, because
// converting the file's LF to CRLF would silently repair a bare `\n`
// our encoder should never have emitted.
//
// The hash is taken over the CRLF bytes, not the LF-transformed ones.

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { calendar as canonicalCalendar, card as canonicalCard } from "../src/canonical/index.js";
import { parse as parseIcs } from "../src/codec/rfc5545/index.js";
import { parse as parseVcf } from "../src/codec/rfc6350/index.js";
import { calendar as hashCalendar, card as hashCard } from "../src/hashing/index.js";
import { CONFORMANCE_DIR, assertBytesEqual, loadFixtures } from "./fixtures.js";

/** Strip `\r` before `\n`, the one licensed comparison transform. */
function crlfToLf(b: Uint8Array): Uint8Array {
  const out = new Uint8Array(b.length);
  let n = 0;
  for (let i = 0; i < b.length; i++) {
    if (b[i] === 0x0d && b[i + 1] === 0x0a) continue;
    out[n++] = b[i] as number;
  }
  return out.subarray(0, n);
}

/** The `.canonical` sibling's raw bytes, or `undefined` when absent. */
function readCanonical(family: string, stem: string): Uint8Array | undefined {
  try {
    return new Uint8Array(readFileSync(join(CONFORMANCE_DIR, family, `${stem}.canonical`)));
  } catch {
    return undefined;
  }
}

/** The `.hash` sibling's contents with its trailing newline trimmed. */
function readHash(family: string, stem: string): string | undefined {
  try {
    return readFileSync(join(CONFORMANCE_DIR, family, `${stem}.hash`), "utf8").trim();
  } catch {
    return undefined;
  }
}

/** Every `(family, ext)` pair in the corpus carrying canonical siblings. */
const CALENDAR_FAMILIES = ["rfc5545", "supersession"] as const;

describe.each(CALENDAR_FAMILIES)("%s canonical bytes and hash", (family) => {
  const fixtures = loadFixtures(family, ".ics");

  it("every fixture has both siblings", () => {
    for (const f of fixtures) {
      expect(readCanonical(family, f.stem), `${f.stem}.canonical`).toBeDefined();
      expect(readHash(family, f.stem), `${f.stem}.hash`).toBeDefined();
    }
  });

  for (const fixture of fixtures) {
    it(`${fixture.stem} canonicalizes to the corpus bytes`, () => {
      const want = readCanonical(family, fixture.stem);
      expect(want).toBeDefined();
      const got = canonicalCalendar(parseIcs(fixture.input));
      assertBytesEqual(crlfToLf(got), want as Uint8Array, `${family}/${fixture.stem}.canonical`);
    });

    it(`${fixture.stem} hashes to the corpus hash`, () => {
      expect(hashCalendar(parseIcs(fixture.input))).toBe(readHash(family, fixture.stem));
    });
  }
});

describe("rfc6350 canonical bytes and hash", () => {
  const fixtures = loadFixtures("rfc6350", ".vcf");

  for (const fixture of fixtures) {
    it(`${fixture.stem} canonicalizes to the corpus bytes`, () => {
      const cards = parseVcf(fixture.input);
      expect(cards).toHaveLength(1);
      const want = readCanonical("rfc6350", fixture.stem);
      expect(want).toBeDefined();
      const got = canonicalCard(cards[0]!);
      assertBytesEqual(crlfToLf(got), want as Uint8Array, `rfc6350/${fixture.stem}.canonical`);
    });

    it(`${fixture.stem} hashes to the corpus hash`, () => {
      const cards = parseVcf(fixture.input);
      expect(hashCard(cards[0]!)).toBe(readHash("rfc6350", fixture.stem));
    });
  }
});

describe("canonical form is CRLF-terminated", () => {
  it("emits no bare LF", () => {
    for (const fixture of loadFixtures("rfc5545", ".ics")) {
      const got = canonicalCalendar(parseIcs(fixture.input));
      for (let i = 0; i < got.length; i++) {
        if (got[i] === 0x0a) {
          expect(got[i - 1], `${fixture.stem}: bare LF at offset ${i}`).toBe(0x0d);
        }
      }
    }
  });
});

describe("hashing is deterministic", () => {
  it("hashes the same input identically 100 times", () => {
    const fixture = loadFixtures("rfc5545", ".ics").find((f) => f.stem === "world");
    expect(fixture).toBeDefined();
    const first = hashCalendar(parseIcs(fixture!.input));
    for (let i = 0; i < 100; i++) {
      expect(hashCalendar(parseIcs(fixture!.input))).toBe(first);
    }
  });
});
