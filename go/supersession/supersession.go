// SPDX-License-Identifier: MIT

// Package supersession encodes V*'s append-only state-change discipline
// from spec/02 §"Status supersession (append-only ledger)".
//
// V* ledgers are append-only: original components MUST NOT be mutated
// after they are appended to a ledger. Status changes are expressed as
// fresh VJOURNAL "supersession" entries that reference the target via
// RELATED-TO and carry:
//
//   - CATEGORIES:status-supersession
//   - X-VSTAR-EFFECTIVE-STATUS:<new status>
//   - X-VSTAR-HASH:<canonical hash of the supersession journal>
//
// This package provides two primitives:
//
//   - Supersedes constructs a fresh supersession VJOURNAL targeting an
//     existing component. It performs an integrity check on the
//     target — if the target carries an X-VSTAR-HASH that does not
//     verify against its own canonical form, Supersedes refuses to
//     construct (returning ErrTargetCorrupted). Targets without
//     X-VSTAR-HASH are accepted (no integrity guarantee, but no
//     refusal either).
//
//   - Superseded queries a ledger for the most recent supersession
//     pointing at a component and returns the latest effective status
//     by DTSTAMP. It is a read-only projection helper, not a
//     validator: ledger entries with unparseable DTSTAMP sort to the
//     start (effectively skipped under "latest wins"); they are not
//     errors.
//
// V* itself defines only the supersession encoding. Full ledger
// projection (state-from-log) is the consumer's job per spec/02.
package supersession

import (
	"errors"
	"strings"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/hashing"
)

// CategoryStatusSupersession is the CATEGORIES wire string that
// marks a VJOURNAL as a supersession entry per spec/02. Consumers
// projecting a ledger MUST match on this exact (case-insensitive)
// value to identify state-change journals.
const CategoryStatusSupersession = "status-supersession"

// PropEffectiveStatus is the X-VSTAR-EFFECTIVE-STATUS property name
// that carries the new status value on a supersession VJOURNAL per
// spec/02. The property's value is opaque to V* — the spec example
// shows VTODO statuses (see vstar.TodoCompleted, vstar.TodoCancelled)
// but applications MAY use any status vocabulary their domain demands.
const PropEffectiveStatus = "X-VSTAR-EFFECTIVE-STATUS"

// uidTimeLayout is the deterministic UID timestamp layout —
// RFC 5545 §3.3.5 form #2 minus colons (already compact). Using the
// canonical wire form keeps UIDs stable, sortable, and trivially
// re-derivable from a (target, time) pair.
const uidTimeLayout = "20060102T150405Z"

// uidPrefix is the literal "journal:status:" prefix carried by every
// supersession UID. Consumers MAY filter on this prefix to enumerate
// supersession journals without parsing CATEGORIES, but matching on
// CategoryStatusSupersession is the spec-blessed path.
const uidPrefix = "journal:status:"

// ErrTargetCorrupted is returned by Supersedes when the supplied
// target component carries an X-VSTAR-HASH property whose value does
// not match the recomputed canonical hash. Targets without
// X-VSTAR-HASH are accepted (no integrity check possible) and do not
// trigger this error.
//
// Callers should treat this sentinel as a hard refusal: the target
// has been mutated post-hash and cannot be safely superseded without
// first reconstructing it from a trusted source.
var ErrTargetCorrupted = errors.New("supersession: target component X-VSTAR-HASH does not match canonical form")

