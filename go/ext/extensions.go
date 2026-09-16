// SPDX-License-Identifier: MIT

// Package ext provides predicates and accessors for the V* X-*
// extension namespace defined in spec/04 (Extension Discipline).
//
// V* extensions live in three tiers of X-* properties (see spec/04):
//
//   - X-VSTAR-*           Cross-system V* extensions on a stabilization track.
//   - X-<SYSTEM>-*        One specific consuming system (e.g. X-AGR-INTENT).
//   - X-EXP-*             Experimental / unstable; no guarantees.
//
// Promotion path: an experimental property starts as X-EXP-FOO,
// graduates to X-<SYSTEM>-FOO when one system commits to it, then
// promotes to X-VSTAR-FOO once two independent systems implement it
// with compatible semantics.
//
// Callers will typically import this package as ext and call
// ext.IsExtension, ext.ScopeOf, ext.SystemName, etc.
package ext

import (
	"strings"

	vstar "hop.top/vstar"
)

// IsExtension reports whether name has an X- prefix per RFC 5545
// §3.8.8 / spec/04. Comparison is case-insensitive: both "X-FOO"
// and "x-foo" return true. Plain identifiers without the hyphen
// (e.g. "X", "FOO", "DTSTART") and the empty string return false.
//
// The hyphen is required: "X" is a regular IANA-style identifier,
// while "X-" is an (ill-formed) extension. To classify a name,
// pair this with ScopeOf.
func IsExtension(name string) bool {
	if len(name) < 2 {
		return false
	}
	return (name[0] == 'X' || name[0] == 'x') && name[1] == '-'
}

// Scope classifies an X-* extension name according to spec/04
// (Extension Discipline). A non-extension name has ScopeNone; an
// extension that does not match any of the three sanctioned tiers
// has ScopeUnknown.
//
// The three sanctioned tiers and their promotion path:
//
//	X-EXP-*   →  X-<SYSTEM>-*  →  X-VSTAR-*
//
// Promotion rules per spec/04: an experimental property starts as
// X-EXP-FOO. When one consuming system commits to it, it graduates
// to X-<SYSTEM>-FOO (e.g. X-AGR-INTENT). Once two independent
// systems implement the property with compatible semantics, it can
// be promoted to X-VSTAR-FOO and tracked for the V* spec proper.
//
// Removing an extension is a breaking change for consumers; the
// pattern is promote-then-replace, never rename.
//
// Naming note: the function is ScopeOf, not Scope, because Go does
// not allow a type and a function to share an identifier in the
// same package; "Scope" reads naturally as the type and "ScopeOf"
// reads naturally as the classifier.
type Scope int

// Scope values. ScopeNone is the zero value so the type's default
// state is "not an extension".
const (
	// ScopeNone — name is not an X-* extension at all.
	ScopeNone Scope = iota
	// ScopeVStar — X-VSTAR-* cross-system V* extension on the
	// stabilization track. X-VSTAR-HASH (per spec/02) is the only
	// mandatory member in v0.1.
	ScopeVStar
	// ScopeSystem — X-<SYSTEM>-* extension owned by one consuming
	// system (e.g. X-AGR-INTENT). SYSTEM is any uppercase slug
	// other than VSTAR (reserved for ScopeVStar) and EXP (reserved
	// for ScopeExperimental).
	ScopeSystem
	// ScopeExperimental — X-EXP-* unstable extension. Senders MUST
	// NOT depend on receivers honoring these (spec/04).
	ScopeExperimental
	// ScopeUnknown — has the X- prefix but does not match any
	// sanctioned tier. Examples: "X-" (no slug), "X-VSTAR-" (no
	// name after the VSTAR prefix), "X-FOO" (no name after the
	// system slug). Treat as opaque; receivers MUST still ignore
	// per RFC 5545 compatibility rules.
	ScopeUnknown
)

// String returns a stable, human-readable name for the scope.
// Useful for error messages and debug logging.
func (s Scope) String() string {
	switch s {
	case ScopeNone:
		return "None"
	case ScopeVStar:
		return "VStar"
	case ScopeSystem:
		return "System"
	case ScopeExperimental:
		return "Experimental"
	case ScopeUnknown:
		return "Unknown"
	default:
		return "Scope(?)"
	}
}

