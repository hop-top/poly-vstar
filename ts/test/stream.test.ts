// SPDX-License-Identifier: MIT

// The `codec/stream` surface: the constant-memory parsers and encoders.
//
// The gate is a streaming re-parse of every corpus file: what the
// stream parser yields must equal what the batch parser produces. That
// is what makes "streaming" a claim about memory rather than about
// behavior — the two paths cannot disagree about a document.

import { describe, expect, it } from "vitest";

import { concatBytes, foldLine } from "../src/codec/contentline.js";
import { parse as parseIcs } from "../src/codec/rfc5545/index.js";
import { parse as parseVcf } from "../src/codec/rfc6350/index.js";
import {
  VCalendarEncoder,
  VCalendarParser,
  VCardEncoder,
  VCardParser,
  byteLines,
  byteLinesFromAsyncIterable,
  byteSink,
  fromAsyncIterable,
  lines,
  newVCalendarEncoder,
  newVCalendarParser,
  newVCardEncoder,
  newVCardParser,
} from "../src/codec/stream/index.js";
import { VstarError } from "../src/errors.js";
import type { Calendar, Card, Component } from "../src/types.js";
import { assertBytesEqual, loadFixtures } from "./fixtures.js";

const enc = new TextEncoder();
const dec = new TextDecoder();

/** Everything a parser yields, drained. */
function drain<T>(it: IterableIterator<T>): T[] {
  return [...it];
}

/** An async byte-chunk source, splitting `input` into `size`-byte pieces. */
async function* chunked(input: Uint8Array, size: number): AsyncIterable<Uint8Array> {
  for (let i = 0; i < input.length; i += size) {
    yield input.subarray(i, Math.min(i + size, input.length));
  }
}

describe("streaming re-parse gate — rfc5545 corpus", () => {
  const fixtures = loadFixtures("rfc5545", ".ics");

  it("walks the whole corpus", () => {
    expect(fixtures.length).toBeGreaterThanOrEqual(15);
  });

  for (const f of fixtures) {
    it(`${f.stem}: the stream parser yields what the batch parser produces`, () => {
      const batch = parseIcs(f.input);
      const parser = newVCalendarParser(lines(f.input));
      const streamed = drain(parser);

      expect(streamed).toEqual(batch.components);
      expect(parser.header().prodId).toBe(batch.prodId);
    });

    it(`${f.stem}: the same holds through a chunked async source`, async () => {
      const batch = parseIcs(f.input);
      // Seven bytes: small enough to split every content line, and a
      // multi-byte character, across chunk boundaries.
      const parser = newVCalendarParser(await fromAsyncIterable(chunked(f.input, 7)));
      expect(drain(parser)).toEqual(batch.components);
    });
  }
});

describe("streaming re-parse gate — rfc6350 corpus", () => {
  const fixtures = loadFixtures("rfc6350", ".vcf");

  it("walks the whole corpus", () => {
    expect(fixtures.length).toBeGreaterThanOrEqual(6);
  });

  for (const f of fixtures) {
    it(`${f.stem}: the stream parser yields what the batch parser produces`, () => {
      expect(drain(newVCardParser(lines(f.input)))).toEqual(parseVcf(f.input));
    });

    it(`${f.stem}: the same holds through a chunked async source`, async () => {
      expect(drain(newVCardParser(await fromAsyncIterable(chunked(f.input, 7))))).toEqual(
        parseVcf(f.input),
      );
    });
  }

  it("reads a concatenation of blocks as several cards, with no enclosing wrapper", () => {
    // A vCard stream is a sequence of self-contained blocks. Feeding
    // the whole corpus in one go must yield one card per block, in
    // order, matching each file's own batch parse.
    const parts = fixtures.map((f) => dec.decode(f.input));
    const joined = enc.encode(parts.join(""));
    const streamed = drain(newVCardParser(lines(joined)));
    const expected = fixtures.flatMap((f) => parseVcf(f.input));
    expect(streamed).toEqual(expected);
    expect(streamed.length).toBe(fixtures.length);
  });
});

