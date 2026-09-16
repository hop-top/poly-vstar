// SPDX-License-Identifier: MIT

import js from "@eslint/js";
import tseslint from "typescript-eslint";

export default tseslint.config(
  // Build output and generated sources are never linted: `dist` and
  // `parity-dist` are tsc's own emitted JavaScript (the latter from
  // tsconfig.parity.json), and `src/generated` is rendered from
  // spec/registry/ by `make registry-gen`.
  { ignores: ["dist/**", "parity-dist/**", "src/generated/**"] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
);
