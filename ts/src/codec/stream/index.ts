// SPDX-License-Identifier: MIT

/**
 * The `codec/stream` subpath export: constant-memory, iterator-style
 * codecs for the V* wire formats.
 *
 * Where the batch codecs load a whole `Calendar` or `Card` list into
 * memory, these surface one `Component` or `Card` at a time, so a
 * caller processing a large ledger keeps memory flat regardless of
 * input size.
 *
 * ## Input is a physical-line source, and it is pulled lazily
 *
 * A parser takes a {@link LineSource}: an iterable of *physical* lines —
 * the fold continuations still attached, terminators already stripped —
 * whose elements are either `Uint8Array` or `string`. It pulls from that
 * iterable only as far as the component it is currently assembling
 * requires, so handing it a generator that reads a file incrementally
 * keeps the whole pipeline incremental. Handing it an array works too
 * and is what the corpus tests do; the array is simply already in
 * memory. A whole `string` or `Uint8Array` document is accepted directly
 * and split lazily, so the common case needs no adapter at all.
 *
 * ## Prefer the byte form: folding splits characters
 *
 * Folding counts **octets** (canonicalization rule 3), so a multi-byte
 * UTF-8 sequence straddling the 75-octet boundary is split across the
 * fold and an individual physical line is not necessarily valid UTF-8
 * on its own. A JavaScript `string` cannot hold half of such a
 * sequence: `TextDecoder` substitutes U+FFFD for each half, and that
 * substitution is lossy — rejoining two replacement characters does not
 * recover the code point.
 *
 * So a caller streaming genuinely folded canonical bytes MUST feed
 * bytes: a `Uint8Array` document, an `Iterable<Uint8Array>` of physical
 * lines, or {@link byteLines} / {@link byteLinesFromAsyncIterable}. The
 * parsers unfold over those bytes and decode only once a logical line
 * is whole, which is what rule 3 means by "decoders MUST unfold before
 * any character-level interpretation".
 *
 * The `string` forms remain supported, and are correct for input that
 * is already unfolded or that no fold ever split. They cannot be made
 * correct for a split sequence, because the loss happened in whatever
 * decoder produced the string, before the parser ever saw it.
 *
 * {@link byteLinesFromAsyncIterable} adapts an `AsyncIterable<Uint8Array>`
 * — a Node stream, a `fetch` body — into a lazy byte-line iterable,
 * holding at most one partial line. {@link fromAsyncIterable} is its
 * string-producing predecessor, kept for callers whose input is already
 * unfolded.
 *
 * ## Exhaustion is never an error
 *
 * Both parsers implement the iterator protocol: `next()` returns
 * `{done: true}` at exhaustion and throws a `VstarError` on a real
 * parse failure. Both are iterable, so `for (const c of parser)` works.
 *
 * ## Encoder lifecycle
 *
 * `close()` twice, or `encode()` after `close()`, throws
 * `ErrAlreadyClosed`. `setHeader()` after the first `encode()` throws
 * `ErrHeaderLocked` — the header locks at the first encode so the
 * `BEGIN:VCALENDAR` / `VERSION` / `PRODID` ordering on the wire is
 * deterministic.
 *
 * {@link VCardEncoder} has **no** `setHeader`: a VCARD stream has no
 * enclosing wrapper, each `encode` writing a complete, self-contained
 * `BEGIN:VCARD…END:VCARD` block. A port that adds one for symmetry is
 * broken — there is nothing for it to do, and its existence invites a
 * caller to emit a wrapper the parsers reject.
 */

import { VstarError } from "../../errors.js";
import type { Calendar, Card, CompType, Component } from "../../types.js";
import { concatBytes, foldLine, unescapeText } from "../contentline.js";
import { encodeComponent, encodeContentLine, parseContentLine } from "../rfc5545/index.js";
import { isTextProperty } from "../rfc5545/text-properties.js";
import { encode as encodeCard } from "../rfc6350/encoder.js";
import { parseContentLine as parseCardContentLine } from "../rfc6350/parser.js";