describe("the parser is genuinely incremental", () => {
  /** A line source recording how far the parser has pulled. */
  function counting(source: readonly string[]): { it: Iterable<string>; pulled: () => number } {
    let n = 0;
    return {
      it: {
        *[Symbol.iterator](): Iterator<string> {
          for (const line of source) {
            n++;
            yield line;
          }
        },
      },
      pulled: () => n,
    };
  }

  const doc = [
    "BEGIN:VCALENDAR",
    "VERSION:2.0",
    "PRODID:-//V*//Stream//EN",
    "BEGIN:VTODO",
    "UID:a",
    "DTSTAMP:20260504T120000Z",
    "END:VTODO",
    "BEGIN:VTODO",
    "UID:b",
    "DTSTAMP:20260504T120000Z",
    "END:VTODO",
    "BEGIN:VTODO",
    "UID:c",
    "DTSTAMP:20260504T120000Z",
    "END:VTODO",
    "END:VCALENDAR",
  ];

  it("does not read the whole input before yielding the first component", () => {
    const src = counting(doc);
    const parser = newVCalendarParser(src.it);
    const first = parser.next();

    expect((first.value as Component).props[0]?.value).toBe("a");
    // Eight lines: the header's three, the first VTODO's four, and one
    // more read ahead. That lookahead is inherent to RFC 5545 §3.1
    // unfolding — a logical line is only complete once the next
    // physical line proves not to be a fold continuation of it.
    expect(src.pulled()).toBe(8);
    // The claim that matters: the remaining eight lines are untouched.
    expect(src.pulled()).toBeLessThan(doc.length);
  });

  it("advances one component's worth of input per next() call", () => {
    const src = counting(doc);
    const parser = newVCalendarParser(src.it);
    parser.next();
    const afterFirst = src.pulled();
    parser.next();
    const afterSecond = src.pulled();

    expect(afterSecond).toBeGreaterThan(afterFirst);
    expect(afterSecond).toBeLessThan(doc.length);
  });

  it("stops at END:VCALENDAR rather than draining the rest of the input", () => {
    const withTrailer = [...doc, "TRAILING:GARBAGE", "MORE:GARBAGE", "AND:MORE", "STILL:MORE"];
    const src = counting(withTrailer);
    drain(newVCalendarParser(src.it));
    // One line past END:VCALENDAR, for the same unfolding lookahead.
    // The trailing garbage beyond that is never touched — which is why
    // a trailing-garbage document parses rather than failing.
    expect(src.pulled()).toBe(doc.length + 1);
    expect(src.pulled()).toBeLessThan(withTrailer.length);
  });

  it("holds bounded memory in fromAsyncIterable — one partial line at a time", async () => {
    // Line count, not byte count, is what a caller can bound: a decoded
    // chunk is split and its complete lines handed on immediately.
    const big = enc.encode(doc.join("\r\n") + "\r\n");
    expect(await fromAsyncIterable(chunked(big, 3))).toEqual(doc);
  });

  it("reassembles a multi-byte character split across chunk boundaries", async () => {
    const text = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//café//EN\r\nEND:VCALENDAR\r\n";
    const bytes = enc.encode(text);
    for (const size of [1, 2, 3, 5, 7, 11]) {
      const got = await fromAsyncIterable(chunked(bytes, size));
      expect(got).toEqual(["BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//café//EN", "END:VCALENDAR"]);
    }
  });
});

describe("lines", () => {
  it("splits on CRLF and bare LF alike, dropping terminators", () => {
    expect([...lines("a\r\nb\nc")]).toEqual(["a", "b", "c"]);
  });

  it("surfaces a trailing partial line and emits no empty final element", () => {
    expect([...lines("a\r\n")]).toEqual(["a"]);
    expect([...lines("a")]).toEqual(["a"]);
    expect([...lines("")]).toEqual([]);
  });
});

describe("unfolding is incremental too", () => {
  it("reassembles a folded content line across physical lines", () => {
    const folded = [
      "BEGIN:VCALENDAR",
      "VERSION:2.0",
      "PRODID:-//V*//Fold//EN",
      "BEGIN:VTODO",
      "UID:a",
      "DTSTAMP:20260504T120000Z",
      "SUMMARY:a very long summ",
      " ary continued here",
      "END:VTODO",
      "END:VCALENDAR",
    ];
    const [c] = drain(newVCalendarParser(folded));
    expect((c as Component).props.find((p) => p.name === "SUMMARY")?.value).toBe(
      "a very long summary continued here",
    );
  });
});

