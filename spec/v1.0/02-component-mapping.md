<!-- SPDX-License-Identifier: CC-BY-4.0 -->

# V* — Component Mapping

## Core mapping

| Agentic concept | V\* component | RFC |
|---|---|---|
| World | VCALENDAR | RFC 5545 |
| Environment | VJOURNAL or VCALENDAR category | RFC 5545 |
| Player | VCARD | RFC 6350 |
| Group | VCARD `KIND:group` | RFC 6350 |
| Mission | VTODO | RFC 5545 |
| Assignment | VTODO | RFC 5545 |
| Turn | VEVENT | RFC 5545 |
| Playthrough | VEVENT | RFC 5545 |
| Action | VJOURNAL | RFC 5545 |
| Observation | VJOURNAL | RFC 5545 |
| Decision | VJOURNAL | RFC 5545 |
| Score | VJOURNAL | RFC 5545 |
| Memory | VJOURNAL | RFC 5545 |
| Artifact | VJOURNAL + ATTACH | RFC 5545 |
| Resource delta | VJOURNAL | RFC 5545 |
| Learning | VJOURNAL | RFC 5545 |
| Availability | VFREEBUSY | RFC 5545 |
| Timeout / escalation | VALARM | RFC 5545 |
| Timezone | VTIMEZONE | RFC 5545 |

## Required common properties

Every V\* component MUST carry:

```text
UID
DTSTAMP
X-VSTAR-HASH
```

`X-VSTAR-HASH` is the canonical content hash defined in
`03-canonicalization.md`.

## Recommended properties

These are not required but strongly encouraged:

```text
CREATED
LAST-MODIFIED
SEQUENCE
RELATED-TO
CATEGORIES
```

`RELATED-TO` is the primary mechanism for expressing
component-to-component relationships (assignment → mission, action
→ turn, observation → action, etc.).

## Relationship types (RELTYPE)

`RELATED-TO` carries an optional `RELTYPE` parameter (RFC 5545 §3.2.15,
extended by RFC 9253 §4 and §5; registry in RFC 9253 §11.4). The
registered values and their V\* meaning, read from the referencing
component towards the referenced one:

| RELTYPE | RFC | Meaning |
|---|---|---|
| `PARENT` | RFC 5545 §3.2.15 | The referenced component contains this one: assignment → mission, action → turn, observation → action. Default when `RELTYPE` is omitted. |
| `CHILD` | RFC 5545 §3.2.15 | The referenced component is contained by this one. |
| `SIBLING` | RFC 5545 §3.2.15 | The referenced component is a peer of this one under the same parent. |
| `DEPENDS-ON` | RFC 9253 §5 | This component cannot proceed until the referenced one is satisfied. |
| `FINISHTOSTART` | RFC 9253 §4 | The referenced component cannot start until this one finishes. |
| `FINISHTOFINISH` | RFC 9253 §4 | The referenced component cannot finish until this one finishes. |
| `STARTTOFINISH` | RFC 9253 §4 | The referenced component cannot finish until this one starts. |
| `STARTTOSTART` | RFC 9253 §4 | The referenced component cannot start until this one starts. |
| `FIRST` | RFC 9253 §5 | The referenced component is the first in an ordered chain this one belongs to (turns in a playthrough, actions in a turn). |
| `NEXT` | RFC 9253 §5 | The referenced component is the next in an ordered chain this one belongs to. |
| `CONCEPT` | RFC 9253 §5 | The value names a `CONCEPT` property value; every component carrying that `CONCEPT` is referenced. |
| `REFID` | RFC 9253 §5 | The value names a `REFID` property value; every component carrying that `REFID` is referenced. |

Rules:

1. **Direction.** `RELATED-TO` lives on the referencing component and
   points at the referenced one. Emitters SHOULD encode each edge
   once, on the contained component in the `PARENT` direction, and
   SHOULD NOT add a `CHILD` back-reference on the referenced
   component.
2. **Default.** An omitted `RELTYPE` means `PARENT` (RFC 5545
   §3.2.15). Emitters MUST omit `RELTYPE` when its value is `PARENT`:
   `RELATED-TO:x` and `RELATED-TO;RELTYPE=PARENT:x` name the same
   edge but canonicalize, and therefore hash, differently.
3. **Case.** `RELTYPE` values compare case-insensitively (RFC 5545
   §3.2). Emitters SHOULD write them in uppercase; consumers MUST
   accept any case. Canonicalization does not fold parameter-value
   case (`03-canonicalization.md` rule 9 normalizes to NFC only), so
   differently-cased spellings of one value canonicalize, and hash,
   differently.
4. **Extensions.** Consumers MUST accept `RELTYPE` values outside the
   table above (`X-` prefixed, or registered with IANA after RFC
   9253) and SHOULD preserve them verbatim when re-emitting.

## Status supersession (append-only ledger)

V\* is append-only. Original components MUST NOT be mutated. State
changes are expressed as superseding VJOURNAL entries that
reference the original via `RELATED-TO`:

```text
BEGIN:VJOURNAL
UID:journal:status:assignment:todo-factory:scaffold-app:2026-05-04T18:00:00Z
DTSTAMP:20260504T180000Z
RELATED-TO:assignment:todo-factory:scaffold-app
CATEGORIES:status-supersession
X-VSTAR-EFFECTIVE-STATUS:COMPLETED
X-VSTAR-HASH:<canonical-hash>
END:VJOURNAL
```

The value of `X-VSTAR-EFFECTIVE-STATUS` MUST be drawn from the
`STATUS` vocabulary RFC 5545 §3.8.1.11 scopes to the *superseded*
component's type: VTODO values for a mission or assignment, VEVENT
values for a turn.

Projection (state-from-log) is performed by consumers; V\* itself
defines only the supersession encoding.
