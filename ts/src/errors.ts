// SPDX-License-Identifier: MIT

/**
 * The twelve V* failure classes, spelled exactly as the Go reference
 * spells them. The identifier is the contract: `malformed/*.error`
 * files and `rrule/**\/*.expect.json` sidecars name a class by this
 * string, so a port that re-spells one cannot run the shared corpus.
 *
 * Only the first four are reachable at the codec layer; the rest are
 * declared here so the union is complete from the start and the later
 * layers have nothing to widen.
 */
export const VSTAR_ERROR_CODES = [
  "ErrMalformed",
  "ErrUnclosedBlock",
  "ErrUnsupportedVersion",
  "ErrMissingUID",
  "ErrUnsupportedRRule",
  "ErrIterationCap",
  "ErrUnboundedExpansion",
  "ErrTargetCorrupted",
  "ErrAlreadyClosed",
  "ErrHeaderLocked",
  "ErrNoTrigger",
  "ErrNoAnchor",
] as const;

/** A string-literal union of exactly the twelve sentinel identifiers. */
export type VstarErrorCode = (typeof VSTAR_ERROR_CODES)[number];

/** Optional positional and causal context attached to a failure. */
export interface VstarErrorOptions {
  /** 1-based content-line number, where the failure site is known. */
  readonly line?: number;
  /** The underlying failure, when this one wraps another. */
  readonly cause?: unknown;
}

/**
 * The single error class every V* failure travels in.
 *
 * Dispatch on {@link VstarError.code}, not on the class:
 *
 * ```ts
 * try {
 *   parse(input);
 * } catch (e) {
 *   if (e instanceof VstarError && e.code === "ErrMalformed") { /* … *\/ }
 * }
 * ```
 *
 * There is deliberately no subclass per sentinel. `instanceof` is
 * unreliable across module boundaries when a bundler duplicates the
 * module; a string comparison is not.
 */
export class VstarError extends Error {
  /** The sentinel identifier, verbatim in the Go spelling. */
  readonly code: VstarErrorCode;
  /** 1-based content-line number, where known. */
  readonly line: number | undefined;

  constructor(code: VstarErrorCode, message: string, options: VstarErrorOptions = {}) {
    super(
      options.line === undefined ? `${code}: ${message}` : `${code}: line ${options.line}: ${message}`,
      options.cause === undefined ? undefined : { cause: options.cause },
    );
    this.name = "VstarError";
    this.code = code;
    this.line = options.line;
  }
}
