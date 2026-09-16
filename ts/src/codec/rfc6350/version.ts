// SPDX-License-Identifier: MIT

/**
 * The only vCard VERSION value accepted at v1.0. Whether to accept 3.0
 * on read is a parked spec question; until it resolves, anything else
 * is `ErrUnsupportedVersion`.
 */
export const SUPPORTED_VERSION = "4.0";
