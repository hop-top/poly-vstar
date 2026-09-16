// SPDX-License-Identifier: MIT

// The `ext` surface — extension classification under spec/04 — gated by
// `spec/behavior/ext/scopes.json`.

import { describe, expect, it } from "vitest";

import {
  SCOPE_EXPERIMENTAL,
  SCOPE_NONE,
  SCOPE_SYSTEM,
  SCOPE_UNKNOWN,
  SCOPE_VSTAR,
  type Scope,
  extensionsByScope,
  isExtension,
  scopeOf,
  scopeToString,
  systemName,
} from "../src/ext/index.js";
import type { Component, Property } from "../src/types.js";
import { type ScopeCase, loadBehaviorJson } from "./behavior.js";

function prop(name: string, value: string): Property {
  return { name, params: [], value };
}

describe("ext scopes — spec/behavior/ext/scopes.json", () => {
  const cases = loadBehaviorJson<ScopeCase[]>("ext", "scopes.json");

  it("covers the whole table", () => {
    // Seventeen names, spanning every scope including system-name
    // extraction. A shrinking table means a fixture went missing.
    expect(cases.length).toBe(17);
  });

  it("exercises every scope the type admits", () => {
    const seen = new Set(cases.map((c) => c.scope));
    expect([...seen].sort()).toEqual(["experimental", "none", "system", "unknown", "vstar"]);
  });

  for (const [i, c] of cases.entries()) {
    it(`[${i}] ${JSON.stringify(c.name)} is ${c.scope}`, () => {
      expect(scopeOf(c.name)).toBe(c.scope);
      expect(systemName(c.name)).toBe(c.system ?? undefined);
    });
  }

  it("agrees with isExtension: only a non-extension is scope none", () => {
    for (const c of cases) {
      expect(isExtension(c.name)).toBe(c.scope !== SCOPE_NONE);
    }
  });

  it("carries a system slug for exactly the system-scoped names", () => {
    for (const c of cases) {
      expect(c.system !== null).toBe(c.scope === SCOPE_SYSTEM);
    }
  });
});

describe("isExtension", () => {
  it.each([
    ["X-FOO", true],
    ["x-foo", true],
    ["X-AGR-INTENT", true],
    // The hyphen is required: a bare X is an ordinary identifier, while
    // "X-" is an ill-formed extension.
    ["X-", true],
    ["X", false],
    ["DTSTART", false],
    ["FOO", false],
    ["", false],
  ])("%s → %s", (name, want) => {
    expect(isExtension(name)).toBe(want);
  });
});

describe("scopeOf classification", () => {
  it("is case-insensitive", () => {
    expect(scopeOf("x-vstar-hash")).toBe(scopeOf("X-VSTAR-HASH"));
    expect(scopeOf("x-agr-intent")).toBe(scopeOf("X-AGR-INTENT"));
    expect(scopeOf("x-exp-foo")).toBe(scopeOf("X-EXP-FOO"));
  });

  it("reserves the whole VSTAR slug, not a prefix of it", () => {
    expect(scopeOf("X-VSTAR-FOO")).toBe(SCOPE_VSTAR);
    // X-VSTARLIKE-FOO is an ordinary system extension owned by
    // VSTARLIKE; the reservation covers the segment, not its prefix.
    expect(scopeOf("X-VSTARLIKE-FOO")).toBe(SCOPE_SYSTEM);
    expect(systemName("X-VSTARLIKE-FOO")).toBe("VSTARLIKE");
  });

  it("reserves the whole EXP slug, not a prefix of it", () => {
    expect(scopeOf("X-EXP-FOO")).toBe(SCOPE_EXPERIMENTAL);
    expect(scopeOf("X-EXPORT-FOO")).toBe(SCOPE_SYSTEM);
    expect(systemName("X-EXPORT-FOO")).toBe("EXPORT");
  });

  it("calls a prefixed name with no suffix unknown", () => {
    for (const name of ["X-", "X-FOO", "X-VSTAR-", "X-EXP-", "X-AGR-"]) {
      expect(scopeOf(name)).toBe(SCOPE_UNKNOWN);
    }
  });
});

