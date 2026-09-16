// SPDX-License-Identifier: MIT

/**
 * Package version. release-please rewrites the literal below on every
 * release through this file's `extra-files` entry, keyed on the
 * trailing annotation, and `make version-check` refuses an unannotated
 * copy of the manifest version anywhere else. The test suite pins it
 * equal to `package.json`.
 */
export const version = "1.0.0-alpha.0"; // x-release-please-version