// ScopeOf classifies name into one of the five Scope values per
// spec/04. The classification is case-insensitive: "X-VSTAR-HASH"
// and "x-vstar-hash" both return ScopeVStar.
//
// Decision tree:
//
//   - No X- prefix                 → ScopeNone
//   - X-VSTAR-<NAME>, NAME != ""   → ScopeVStar
//   - X-EXP-<NAME>,   NAME != ""   → ScopeExperimental
//   - X-<SYSTEM>-<NAME>            → ScopeSystem
//     (SYSTEM != VSTAR/EXP, both non-empty)
//   - anything else with X- prefix → ScopeUnknown
//
// To extract the system slug from a ScopeSystem name, use
// SystemName.
func ScopeOf(name string) Scope {
	if !IsExtension(name) {
		return ScopeNone
	}
	rest := name[2:] // strip "X-" / "x-"
	if rest == "" {
		return ScopeUnknown
	}
	slug, suffix, ok := cutSlug(rest)
	if !ok || suffix == "" {
		// "X-FOO" — slug present but no suffix; or no slug at all.
		return ScopeUnknown
	}
	switch strings.ToUpper(slug) {
	case "VSTAR":
		return ScopeVStar
	case "EXP":
		return ScopeExperimental
	default:
		return ScopeSystem
	}
}

// cutSlug splits "SLUG-REST" at the first hyphen and reports
// whether a hyphen was present. Returns ("", "", false) when there
// is no hyphen.
func cutSlug(s string) (slug, rest string, ok bool) {
	i := strings.IndexByte(s, '-')
	if i < 0 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

// SystemName extracts the system slug from an X-<SYSTEM>-<NAME>
// extension and reports ok=true. The slug is normalized to upper
// case so callers can compare directly without re-normalizing
// (e.g. SystemName("x-agr-intent") returns ("AGR", true)).
//
// Returns ("", false) for any name that is not in ScopeSystem:
// non-extensions, X-VSTAR-* (use spec/04 directly), X-EXP-* (no
// owner system), and malformed names lacking a <SYSTEM>-<NAME>
// structure.
//
// This is the cross-system introspection helper: "what system owns
// this property?". For the bare scope classification, use ScopeOf.
func SystemName(name string) (string, bool) {
	if !IsExtension(name) {
		return "", false
	}
	rest := name[2:]
	slug, suffix, ok := cutSlug(rest)
	if !ok || slug == "" || suffix == "" {
		return "", false
	}
	upper := strings.ToUpper(slug)
	if upper == "VSTAR" || upper == "EXP" {
		return "", false
	}
	return upper, true
}

// ExtensionsByScope returns every property on c whose Name
// classifies into the given scope. Order is the same as in c.Props
// (no sort). Returns nil — not an empty slice — when no property
// matches, so callers can use the result directly as a "no
// extensions" signal.
//
// Common uses:
//
//   - ExtensionsByScope(c, ScopeSystem)        — every X-<SYSTEM>-*
//     on the component (e.g. all X-AGR-*, X-CRM-* together). Pair
//     with SystemName to group by system.
//   - ExtensionsByScope(c, ScopeExperimental)  — every X-EXP-*; AGR
//     surfaces a warning when this is non-empty per spec/04.
//   - ExtensionsByScope(c, ScopeVStar)         — every X-VSTAR-*;
//     handy for hash-discipline checks.
//   - ExtensionsByScope(c, ScopeNone)          — every non-extension
//     property (UID, DTSTART, etc.); useful for diffing the V*
//     core surface.
//
// The function does not recurse into c.Sub; callers that want the
// full tree should walk sub-components themselves.
func ExtensionsByScope(c vstar.Component, scope Scope) []vstar.Property {
	var out []vstar.Property
	for _, p := range c.Props {
		if ScopeOf(p.Name) == scope {
			out = append(out, p)
		}
	}
	return out
}
