<!-- SPDX-License-Identifier: CC-BY-4.0 -->

# V* — Overview

## Design principles

1. **Reuse RFC 5545 / RFC 6350.** Every V\* document is a valid
   iCalendar/vCard document. Generic tooling (parsers, validators,
   calendar UIs, contact-management software) can read V\* without
   knowing it exists.

2. **Component types are the data model.** Agentic concepts are
   expressed as the existing component types — VCALENDAR, VTODO,
   VEVENT, VJOURNAL, VFREEBUSY, VTIMEZONE, VALARM, VCARD — with no
   new top-level kinds.

3. **All extensions live in `X-*` namespaces.** Per-system extensions
   use `X-<SYSTEM>-*` (e.g. `X-AGR-INTENT`); cross-system extensions
   intended for stabilization use `X-VSTAR-*`.

4. **Determinism first.** Two implementations emitting the same
   logical content MUST produce byte-identical canonical forms (see
   `03-canonicalization.md`).

5. **No required runtime.** V\* is a convention over RFC 5545/6350,
   not an implementation. A V\* document is valid in isolation;
   runtime semantics (orchestration, projection, scoring) are
   layered on top by consuming systems like AGR.

## Scope

V\* covers:

- **Mapping** of agentic concepts to V\* component types
- **Required + recommended properties** on each component
- **Extension namespace** rules
- **Canonicalization** for deterministic equality + hashing
- **Conformance** criteria for V\*-claiming implementations

V\* explicitly does NOT cover:

- Orchestration semantics (whose turn is it; runnable selection)
- Projection / state-from-log algorithms
- Storage (JSONL, SQLite, S3, …)
- Transport / wire format beyond the RFC 5545/6350 text form

Those concerns belong to consuming systems (e.g. AGR).

## Non-goals

- Replacing RFC 5545 or RFC 6350.
- Defining a new file format. V\* documents are RFC 5545/6350 text.
- Standardizing agent protocols (see AGNTCY for that).

## Reading order

1. This document — design principles
2. `02-component-mapping.md` — concept → component table
3. `03-canonicalization.md` — determinism + hashing rules
4. `04-extensions.md` — `X-*` discipline
5. `05-conformance.md` — what a conformant impl must do