// Supersedes constructs a fresh supersession VJOURNAL that supersedes
// target with the new status at the supplied time. The constructed
// component is returned with a freshly computed X-VSTAR-HASH; the
// target is NOT mutated.
//
// Properties set on the returned component (in order):
//
//   - UID            — "journal:status:<target.UID>:<RFC5545 form #2 t>"
//   - DTSTAMP        — vstar.FormatTime(t)
//   - RELATED-TO     — target.UID()
//   - CATEGORIES     — "status-supersession"
//   - X-VSTAR-EFFECTIVE-STATUS — status (verbatim)
//   - X-VSTAR-HASH   — set last via hashing.SetXVSTAR
//
// Integrity check: if target carries an X-VSTAR-HASH property,
// Supersedes calls hashing.VerifyXVSTAR(target). If verification
// fails, Supersedes returns the zero Component and ErrTargetCorrupted
// without touching anything else. Targets without X-VSTAR-HASH skip
// the check (the caller has implicitly opted out of the integrity
// guarantee).
//
// The append-only contract: callers MUST NOT mutate target after this
// call. Supersedes is the canonical mechanism for changing a V*
// component's status; in-place mutation breaks the ledger model and
// invalidates every downstream X-VSTAR-HASH that referenced the
// pre-mutation form.
func Supersedes(target vstar.Component, status string, t time.Time) (vstar.Component, error) {
	// Integrity check on target — only when an X-VSTAR-HASH is present.
	// A target without X-VSTAR-HASH carries no integrity claim, so
	// there is nothing to verify; proceed.
	if _, hasHash := hashing.GetXVSTAR(target); hasHash {
		ok, _, _ := hashing.VerifyXVSTAR(target)
		if !ok {
			return vstar.Component{}, ErrTargetCorrupted
		}
	}

	uid := uidPrefix + target.UID() + ":" + t.UTC().Format(uidTimeLayout)

	c := vstar.Component{Type: vstar.CompJournal}
	c.Set(vstar.Property{Name: "UID", Value: uid})
	c.Set(vstar.Property{Name: "DTSTAMP", Value: vstar.FormatTime(t)})
	c.Set(vstar.Property{Name: "RELATED-TO", Value: target.UID()})
	c.Set(vstar.Property{Name: "CATEGORIES", Value: CategoryStatusSupersession})
	c.Set(vstar.Property{Name: PropEffectiveStatus, Value: status})

	// Hash refresh MUST be the last mutation; SetXVSTAR strips any
	// existing X-VSTAR-HASH before computing, so the stored value
	// covers every property we just set.
	hashing.SetXVSTAR(&c)

	return c, nil
}

// Superseded walks ledger looking for VJOURNAL components that
// supersede c — i.e. components whose RELATED-TO matches c.UID() AND
// whose CATEGORIES contains "status-supersession" (case-insensitive).
// It returns the X-VSTAR-EFFECTIVE-STATUS value of the LATEST entry
// by parsed DTSTAMP, along with ok=true.
//
// Returns ("", false) when:
//
//   - ledger is empty
//   - c has no UID (nothing to match against)
//   - no entry in ledger supersedes c
//   - matching entries exist but none carry X-VSTAR-EFFECTIVE-STATUS
//
// Tie-breaking: when two entries share the maximum DTSTAMP, the one
// later in the ledger wins (stable scan). Entries whose DTSTAMP cannot
// be parsed sort to the zero time and so are effectively skipped under
// "latest wins" — Superseded is a query, not a validator, and does
// not return errors for malformed ledger data.
func Superseded(c vstar.Component, ledger []vstar.Component) (status string, ok bool) {
	targetUID := c.UID()
	if targetUID == "" {
		return "", false
	}

	var (
		bestT      time.Time
		bestStatus string
		found      bool
	)

	for i := range ledger {
		entry := ledger[i]
		if entry.Type != vstar.CompJournal {
			continue
		}
		rel, hasRel := entry.Get("RELATED-TO")
		if !hasRel || rel.Value != targetUID {
			continue
		}
		if !categoriesContainSupersession(entry) {
			continue
		}
		statusProp, hasStatus := entry.Get(PropEffectiveStatus)
		if !hasStatus {
			continue
		}

		entryT := parseEntryDTSTAMP(entry)
		// Stable "latest wins": >= picks the later ledger position
		// when timestamps tie, since we scan in order.
		if !found || !entryT.Before(bestT) {
			bestT = entryT
			bestStatus = statusProp.Value
			found = true
		}
	}

	if !found {
		return "", false
	}
	return bestStatus, true
}

// categoriesContainSupersession reports whether c carries a
// CATEGORIES property whose value contains
// CategoryStatusSupersession (comma-separated, case-insensitive). Per
// RFC 5545 §3.8.1.2 CATEGORIES values are comma-delimited; we trim
// and compare each token rather than substring-matching the raw value
// (so e.g. "status-supersession-deferred" would NOT falsely match).
func categoriesContainSupersession(c vstar.Component) bool {
	for _, p := range c.GetAll("CATEGORIES") {
		for _, tok := range strings.Split(p.Value, ",") {
			if strings.EqualFold(strings.TrimSpace(tok), CategoryStatusSupersession) {
				return true
			}
		}
	}
	return false
}

// parseEntryDTSTAMP returns the parsed DTSTAMP of c, or the zero
// time when DTSTAMP is missing or unparseable. Superseded uses zero
// as "sorts to the start" — ledger noise is silently demoted, not
// fatal.
func parseEntryDTSTAMP(c vstar.Component) time.Time {
	raw := c.DTSTAMPRaw()
	if raw == "" {
		return time.Time{}
	}
	t, ok := vstar.ParseTime(raw)
	if !ok {
		return time.Time{}
	}
	return t
}
