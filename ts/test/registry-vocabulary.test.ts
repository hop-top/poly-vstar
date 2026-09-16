// SPDX-License-Identifier: MIT

// Reconciles the value vocabularies this port validates against with
// the registry's cross-language copies.
//
// The two are built from different sources on purpose. The validator's
// tables are projected from the wire constants the codec encodes
// against, so they cannot disagree with what the library writes; the
// generated tables are rendered from
// `spec/registry/status-vocabulary.json`, so they cannot disagree with
// the other four ports. These tests are the join — what lets both
// guarantees hold at once. Without them, generating the tables would
// buy cross-language agreement by giving up the codec linkage, and
// deriving them would buy the codec linkage by giving up
// cross-language agreement.
//
// Order matters, not just membership: the VS044 message joins the
// allowed values, so a reordering is a user-visible change that the
// behavior fixtures pin.

import { describe, expect, it } from "vitest";

import {
  CLASS_VOCABULARY,
  RELTYPE_VOCABULARY,
  STATUS_VOCABULARY,
  TRANSP_VOCABULARY,
} from "../src/generated/codes.js";
import {
  CLASS_CONFIDENTIAL,
  CLASS_PRIVATE,
  CLASS_PUBLIC,
  COMP_EVENT,
  COMP_JOURNAL,
  COMP_TODO,
  EVENT_CANCELLED,
  EVENT_CONFIRMED,
  EVENT_TENTATIVE,
  JOURNAL_CANCELLED,
  JOURNAL_DRAFT,
  JOURNAL_FINAL,
  REL_CHILD,
  REL_CONCEPT,
  REL_DEPENDS_ON,
  REL_FINISH_TO_FINISH,
  REL_FINISH_TO_START,
  REL_FIRST,
  REL_NEXT,
  REL_PARENT,
  REL_REF_ID,
  REL_SIBLING,
  REL_START_TO_FINISH,
  REL_START_TO_START,
  TODO_CANCELLED,
  TODO_COMPLETED,
  TODO_IN_PROCESS,
  TODO_NEEDS_ACTION,
  TRANSP_OPAQUE,
  TRANSP_TRANSPARENT,
} from "../src/types.js";

// Projected from the wire constants, exactly as the validator's table
// is — not read back out of the generated module, which would make the
// assertion vacuous.
const FROM_CONSTANTS: Readonly<Record<string, readonly string[]>> = {
  [COMP_EVENT]: [EVENT_TENTATIVE, EVENT_CONFIRMED, EVENT_CANCELLED],
  [COMP_TODO]: [TODO_NEEDS_ACTION, TODO_IN_PROCESS, TODO_COMPLETED, TODO_CANCELLED],
  [COMP_JOURNAL]: [JOURNAL_DRAFT, JOURNAL_FINAL, JOURNAL_CANCELLED],
};

describe("registry STATUS vocabulary", () => {
  it("scopes STATUS to the same component types as the wire constants", () => {
    expect(Object.keys(STATUS_VOCABULARY).sort()).toEqual(
      Object.keys(FROM_CONSTANTS).sort(),
    );
  });

  for (const [comp, want] of Object.entries(FROM_CONSTANTS)) {
    it(`matches the wire constants for ${comp}`, () => {
      expect(STATUS_VOCABULARY[comp]).toEqual(want);
    });
  }
});

describe("registry flat vocabularies", () => {
  // CLASS and TRANSP are closed vocabularies. RELTYPE is not — RFC 5545
  // §3.2.15 admits IANA and X- values — so the assertion is that the
  // registry lists exactly the *registered* set this port names, not
  // that no other value may appear on the wire.
  const cases: ReadonlyArray<[string, readonly string[], readonly string[]]> = [
    ["CLASS", CLASS_VOCABULARY, [CLASS_PUBLIC, CLASS_PRIVATE, CLASS_CONFIDENTIAL]],
    ["TRANSP", TRANSP_VOCABULARY, [TRANSP_OPAQUE, TRANSP_TRANSPARENT]],
    [
      "RELTYPE",
      RELTYPE_VOCABULARY,
      [
        REL_PARENT,
        REL_CHILD,
        REL_SIBLING,
        REL_FINISH_TO_START,
        REL_FINISH_TO_FINISH,
        REL_START_TO_FINISH,
        REL_START_TO_START,
        REL_DEPENDS_ON,
        REL_FIRST,
        REL_NEXT,
        REL_CONCEPT,
        REL_REF_ID,
      ],
    ],
  ];

  for (const [name, registry, want] of cases) {
    it(`${name} matches the wire constants`, () => {
      expect(registry).toEqual(want);
    });
  }
});
