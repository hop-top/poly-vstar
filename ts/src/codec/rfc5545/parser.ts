// SPDX-License-Identifier: MIT

import { VstarError } from "../../errors.js";
import type { Calendar, CompType, Component, Param, Property } from "../../types.js";
import {
  findFirstUnquoted,
  newScanner,
  splitUnquoted,
  unescapeText,
  unquoteParamValue,
} from "../contentline.js";
import { SUPPORTED_VERSION, isTextProperty } from "./text-properties.js";

/**
 * Parse a single VCALENDAR from `input`.
 *
 * Property order, parameter order and component order are preserved
 * verbatim. No canonicalization happens here — that is the canonical
 * layer's business.
 *
 * The parser is liberal about line endings (CRLF, LF, or a mixture)
 * and strict about structure. Failures throw a {@link VstarError}
 * carrying `ErrMalformed`, `ErrUnclosedBlock` or
 * `ErrUnsupportedVersion`.
 */
export function parse(input: string | Uint8Array): Calendar {
  const scanner = newScanner(input);

  const first = scanner.next();
  if (first === undefined) {
    throw new VstarError("ErrMalformed", "empty input");
  }
  if (first.toUpperCase() !== "BEGIN:VCALENDAR") {
    throw new VstarError("ErrMalformed", `expected BEGIN:VCALENDAR, got ${JSON.stringify(first)}`);
  }

  const root = parseBlock(scanner, "VCALENDAR");

  const cal: Calendar = { prodId: "", components: root.sub };
  for (const p of root.props) {
    switch (p.name.toUpperCase()) {
      case "VERSION":
        if (p.value !== SUPPORTED_VERSION) {
          throw new VstarError(
            "ErrUnsupportedVersion",
            `VERSION=${JSON.stringify(p.value)} (only ${JSON.stringify(SUPPORTED_VERSION)} supported)`,
          );
        }
        break;
      case "PRODID":
        cal.prodId = p.value;
        break;
      default:
        break;
    }
  }

  // Trailing content after END:VCALENDAR is ignored on purpose:
  // producers concatenate streams and scanners round up trailing
  // whitespace. The stance is strict-but-not-pedantic — we got a valid
  // calendar, stop reading.
  return cal;
}

/**
 * Consume content lines until `END:<typeName>`, assembling one
 * component. Nested `BEGIN:` blocks recurse onto `sub`; a mismatched
 * `END` is `ErrMalformed`; end of input before `END` is
 * `ErrUnclosedBlock`.
 */
function parseBlock(scanner: ReturnType<typeof newScanner>, typeName: string): Component {
  const out: Component = {
    type: typeName.toUpperCase() as CompType,
    props: [],
    sub: [],
  };

  for (;;) {
    const line = scanner.next();
    if (line === undefined) {
      throw new VstarError("ErrUnclosedBlock", `BEGIN:${typeName} never closed`);
    }

    const prop = parseContentLine(line);
    switch (prop.name.toUpperCase()) {
      case "BEGIN":
        out.sub.push(parseBlock(scanner, prop.value));
        break;
      case "END":
        if (prop.value.toUpperCase() !== typeName.toUpperCase()) {
          throw new VstarError(
            "ErrMalformed",
            `END:${prop.value} does not match BEGIN:${typeName}`,
          );
        }
        return out;
      default:
        // Unescape TEXT-typed values so the in-memory model holds raw
        // values; the encoder re-applies escaping symmetrically, which
        // is what makes a parse → encode round-trip byte-stable.
        if (isTextProperty(prop.name)) {
          prop.value = unescapeText(prop.value, "drop-backslash");
        }
        out.props.push(prop);
    }
  }
}

/**
 * Parse a single, already-unfolded RFC 5545 content line.
 *
 * The grammar (RFC 5545 §3.1):
 *
 * ```text
 * contentline = name *(";" param) ":" value
 * param       = param-name "=" param-value *("," param-value)
 * param-value = paramtext / quoted-string
 * ```
 *
 * A DQUOTE-wrapped parameter value may contain commas, semicolons and
 * colons; an unquoted one may not. The wire case of names and
 * parameter names is preserved verbatim — case-insensitive matching is
 * the consumer's job.
 *
 * Throws `ErrMalformed` for a missing colon, an empty name, a parameter
 * without `=`, or an unbalanced DQUOTE.
 */
export function parseContentLine(line: string): Property {
  const colon = findFirstUnquoted(line, ":");
  if (colon === "unbalanced") {
    throw new VstarError("ErrMalformed", `unbalanced quote in ${JSON.stringify(line)}`);
  }
  if (colon < 0) {
    throw new VstarError("ErrMalformed", `missing colon in content line ${JSON.stringify(line)}`);
  }

  const head = line.slice(0, colon);
  const value = line.slice(colon + 1);

  const segs = splitUnquoted(head, ";");
  if (segs === "unbalanced") {
    throw new VstarError("ErrMalformed", `unbalanced quote in ${JSON.stringify(line)}`);
  }
  const name = segs[0];
  if (name === undefined || name === "") {
    throw new VstarError("ErrMalformed", `empty property name in ${JSON.stringify(line)}`);
  }

  const params: Param[] = [];
  for (const seg of segs.slice(1)) {
    const eq = seg.indexOf("=");
    if (eq <= 0) {
      throw new VstarError(
        "ErrMalformed",
        `malformed parameter ${JSON.stringify(seg)} in ${JSON.stringify(line)}`,
      );
    }
    params.push({ name: seg.slice(0, eq), value: unquoteParamValue(seg.slice(eq + 1)) });
  }

  return { name, params, value };
}
