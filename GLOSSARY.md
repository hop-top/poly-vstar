# V* — Glossary

| Term | Definition |
|---|---|
| **V\*** | Convention for representing agentic-system state as iCalendar (RFC 5545) + vCard (RFC 6350) components. Pronounced "vee-star". |
| **Component** | An iCalendar/vCard top-level entity: VCALENDAR, VTODO, VEVENT, VJOURNAL, VFREEBUSY, VTIMEZONE, VALARM, VCARD. |
| **Property** | A name/value pair inside a component (e.g. `UID:foo`, `DTSTAMP:20260504T180000Z`). |
| **Parameter** | A name/value pair on a property (e.g. `TZID=America/Montreal` in `DTSTART;TZID=America/Montreal:20260504T140000`). |
| **Canonical form** | The deterministic byte sequence for a component, used for equality + hashing. See [`03-canonicalization.md`](spec/v1.0/03-canonicalization.md). |
| **Supersession** | Append-only state change pattern using `RELATED-TO` + `X-VSTAR-EFFECTIVE-STATUS`. See [`02-component-mapping.md`](spec/v1.0/02-component-mapping.md). |
| **Emitter** | Implementation that produces V\* documents. |
| **Consumer** | Implementation that reads V\* documents and projects state. |
| **Round-trip** | Emitter + Consumer with byte-identical re-emission of self-emitted documents. |
| **Ledger** | An append-only sequence of V\* components. Storage format is consumer-defined (JSONL, SQLite, …); V\* defines only the components themselves. |
| **Projection** | The act of folding a ledger into current state. Consumer-defined; outside V\*'s scope. |
| **Reference implementation** | The Go module `hop.top/vstar` in [`go/`](go/). Where it and a document disagree about behaviour, it is the behaviour of record. |
| **Port** | A V\* implementation in a language other than Go (`ts/`, `py/`, `rs/`, `php/`), held byte-identical to the reference over the shared corpus. |
| **Conformance corpus** | The canonical fixture set under [`spec/v1.0/conformance/`](spec/v1.0/conformance/) that every implementation is checked against. [`go/testdata/`](go/testdata/) is its generated mirror. |
| **Fixture** | One corpus case: an input document plus the sibling files stating what it must produce (`.canonical`, `.hash`, `.error`). |
| **Sidecar** | A sibling file asserting one property of a fixture — `.expect.json`, `.next.json`, `.formatted`. Chiefly the `rrule/` cases. |
| **Sentinel** | A named failure class (e.g. `ErrMalformed`). The Go spelling is the stable cross-language identifier the corpus asserts, not a Go implementation detail. |
| **Diagnostic** | A finding from `validate` — a stable code, a severity, and a path locating it. Validation reports diagnostics; it does not fail. |
| **Registry** | The machine-readable tables in [`spec/registry/`](spec/registry/) — diagnostic codes, property allow-list, extension scopes, value vocabularies — rendered into each port's generated module. Never hand-edited. |
| **Parity harness** | `make test-parity`: each implementation emits one JSON document over the corpus and every document must be identical. |
| **Mirror** | A read-only per-language repository republished from one tree on release. The source of truth is always this repository. |
