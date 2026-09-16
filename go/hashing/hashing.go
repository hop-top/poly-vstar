// SPDX-License-Identifier: MIT

// Package hashing produces SHA-256 content hashes of V* objects in
// the `sha256:<hex>` form mandated by spec/02 and spec/03. Hashes
// are computed over the canonical byte form (see ../canonical), so
// two implementations that agree on canonical bytes will produce
// identical hashes — the foundational invariant for cross-language
// integrity.
//
// Public API:
//
//   - Component(Component) string — sha256 of canonical.Component
//     (verbatim datetime emit; see TZID note on Component).
//   - Calendar(Calendar) string  — sha256 of canonical.Calendar
//     (full VCALENDAR; routes datetimes through Calendar's
//     VTIMEZONE registry).
//   - Card(Card) string          — sha256 of canonical.Card.
//   - SetXVSTAR(c *Component)    — compute Component hash and set
//     X-VSTAR-HASH on the component, replacing any existing value.
//   - GetXVSTAR(c) (string, bool) — read the stored X-VSTAR-HASH.
//   - VerifyXVSTAR(c) (ok, want, got string) — recompute and
//     compare against stored value.
//
// All Hash functions are deterministic and pure; they do not
// mutate their inputs. Only SetXVSTAR mutates (its argument is a
// pointer).
//
// Hash exclusion (spec/03 §7): Component, Calendar, and Card
// strip any existing X-VSTAR-HASH property from the input before
// hashing. Otherwise the stored hash would feed back into its own
// digest. Defense-in-depth — the canonical layer also strips
// X-VSTAR-HASH per its own contract; hashing strips again to
// document the invariant at the API boundary.
//
// Algorithm migration: the literal "sha256:" prefix exists per
// spec/03 §7 to allow a future "sha3-256:" or "blake3:" prefix
// without ambiguity. Only "sha256:" is emitted at v1.0.
//
// Package layout note: hashing lives at go/hashing/ rather than at
// the package vstar root because it depends on canonical (which
// itself imports package vstar via codec/rfc5545 — a top-level
// vstar.Hash function would form an import cycle).
package hashing

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	vstar "hop.top/vstar"
	"hop.top/vstar/canonical"
)

// XVSTARHashProperty is the property name V* uses to carry the
// content hash. Per spec/02 every V* component MUST carry this
// property; per spec/03 §7 the property is stripped from input
// before its own value is computed.
const XVSTARHashProperty = "X-VSTAR-HASH"

const sha256Prefix = "sha256:"

// Component returns the `sha256:<hex>` digest of the canonical byte
// form of c per spec/03 §7.
//
// Hash exclusion: any existing X-VSTAR-HASH property on c is
// stripped before computing the canonical bytes — otherwise the
// stored hash would feed back into its own digest. Stripping is
// defense-in-depth; the canonical layer also strips X-VSTAR-HASH
// per its own contract (see canonical.Component).
//
// TZID-context asymmetry: Component delegates to canonical.Component
// (NOT canonical.ComponentInContext). Components carrying TZID-
// tagged datetimes therefore hash over wire-form bytes — different
// timezone resolutions of the same logical instant will hash
// differently. For Calendar-aware hashing that resolves TZIDs to
// UTC via a parent VCALENDAR's VTIMEZONE registry, hash the whole
// Calendar via Calendar() (which routes through
// canonical.Calendar). This mirrors the canonical package's own
// asymmetry and is correct: a Component without a parent Calendar
// has no VTIMEZONE registry to consult.
//
// Component does not mutate c. Internally it builds a stripped
// copy of c.Props with X-VSTAR-HASH filtered out.
func Component(c vstar.Component) string {
	stripped := stripHashProp(c)
	return digest(canonical.Component(stripped))
}

// Calendar returns the `sha256:<hex>` digest of the canonical byte
// form of cal per spec/03 §7.
//
// X-VSTAR-HASH is stripped from cal.Components (top-level) AND
// from each component's Sub list (recursively) before computing
// the canonical bytes. The canonical layer also strips
// X-VSTAR-HASH per its own contract — defense-in-depth.
//
// Datetime handling: cal.Components are canonicalized via
// canonical.Calendar, which threads cal's own VTIMEZONE registry
// through prepareComponent — TZID-tagged datetimes resolve to UTC
// form #2 when the matching VTIMEZONE is present and inside the
// spec's VTIMEZONE subset (spec/03 §Datetime resolution).
//
// Calendar does not mutate cal.
func Calendar(cal vstar.Calendar) string {
	stripped := stripCalendarHashProps(cal)
	return digest(canonical.Calendar(stripped))
}

