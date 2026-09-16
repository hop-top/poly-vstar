// SPDX-License-Identifier: MIT

// spec/05 §8 — the CLASS and TRANSP value domains.

import { CODES, type Code } from "../generated/codes.js";
import {
  CLASS_CONFIDENTIAL,
  CLASS_PRIVATE,
  CLASS_PUBLIC,
  TRANSP_OPAQUE,
  TRANSP_TRANSPARENT,
  get,
  type Component,
} from "../types.js";
import { diagnostic, equalFold, type Diagnostic } from "./internal.js";

const CLASS = "CLASS";
const TRANSP = "TRANSP";

/**
 * The wire values RFC 5545 §3.8.1.3 and §3.8.2.7 allow.
 *
 * Built from the port's own wire constants rather than read out of the
 * generated `CLASS_VOCABULARY` / `TRANSP_VOCABULARY`, for the reason
 * `status.ts` gives: these are the values the codec encodes against, so
 * a table built from them cannot disagree with what this library
 * actually writes. The registry's cross-language copy is reconciled
 * against these constants in `test/registry-vocabulary.test.ts`.
 */
const CLASS_VALUES: readonly string[] = [CLASS_PUBLIC, CLASS_PRIVATE, CLASS_CONFIDENTIAL];
const TRANSP_VALUES: readonly string[] = [TRANSP_OPAQUE, TRANSP_TRANSPARENT];

/**
 * Flag a CLASS outside RFC 5545 §3.8.1.3's vocabulary and a TRANSP
 * outside §3.8.2.7's.
 *
 * Comparison is case-insensitive (spec/05 §8, RFC 5545 §3.1). The value
 * is checked on any component carrying the property — there is no type
 * gating, because V\* diagnoses no scope rule for any property. An `X-`
 * or IANA token on CLASS, which the RFC's ABNF admits, is still flagged:
 * spec/05 §8 binds the value to the three registered names.
 */
export function checkClassification(c: Component, path: string): Diagnostic[] {
  const out: Diagnostic[] = [];
  const cls = vocabularyDiagnostic(c, path, CLASS, CLASS_VALUES, CODES.CodeClassNotInVocabulary, "RFC 5545 §3.8.1.3");
  if (cls !== undefined) out.push(cls);
  const transp = vocabularyDiagnostic(
    c,
    path,
    TRANSP,
    TRANSP_VALUES,
    CODES.CodeTranspNotInVocabulary,
    "RFC 5545 §3.8.2.7",
  );
  if (transp !== undefined) out.push(transp);
  return out;
}

/**
 * The status-shaped diagnostic for property `name` on `c` when its
 * value is outside `allowed`; `undefined` when the property is absent
 * or its value is allowed.
 */
function vocabularyDiagnostic(
  c: Component,
  path: string,
  name: string,
  allowed: readonly string[],
  code: Code,
  ref: string,
): Diagnostic | undefined {
  const p = get(c, name);
  if (p === undefined) return undefined;
  if (allowed.some((want) => equalFold(p.value, want))) return undefined;
  return diagnostic(
    code,
    `${name} value ${p.value} is not valid; allowed: ${allowed.join(", ")} (${ref})`,
    `${path}.${name}`,
  );
}