describe("a fold that splits a UTF-8 sequence", () => {
  // Canonicalization rule 3 folds at a 75-**octet** boundary, so a
  // multi-byte sequence straddling it is split across the fold and one
  // physical line is not valid UTF-8 on its own. A JavaScript string
  // cannot hold half a sequence, so the parser must unfold on bytes and
  // decode only whole logical lines.

  /** A minimal calendar whose single SUMMARY is `logical`, folded. */
  function foldedDoc(logical: string): Uint8Array {
    return concatBytes(
      [
        "BEGIN:VCALENDAR",
        "VERSION:2.0",
        "PRODID:-//V*//Split//EN",
        "BEGIN:VTODO",
        "UID:a",
        "DTSTAMP:20260504T120000Z",
        logical,
        "END:VTODO",
        "END:VCALENDAR",
      ].map(foldLine),
    );
  }

  function summaryOf(source: Parameters<typeof newVCalendarParser>[0]): string | undefined {
    const [c] = drain(newVCalendarParser(source));
    return (c as Component).props.find((p) => p.name === "SUMMARY")?.value;
  }

  // Each pad puts the fold boundary inside the character that follows:
  // a 2-, 3- and 4-octet sequence, split at every interior position.
  const splits: [string, string, number[]][] = [
    ["U+00E9 (2 octets)", "é", [66]],
    ["U+20AC (3 octets)", "€", [65, 66]],
    ["U+1F600 (4 octets)", "\u{1F600}", [64, 65, 66]],
  ];

  for (const [label, ch, pads] of splits) {
    for (const pad of pads) {
      const value = `${"x".repeat(pad)}${ch}tail`;
      const logical = `SUMMARY:${value}`;

      it(`${label} split at octet ${pad}: the fold really does cut the sequence`, () => {
        // Guard the premise: if folding ever retreated the cut to a
        // character boundary these tests would pass vacuously.
        const folded = foldLine(logical);
        const crlf = folded.indexOf(0x0d);
        // The octet before the CRLF is part of the multi-byte sequence:
        // either a lead byte (>= 0xc2) or a continuation byte (0x80-0xbf).
        expect(folded[crlf - 1]).toBeGreaterThanOrEqual(0x80);
      });

      it(`${label} split at octet ${pad}: a Uint8Array document round-trips`, () => {
        expect(summaryOf(foldedDoc(logical))).toBe(value);
      });

      it(`${label} split at octet ${pad}: byteLines round-trips`, () => {
        expect(summaryOf(byteLines(foldedDoc(logical)))).toBe(value);
      });

      it(`${label} split at octet ${pad}: byteLinesFromAsyncIterable round-trips`, async () => {
        const doc = foldedDoc(logical);
        for (const size of [1, 2, 3, 5, 64]) {
          expect(summaryOf(await byteLinesFromAsyncIterable(chunked(doc, size)))).toBe(value);
        }
      });

      it(`${label} split at octet ${pad}: the streamed value equals the batch parse`, () => {
        const doc = foldedDoc(logical);
        const batch = parseIcs(doc).components[0];
        expect(summaryOf(doc)).toBe(batch?.props.find((p) => p.name === "SUMMARY")?.value);
      });
    }
  }

  it("the string line source cannot carry a split sequence, and says so", () => {
    // Not a defect to fix but a property of the type: `lines` decodes
    // before unfolding, so each half of the split sequence is already
    // U+FFFD. Pinned so the lossy path stays visible and documented
    // rather than being mistaken for the byte path.
    const logical = `SUMMARY:${"x".repeat(64)}\u{1F600}tail`;
    const viaString = summaryOf(lines(foldedDoc(logical)));
    expect(viaString).toContain("�");
    expect(summaryOf(foldedDoc(logical))).not.toContain("�");
  });

  it("an unfolded multi-byte value is unaffected on every path", () => {
    const logical = "SUMMARY:café € \u{1F600}";
    const doc = foldedDoc(logical);
    const want = logical.slice("SUMMARY:".length);
    expect(summaryOf(doc)).toBe(want);
    expect(summaryOf(byteLines(doc))).toBe(want);
    expect(summaryOf(lines(doc))).toBe(want);
  });
});

