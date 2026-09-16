// SPDX-License-Identifier: MIT

// Cross-implementation parity against the Go reference encoder.
//
// Round-trip tests prove a port is self-consistent; they do not prove it
// agrees with any other implementation. This file pins the encoder's
// output length and an FNV-1a fingerprint of its bytes for every
// conformance fixture, captured from the Go reference at the commit this
// port was written against:
//
//     go/codec/rfc5545.Encode  (Parse -> Encode)
//     go/codec/rfc6350.Encode  (Parse -> Encode per card, concatenated)
//
// A fingerprint mismatch means the two implementations produce different
// bytes for the same input — the exact failure the specification exists
// to prevent — and it surfaces here rather than several layers away as a
// canonical-byte or hash mismatch.
//
// Regenerating: these are reference values, not port values. Do not
// "fix" a failure by re-recording from this port. Re-run the Go encoder
// over the corpus and take its numbers.

import { describe, expect, it } from "vitest";

import { encode as encodeIcs, parse as parseIcs } from "../src/codec/rfc5545/index.js";
import { encode as encodeVcf, parse as parseVcf } from "../src/codec/rfc6350/index.js";
import { loadFixtures } from "./fixtures.js";

/** `stem` → `[byte length, FNV-1a/32 of the encoded bytes]`, from Go. */
const REFERENCE_ICS: Readonly<Record<string, readonly [number, string]>> = {
  all_day_vtodo: [236, "9dd31ea5"],
  all_day_vtodo_variant: [258, "87ce6c23"],
  attach_binary: [281, "5851e5ba"],
  empty: [70, "2f1663f4"],
  escaping: [328, "cb662b54"],
  fold_split_utf8: [262, "6ed7b525"],
  nested_vtimezone: [331, "a21533fe"],
  nfc_decomposed: [234, "48d3487d"],
  one_vtodo: [166, "95720d63"],
  related_reltype: [585, "7a456f7b"],
  sort_utf8_uids: [468, "0da700aa"],
  valarm_absolute_trigger: [328, "26571f9c"],
  vevent_duration_alarm: [342, "b8ad3ec8"],
  vevent_status_class_transp: [283, "9912fbb9"],
  vevent_valarm: [293, "0c2c8733"],
  vfreebusy: [351, "9a5cd9a0"],
  vjournal: [264, "02a2dddb"],
  vjournal_status: [217, "30ed2d0d"],
  vtodo_sequence_percent: [250, "a8720014"],
  world: [538, "efd06e5e"],
};

const REFERENCE_VCF: Readonly<Record<string, readonly [number, string]>> = {
  escaping: [161, "d244b29f"],
  fold_long_note: [482, "35d23e1b"],
  grouped: [168, "2d3957c2"],
  kind_group: [286, "3502fb09"],
  kind_org: [119, "5e2182c1"],
  minimal: [102, "d33d501b"],
  with_extensions: [221, "276195de"],
};

/**
 * The reference length and fingerprint for one fixture, or a clear
 * failure naming it.
 *
 * The suites below enumerate the *corpus*, not this table, so a fixture
 * added upstream becomes a test case immediately. What it must not
 * become is a silently passing one, so an absent entry fails here rather
 * than skipping the only assertion that pins agreement with Go.
 */
function reference(
  table: Readonly<Record<string, readonly [number, string]>>,
  family: string,
  stem: string,
): readonly [number, string] {
  const entry = table[stem];
  if (entry === undefined) {
    throw new Error(
      `${family}/${stem} has no entry in this file's reference table. ` +
        `Run the Go encoder under go/ over the fixture and record ` +
        `[length, fnv1a] of its exact CRLF output; never record this ` +
        `port's own bytes.`,
    );
  }
  return entry;
}

describe("rfc5545 parity with the Go reference encoder", () => {
  const fixtures = loadFixtures("rfc5545", ".ics");

  it("has no reference entry the corpus dropped its fixture for", () => {
    // The per-fixture cases guarantee the other direction. This catches
    // a renamed or deleted fixture leaving a dead entry behind.
    expect(Object.keys(REFERENCE_ICS).sort()).toEqual(fixtures.map((f) => f.stem).sort());
  });

  for (const fixture of fixtures) {
    it(`${fixture.stem} encodes to the reference bytes`, () => {
      const out = encodeIcs(parseIcs(fixture.input));
      expect([out.length, fnv1a(out)]).toEqual(reference(REFERENCE_ICS, "rfc5545", fixture.stem));
    });
  }
});

describe("rfc6350 parity with the Go reference encoder", () => {
  const fixtures = loadFixtures("rfc6350", ".vcf");

  it("has no reference entry the corpus dropped its fixture for", () => {
    expect(Object.keys(REFERENCE_VCF).sort()).toEqual(fixtures.map((f) => f.stem).sort());
  });

  for (const fixture of fixtures) {
    it(`${fixture.stem} encodes to the reference bytes`, () => {
      const cards = parseVcf(fixture.input);
      const out = concat(cards.map((c) => encodeVcf(c)));
      expect([out.length, fnv1a(out)]).toEqual(reference(REFERENCE_VCF, "rfc6350", fixture.stem));
    });
  }
});

/** FNV-1a (32-bit) over raw bytes, as eight lowercase hex digits. */
function fnv1a(bytes: Uint8Array): string {
  let h = 2166136261 >>> 0;
  for (const b of bytes) {
    h = (h ^ b) >>> 0;
    h = Math.imul(h, 16777619) >>> 0;
  }
  return h.toString(16).padStart(8, "0");
}

function concat(chunks: Uint8Array[]): Uint8Array {
  const out = new Uint8Array(chunks.reduce((n, c) => n + c.length, 0));
  let at = 0;
  for (const c of chunks) {
    out.set(c, at);
    at += c.length;
  }
  return out;
}