describe("systemName", () => {
  it("uppercases the slug so callers compare without re-normalizing", () => {
    expect(systemName("x-agr-intent")).toBe("AGR");
    expect(systemName("X-CRM-DEAL-STAGE")).toBe("CRM");
  });

  it("excludes the two reserved tiers — they have no owning system", () => {
    expect(systemName("X-VSTAR-HASH")).toBeUndefined();
    expect(systemName("x-vstar-hash")).toBeUndefined();
    expect(systemName("X-EXP-FOO")).toBeUndefined();
  });

  it("excludes non-extensions and malformed names", () => {
    for (const name of ["DTSTART", "", "X-", "X-AGR-", "X-FOO"]) {
      expect(systemName(name)).toBeUndefined();
    }
  });
});

describe("scopeToString", () => {
  it.each<[Scope, string]>([
    [SCOPE_NONE, "None"],
    [SCOPE_VSTAR, "VStar"],
    [SCOPE_SYSTEM, "System"],
    [SCOPE_EXPERIMENTAL, "Experimental"],
    [SCOPE_UNKNOWN, "Unknown"],
  ])("renders %s as the reference's display spelling %s", (scope, want) => {
    expect(scopeToString(scope)).toBe(want);
  });
});

describe("extensionsByScope", () => {
  const c: Component = {
    type: "VEVENT",
    props: [
      prop("UID", "u@example"),
      prop("DTSTART", "20260504T000000Z"),
      prop("X-VSTAR-HASH", "sha256:abc"),
      prop("X-AGR-INTENT", "schedule"),
      prop("X-AGR-ROLE", "host"),
      prop("X-CRM-DEAL", "open"),
      prop("X-EXP-FOO", "bar"),
      prop("X-", "malformed"),
      prop("X-AGR-", "malformed slug"),
    ],
    sub: [],
  };

  it("selects the vstar tier", () => {
    expect(extensionsByScope(c, SCOPE_VSTAR).map((p) => p.name)).toEqual(["X-VSTAR-HASH"]);
  });

  it("selects every system-scoped property regardless of owner", () => {
    expect(extensionsByScope(c, SCOPE_SYSTEM).map((p) => p.name)).toEqual([
      "X-AGR-INTENT",
      "X-AGR-ROLE",
      "X-CRM-DEAL",
    ]);
  });

  it("selects the experimental tier", () => {
    expect(extensionsByScope(c, SCOPE_EXPERIMENTAL).map((p) => p.name)).toEqual(["X-EXP-FOO"]);
  });

  it("selects the malformed names as unknown", () => {
    expect(extensionsByScope(c, SCOPE_UNKNOWN).map((p) => p.name)).toEqual(["X-", "X-AGR-"]);
  });

  it("selects the non-extension core surface under scope none", () => {
    expect(extensionsByScope(c, SCOPE_NONE).map((p) => p.name)).toEqual(["UID", "DTSTART"]);
  });

  it("preserves the component's property order — no sort", () => {
    const reordered: Component = {
      ...c,
      props: [prop("X-CRM-DEAL", "open"), prop("X-AGR-INTENT", "schedule")],
    };
    expect(extensionsByScope(reordered, SCOPE_SYSTEM).map((p) => p.name)).toEqual([
      "X-CRM-DEAL",
      "X-AGR-INTENT",
    ]);
  });

  it("returns an empty list, never a nullish value, when nothing matches", () => {
    expect(extensionsByScope({ type: "VTODO", props: [], sub: [] }, SCOPE_VSTAR)).toEqual([]);
  });

  it("does not recurse into sub-components", () => {
    const parent: Component = {
      type: "VEVENT",
      props: [prop("UID", "u")],
      sub: [{ type: "VALARM", props: [prop("X-AGR-NESTED", "v")], sub: [] }],
    };
    expect(extensionsByScope(parent, SCOPE_SYSTEM)).toEqual([]);
  });
});