export { version } from "../../version.js";

/** The one iCalendar VERSION v0.1 supports. */
const SUPPORTED_CALENDAR_VERSION = "2.0";

/** The one vCard VERSION v0.1 supports. */
const SUPPORTED_CARD_VERSION = "4.0";

/**
 * The `PRODID` {@link VCalendarEncoder} emits when `setHeader` was not
 * called, mirroring the batch encoder's default so batch and stream
 * output for the same logical calendar stay byte-stable.
 */
const DEFAULT_PROD_ID = "-//hop-top//vstar-go v0.1.0//EN";

const encoder = new TextEncoder();
const decoder = new TextDecoder("utf8");

/** Line-terminator and leading-WSP octets, tested before any decode. */
const LF = 0x0a;
const CR = 0x0d;
const SP = 0x20;
const HTAB = 0x09;

const KW_BEGIN = "BEGIN";
const KW_END = "END";

/**
 * What the stream parsers accept as input.
 *
 * A whole document (`string` or `Uint8Array`) is split into physical
 * lines lazily; an iterable of physical lines is consumed as-is, its
 * elements either bytes or strings and free to mix.
 *
 * Only the byte forms can carry a UTF-8 sequence that folding split —
 * see the module doc. Python's port spells the same union as
 * `str | bytes | bytearray | IO[bytes] | IO[str]`; Go takes an
 * `io.Reader`, Rust a `BufRead`, PHP a stream resource.
 */
export type LineSource = string | Uint8Array | Iterable<string | Uint8Array>;

/** The iterator-style read interface for a VCALENDAR stream. */
export interface StreamParser extends IterableIterator<Component> {
  /** The calendar-level header captured from `BEGIN:VCALENDAR`. */
  header(): Calendar;
}

/** The iterator-style write interface for a VCALENDAR stream. */
export interface StreamEncoder {
  setHeader(h: Calendar): void;
  encode(c: Component): void;
  close(): void;
}

/** The VCARD analog of {@link StreamParser}. */
export type CardStreamParser = IterableIterator<Card>;

/**
 * The VCARD analog of {@link StreamEncoder} — deliberately without
 * `setHeader`. See the module doc.
 */
export interface CardStreamEncoder {
  encode(c: Card): void;
  close(): void;
}

/**
 * A sink the stream encoders write completed byte chunks to.
 *
 * Go's encoders take an `io.Writer`; TypeScript has no single idiomatic
 * equivalent, so the encoders take this one-method shape. A caller
 * writing to a file passes a wrapper around the file handle; a caller
 * collecting bytes in memory uses {@link byteSink}.
 */
export interface ByteSink {
  write(chunk: Uint8Array): void;
}

/** A {@link ByteSink} accumulating into memory, plus the bytes it holds. */
export interface MemoryByteSink extends ByteSink {
  /** Everything written so far, concatenated. */
  bytes(): Uint8Array;
}

/** A fresh in-memory {@link ByteSink}. */
export function byteSink(): MemoryByteSink {
  const chunks: Uint8Array[] = [];
  return {
    write(chunk: Uint8Array): void {
      chunks.push(chunk);
    },
    bytes(): Uint8Array {
      return concatBytes(chunks);
    },
  };
}

// ---------------------------------------------------------------------
// Line sources
// ---------------------------------------------------------------------

/**
 * Split `input` into physical lines lazily, on CRLF or bare LF, with
 * the terminators dropped.
 *
 * A trailing partial line — no final terminator — is surfaced as the
 * last element; a trailing terminator produces no empty final element.
 *
 * **Lossy for folded canonical bytes.** Decoding a `Uint8Array` here
 * happens before unfolding, so a UTF-8 sequence the folder split across
 * the boundary is already two U+FFFD by the time a line is yielded.
 * Use {@link byteLines} — or hand the `Uint8Array` straight to the
 * parser, which does the same thing — whenever the input may be folded.
 * This stays for callers whose input is already unfolded and who want
 * strings.
 */