// Card returns the `sha256:<hex>` digest of the canonical byte
// form of c per spec/03 §7.
//
// X-VSTAR-HASH is stripped from c.Props before computing the
// canonical bytes. The canonical layer also strips X-VSTAR-HASH
// per its own contract — defense-in-depth.
//
// Card does not mutate c.
func Card(c vstar.Card) string {
	stripped := vstar.Card{
		UID:   c.UID,
		Kind:  c.Kind,
		Props: filterOutHash(c.Props),
	}
	return digest(canonical.Card(stripped))
}

// SetXVSTAR computes Component(c) and writes the result to
// c.Props as the X-VSTAR-HASH property. If X-VSTAR-HASH already
// exists, it is replaced (not duplicated). Per spec/02 every V*
// component MUST carry X-VSTAR-HASH; SetXVSTAR is the canonical
// way to add it.
//
// SetXVSTAR mutates c (c is a pointer). The hash is computed over
// the X-VSTAR-HASH-stripped canonical bytes (see Component), so
// calling SetXVSTAR repeatedly on the same logical component is
// idempotent — the second call computes the same hash and rewrites
// the same value.
//
// SetXVSTAR is a no-op when c is nil.
func SetXVSTAR(c *vstar.Component) {
	if c == nil {
		return
	}
	h := Component(*c)
	c.Set(vstar.Property{Name: XVSTARHashProperty, Value: h})
}

// GetXVSTAR returns the X-VSTAR-HASH property value stored on c
// along with an ok flag. ok is false (and the string empty) when
// the property is absent.
//
// GetXVSTAR does not validate the format of the stored value — a
// caller wanting to confirm "sha256:<hex>" shape and freshness
// should use VerifyXVSTAR.
func GetXVSTAR(c vstar.Component) (string, bool) {
	if p, ok := c.Get(XVSTARHashProperty); ok {
		return p.Value, true
	}
	return "", false
}

// VerifyXVSTAR recomputes the hash of c (with X-VSTAR-HASH
// stripped, per Component) and compares it against the stored
// X-VSTAR-HASH value.
//
// Return values:
//
//   - ok    — true iff a stored hash exists AND equals the
//     recomputed hash exactly.
//   - want  — the recomputed (correct) hash; always populated
//     regardless of ok.
//   - got   — the stored hash; empty when no X-VSTAR-HASH
//     property exists.
//
// All three values are returned regardless of ok so callers can
// log the mismatch detail. The recomputed canonical bytes do NOT
// include the stored hash property itself (spec/03 §7).
func VerifyXVSTAR(c vstar.Component) (ok bool, want, got string) {
	want = Component(c)
	stored, exists := GetXVSTAR(c)
	if !exists {
		return false, want, ""
	}
	return stored == want, want, stored
}

// digest computes the literal hex(sha256(b)) digest with the
// "sha256:" prefix per spec/03 §7. Hex is lowercase.
func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return sha256Prefix + hex.EncodeToString(sum[:])
}

// stripHashProp returns a shallow copy of c with any existing
// X-VSTAR-HASH property filtered out of c.Props. Sub is shared by
// reference (immutable read by canonical). Type is copied as-is.
//
// Building a fresh Component (rather than mutating c.Props in
// place) means callers do not observe the strip — Hash is pure.
func stripHashProp(c vstar.Component) vstar.Component {
	return vstar.Component{
		Type:  c.Type,
		Props: filterOutHash(c.Props),
		Sub:   c.Sub,
	}
}

// stripCalendarHashProps returns a copy of cal with X-VSTAR-HASH
// stripped from every component (top-level and nested). Used by
// Calendar to remove the property at every depth before the
// canonical pass — defense-in-depth alongside canonical's own
// strip.
func stripCalendarHashProps(cal vstar.Calendar) vstar.Calendar {
	out := vstar.Calendar{
		ProdID:     cal.ProdID,
		Components: make([]vstar.Component, len(cal.Components)),
	}
	for i, sub := range cal.Components {
		out.Components[i] = stripComponentRecursive(sub)
	}
	return out
}

// stripComponentRecursive returns a copy of c with X-VSTAR-HASH
// removed from c.Props and every nested c.Sub.
func stripComponentRecursive(c vstar.Component) vstar.Component {
	out := vstar.Component{
		Type:  c.Type,
		Props: filterOutHash(c.Props),
	}
	if len(c.Sub) > 0 {
		out.Sub = make([]vstar.Component, len(c.Sub))
		for i, s := range c.Sub {
			out.Sub[i] = stripComponentRecursive(s)
		}
	}
	return out
}

// filterOutHash returns a copy of props with every property whose
// Name matches X-VSTAR-HASH (case-insensitive) removed. Returns
// nil when the result would be empty so callers can treat a nil
// Props slice as "no properties" without an extra length check.
func filterOutHash(props []vstar.Property) []vstar.Property {
	if len(props) == 0 {
		return nil
	}
	out := make([]vstar.Property, 0, len(props))
	for _, p := range props {
		if strings.EqualFold(p.Name, XVSTARHashProperty) {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
