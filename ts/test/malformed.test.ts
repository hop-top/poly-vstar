// SPDX-License-Identifier: MIT

// Every fixture under `spec/v1.0/conformance/malformed/` MUST fail, and
// MUST fail with the sentinel its `.error` sibling names. Failing is not
// enough — failing correctly is the gate.

import { describe, expect, it } from "vitest";

import type { Card } from "../src/index.js";
import { VstarError } from "../src/index.js";
import { parse as parseIcs } from "../src/codec/rfc5545/index.js";
import { encode as encodeVcf, parse as parseVcf } from "../src/codec/rfc6350/index.js";
import { loadFixtures } from "./fixtures.js";

const icsFixtures = loadFixtures("malformed", ".ics");
const vcfFixtures = loadFixtures("malformed", ".vcf");

describe("malformed rfc5545", () => {
  it("finds the .ics fixtures", () => {
    expect(icsFixtures.length).toBeGreaterThan(0);
  });

  for (const fixture of icsFixtures) {
    it(`${fixture.stem} fails with ${fixture.sentinel}`, () => {
      expect(fixture.sentinel, `${fixture.stem} has no .error sibling`).toBeDefined();
      const err = capture(() => parseIcs(fixture.input));
      expect(err, `${fixture.stem}: parse succeeded`).toBeInstanceOf(VstarError);
      expect((err as VstarError).code).toBe(fixture.sentinel);
    });
  }
});

describe("malformed rfc6350", () => {
  it("finds the .vcf fixtures", () => {
    expect(vcfFixtures.length).toBeGreaterThan(0);
  });

  // ErrMissingUID is encoder-only at v1.0: the parser accepts a
  // UID-less VCARD, so the fixture is exercised by parsing, then
  // re-encoding the parsed Card. A port that only checks the parse
  // side passes this fixture silently and is wrong.
  for (const fixture of vcfFixtures) {
    it(`${fixture.stem} fails with ${fixture.sentinel} on parse or encode`, () => {
      expect(fixture.sentinel, `${fixture.stem} has no .error sibling`).toBeDefined();

      let cards: Card[];
      const parseErr = capture(() => {
        cards = parseVcf(fixture.input);
      });
      if (parseErr !== undefined) {
        expect(parseErr).toBeInstanceOf(VstarError);
        expect((parseErr as VstarError).code).toBe(fixture.sentinel);
        return;
      }

      // Parse succeeded — the fixture targets an encoder-time sentinel.
      expect(cards!.length, `${fixture.stem}: no cards to re-encode`).toBeGreaterThan(0);
      const encodeErr = capture(() => {
        for (const card of cards) encodeVcf(card);
      });
      expect(
        encodeErr,
        `${fixture.stem}: expected ${fixture.sentinel} from parse or encode; both succeeded`,
      ).toBeInstanceOf(VstarError);
      expect((encodeErr as VstarError).code).toBe(fixture.sentinel);
    });
  }
});

describe("malformed fixtures cover the parse-time sentinels", () => {
  it("names at least ErrMalformed, ErrUnclosedBlock, ErrUnsupportedVersion, ErrMissingUID", () => {
    const seen = new Set([...icsFixtures, ...vcfFixtures].map((f) => f.sentinel));
    expect(seen).toContain("ErrMalformed");
    expect(seen).toContain("ErrUnclosedBlock");
    expect(seen).toContain("ErrUnsupportedVersion");
    expect(seen).toContain("ErrMissingUID");
  });
});

/** Run `fn` and return whatever it threw, or `undefined` if it did not. */
function capture(fn: () => void): unknown {
  try {
    fn();
    return undefined;
  } catch (e) {
    return e;
  }
}
