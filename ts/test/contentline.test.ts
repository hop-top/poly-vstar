// SPDX-License-Identifier: MIT

import { describe, expect, it } from "vitest";

import { foldLine, newScanner, scanAll } from "../src/codec/contentline.js";

const dec = new TextDecoder();

describe("content-line scanner unfolds liberally", () => {
  it("accepts CRLF terminators", () => {
    expect(scanAll("A:1\r\nB:2\r\n")).toEqual(["A:1", "B:2"]);
  });

  it("accepts bare LF terminators", () => {
    expect(scanAll("A:1\nB:2\n")).toEqual(["A:1", "B:2"]);
  });

  it("accepts a mixture of both", () => {
    expect(scanAll("A:1\r\nB:2\nC:3\r\n")).toEqual(["A:1", "B:2", "C:3"]);
  });

  it("surfaces a trailing partial line with no terminator", () => {
    expect(scanAll("A:1\r\nB:2")).toEqual(["A:1", "B:2"]);
  });

  // RFC 5545 §3.1: the fold sequence is CRLF + (SP | HTAB), and BOTH
  // the terminator and the WSP octet are part of it. Neither survives
  // into the logical line — a continuation that begins with a space
  // does not reintroduce one.
  it("joins a SP-prefixed continuation, consuming the fold octet", () => {
    expect(scanAll("SUMMARY:hello\r\n world\r\n")).toEqual(["SUMMARY:helloworld"]);
  });

  it("joins an HTAB-prefixed continuation", () => {
    expect(scanAll("SUMMARY:hello\r\n\tworld\r\n")).toEqual(["SUMMARY:helloworld"]);
  });

  it("round-trips a fold inserted mid-word by the encoder", () => {
    const folded = "X:" + "A".repeat(73) + "\r\n " + "B".repeat(10) + "\r\n";
    expect(scanAll(folded)).toEqual(["X:" + "A".repeat(73) + "B".repeat(10)]);
  });

  it("skips blank physical lines outside a fold sequence", () => {
    expect(scanAll("A:1\r\n\r\nB:2\r\n")).toEqual(["A:1", "B:2"]);
  });

  it("starts a fresh logical line when a WSP line follows a blank", () => {
    expect(scanAll("A:1\r\n\r\n more\r\n")).toEqual(["A:1", "more"]);
  });

  it("returns nothing for empty input", () => {
    expect(scanAll("")).toEqual([]);
  });

  it("accepts Uint8Array input and decodes UTF-8", () => {
    const bytes = new TextEncoder().encode("SUMMARY:café\r\n");
    const s = newScanner(bytes);
    expect(s.next()).toBe("SUMMARY:café");
    expect(s.next()).toBeUndefined();
  });

  // Folding counts octets (spec rule 3), so a two-octet sequence sitting
  // on the boundary is split across the fold and neither physical line
  // is valid UTF-8 alone. The scanner must unfold on BYTES and decode
  // once the logical line is whole. Decoding the physical lines first
  // substitutes U+FFFD for each half, and that loss is not recoverable
  // by rejoining afterwards — which is exactly the defect this pins.
  it("rejoins a UTF-8 sequence the folder split across the boundary", () => {
    // "é" is C3 A9; C3 ends the first physical line, A9 opens the next.
    const folded = new Uint8Array([
      ...new TextEncoder().encode("SUMMARY:"),
      0xc3,
      0x0d,
      0x0a,
      0x20,
      0xa9,
      0x0d,
      0x0a,
    ]);
    expect(scanAll(folded)).toEqual(["SUMMARY:é"]);
  });

  it("rejoins a sequence split with three continuation octets to come", () => {
    // U+1F600 is F0 9F 98 80 — split after its first octet.
    const folded = new Uint8Array([
      ...new TextEncoder().encode("X:"),
      0xf0,
      0x0d,
      0x0a,
      0x20,
      0x9f,
      0x98,
      0x80,
      0x0d,
      0x0a,
    ]);
    expect(scanAll(folded)).toEqual(["X:\u{1F600}"]);
  });
});

describe("folding is 75 OCTETS, measured in UTF-8 bytes", () => {
  it("leaves a 75-octet line unfolded", () => {
    const line = "A".repeat(75);
    expect(dec.decode(foldLine(line))).toBe(line + "\r\n");
  });

  it("folds at the 76th octet", () => {
    const line = "A".repeat(76);
    expect(dec.decode(foldLine(line))).toBe("A".repeat(75) + "\r\n A\r\n");
  });

  it("counts UTF-8 bytes, not UTF-16 code units", () => {
    // 40 'é': 40 UTF-16 code units (so String#length says 40, under
    // the limit) but 80 UTF-8 bytes, so it MUST fold.
    const line = "é".repeat(40);
    expect(line.length).toBeLessThan(75);
    const out = foldLine(line);
    expect(out.length).toBeGreaterThan(80 + 2);
    for (const physical of split(out)) {
      expect(physical.length).toBeLessThanOrEqual(75);
    }
  });

  it("gives continuation lines 74 octets of payload after the leading SP", () => {
    const out = split(foldLine("A".repeat(75 + 74 + 1)));
    expect(out.map((l) => l.length)).toEqual([75, 75, 2]);
  });

  it("always terminates with CRLF", () => {
    const out = foldLine("A".repeat(200));
    expect(out[out.length - 2]).toBe(0x0d);
    expect(out[out.length - 1]).toBe(0x0a);
  });
});

function split(bytes: Uint8Array): Uint8Array[] {
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
