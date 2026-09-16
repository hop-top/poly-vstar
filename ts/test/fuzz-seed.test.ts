// SPDX-License-Identifier: MIT

// The fuzz-seed corpus is the robustness gate: every seed either parses
// or throws a `VstarError`. Nothing else is acceptable — a `TypeError`
// or a `RangeError` escaping the codec is an unhandled edge case, not a
// documented failure class.

import { describe, expect, it } from "vitest";

import { VstarError } from "../src/index.js";
import { encode as encodeIcs, parse as parseIcs } from "../src/codec/rfc5545/index.js";
import { encode as encodeVcf, parse as parseVcf } from "../src/codec/rfc6350/index.js";
import { loadFuzzSeeds } from "./fixtures.js";

const icsSeeds = loadFuzzSeeds("rfc5545");
const vcfSeeds = loadFuzzSeeds("rfc6350");

describe("fuzz-seed rfc5545", () => {
  it("finds the seeds", () => {
    expect(icsSeeds.length).toBeGreaterThan(0);
  });

  for (const seed of icsSeeds) {
    it(`${seed.stem} never throws anything but VstarError`, () => {
      assertOnlyVstarError(() => {
        // Re-encoding the parse result exercises the encoder on every
        // seed that parses, so an encoder crash surfaces here too.
        encodeIcs(parseIcs(seed.input));
      }, seed.stem);
    });
  }
});

describe("fuzz-seed rfc6350", () => {
  it("finds the seeds", () => {
    expect(vcfSeeds.length).toBeGreaterThan(0);
  });

  for (const seed of vcfSeeds) {
    it(`${seed.stem} never throws anything but VstarError`, () => {
      assertOnlyVstarError(() => {
        for (const card of parseVcf(seed.input)) encodeVcf(card);
      }, seed.stem);
    });
  }
});

function assertOnlyVstarError(fn: () => void, label: string): void {
  try {
    fn();
  } catch (e) {
    if (e instanceof VstarError) {
      // A documented failure class, carrying a recoverable identifier.
      expect(typeof e.code).toBe("string");
      return;
    }
    const kind = e instanceof Error ? `${e.name}: ${e.message}` : String(e);
    throw new Error(`${label}: expected VstarError, got ${kind}`);
  }
}
