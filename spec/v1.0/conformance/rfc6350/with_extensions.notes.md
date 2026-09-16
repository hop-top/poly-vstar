# with_extensions.vcf

A vCard carrying one extension property per spec/04 scope class so
all three `ext.ScopeOf` paths are exercised by the corpus:

- `X-VSTAR-FOO`            → `ext.ScopeVStar`
- `X-AGR-INTENT`           → `ext.ScopeSystem` (system slug "AGR")
- `X-EXP-EXPERIMENTAL`     → `ext.ScopeExperimental`

## Shape

- VERSION:4.0
- UID (urn:uuid)
- KIND:individual
- FN
- Three X-* properties exercising the three sanctioned tiers from
  spec/04 (Extension Discipline)

## Why

Coverage for the V* extension discipline against vCards.
`ext/extensions.go` defines the predicate + classifier; this
fixture is the round-trip + canonical anchor that proves the
classifier sees all three tiers in a real document.

## RFC anchors

- RFC 6350 §6.10 — extended properties (X-* prefix).
- spec/04 — V* Extension Discipline.