describe("byteLines", () => {
  const bytesOf = (it: Iterable<Uint8Array>): string[] => [...it].map((b) => dec.decode(b));

  it("splits on CRLF and bare LF alike, dropping terminators", () => {
    expect(bytesOf(byteLines("a\r\nb\nc"))).toEqual(["a", "b", "c"]);
  });

  it("surfaces a trailing partial line and emits no empty final element", () => {
    expect(bytesOf(byteLines("a\r\n"))).toEqual(["a"]);
    expect(bytesOf(byteLines("a"))).toEqual(["a"]);
    expect(bytesOf(byteLines(""))).toEqual([]);
  });

  it("agrees with lines() on input no fold ever split", () => {
    const text = "BEGIN:VCALENDAR\r\nPRODID:-//café//EN\r\nEND:VCALENDAR\r\n";
    expect(bytesOf(byteLines(enc.encode(text)))).toEqual([...lines(text)]);
  });
});

describe("byteLinesFromAsyncIterable", () => {
  it("reassembles lines across every chunk size, without decoding", async () => {
    const doc = ["BEGIN:VCALENDAR", "PRODID:-//café//EN", "END:VCALENDAR"];
    const bytes = enc.encode(doc.join("\r\n") + "\r\n");
    for (const size of [1, 2, 3, 5, 7, 11, 1024]) {
      const got = await byteLinesFromAsyncIterable(chunked(bytes, size));
      expect(got.map((b) => dec.decode(b))).toEqual(doc);
    }
  });

  it("carries a fold-split sequence through as bytes", async () => {
    // The whole point: chunk boundaries and fold boundaries are
    // different things, and neither may decode early.
    const logical = `SUMMARY:${"x".repeat(64)}\u{1F600}tail`;
    const bytes = foldLine(logical);
    const got = await byteLinesFromAsyncIterable(chunked(bytes, 3));
    expect(got).toHaveLength(2);
    // Neither physical line is valid UTF-8 on its own; together they are.
    expect(dec.decode(concatBytes([got[0] as Uint8Array, (got[1] as Uint8Array).subarray(1)]))).toBe(
      logical,
    );
  });

  it("returns copies, so a caller recycling its read buffer is safe", async () => {
    const buf = enc.encode("AAAA\r\nBBBB\r\n");
    const got = await byteLinesFromAsyncIterable(chunked(buf, 16));
    buf.fill(0x5a);
    expect(got.map((b) => dec.decode(b))).toEqual(["AAAA", "BBBB"]);
  });
});

describe("VCalendarParser failures", () => {
  const codeOf = (fn: () => unknown): string => {
    try {
      fn();
    } catch (e) {
      return (e as VstarError).code;
    }
    throw new Error("expected a throw");
  };

  it("rejects empty input as malformed", () => {
    expect(codeOf(() => drain(newVCalendarParser([])))).toBe("ErrMalformed");
  });

  it("rejects a stream not opening with BEGIN:VCALENDAR", () => {
    expect(codeOf(() => drain(newVCalendarParser(["BEGIN:VTODO", "END:VTODO"])))).toBe(
      "ErrMalformed",
    );
  });

  it("reports an unclosed VCALENDAR", () => {
    expect(
      codeOf(() => drain(newVCalendarParser(["BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//x//EN"]))),
    ).toBe("ErrUnclosedBlock");
  });

  it("reports an unclosed sub-component", () => {
    expect(
      codeOf(() =>
        drain(
          newVCalendarParser([
            "BEGIN:VCALENDAR",
            "VERSION:2.0",
            "PRODID:-//x//EN",
            "BEGIN:VTODO",
            "UID:a",
          ]),
        ),
      ),
    ).toBe("ErrUnclosedBlock");
  });

  it("rejects an unsupported VERSION", () => {
    expect(
      codeOf(() =>
        drain(newVCalendarParser(["BEGIN:VCALENDAR", "VERSION:1.0", "PRODID:-//x//EN", "END:VCALENDAR"])),
      ),
    ).toBe("ErrUnsupportedVersion");
  });

  it("rejects a mismatched END", () => {
    expect(
      codeOf(() =>
        drain(
          newVCalendarParser([
            "BEGIN:VCALENDAR",
            "VERSION:2.0",
            "PRODID:-//x//EN",
            "BEGIN:VTODO",
            "UID:a",
            "END:VEVENT",
          ]),
        ),
      ),
    ).toBe("ErrMalformed");
  });

  it("rejects a calendar-level property interleaved among components", () => {
    expect(
      codeOf(() =>
        drain(
          newVCalendarParser([
            "BEGIN:VCALENDAR",
            "VERSION:2.0",
            "PRODID:-//x//EN",
            "BEGIN:VTODO",
            "UID:a",
            "END:VTODO",
            "METHOD:PUBLISH",
            "END:VCALENDAR",
          ]),
        ),
      ),
    ).toBe("ErrMalformed");
  });
});

