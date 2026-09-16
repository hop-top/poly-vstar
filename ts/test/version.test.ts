// SPDX-License-Identifier: MIT

import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import { version } from "../src/index.js";
import { version as canonicalVersion } from "../src/canonical/index.js";
import { version as rfc5545Version } from "../src/codec/rfc5545/index.js";

const here = dirname(fileURLToPath(import.meta.url));

describe("version", () => {
  it("is a semver string", () => {
    expect(version).toMatch(/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/);
  });

  it("equals the package.json version release-please rewrites alongside it", () => {
    const manifest = JSON.parse(readFileSync(join(here, "..", "package.json"), "utf8")) as {
      version: string;
    };
    expect(version).toBe(manifest.version);
  });

  it("is the same symbol through every subpath export", () => {
    expect(canonicalVersion).toBe(version);
    expect(rfc5545Version).toBe(version);
  });
});
