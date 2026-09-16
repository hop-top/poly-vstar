<!-- SPDX-License-Identifier: CC-BY-4.0 -->

# V* — Extension Discipline

## Namespaces

V\* extensions live in three tiers of `X-*` properties.

| Prefix | Scope | Stability |
|---|---|---|
| `X-VSTAR-*` | Cross-system V\* extensions intended for the spec | Stabilization track |
| `X-<SYSTEM>-*` | One specific consuming system | Stable per that system |
| `X-EXP-*` | Experimental / unstable | No guarantees |

`X-VSTAR-HASH` (defined in `02-component-mapping.md`) is the only
mandatory `X-VSTAR-*` property in v0.1.

## Examples

System-specific (AGR):

```text
X-AGR-EFFECTIVE-STATUS
X-AGR-CAPABILITIES
X-AGR-REQUIRES
X-AGR-ENV
X-AGR-ROLE
X-AGR-INTENT
X-AGR-PAYLOAD
X-AGR-DELTAS
X-AGR-CONFIDENCE
X-AGR-PROVENANCE
```

Cross-system V\* candidates (under discussion):

```text
X-VSTAR-EFFECTIVE-STATUS
X-VSTAR-PROVENANCE
X-VSTAR-CONFIDENCE
```

## Promotion path

`X-EXP-*` → `X-<SYSTEM>-*` → `X-VSTAR-*` once two independent
systems implement the same extension with compatible semantics.

## Registration

There is no central registry yet. v0.2 will define a registration
process (likely a `EXTENSIONS.md` manifest in this repo, PR-driven).
For now, system maintainers SHOULD document their `X-<SYSTEM>-*`
extensions in their own repos and cross-link.

## Compatibility rules

- Receivers MUST ignore unknown `X-*` properties (per RFC 5545).
- Senders MUST NOT depend on receivers honoring `X-EXP-*`.
- Removing an extension is a breaking change for consumers;
  promote-then-replace rather than rename.