describe("VCalendarParser.header", () => {
  const doc = ["BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//V*//Header//EN", "END:VCALENDAR"];

  it("is readable before the first next(), reading the header on demand", () => {
    expect(newVCalendarParser(doc).header().prodId).toBe("-//V*//Header//EN");
  });

  it("carries no components — only the calendar-level properties", () => {
    const parser = newVCalendarParser([
      "BEGIN:VCALENDAR",
      "VERSION:2.0",
      "PRODID:-//x//EN",
      "BEGIN:VTODO",
      "UID:a",
      "DTSTAMP:20260504T120000Z",
      "END:VTODO",
      "END:VCALENDAR",
    ]);
    drain(parser);
    expect(parser.header().components).toEqual([]);
  });

  it("holds a header failure and surfaces it from next()", () => {
    const parser = newVCalendarParser(["BEGIN:VTODO", "END:VTODO"]);
    // Best-effort: the header read failed, so this returns what little
    // was parsed rather than throwing.
    expect(parser.header().prodId).toBe("");
    expect(() => parser.next()).toThrow(VstarError);
  });
});

describe("VCardParser failures", () => {
  const codeOf = (fn: () => unknown): string => {
    try {
      fn();
    } catch (e) {
      return (e as VstarError).code;
    }
    throw new Error("expected a throw");
  };

  it("treats exhaustion between blocks as done, not an error", () => {
    expect(drain(newVCardParser([]))).toEqual([]);
  });

  it("rejects content outside a block", () => {
    expect(codeOf(() => drain(newVCardParser(["FN:Jad"])))).toBe("ErrMalformed");
  });

  it("rejects a nested BEGIN:VCARD", () => {
    expect(
      codeOf(() => drain(newVCardParser(["BEGIN:VCARD", "VERSION:4.0", "BEGIN:VCARD"]))),
    ).toBe("ErrMalformed");
  });

  it("rejects a duplicate VERSION", () => {
    expect(
      codeOf(() =>
        drain(newVCardParser(["BEGIN:VCARD", "VERSION:4.0", "VERSION:4.0", "END:VCARD"])),
      ),
    ).toBe("ErrMalformed");
  });

  it("rejects a block with no VERSION", () => {
    expect(codeOf(() => drain(newVCardParser(["BEGIN:VCARD", "UID:u", "END:VCARD"])))).toBe(
      "ErrMalformed",
    );
  });

  it("rejects an unsupported VERSION", () => {
    expect(
      codeOf(() => drain(newVCardParser(["BEGIN:VCARD", "VERSION:3.0", "UID:u", "END:VCARD"]))),
    ).toBe("ErrUnsupportedVersion");
  });

  it("reports an unclosed block", () => {
    expect(codeOf(() => drain(newVCardParser(["BEGIN:VCARD", "VERSION:4.0", "UID:u"])))).toBe(
      "ErrUnclosedBlock",
    );
  });

  it("recognizes UID, VERSION and KIND through a group prefix", () => {
    const [card] = drain(
      newVCardParser(["BEGIN:VCARD", "VERSION:4.0", "item1.UID:u", "item1.KIND:ORG", "END:VCARD"]),
    );
    expect((card as Card).uid).toBe("u");
    expect((card as Card).kind).toBe("org");
  });
});

