# kind_group.vcf

A vCard with `KIND:group` and three MEMBER properties pointing at
fake UIDs of group members.

## Shape

- VERSION:4.0
- UID (urn:uuid)
- KIND:group
- FN
- Three MEMBER properties (urn:uuid references to fake members)

## Why

Round-trip + canonical + hash coverage for the third RFC 6350
KIND value. Pairs with `minimal.vcf` (KindIndividual implicit) and
`kind_org.vcf` to give all three V*-defined Kind enum values
(individual, org, group; see types.go) at least one fixture.

The MEMBER property is special to `KIND:group` per RFC 6350 §6.6.5
and exercises the same-name multi-property path on a Card (rather
than on a Component as in `world.ics`).

## RFC anchors

- RFC 6350 §6.1.4 — KIND values.
- RFC 6350 §6.6.5 — MEMBER property (group membership).
