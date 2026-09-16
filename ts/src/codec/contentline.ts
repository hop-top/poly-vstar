// SPDX-License-Identifier: MIT

// The shared RFC 5545 §3.1 content-line scanner and folder, used by
// both the iCalendar (rfc5545) and vCard (rfc6350) codecs. RFC 6350
// §3.2 references RFC 5545 §3.1 for line folding, so the two formats
// need byte-identical scanning behavior.

/** The RFC 5545 §3.1 fold limit, in OCTETS, excluding the terminator. */
export const MAX_LINE_OCTETS = 75;

/** The wire-format physical-line terminator. */
export const CRLF = "\r\n";

const decoder = new TextDecoder("utf8");
const encoder = new TextEncoder();

/** Decode codec input, which arrives as text or as raw bytes. */
export function decodeInput(input: string | Uint8Array): string {
  return typeof input === "string" ? input : decoder.decode(input);
}

/**
 * A stream of logical content lines, with RFC 5545 §3.1 line unfolding
 * applied.
 *
 * The wire format permits a logical line to be split across physical
 * lines by inserting CRLF + (SP | HTAB); on read both the terminator
 * and the leading WSP octet are consumed and the continuation is
 * appended to the previous logical line.
 *
 * ## Unfolding happens on bytes, decoding on whole logical lines
 *
 * Folding counts **octets** (spec rule 3), so a multi-byte UTF-8
 * sequence straddling the 75-octet boundary is split across the fold
 * and an individual physical line is not necessarily valid UTF-8 on
 * its own. `TextDecoder` replaces each invalid half with U+FFFD, and
 * that substitution is lossy — rejoining two replacement characters
 * does not recover the original code point. So the scanner unfolds
 * over the raw bytes and decodes only once a logical line is whole,
 * which is what spec rule 3 means by "decoders MUST unfold before any
 * character-level interpretation".
 *
 * A `string` input is encoded to bytes on the way in. That input has
 * already been through a decoder, so a split sequence in it is already
 * lost; the conversion keeps one code path rather than two and costs
 * nothing a caller can observe.
 *
 * The scanner is **liberal on input**: CRLF, bare LF, and a mixture are
 * all accepted as terminators, blank physical lines outside a fold
 * sequence are skipped, and a WSP-prefixed line with no pending logical
 * line has its leading WSP stripped and starts a fresh line (which is
 * what real-world vCard parsers do, and what keeps a malformed
 * blank-then-continuation input from either growing a leading space or
 * being silently dropped).
 */
export class Scanner {
  readonly #physical: Uint8Array[];
  #at = 0;
  #pending: Uint8Array[] = [];
  #hasPending = false;

  constructor(input: string | Uint8Array) {
    this.#physical = splitPhysical(typeof input === "string" ? encoder.encode(input) : input);
  }