export function* lines(input: string | Uint8Array): Iterable<string> {
  const text = typeof input === "string" ? input : new TextDecoder("utf8").decode(input);
  if (text === "") return;
  let start = 0;
  for (;;) {
    const nl = text.indexOf("\n", start);
    if (nl < 0) {
      const tail = text.slice(start);
      if (tail !== "") yield stripCr(tail);
      return;
    }
    yield stripCr(text.slice(start, nl));
    start = nl + 1;
    if (start >= text.length) return;
  }
}

/**
 * Split `input` into physical lines lazily **without decoding**, on
 * CRLF or bare LF, with the terminators dropped.
 *
 * This is {@link lines} for bytes, and the one a caller streaming
 * canonical form wants: a fold-split UTF-8 sequence survives as the two
 * byte halves it really is, for the parser to rejoin before decoding.
 *
 * Scanning for the LF octet is safe pre-decode because every byte of a
 * multi-byte UTF-8 sequence has its high bit set, so 0x0A and 0x0D
 * never occur inside one.
 */
export function* byteLines(input: string | Uint8Array): Iterable<Uint8Array> {
  const bytes = typeof input === "string" ? encoder.encode(input) : input;
  if (bytes.length === 0) return;
  let start = 0;
  for (let i = 0; i < bytes.length; i++) {
    if (bytes[i] !== LF) continue;
    let end = i;
    if (end > start && bytes[end - 1] === CR) end--;
    yield bytes.subarray(start, end);
    start = i + 1;
  }
  if (start < bytes.length) {
    let end = bytes.length;
    if (end > start && bytes[end - 1] === CR) end--;
    yield bytes.subarray(start, end);
  }
}

/**
 * Adapt an `AsyncIterable<Uint8Array>` — a Node readable stream, a
 * `fetch` body — into the line array the parsers consume, decoding
 * chunk-wise with a streaming decoder so a multi-byte character split
 * across two chunks survives.
 *
 * Memory is bounded by the longest single line, not by the input: a
 * chunk is decoded, split, and its complete lines handed on immediately,
 * with only the trailing partial line held. That is the property this
 * function exists for — a caller with a 200 MB `.ics` on disk gets a
 * component at a time, not a 200 MB string.
 *
 * The result is an array rather than an async generator because the
 * parsers are synchronous iterators; a caller wanting lazy async
 * consumption pairs this with its own chunking rather than materializing
 * the whole document. For that reason prefer {@link lines} over a
 * `fromAsyncIterable` of an already-loaded buffer.
 *
 * **Lossy for folded canonical bytes**, for the same reason as
 * {@link lines}: the streaming decoder rejoins a character split across
 * two *chunks*, but a character split across a *fold* is two separate
 * physical lines and cannot be rejoined after decoding. Use
 * {@link byteLinesFromAsyncIterable} whenever the input may be folded.
 */
export async function fromAsyncIterable(chunks: AsyncIterable<Uint8Array>): Promise<string[]> {
  const decoder = new TextDecoder("utf8");
  const out: string[] = [];
  let pending = "";
  for await (const chunk of chunks) {
    pending += decoder.decode(chunk, { stream: true });
    for (;;) {
      const nl = pending.indexOf("\n");
      if (nl < 0) break;
      out.push(stripCr(pending.slice(0, nl)));
      pending = pending.slice(nl + 1);
    }
  }
  pending += decoder.decode();
  if (pending !== "") out.push(stripCr(pending));
  return out;
}

/**
 * Adapt an `AsyncIterable<Uint8Array>` — a Node readable stream, a
 * `fetch` body — into the physical-line **byte** array the parsers
 * consume, without decoding.
 *
 * This is {@link fromAsyncIterable} for bytes, and the one to reach for
 * when the input may be folded canonical form: nothing is decoded here,
 * so a UTF-8 sequence split across a fold reaches the parser as the two
 * byte halves it really is.
 *
 * Memory is bounded by the longest single line, not by the input: each
 * chunk's complete lines are handed on immediately, with only the
 * trailing partial line held.
 *
 * Returned lines are copies, not views into the caller's chunks, so a
 * caller recycling a read buffer is safe.
 */
