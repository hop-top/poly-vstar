// SPDX-License-Identifier: MIT

// The `hashing` surface: the three hash functions and the
// X-VSTAR-HASH read/write/verify trio.

import { describe, expect, it } from "vitest";

import { component as canonicalComponent } from "../src/canonical/index.js";
import {
  X_VSTAR_HASH_PROPERTY,
  calendar as hashCalendar,
  card as hashCard,
  component as hashComponent,
  getXVstar,
  setXVstar,
  verifyXVstar,
} from "../src/hashing/index.js";
import type { Calendar, Card, Component, Property } from "../src/types.js";
import { get } from "../src/types.js";

function prop(name: string, value: string): Property {
  return { name, params: [], value };
}

function todo(): Component {
  return {
    type: "VTODO",
    props: [prop("UID", "u"), prop("DTSTAMP", "20260504T120000Z"), prop("SUMMARY", "s")],
    sub: [],
  };
}

const SHA256_HEX = /^sha256:[0-9a-f]{64}$/;

describe("hash shape", () => {
  it("is sha256: plus 64 lowercase hex digits", () => {
    expect(hashComponent(todo())).toMatch(SHA256_HEX);
    expect(hashCalendar({ prodId: "-//V*//T//EN", components: [todo()] })).toMatch(SHA256_HEX);
    expect(hashCard({ uid: "u", kind: "individual", props: [prop("FN", "Jad")] })).toMatch(
      SHA256_HEX,
    );
  });

  it("keeps the prefix as part of the value", () => {
    expect(hashComponent(todo()).startsWith("sha256:")).toBe(true);
  });
});

describe("the hash is over the canonical bytes", () => {
  it("matches an independently computed digest of canonical.component", async () => {
    const { sha256 } = await import("@noble/hashes/sha2");
    const { bytesToHex } = await import("@noble/hashes/utils");
    const c = todo();
    expect(hashComponent(c)).toBe(`sha256:${bytesToHex(sha256(canonicalComponent(c)))}`);
  });

  it("changes when any canonical byte changes", () => {
    const a = todo();
    const b = todo();
    (b.props[2] as Property).value = "t";
    expect(hashComponent(a)).not.toBe(hashComponent(b));
  });
});

describe("setXVstar / getXVstar / verifyXVstar", () => {
  it("writes the property", () => {
    const c = todo();
    setXVstar(c);
    expect(getXVstar(c)).toMatch(SHA256_HEX);
  });

  it("is idempotent — a second call rewrites the same value", () => {
    const c = todo();
    setXVstar(c);
    const first = getXVstar(c);
    setXVstar(c);
    expect(getXVstar(c)).toBe(first);
    expect(c.props.filter((p) => p.name === X_VSTAR_HASH_PROPERTY)).toHaveLength(1);
  });

  it("replaces an existing value rather than duplicating it", () => {
    const c = todo();
    c.props.push(prop(X_VSTAR_HASH_PROPERTY, "sha256:stale"));
    setXVstar(c);
    expect(c.props.filter((p) => p.name === X_VSTAR_HASH_PROPERTY)).toHaveLength(1);
    expect(getXVstar(c)).not.toBe("sha256:stale");
  });

  it("reports undefined when the property is absent", () => {
    expect(getXVstar(todo())).toBeUndefined();
  });

  it("verifies a freshly stamped component", () => {
    const c = todo();
    setXVstar(c);
    const { ok, want, got } = verifyXVstar(c);
    expect(ok).toBe(true);
    expect(got).toBe(want);
  });

  it("reports what differed on a mismatch", () => {
    const c = todo();
    setXVstar(c);
    (get(c, "SUMMARY") as Property).value = "mutated";
    const { ok, want, got } = verifyXVstar(c);
    expect(ok).toBe(false);
    expect(got).not.toBe(want);
    expect(want).toMatch(SHA256_HEX);
    expect(got).toMatch(SHA256_HEX);
  });

  it("returns want and an empty got when no hash is stored", () => {
    const { ok, want, got } = verifyXVstar(todo());
    expect(ok).toBe(false);
    expect(want).toMatch(SHA256_HEX);
    expect(got).toBe("");
  });
});

describe("purity", () => {
  it("hashing does not mutate the component", () => {
    const c = todo();
    c.props.push(prop(X_VSTAR_HASH_PROPERTY, "sha256:00"));
    const before = JSON.stringify(c);
    hashComponent(c);
    verifyXVstar(c);
    expect(JSON.stringify(c)).toBe(before);
  });

  it("hashing does not mutate the calendar", () => {
    const cal: Calendar = { prodId: "-//V*//T//EN", components: [todo()] };
    const before = JSON.stringify(cal);
    hashCalendar(cal);
    expect(JSON.stringify(cal)).toBe(before);
  });

  it("hashing does not mutate the card", () => {
    const card: Card = {
      uid: "u",
      kind: "individual",
      props: [prop("FN", "Jad"), prop(X_VSTAR_HASH_PROPERTY, "sha256:00")],
    };
    const before = JSON.stringify(card);
    hashCard(card);
    expect(JSON.stringify(card)).toBe(before);
  });
});

describe("the hash property name lives in hashing", () => {
  it("is the RFC-shaped extension name", () => {
    expect(X_VSTAR_HASH_PROPERTY).toBe("X-VSTAR-HASH");
  });
});
