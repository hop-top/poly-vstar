// SPDX-License-Identifier: MIT

//! Registry-generated tables.
//!
//! Everything under this module is rendered from `spec/registry/` by
//! `tools/registry/gen.py`. Run `make registry-gen` after editing the
//! registry JSON; `make registry-check` fails on drift.

// rustfmt would reflow the generated arrays, `make registry-check`
// would then report drift, and the only fix would be a hand-edit to a
// generated file. `ignore` in rustfmt.toml is nightly-only, so the skip
// rides on the module declaration instead -- it covers the whole file.
#[rustfmt::skip]
#[allow(missing_docs)]
pub mod codes;