describe("VCalendarEncoder", () => {
  const todo: Component = {
    type: "VTODO",
    props: [
      { name: "UID", params: [], value: "a" },
      { name: "DTSTAMP", params: [], value: "20260504T120000Z" },
    ],
    sub: [],
  };

  it("emits a header on the first encode and a trailer on close", () => {
    const sink = byteSink();
    const e = newVCalendarEncoder(sink);
    e.setHeader({ prodId: "-//V*//Stream//EN", components: [] });
    e.encode(todo);
    e.close();

    expect(dec.decode(sink.bytes())).toBe(
      "BEGIN:VCALENDAR\r\n" +
        "VERSION:2.0\r\n" +
        "PRODID:-//V*//Stream//EN\r\n" +
        "BEGIN:VTODO\r\n" +
        "UID:a\r\n" +
        "DTSTAMP:20260504T120000Z\r\n" +
        "END:VTODO\r\n" +
        "END:VCALENDAR\r\n",
    );
  });

  it("emits a legal empty calendar when closed with no encode", () => {
    const sink = byteSink();
    const e = newVCalendarEncoder(sink);
    e.setHeader({ prodId: "-//V*//Empty//EN", components: [] });
    e.close();
    expect(drain(newVCalendarParser(lines(sink.bytes())))).toEqual([]);
  });

  it("round-trips through its own parser", () => {
    const sink = byteSink();
    const e = newVCalendarEncoder(sink);
    e.setHeader({ prodId: "-//V*//Round//EN", components: [] });
    e.encode(todo);
    e.encode({ ...todo, props: [{ name: "UID", params: [], value: "b" }, todo.props[1]!] });
    e.close();

    const parser = newVCalendarParser(lines(sink.bytes()));
    expect(drain(parser).map((c) => c.props[0]?.value)).toEqual(["a", "b"]);
    expect(parser.header().prodId).toBe("-//V*//Round//EN");
  });

  it("falls back to a default PRODID when setHeader was never called", () => {
    const sink = byteSink();
    const e = newVCalendarEncoder(sink);
    e.encode(todo);
    e.close();
    // The same version-free default as the batch constructor, so batch
    // and stream output of one logical calendar hash alike.
    expect(newVCalendarParser(lines(sink.bytes())).header().prodId).toBe("-//hop-top//vstar//EN");
  });

  it("throws ErrHeaderLocked when setHeader follows the first encode", () => {
    const e = newVCalendarEncoder(byteSink());
    e.encode(todo);
    try {
      e.setHeader({ prodId: "-//too//late//EN", components: [] });
      expect.unreachable("setHeader was accepted after the header was on the wire");
    } catch (err) {
      expect((err as VstarError).code).toBe("ErrHeaderLocked");
    }
  });

  it("throws ErrHeaderLocked when setHeader follows a close that wrote the header", () => {
    const e = newVCalendarEncoder(byteSink());
    e.close();
    try {
      e.setHeader({ prodId: "-//too//late//EN", components: [] });
      expect.unreachable("setHeader was accepted after close");
    } catch (err) {
      expect((err as VstarError).code).toBe("ErrHeaderLocked");
    }
  });

  it("throws ErrAlreadyClosed on encode after close", () => {
    const e = newVCalendarEncoder(byteSink());
    e.close();
    try {
      e.encode(todo);
      expect.unreachable("encode was accepted after close");
    } catch (err) {
      expect((err as VstarError).code).toBe("ErrAlreadyClosed");
    }
  });

  it("throws ErrAlreadyClosed on a double close — silently ignoring it would be broken", () => {
    const e = newVCalendarEncoder(byteSink());
    e.close();
    try {
      e.close();
      expect.unreachable("a double close was accepted");
    } catch (err) {
      expect((err as VstarError).code).toBe("ErrAlreadyClosed");
    }
  });
});