  /**
   * The next logical content line, or `undefined` when the input is
   * exhausted. The returned string does not include the terminator.
   */
  next(): string | undefined {
    for (;;) {
      if (this.#at >= this.#physical.length) {
        if (!this.#hasPending) return undefined;
        const out = this.#flush();
        this.#hasPending = false;
        return out === "" ? undefined : out;
      }

      const raw = this.#physical[this.#at] as Uint8Array;
      this.#at++;

      if (raw.length > 0 && (raw[0] === 0x20 || raw[0] === 0x09)) {
        // A fold continuation — or, with nothing pending, a fresh
        // logical line whose leading WSP is stripped.
        if (!this.#hasPending) {
          this.#pending = [];
          this.#hasPending = true;
        }
        this.#pending.push(raw.subarray(1));
        continue;
      }

      // A new logical line starts here; flush any pending one first.
      if (this.#hasPending) {
        const out = this.#flush();
        this.#pending = [raw];
        if (raw.length === 0) this.#hasPending = false;
        if (out === "") continue;
        return out;
      }

      if (raw.length === 0) continue;
      this.#pending = [raw];
      this.#hasPending = true;
    }
  }

  /**
   * Decode the pending logical line — now whole, so any UTF-8 sequence
   * the folder split is back in one piece — and reset the accumulator.
   */
  #flush(): string {
    const out = decoder.decode(concatBytes(this.#pending));
    this.#pending = [];
    return out;
  }

  /** Iterate every remaining logical line. */
  *[Symbol.iterator](): Generator<string, void, undefined> {
    for (let line = this.next(); line !== undefined; line = this.next()) {
      yield line;
    }
  }
}

/** A {@link Scanner} over `input`. */
export function newScanner(input: string | Uint8Array): Scanner {
  return new Scanner(input);
}

/** Every logical content line of `input`, in input order. */
export function scanAll(input: string | Uint8Array): string[] {
  return [...new Scanner(input)];
}

/**
 * Split bytes into physical lines on CRLF or bare LF, dropping the
 * terminators. A trailing partial line (no final terminator) is
 * surfaced as a final element; a trailing terminator does not produce
 * an empty final element.
 *
 * Splitting on the LF octet is safe without decoding: in UTF-8 every
 * byte of a multi-byte sequence has its high bit set, so 0x0A and
 * 0x0D never occur inside one. That is what lets the scanner find the
 * fold boundaries before it knows whether the text between them is
 * valid UTF-8.
 */
function splitPhysical(bytes: Uint8Array): Uint8Array[] {
  if (bytes.length === 0) return [];
  const out: Uint8Array[] = [];
  let start = 0;
  for (let i = 0; i < bytes.length; i++) {
    if (bytes[i] !== 0x0a) continue;
    let end = i;
    if (end > start && bytes[end - 1] === 0x0d) end--;
    out.push(bytes.subarray(start, end));
    start = i + 1;
  }
  if (start < bytes.length) {
    let end = bytes.length;
    if (end > start && bytes[end - 1] === 0x0d) end--;
    out.push(bytes.subarray(start, end));
  }
  return out;
}

/**
 * Fold one assembled logical line into CRLF-terminated physical lines,
 * none exceeding {@link MAX_LINE_OCTETS} octets, and return the bytes.
 *
 * Two rules the whole canonical form rests on:
 *
 * - **Octets, measured in UTF-8 bytes.** `String#length` counts UTF-16
 *   code units: `'é'` is 1 there and 2 bytes on the wire. Folding on
 *   the string length puts the fold at the wrong octet, and the damage
 *   surfaces as a canonical-byte mismatch several layers away.
 * - **Applied AFTER property assembly** (spec rule 3). The caller hands
 *   over the complete logical line — name, every parameter, value, with
 *   escaping already applied — and this folds that. Folding a value
 *   before appending parameters puts the fold points in the wrong place
 *   and still unfolds to the same logical line, so only a byte
 *   comparison catches it.
 *
 * Continuation lines are prefixed with a single SP, which costs one of
 * the 75 octets, leaving 74 of payload.
 */
export function foldLine(line: string): Uint8Array {
  const bytes = encoder.encode(line);
  if (bytes.length <= MAX_LINE_OCTETS) {
    return concatBytes([bytes, encoder.encode(CRLF)]);
  }

  const chunks: Uint8Array[] = [];
  const crlf = encoder.encode(CRLF);
  const sp = encoder.encode(" ");

  chunks.push(bytes.subarray(0, MAX_LINE_OCTETS), crlf);
  let rest = bytes.subarray(MAX_LINE_OCTETS);

  const contPayload = MAX_LINE_OCTETS - 1;
  while (rest.length > 0) {
    const take = Math.min(contPayload, rest.length);
    chunks.push(sp, rest.subarray(0, take), crlf);
    rest = rest.subarray(take);
  }
  return concatBytes(chunks);
}

/** Concatenate byte chunks into one buffer. */
export function concatBytes(chunks: readonly Uint8Array[]): Uint8Array {
  let total = 0;
  for (const c of chunks) total += c.length;
  const out = new Uint8Array(total);
  let at = 0;
  for (const c of chunks) {
    out.set(c, at);
    at += c.length;
  }
  return out;
}

/**
 * The byte index of the first occurrence of `target` in `s` that lies
 * outside any DQUOTE-delimited span, or `-1` when there is none.
 *
 * Returns `"unbalanced"` when the string ends inside an open quote —
 * the caller decides which sentinel that maps to.
 */
export function findFirstUnquoted(s: string, target: string): number | "unbalanced" {
  let inQuote = false;
  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if (c === '"') {
      inQuote = !inQuote;
      continue;
    }
    if (!inQuote && c === target) return i;
  }
  return inQuote ? "unbalanced" : -1;
}

/**
 * Split `s` on every occurrence of `sep` outside a DQUOTE-delimited
 * span. Returns `"unbalanced"` when the string ends inside an open
 * quote.
 */
export function splitUnquoted(s: string, sep: string): string[] | "unbalanced" {
  const out: string[] = [];
  let inQuote = false;
  let start = 0;
  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if (c === '"') {
      inQuote = !inQuote;
      continue;
    }
    if (c === sep && !inQuote) {
      out.push(s.slice(start, i));
      start = i + 1;
    }
  }
  if (inQuote) return "unbalanced";
  out.push(s.slice(start));
  return out;
}