export async function byteLinesFromAsyncIterable(
  chunks: AsyncIterable<Uint8Array>,
): Promise<Uint8Array[]> {
  const out: Uint8Array[] = [];
  // The bytes of the line currently being assembled — never more than
  // one line's worth, which is the memory bound this function promises.
  let pending: Uint8Array<ArrayBufferLike> = new Uint8Array(0);

  for await (const chunk of chunks) {
    pending = concatBytes([pending, chunk]);
    let start = 0;
    for (;;) {
      const nl = pending.indexOf(LF, start);
      if (nl < 0) break;
      let end = nl;
      if (end > start && pending[end - 1] === CR) end--;
      out.push(pending.slice(start, end));
      start = nl + 1;
    }
    pending = pending.slice(start);
  }

  if (pending.length > 0) {
    let end = pending.length;
    if (pending[end - 1] === CR) end--;
    if (end > 0) out.push(pending.slice(0, end));
  }
  return out;
}

/** Drop a trailing CR from a physical line split on LF. */
function stripCr(s: string): string {
  return s.endsWith("\r") ? s.slice(0, -1) : s;
}

/**
 * Normalize a {@link LineSource} into an iterator of physical lines as
 * bytes.
 *
 * A whole document is split with {@link byteLines}, which does not
 * decode. An iterable's `string` elements are encoded on the way in, so
 * the unfolder has one code path rather than two. That encode costs a
 * `string` caller nothing observable: such an input has already been
 * through a decoder, so a fold-split sequence in it was lost before
 * this function ever saw it.
 */
function physicalLines(source: LineSource): Iterator<Uint8Array> {
  if (typeof source === "string" || source instanceof Uint8Array) {
    return byteLines(source)[Symbol.iterator]();
  }
  const it = source[Symbol.iterator]();
  return {
    next(): IteratorResult<Uint8Array> {
      const step = it.next();
      if (step.done === true) return { done: true, value: undefined };
      const v = step.value;
      return { done: false, value: typeof v === "string" ? encoder.encode(v) : v };
    },
  };
}

/**
 * An incremental RFC 5545 §3.1 unfolder over a physical-line iterable.
 *
 * It holds exactly one assembled logical line plus the one physical
 * line it has read ahead, so memory stays flat however long the input
 * is. A continuation line — one starting with SP or HTAB — appends to
 * the pending logical line with its single leading WSP stripped.
 *
 * ## It accumulates bytes and decodes whole logical lines
 *
 * Folding counts octets, so a physical line can end mid-UTF-8-sequence
 * and is not necessarily valid UTF-8 on its own. Decoding per physical
 * line would substitute U+FFFD for each half of a split sequence, and
 * that is not recoverable by concatenating afterwards. So the pending
 * line accumulates as bytes and is decoded only once it is whole — the
 * same discipline the batch `Scanner` keeps, for the same reason.
 *
 * Testing the leading octet for SP (0x20) or HTAB (0x09) is safe before
 * decoding: no UTF-8 continuation byte falls below 0x80.
 */
class Unfolder {
  readonly #it: Iterator<Uint8Array>;
  #pending: Uint8Array[] = [];
  #hasPending = false;
  #exhausted = false;

  constructor(source: LineSource) {
    this.#it = physicalLines(source);
  }