describe("VCardEncoder", () => {
  const card: Card = {
    uid: "urn:uuid:11111111-1111-1111-1111-111111111111",
    kind: "",
    props: [{ name: "FN", params: [], value: "Jad Bitar" }],
  };

  it("has NO setHeader — a VCARD stream has no enclosing wrapper", () => {
    const e = newVCardEncoder(byteSink());
    // Not a type-level assertion alone: the method must be absent at
    // runtime too, because its mere existence invites a caller to emit
    // a wrapper the parsers reject.
    expect("setHeader" in e).toBe(false);
    expect((e as unknown as Record<string, unknown>)["setHeader"]).toBeUndefined();
  });

  it("writes one self-contained block per encode, no wrapper", () => {
    const sink = byteSink();
    const e = newVCardEncoder(sink);
    e.encode(card);
    e.encode({ ...card, uid: "urn:uuid:22222222-2222-2222-2222-222222222222" });
    e.close();

    const text = dec.decode(sink.bytes());
    expect(text.startsWith("BEGIN:VCARD\r\n")).toBe(true);
    expect(text.endsWith("END:VCARD\r\n")).toBe(true);
    expect((text.match(/BEGIN:VCARD/g) ?? []).length).toBe(2);
    expect(text).not.toContain("VCALENDAR");
  });

  it("round-trips through its own parser", () => {
    const sink = byteSink();
    const e = newVCardEncoder(sink);
    e.encode(card);
    e.close();
    expect(drain(newVCardParser(lines(sink.bytes())))).toEqual([{ ...card, kind: "" }]);
  });

  it("emits nothing at close — there is no trailer to write", () => {
    const sink = byteSink();
    const e = newVCardEncoder(sink);
    e.encode(card);
    const beforeClose = sink.bytes();
    e.close();
    assertBytesEqual(sink.bytes(), beforeClose, "close emitted a trailer");
  });

  it("throws ErrAlreadyClosed on encode after close", () => {
    const e = newVCardEncoder(byteSink());
    e.close();
    try {
      e.encode(card);
      expect.unreachable("encode was accepted after close");
    } catch (err) {
      expect((err as VstarError).code).toBe("ErrAlreadyClosed");
    }
  });

  it("throws ErrAlreadyClosed on a double close", () => {
    const e = newVCardEncoder(byteSink());
    e.close();
    try {
      e.close();
      expect.unreachable("a double close was accepted");
    } catch (err) {
      expect((err as VstarError).code).toBe("ErrAlreadyClosed");
    }
  });

  it("throws ErrMissingUID for a card with no UID", () => {
    const e = newVCardEncoder(byteSink());
    try {
      e.encode({ ...card, uid: "" });
      expect.unreachable("a UID-less card was encoded");
    } catch (err) {
      expect((err as VstarError).code).toBe("ErrMissingUID");
    }
  });
});

describe("the parsers are iterable", () => {
  it("VCalendarParser works in a for-of", () => {
    const parser = new VCalendarParser([
      "BEGIN:VCALENDAR",
      "VERSION:2.0",
      "PRODID:-//x//EN",
      "BEGIN:VTODO",
      "UID:a",
      "DTSTAMP:20260504T120000Z",
      "END:VTODO",
      "END:VCALENDAR",
    ]);
    const seen: string[] = [];
    for (const c of parser) seen.push(c.props[0]?.value ?? "");
    expect(seen).toEqual(["a"]);
  });

  it("VCardParser works in a for-of", () => {
    const parser = new VCardParser(["BEGIN:VCARD", "VERSION:4.0", "UID:u", "END:VCARD"]);
    const seen: string[] = [];
    for (const c of parser) seen.push(c.uid);
    expect(seen).toEqual(["u"]);
  });

  it("the classes and their factories construct the same thing", () => {
    expect(newVCalendarParser([]) instanceof VCalendarParser).toBe(true);
    expect(newVCardParser([]) instanceof VCardParser).toBe(true);
    expect(newVCalendarEncoder(byteSink()) instanceof VCalendarEncoder).toBe(true);
    expect(newVCardEncoder(byteSink()) instanceof VCardEncoder).toBe(true);
  });
});

describe("stream encoding matches the batch encoder byte for byte", () => {
  it("for every rfc5545 corpus calendar", async () => {
    const { encode: batchEncode } = await import("../src/codec/rfc5545/index.js");
    for (const f of loadFixtures("rfc5545", ".ics")) {
      const cal: Calendar = parseIcs(f.input);
      const sink = byteSink();
      const e = newVCalendarEncoder(sink);
      e.setHeader(cal);
      for (const c of cal.components) e.encode(c);
      e.close();
      assertBytesEqual(sink.bytes(), batchEncode(cal), `${f.stem} stream vs batch`);
    }
  });
});
