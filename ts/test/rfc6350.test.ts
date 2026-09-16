// SPDX-License-Identifier: MIT

import { describe, expect, it } from "vitest";

import type { Card, Property } from "../src/index.js";
import { propertyEqual, VstarError } from "../src/index.js";
import { encode, parse } from "../src/codec/rfc6350/index.js";
import { assertBytesEqual, loadFixtures } from "./fixtures.js";

const fixtures = loadFixtures("rfc6350", ".vcf");

describe("rfc6350 corpus", () => {
  it("finds the conformance fixtures", () => {
    expect(fixtures.length).toBeGreaterThan(0);
  });

  for (const fixture of fixtures) {
    describe(fixture.stem, () => {
      it("parse returns a LIST of cards", () => {
        const cards = parse(fixture.input);
        expect(Array.isArray(cards)).toBe(true);
        expect(cards.length).toBeGreaterThan(0);
      });

      it("round-trips: parse -> encode -> parse is semantically equal", () => {
        const first = parse(fixture.input);
        const reencoded = concat(first.map((c) => encode(c)));
        const second = parse(reencoded);
        expect(second.length).toBe(first.length);
        for (let i = 0; i < first.length; i++) {
          expectCardsEqual(second[i] as Card, first[i] as Card);
        }
      });

      it("is byte-stable on re-encode and emits CRLF", () => {
        const once = concat(parse(fixture.input).map((c) => encode(c)));
        const twice = concat(parse(once).map((c) => encode(c)));
        assertBytesEqual(twice, once, fixture.stem);
        for (let i = 0; i < once.length; i++) {
          if (once[i] === 0x0a) {
            expect(once[i - 1], `bare LF at offset ${i} in ${fixture.stem}`).toBe(0x0d);
          }
        }
      });

      it("folds every physical line at 75 octets", () => {
        for (const line of physicalLines(concat(parse(fixture.input).map((c) => encode(c))))) {
          expect(line.length, `over-long physical line in ${fixture.stem}`).toBeLessThanOrEqual(75);
        }
      });
    });
  }
});

describe("rfc6350 multi-card streams", () => {
  const twoCards =
    "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:a\r\nFN:A\r\nEND:VCARD\r\n" +
    "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:b\r\nFN:B\r\nEND:VCARD\r\n";

  it("parses each BEGIN/END block into its own Card", () => {
    const cards = parse(twoCards);
    expect(cards.length).toBe(2);
    expect(cards.map((c) => c.uid)).toEqual(["a", "b"]);
  });

  it("encodes exactly one card per call", () => {
    const cards = parse(twoCards);
    const text = new TextDecoder().decode(encode(cards[0] as Card));
    expect(text.match(/BEGIN:VCARD/g)?.length).toBe(1);
  });
});

describe("rfc6350 UID handling is asymmetric", () => {
  const uidless = "BEGIN:VCARD\r\nVERSION:4.0\r\nFN:No UID Here\r\nEND:VCARD\r\n";

  it("the PARSER accepts a UID-less VCARD", () => {
    const cards = parse(uidless);
    expect(cards.length).toBe(1);
    expect((cards[0] as Card).uid).toBe("");
  });

  it("the ENCODER refuses a UID-less Card with ErrMissingUID", () => {
    const card = parse(uidless)[0] as Card;
    expect(() => encode(card)).toThrowError(VstarError);
    try {
      encode(card);
      expect.unreachable("encode should have thrown");
    } catch (e) {
      expect(e).toBeInstanceOf(VstarError);
      expect((e as VstarError).code).toBe("ErrMissingUID");
    }
  });
});

describe("rfc6350 group prefixes", () => {
  it("keeps the group prefix and its case on the property name", () => {
    const cards = parse("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:g\r\nhome.TEL:+1\r\nEND:VCARD\r\n");
    expect((cards[0] as Card).props[0]?.name).toBe("home.TEL");
  });
});

function concat(chunks: Uint8Array[]): Uint8Array {
  const total = chunks.reduce((n, c) => n + c.length, 0);
  const out = new Uint8Array(total);
  let at = 0;
  for (const c of chunks) {
    out.set(c, at);
    at += c.length;
  }
  return out;
}

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

function expectCardsEqual(got: Card, want: Card): void {
  expect(got.uid).toBe(want.uid);
  expect(got.kind).toBe(want.kind);
  expect(got.props.length).toBe(want.props.length);
  for (let i = 0; i < want.props.length; i++) {
    expect(
      propertyEqual(got.props[i] as Property, want.props[i] as Property),
      `property ${i}: ${JSON.stringify(got.props[i])} != ${JSON.stringify(want.props[i])}`,
    ).toBe(true);
  }
}