/**
 * Strip a surrounding DQUOTE pair from a parameter value, if present.
 */
export function unquoteParamValue(v: string): string {
  return v.length >= 2 && v.startsWith('"') && v.endsWith('"') ? v.slice(1, -1) : v;
}

/**
 * Wrap `v` in DQUOTEs when it contains a character that would
 * otherwise be ambiguous on the wire (`,` `;` `:`) per RFC 5545 §3.2.
 * Inner DQUOTEs are dropped — the grammar does not permit DQUOTE
 * inside a quoted-string.
 */
export function encodeParamValue(v: string): string {
  const stripped = v.replaceAll('"', "");
  return /[,;:]/.test(stripped) ? `"${stripped}"` : stripped;
}

/**
 * Apply RFC 5545 §3.3.11 / RFC 6350 §3.4 TEXT escaping.
 *
 * `\` → `\\` (first, so a literal backslash is not mis-paired), `,` →
 * `\,`, `;` → `\;`, LF → `\n`. CR is dropped: canonical TEXT uses bare
 * LF for embedded newlines, so a literal CRLF collapses to a single
 * escaped `\n`.
 *
 * Not idempotent — a string containing a literal backslash gets
 * re-escaped on a second pass. The encoder calls it exactly once per
 * emit, paired with {@link unescapeText} on the parse side, which is
 * what makes the model hold raw values and the round-trip byte-stable.
 */
export function escapeText(s: string): string {
  if (!/[\\,;\n\r]/.test(s)) return s;
  let out = "";
  for (const c of s) {
    switch (c) {
      case "\\":
        out += "\\\\";
        break;
      case ",":
        out += "\\,";
        break;
      case ";":
        out += "\\;";
        break;
      case "\n":
        out += "\\n";
        break;
      case "\r":
        break;
      default:
        out += c;
    }
  }
  return out;
}

/**
 * Reverse RFC 5545 §3.3.11 / RFC 6350 §3.4 TEXT escaping.
 *
 * `\\` → `\`, `\,` → `,`, `\;` → `;`, `\n` and `\N` → LF. A trailing
 * solitary backslash is preserved verbatim, which keeps the function
 * total and mirrors real-world parser leniency.
 *
 * `unknownEscape` selects what happens to an unrecognized two-character
 * escape such as `\x`: `"drop-backslash"` keeps only the second
 * character (RFC 5545 common practice), `"keep-both"` keeps the
 * backslash too (the lenient vCard shape). The two codecs differ here,
 * so the choice is the caller's.
 */
export function unescapeText(s: string, unknownEscape: "drop-backslash" | "keep-both"): string {
  if (!s.includes("\\")) return s;
  let out = "";
  for (let i = 0; i < s.length; i++) {
    const c = s[i] as string;
    if (c !== "\\" || i + 1 >= s.length) {
      out += c;
      continue;
    }
    const next = s[i + 1] as string;
    switch (next) {
      case "\\":
        out += "\\";
        break;
      case ",":
        out += ",";
        break;
      case ";":
        out += ";";
        break;
      case "n":
      case "N":
        out += "\n";
        break;
      default:
        out += unknownEscape === "keep-both" ? "\\" + next : next;
    }
    i++;
  }
  return out;
}
