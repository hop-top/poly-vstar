// SPDX-License-Identifier: MIT

import { describe, expect, it } from "vitest";

import { version } from "../src/index.js";
import { version as canonicalVersion } from "../src/canonical/index.js";
import { version as rfc5545Version } from "../src/codec/rfc5545/index.js";

describe("version", () => {
  it("is a semver string", () => {
    expect(version).toMatch(/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/);
  });

  it("is the same symbol through every subpath export", () => {
    expect(canonicalVersion).toBe(version);
    expect(rfc5545Version).toBe(version);
  });
});
