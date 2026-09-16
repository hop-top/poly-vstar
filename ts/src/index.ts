// SPDX-License-Identifier: MIT

/**
 * `@hop-top/vstar` — the V* data model, error sentinels, and the
 * RFC 5545 / RFC 6350 codecs.
 *
 * Every subpath export is also re-exported here under a namespace of
 * the same name, so both spellings work:
 *
 * ```ts
 * import { parse } from "@hop-top/vstar/codec/rfc5545";
 * import { rfc5545 } from "@hop-top/vstar";
 * ```
 */

export { version } from "./version.js";
export {
  CLASS_VOCABULARY,
  CODES,
  CODE_SEVERITIES,
  RELTYPE_VOCABULARY,
  STATUS_VOCABULARY,
  TRANSP_VOCABULARY,
} from "./generated/codes.js";
export type { Code, Severity } from "./generated/codes.js";

export { VSTAR_ERROR_CODES, VstarError } from "./errors.js";
export type { VstarErrorCode, VstarErrorOptions } from "./errors.js";

export type { Codec, Encoder, Parser } from "./codec.js";

export * from "./types.js";
export * from "./date.js";
export * from "./time.js";

import * as canonical from "./canonical/index.js";
import * as diff from "./diff/index.js";
import * as duration from "./duration/index.js";
import * as ext from "./ext/index.js";
import * as hashing from "./hashing/index.js";
import * as helpers from "./helpers/index.js";
import * as rfc5545 from "./codec/rfc5545/index.js";
import * as rfc6350 from "./codec/rfc6350/index.js";
import * as rrule from "./rrule/index.js";
import * as stream from "./codec/stream/index.js";
import * as supersession from "./supersession/index.js";
import * as validate from "./validate/index.js";

export {
  canonical,
  diff,
  duration,
  ext,
  hashing,
  helpers,
  rfc5545,
  rfc6350,
  rrule,
  stream,
  supersession,
  validate,
};
