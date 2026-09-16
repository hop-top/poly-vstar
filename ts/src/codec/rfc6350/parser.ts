// SPDX-License-Identifier: MIT

import { VstarError } from "../../errors.js";
import type { Card, Param, Property } from "../../types.js";
import {
  findFirstUnquoted,
  scanAll,
  unescapeText,
  unquoteParamValue,
} from "../contentline.js";
import { SUPPORTED_VERSION } from "./version.js";

/**
 * Parse every `BEGIN:VCARD…END:VCARD` block in `input` and return them
 * as a **list**.
 *
 * A vCard stream is a sequence of self-contained blocks with no
 * enclosing wrapper, so a file with three cards parses to three
 * {@link Card} values. Empty input returns an empty list — "no cards"
 * is not an error.
 *
 * ## UID handling is asymmetric across this codec
 *
 * The parser **accepts** a VCARD with no UID property, leaving
 * `card.uid` as `""`. The encoder **refuses** such a card with
 * `ErrMissingUID`, and the semantic validation layer reports it as a
 * diagnostic. That asymmetry is deliberate: the parser is permissive so
 * adopters can recover a non-conforming document rather than lose it,
 * and strictness lives where it can be opted into.
 *
 * Failures throw a {@link VstarError} carrying `ErrMalformed`,
 * `ErrUnsupportedVersion` or `ErrUnclosedBlock`.
 */
export function parse(input: string | Uint8Array): Card[] {
  const lines = scanAll(input);

  const cards: Card[] = [];
  let open = false;
  let current: Card = emptyCard();
  let version = "";

  for (const [index, line] of lines.entries()) {
    const at = index + 1;
    if (line === "") continue;

    if (line.toUpperCase() === "BEGIN:VCARD") {
      if (open) throw new VstarError("ErrMalformed", "nested BEGIN:VCARD", { line: at });
      open = true;
      current = emptyCard();
      version = "";
      continue;
    }

    if (line.toUpperCase() === "END:VCARD") {
      if (!open) throw new VstarError("ErrMalformed", "stray END:VCARD", { line: at });
      if (version === "") {
        throw new VstarError("ErrMalformed", "VCARD missing VERSION", { line: at });
      }
      if (version !== SUPPORTED_VERSION) {
        throw new VstarError("ErrUnsupportedVersion", `VERSION:${version}`, { line: at });
      }
      cards.push(current);
      open = false;
      current = emptyCard();
      version = "";
      continue;
    }

    if (!open) {
      // Real vCards never carry content outside BEGIN/END.
      throw new VstarError("ErrMalformed", "content outside VCARD", { line: at });
    }

    const prop = parseContentLine(line, at);

    if (prop.name.toUpperCase() === "VERSION") {
      if (version !== "") {
        throw new VstarError("ErrMalformed", "duplicate VERSION", { line: at });
      }
      version = prop.value;
      continue;
    }
    if (prop.name.toUpperCase() === "UID") {
      current.uid = prop.value;
      continue;
    }
    if (prop.name.toUpperCase() === "KIND") {
      // RFC 6350 §6.1.4 registry values are lowercase; the wire is
      // case-insensitive on read.
      current.kind = prop.value.toLowerCase();
      continue;
    }
    current.props.push(prop);
  }

  if (open) throw new VstarError("ErrUnclosedBlock", "BEGIN:VCARD never closed");
  return cards;
}

function emptyCard(): Card {
  return { uid: "", kind: "", props: [] };
}

/**
 * Decompose one unfolded vCard content line into a property per
 * RFC 6350 §3.3 / §3.4:
 *
 * ```text
 * [group "."] name *(";" param) ":" value
 * ```
 *
 * A group prefix, when present, is preserved verbatim on the property
 * name (e.g. `home.TEL`). TEXT escaping is reversed on the value.
 */
export function parseContentLine(line: string, at?: number): Property {
  const where = at === undefined ? {} : { line: at };

  if (line === "") throw new VstarError("ErrMalformed", "empty line", where);

  const colon = findFirstUnquoted(line, ":");
  if (colon === "unbalanced") {
    throw new VstarError("ErrMalformed", "unterminated quoted value", where);
  }
  if (colon < 0) throw new VstarError("ErrMalformed", "missing value separator", where);

  const head = line.slice(0, colon);
  const rawValue = line.slice(colon + 1);

  const nameEnd = findFirstUnquoted(head, ";");
  if (nameEnd === "unbalanced") {
    throw new VstarError("ErrMalformed", "unterminated quoted value", where);
  }

  const name = nameEnd < 0 ? head : head.slice(0, nameEnd);
  const paramTok = nameEnd < 0 ? "" : head.slice(nameEnd + 1);

  if (name === "") throw new VstarError("ErrMalformed", "empty property name", where);

  return {
    name,
    params: parseParams(paramTok, where),
    value: unescapeText(rawValue, "keep-both"),
  };
}

/**
 * Parse the parameter portion of a content-line head. Each parameter is
 * `NAME=VALUE`; VALUE may be DQUOTE-wrapped to embed `,` `:` or `;`.
 */
function parseParams(s: string, where: { line?: number }): Param[] {
  if (s === "") return [];
  const params: Param[] = [];
  let rest = s;
  while (rest.length > 0) {
    const end = findFirstUnquoted(rest, ";");
    if (end === "unbalanced") {
      throw new VstarError("ErrMalformed", "unterminated quoted value", where);
    }
    const tok = end < 0 ? rest : rest.slice(0, end);
    rest = end < 0 ? "" : rest.slice(end + 1);

    const eq = findFirstUnquoted(tok, "=");
    if (eq === "unbalanced") {
      throw new VstarError("ErrMalformed", "unterminated quoted value", where);
    }
    if (eq < 0) {
      throw new VstarError("ErrMalformed", `param ${JSON.stringify(tok)} missing '='`, where);
    }
    const name = tok.slice(0, eq);
    if (name === "") throw new VstarError("ErrMalformed", "empty param name", where);
    params.push({ name, value: unquoteParamValue(tok.slice(eq + 1)) });
  }
  return params;
}