  /** The next logical content line, or `undefined` at exhaustion. */
  next(): string | undefined {
    for (;;) {
      if (this.#exhausted) {
        if (!this.#hasPending) return undefined;
        const out = this.#flush();
        this.#hasPending = false;
        return out === "" ? undefined : out;
      }

      const step = this.#it.next();
      if (step.done === true) {
        this.#exhausted = true;
        continue;
      }
      const raw = step.value;

      if (raw.length > 0 && (raw[0] === SP || raw[0] === HTAB)) {
        if (!this.#hasPending) {
          this.#pending = [];
          this.#hasPending = true;
        }
        this.#pending.push(raw.subarray(1));
        continue;
      }

      if (this.#hasPending) {
        const out = this.#flush();
        this.#pending = [raw];
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
}

// ---------------------------------------------------------------------
// VCALENDAR
// ---------------------------------------------------------------------

/**
 * The streaming VCALENDAR reader: one top-level component per
 * {@link VCalendarParser.next}.
 *
 * The header is consumed on first use — content lines up to
 * `BEGIN:VCALENDAR`, then the calendar-level properties before the
 * first sub-component, captured for {@link VCalendarParser.header}.
 * Once `END:VCALENDAR` is seen the iterator is done.
 *
 * Not safe for concurrent use; construct one per input.
 */
export class VCalendarParser implements StreamParser {
  readonly #src: Unfolder;
  #headerRead = false;
  #header: Calendar = { prodId: "", components: [] };
  #pendingLine: string | undefined;
  #done = false;

  constructor(source: LineSource) {
    this.#src = new Unfolder(source);
  }

  /**
   * The calendar-level properties captured from the `BEGIN:VCALENDAR`
   * header. `components` is always empty — only the VCALENDAR-level
   * properties populate here.
   *
   * Safe to call before the first `next()`: the header is read on
   * demand. When that read fails the error is held and surfaced from
   * the next `next()` call, so this still returns whatever was parsed
   * and a caller can introspect it.
   */
  header(): Calendar {
    if (!this.#headerRead) {
      try {
        this.#ensureHeader();
      } catch {
        // Held deliberately: the failure surfaces from next().
      }
    }
    return this.#header;
  }

  /**
   * The next top-level component, or `{done: true}` once
   * `END:VCALENDAR` has been consumed.
   *
   * @throws {VstarError} with code `ErrMalformed` or `ErrUnclosedBlock`
   * on a structurally invalid stream, or `ErrUnsupportedVersion` when
   * `VERSION` is not `2.0`.
   */
  next(): IteratorResult<Component> {
    if (this.#done) return { done: true, value: undefined };
    this.#ensureHeader();

    const line = this.#nextLine();
    if (line === undefined) {
      throw new VstarError("ErrUnclosedBlock", "stream/vcalendar: BEGIN:VCALENDAR never closed");
    }

    const prop = parseContentLine(line);
    const name = prop.name.toUpperCase();

    if (name === KW_END) {
      if (prop.value.toUpperCase() !== "VCALENDAR") {
        throw new VstarError(
          "ErrMalformed",
          `stream/vcalendar: END:${prop.value} does not match BEGIN:VCALENDAR`,
        );
      }
      this.#done = true;
      return { done: true, value: undefined };
    }
    if (name === KW_BEGIN) {
      return { done: false, value: this.#readBlock(prop.value.toUpperCase()) };
    }

    // A calendar-level property after the header section. RFC 5545
    // allows METHOD and X-* anywhere inside VCALENDAR before the first
    // sub-component; one appearing among the components means the
    // producer interleaved header properties, which is rejected rather
    // than silently absorbed.
    throw new VstarError(
      "ErrMalformed",
      `stream/vcalendar: unexpected calendar-level property ${JSON.stringify(prop.name)} after components`,
    );
  }

  /** The iterator protocol, so `for (const c of parser)` works. */
  [Symbol.iterator](): IterableIterator<Component> {
    return this;
  }

  /**
   * Read `BEGIN:VCALENDAR` and the calendar-level properties up to the
   * first `BEGIN:<sub>` or `END:VCALENDAR`, stashing that boundary line
   * for the next pull. Runs exactly once.
   */
  #ensureHeader(): void {
    if (this.#headerRead) return;
    this.#headerRead = true;

    const first = this.#src.next();
    if (first === undefined) {
      throw new VstarError("ErrMalformed", "stream/vcalendar: empty input");
    }
    if (first.toUpperCase() !== "BEGIN:VCALENDAR") {
      throw new VstarError(
        "ErrMalformed",
        `stream/vcalendar: expected BEGIN:VCALENDAR, got ${JSON.stringify(first)}`,
      );
    }

    for (;;) {
      const line = this.#src.next();
      if (line === undefined) {
        throw new VstarError("ErrUnclosedBlock", "stream/vcalendar: BEGIN:VCALENDAR never closed");
      }

      const prop = parseContentLine(line);
      const upper = prop.name.toUpperCase();
      if (upper === KW_BEGIN || upper === KW_END) {
        this.#pendingLine = line;
        return;
      }
      if (upper === "VERSION" && prop.value !== SUPPORTED_CALENDAR_VERSION) {
        throw new VstarError(
          "ErrUnsupportedVersion",
          `stream/vcalendar: VERSION=${JSON.stringify(prop.value)} (only ${JSON.stringify(SUPPORTED_CALENDAR_VERSION)} supported)`,
        );
      }
      // PRODID is TEXT-typed, and the batch parser unescapes it on the
      // way through its block parser before lifting it onto the
      // Calendar. Doing the same here is what keeps a streamed header
      // equal to a batch-parsed one.
      if (upper === "PRODID") this.#header.prodId = unescapeText(prop.value, "drop-backslash");
    }
  }

  /** The next logical line, taking any stashed boundary line first. */
  #nextLine(): string | undefined {
    if (this.#pendingLine !== undefined) {
      const line = this.#pendingLine;
      this.#pendingLine = undefined;
      return line;
    }
    return this.#src.next();
  }

  /**
   * Parse a `BEGIN:<typeName> … END:<typeName>` subtree from the line
   * after the `BEGIN`. Reads straight off the shared unfolder, so the
   * stream position stays in lockstep.
   */
  #readBlock(typeName: string): Component {
    const out: Component = { type: typeName as CompType, props: [], sub: [] };
    for (;;) {
      const line = this.#src.next();
      if (line === undefined) {
        throw new VstarError(
          "ErrUnclosedBlock",
          `stream/vcalendar: BEGIN:${typeName} never closed`,
        );
      }

      const prop = parseContentLine(line);
      switch (prop.name.toUpperCase()) {
        case KW_BEGIN:
          out.sub.push(this.#readBlock(prop.value.toUpperCase()));
          break;
        case KW_END:
          if (prop.value.toUpperCase() !== typeName) {
            throw new VstarError(
              "ErrMalformed",
              `stream/vcalendar: END:${prop.value} does not match BEGIN:${typeName}`,
            );
          }
          return out;
        default:
          // Unescape TEXT-typed values, exactly as the batch block
          // parser does, so the in-memory model holds raw values and a
          // streamed parse of a document equals its batch parse. The
          // content-line parser alone does not do this — the escape
          // rules are property-typed, and the type table lives a layer
          // above it.
          if (isTextProperty(prop.name)) {
            prop.value = unescapeText(prop.value, "drop-backslash");
          }
          out.props.push(prop);
          break;
      }
    }
  }
}

/** A {@link VCalendarParser} over `source`. */
export function newVCalendarParser(source: LineSource): VCalendarParser {
  return new VCalendarParser(source);
}

/**
 * The streaming VCALENDAR writer.
 *
 * The first `encode` emits the `BEGIN:VCALENDAR` header — `VERSION`
 * then `PRODID` — and each subsequent one appends a component.
 * `close` emits `END:VCALENDAR`.
 *
 * Not safe for concurrent use; construct one per sink.
 */
export class VCalendarEncoder implements StreamEncoder {
  readonly #sink: ByteSink;
  #header: Calendar = { prodId: "", components: [] };
  #headerWritten = false;
  #closed = false;

  constructor(sink: ByteSink) {
    this.#sink = sink;
  }

  /**
   * Configure the header emitted on the next `encode`. Only `prodId` is
   * consulted — `VERSION` is fixed at `2.0` per the v0.1 supported-
   * version contract.
   *
   * @throws {VstarError} with code `ErrHeaderLocked` when called after
   * the first `encode`: the header is already on the wire and cannot be
   * retroactively changed.
   */
  setHeader(h: Calendar): void {
    if (this.#headerWritten) {
      throw new VstarError("ErrHeaderLocked", "stream/vcalendar: header set after first encode");
    }
    this.#header = h;
  }

  /**
   * Write one component, emitting the header first if this is the first
   * call.
   *
   * @throws {VstarError} with code `ErrAlreadyClosed` when called after
   * `close`.
   */
  encode(c: Component): void {
    if (this.#closed) {
      throw new VstarError("ErrAlreadyClosed", "stream/vcalendar: encode after close");
    }
    this.#writeHeaderOnce();
    this.#sink.write(encodeComponent(c));
  }

  /**
   * Emit `END:VCALENDAR`. Safe on an encoder that never saw an
   * `encode` — the header is written first, so the output is still a
   * legal empty calendar.
   *
   * @throws {VstarError} with code `ErrAlreadyClosed` when called twice.
   * A port that silently ignores a double close is broken.
   */
  close(): void {
    if (this.#closed) {
      throw new VstarError("ErrAlreadyClosed", "stream/vcalendar: close called twice");
    }
    this.#writeHeaderOnce();
    this.#sink.write(foldLine("END:VCALENDAR"));
    this.#closed = true;
  }

  /** Emit `BEGIN:VCALENDAR` + `VERSION` + `PRODID` exactly once. */
  #writeHeaderOnce(): void {
    if (this.#headerWritten) return;
    this.#headerWritten = true;
    const prodId = this.#header.prodId === "" ? DEFAULT_PROD_ID : this.#header.prodId;
    this.#sink.write(foldLine("BEGIN:VCALENDAR"));
    this.#sink.write(foldLine(`VERSION:${SUPPORTED_CALENDAR_VERSION}`));
    // Routed through the batch encoder's content-line renderer rather
    // than escaped inline: PRODID is TEXT-typed, and sharing the
    // renderer is what makes stream and batch output byte-identical
    // for the same calendar.
    this.#sink.write(foldLine(encodeContentLine({ name: "PRODID", params: [], value: prodId })));
  }
}

/** A {@link VCalendarEncoder} writing to `sink`. */
export function newVCalendarEncoder(sink: ByteSink): VCalendarEncoder {
  return new VCalendarEncoder(sink);
}

// ---------------------------------------------------------------------
// VCARD
// ---------------------------------------------------------------------

/**
 * The streaming VCARD reader: one card per {@link VCardParser.next}.
 *
 * A VCARD file is a concatenation of independent
 * `BEGIN:VCARD…END:VCARD` blocks with no outer wrapper, so there is no
 * header to skip and the iterator is simply done when the input runs
 * out between blocks.
 *
 * Not safe for concurrent use; construct one per input.
 */
export class VCardParser implements IterableIterator<Card> {
  readonly #src: Unfolder;
  #done = false;

  constructor(source: LineSource) {
    this.#src = new Unfolder(source);
  }

  /**
   * The next card, or `{done: true}` when the input is exhausted with
   * no block pending.
   *
   * @throws {VstarError} with code `ErrMalformed` for a missing
   * `BEGIN`, a nested `BEGIN:VCARD`, a duplicate `VERSION` or a missing
   * one; `ErrUnsupportedVersion` when `VERSION` is not `4.0`; and
   * `ErrUnclosedBlock` when the input ends inside an open block.
   */
  next(): IteratorResult<Card> {
    if (this.#done) return { done: true, value: undefined };

    // Wait for BEGIN:VCARD, skipping blank lines between blocks.
    for (;;) {
      const line = this.#src.next();
      if (line === undefined) {
        this.#done = true;
        return { done: true, value: undefined };
      }
      if (line === "") continue;
      if (line.toUpperCase() !== "BEGIN:VCARD") {
        throw new VstarError(
          "ErrMalformed",
          `stream/vcard: expected BEGIN:VCARD, got ${JSON.stringify(line)}`,
        );
      }
      break;
    }

    const card: Card = { uid: "", kind: "", props: [] };
    let version = "";

    for (;;) {
      const line = this.#src.next();
      if (line === undefined) {
        throw new VstarError("ErrUnclosedBlock", "stream/vcard: BEGIN:VCARD never closed");
      }
      if (line === "") continue;

      const upperLine = line.toUpperCase();
      if (upperLine === "END:VCARD") {
        if (version === "") {
          throw new VstarError("ErrMalformed", "stream/vcard: VCARD missing VERSION");
        }
        if (version !== SUPPORTED_CARD_VERSION) {
          throw new VstarError("ErrUnsupportedVersion", `stream/vcard: VERSION:${version}`);
        }
        return { done: false, value: card };
      }
      if (upperLine === "BEGIN:VCARD") {
        throw new VstarError("ErrMalformed", "stream/vcard: nested BEGIN:VCARD");
      }

      // The vCard content-line parser, not the iCalendar one: RFC 6350
      // §3.3's grammar is close enough that the latter would read the
      // line, but §3.4 TEXT unescaping differs between the two formats
      // and lives inside each parser. Reaching for the iCalendar
      // parser here would leave `FN:Last\, Comma` escaped, so the
      // stream parse and the batch parse would disagree about the same
      // document — see the port's conformance notes.
      const prop = parseCardContentLine(line);
      switch (bareName(prop.name).toUpperCase()) {
        case "VERSION":
          if (version !== "") {
            throw new VstarError("ErrMalformed", "stream/vcard: duplicate VERSION");
          }
          version = prop.value;
          break;
        case "UID":
          card.uid = prop.value;
          break;
        case "KIND":
          card.kind = prop.value.toLowerCase() as Card["kind"];
          break;
        default:
          card.props.push(prop);
          break;
      }
    }
  }

  /** The iterator protocol, so `for (const c of parser)` works. */
  [Symbol.iterator](): IterableIterator<Card> {
    return this;
  }
}

/** A {@link VCardParser} over `source`. */
export function newVCardParser(source: LineSource): VCardParser {
  return new VCardParser(source);
}

/**
 * The streaming VCARD writer: one self-contained
 * `BEGIN:VCARD…END:VCARD` block per `encode`.
 *
 * There is deliberately **no** `setHeader` — see the module doc. `close`
 * exists for symmetry with {@link VCalendarEncoder} and to mark the
 * stream finished; it emits no trailer, because a VCARD stream has none.
 */
export class VCardEncoder implements CardStreamEncoder {
  readonly #sink: ByteSink;
  #closed = false;

  constructor(sink: ByteSink) {
    this.#sink = sink;
  }

  /**
   * Write one card in canonical VCARD wire form.
   *
   * @throws {VstarError} with code `ErrAlreadyClosed` when called after
   * `close`, or `ErrMissingUID` when the card has no UID.
   */
  encode(c: Card): void {
    if (this.#closed) {
      throw new VstarError("ErrAlreadyClosed", "stream/vcard: encode after close");
    }
    this.#sink.write(encodeCard(c));
  }

  /**
   * Mark the stream finished.
   *
   * @throws {VstarError} with code `ErrAlreadyClosed` when called twice.
   */
  close(): void {
    if (this.#closed) {
      throw new VstarError("ErrAlreadyClosed", "stream/vcard: close called twice");
    }
    this.#closed = true;
  }
}

/** A {@link VCardEncoder} writing to `sink`. */
export function newVCardEncoder(sink: ByteSink): VCardEncoder {
  return new VCardEncoder(sink);
}

/**
 * Strip the optional `group.` prefix from a vCard property name per
 * RFC 6350 §3.3, so `UID`, `VERSION` and `KIND` are recognized whatever
 * group qualifies them on the wire.
 */
function bareName(name: string): string {
  const i = name.indexOf(".");
  return i >= 0 ? name.slice(i + 1) : name;
}
