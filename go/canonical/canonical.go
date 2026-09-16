// SPDX-License-Identifier: MIT

// Package canonical produces the deterministic canonical byte form
// of V* objects per spec/03. The canonical form is the
// equivalence-class representative used as input to the SHA-256
// hash that backs X-VSTAR-HASH (see ../hash, vstar-go-hashing).
//
// Two V* documents containing the same logical content MUST
// produce identical canonical bytes — the foundational invariant
// for cross-implementation hash equality (Go ↔ TypeScript ↔
// AGR-Racket).
//
// Public API:
//
//   - Component(Component) []byte — single-component canonical bytes,
//     verbatim datetime emit (no TZID resolution).
//   - ComponentInContext(Component, Calendar) []byte — same as
//     Component but resolves TZID-tagged datetimes to UTC using the
//     supplied Calendar's VTIMEZONE registry (spec/03 §Datetime resolution).
//   - Calendar(Calendar) []byte — full VCALENDAR canonical bytes.
//   - Card(Card)        []byte — single VCARD canonical bytes.
//
// Canonicalization rules (each backed by a spec/03 rule):
//
//   - Properties sorted alphabetically by Name; parameters within
//     each property sorted alphabetically by Name (spec/03 rule 2).
//   - Property and parameter values normalized to Unicode NFC
//     (spec/03 rule 9).
//   - Datetime properties resolved to UTC form #2 (`Z`-suffixed)
//     when TZID parameter and matching VTIMEZONE are available; the
//     TZID parameter is then stripped (spec/03 rule 5).
//   - DATE-valued properties (VALUE=DATE) emitted verbatim in
//     `YYYYMMDD` form, VALUE=DATE retained and upper-cased, TZID
//     stripped (spec/03 rule 11).
//   - Top-level component sort within a Calendar by UID (or TZID
//     for VTIMEZONE) lexicographically (spec/03 rule 6).
//   - Nested sub-components (STANDARD/DAYLIGHT, VALARM) preserve
//     input append order — they have no canonical sort key.
//   - X-VSTAR-HASH stripped from the output (spec/03 rule 7).
//   - ATTACH properties: VALUE=BINARY/ENCODING=BASE64 stripped to
//     URI form (spec/03 rule 10).
//   - CRLF line endings, 75-octet folding via codec/rfc5545
//     (spec/03 rules 1 + 3).
//
// Package layout note: this package lives at go/canonical/ rather
// than at the package vstar root because it depends on
// codec/rfc5545 (which itself imports package vstar — a top-level
// canonical.X function would form an import cycle). Import as:
//
//	import "hop.top/vstar/canonical"
//	bytes := canonical.Component(myComponent)
package canonical

import (
	"bytes"
	"sort"
	"strings"

	"golang.org/x/text/unicode/norm"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/rfc5545"
)

// Component returns the canonical byte form of a single Component
// per spec/03 in the VERBATIM datetime mode.
//
// This form is intended for Components without TZID-tagged
// datetimes (DTSTAMP, DTSTART/DTEND/DUE/COMPLETED/RECURRENCE-ID/
// CREATED/LAST-MODIFIED in UTC form #2 only). When the Component
// contains a TZID-tagged datetime, Component cannot resolve the
// reference (the VTIMEZONE registry lives on Calendar, not on
// Component) and emits the value verbatim with the TZID parameter
// retained — output is therefore non-canonical for such
// Components. Use ComponentInContext to thread the parent Calendar
// and resolve TZIDs to UTC form #2 (spec/03 rule 5).
//
// Per V*'s design, producers SHOULD emit UTC form #2 (`Z`-suffixed)
// times whenever possible. Component preserves whatever the
// producer wrote.
//
// Component does not mutate the input. Internal transformations
// operate on copies.
func Component(c vstar.Component) []byte {
	return ComponentInContext(c, vstar.Calendar{})
}

// ComponentInContext returns the canonical byte form of a single
// Component per spec/03, resolving TZID-tagged datetimes against
// cal's VTIMEZONE registry.
//
// For each datetime property (DTSTAMP, DTSTART, DTEND, DUE,
// COMPLETED, RECURRENCE-ID, CREATED, LAST-MODIFIED) carrying a
// TZID parameter:
//
//   - Look up the matching VTIMEZONE in cal via
//     vstar.ParseTimeWithTZID. On success, re-emit the value as
//     vstar.FormatTime UTC form (`YYYYMMDDTHHMMSSZ`) and DROP the
//     TZID parameter from the canonical property.
//   - If TZID resolution fails (matching VTIMEZONE missing or
//     outside the v0.1 VTIMEZONE subset — see spec/03
//     §VTIMEZONE subset for TZID resolution), the
//     value AND the TZID parameter are emitted verbatim. Canonical
//     bytes for such Components are NOT deterministic across
//     calendars carrying different VTIMEZONE definitions; the
//     producer is expected to ship a self-contained calendar with
//     all referenced VTIMEZONE components present and inside the
//     v0.1 RRULE subset.
//
// Datetime values already in UTC form #2 (`Z`-suffixed) and values
// without a TZID parameter pass through verbatim — there is
// nothing to resolve.
//
// Nested sub-components (STANDARD/DAYLIGHT inside VTIMEZONE,
// VALARM inside VEVENT) are recursively prepared. STANDARD/
// DAYLIGHT children carry a wall-clock DTSTART that defines the
// transition rule itself; these are deliberately NOT TZID-tagged
// and are passed through verbatim by design.
//
// ComponentInContext does not mutate either input.
func ComponentInContext(c vstar.Component, cal vstar.Calendar) []byte {
	prepared := prepareComponent(c, cal)
	var buf bytes.Buffer
	if err := rfc5545.EncodeComponent(&buf, prepared); err != nil {
		// EncodeComponent writes to a bytes.Buffer; bytes.Buffer's
		// Write never returns an error, so this path is unreachable.
		return nil
	}
	return buf.Bytes()
}

// Calendar returns the canonical byte form of a full VCALENDAR
// per spec/03.
//
// Top-level components are sorted per spec/03 rule 6 (UID lexicographic;
// VTIMEZONE by TZID; key-less components last in stable input
// order). Each component is canonicalized via prepareComponent
// (which resolves TZID-tagged datetimes within child components
// against c's own VTIMEZONE registry — spec/03 rule 5)
// and then handed off to rfc5545.Encode for the VCALENDAR wrapper
// (VERSION:2.0 + PRODID line + components + END:VCALENDAR).
//
// The PRODID property value is NFC-normalized; the rfc5545 encoder
// owns RFC §3.3.11 TEXT escaping for PRODID and every other
// TEXT-typed value, so canonical does not pre-escape here.
func Calendar(c vstar.Calendar) []byte {
	sorted := sortedComponents(c.Components)
	prepared := make([]vstar.Component, len(sorted))
	for i, sub := range sorted {
		prepared[i] = prepareComponent(sub, c)
	}
	wire := vstar.Calendar{
		ProdID:     nfc(c.ProdID),
		Components: prepared,
	}
	var buf bytes.Buffer
	if err := rfc5545.Encode(&buf, wire); err != nil {
		// Encode writes to a bytes.Buffer; bytes.Buffer's Write
		// never returns an error, so this path is unreachable.
		return nil
	}
	return buf.Bytes()
}

// Card returns the canonical byte form of a single VCARD per
// spec/03.
//
// Output shape:
//
//	BEGIN:VCARD
//	VERSION:4.0
//	<sorted properties; UID is one of them>
//	END:VCARD
//
// Properties are sorted alphabetically by Name (case-insensitive
// comparison; emission upper-cases per RFC 6350). Card.UID is
// emitted as a UID property (or absorbed when Card.Props already
// carries one). Card.Kind is emitted as KIND if non-empty (or
// absorbed when Card.Props already carries KIND).
//
// Same canonicalization rules as Component: NFC values,
// X-VSTAR-HASH stripped, parameter ordering alphabetical, CRLF +
// 75-octet fold.
func Card(c vstar.Card) []byte {
	props := make([]vstar.Property, 0, len(c.Props)+3)
	props = append(props, vstar.Property{Name: "VERSION", Value: "4.0"})
	if c.UID != "" && !hasProp(c.Props, "UID") {
		props = append(props, vstar.Property{Name: "UID", Value: c.UID})
	}
	if c.Kind != "" && !hasProp(c.Props, "KIND") {
		props = append(props, vstar.Property{Name: "KIND", Value: string(c.Kind)})
	}
	props = append(props, c.Props...)

	synth := vstar.Component{
		// Wire-string CompType so EncodeComponent emits BEGIN:VCARD/
		// END:VCARD; CompType is a string alias.
		Type:  "VCARD",
		Props: props,
	}
	// VCARD has no datetime/TZID concerns — pass an empty Calendar.
	prepared := prepareComponent(synth, vstar.Calendar{})

	// VERSION must come immediately after BEGIN:VCARD per RFC 6350
	// §3.3, ahead of alphabetical order. Promote it.
	out := make([]vstar.Property, 0, len(prepared.Props))
	var version vstar.Property
	versionFound := false
	for _, p := range prepared.Props {
		if !versionFound && strings.EqualFold(p.Name, "VERSION") {
			version = p
			versionFound = true
			continue
		}
		out = append(out, p)
	}
	if versionFound {
		out = append([]vstar.Property{version}, out...)
	}
	prepared.Props = out

	var buf bytes.Buffer
	if err := rfc5545.EncodeComponent(&buf, prepared); err != nil {
		return nil
	}
	return buf.Bytes()
}

// prepareComponent returns a copy of c with all canonicalization
// transforms applied EXCEPT folding/encoding (which the caller does
// via rfc5545.EncodeComponent). Specifically:
//
//   - X-VSTAR-HASH stripped from Props.
//   - ATTACH properties: VALUE=BINARY and ENCODING=BASE64 params
//     stripped (spec/03 rule 10).
//   - Datetime properties with TZID parameter resolved against cal
//     and re-emitted as UTC form #2; the TZID parameter is dropped
//     on success (spec/03 §Datetime resolution). On resolution
//     failure the value and TZID parameter are preserved verbatim.
//   - Property values NFC-normalized.
//   - Parameter values NFC-normalized.
//   - Properties sorted alphabetically by upper-cased Name.
//   - Parameters within each Property sorted alphabetically by
//     upper-cased Name.
//   - Sub-components recursively prepared (no sort — append order
//     preserved per spec/03 rule 6 nested-component rule). VTIMEZONE
//     STANDARD/DAYLIGHT children carry wall-clock DTSTART that
//     defines the rule itself, NOT a TZID-tagged value, so their
//     DTSTART passes through untouched.
//
// prepareComponent does NOT mutate either input.
func prepareComponent(c vstar.Component, cal vstar.Calendar) vstar.Component {
	out := vstar.Component{Type: c.Type}

	out.Props = make([]vstar.Property, 0, len(c.Props))
	for _, p := range c.Props {
		if strings.EqualFold(p.Name, "X-VSTAR-HASH") {
			continue
		}
		out.Props = append(out.Props, prepareProperty(p, cal))
	}

	sort.SliceStable(out.Props, func(i, j int) bool {
		return strings.ToUpper(out.Props[i].Name) < strings.ToUpper(out.Props[j].Name)
	})

	if len(c.Sub) > 0 {
		out.Sub = make([]vstar.Component, len(c.Sub))
		for i, s := range c.Sub {
			out.Sub[i] = prepareComponent(s, cal)
		}
	}
	return out
}

// prepareProperty returns a copy of p with canonicalization
// transforms applied: NFC value, NFC + sorted params, ATTACH
// VALUE=BINARY/ENCODING=BASE64 params stripped per spec/03 rule 10,
// datetime TZID resolved against cal per spec/03 §Datetime resolution,
// DATE values emitted verbatim with VALUE=DATE retained per spec/03
// rule 11.
//
// RFC 5545 §3.3.11 / RFC 6350 §3.4 TEXT escaping is owned by the
// codec/rfc5545 encoder (the encoder uses its own internal allow-
// list of TEXT-typed property names). canonical does not
// pre-escape here — the encoder applies the single, authoritative
// escape pass on emit.
//
// For datetime properties (per the datetimeProperty allow-list)
// carrying a TZID parameter, the value is resolved via
// vstar.ParseTimeWithTZID(value, tzid, cal); on success the value
// is re-emitted as vstar.FormatTime UTC form #2 and the TZID
// parameter is dropped from the canonical Params slice. On failure
// (TZID resolution returns false — no matching VTIMEZONE in cal,
// or the VTIMEZONE is outside the v0.1 VTIMEZONE subset of spec/03
// §Datetime resolution), the value AND TZID parameter pass through
// verbatim. Datetime values without a TZID parameter, and values
// already in UTC form #2, pass through unchanged.
//
// For datetime-named properties declaring VALUE=DATE (spec/03 rule
// 11) the TZID branch is skipped entirely: an RFC 5545 §3.3.4 DATE
// has no time to convert and no zone to resolve. The eight-octet
// value passes through verbatim, the VALUE parameter's argument is
// upper-cased so `VALUE=date` and `VALUE=DATE` converge, and any
// TZID parameter is stripped — RFC 5545 §3.2.19 scopes TZID to
// DATE-TIME and TIME values, so on a DATE it is a producer bug that
// must not leak into the canonical bytes.
func prepareProperty(p vstar.Property, cal vstar.Calendar) vstar.Property {
	value := nfc(p.Value)

	// DATE canonicalization (spec/03 rule 11): a date-only value has
	// no time to convert and no zone to resolve, so it is emitted
	// verbatim. The VALUE=DATE parameter is RETAINED — unlike a
	// resolved TZID it is load-bearing, since the default value type
	// for these properties is DATE-TIME and an untagged eight-octet
	// value is a malformed DATE-TIME, not a DATE. Its argument is
	// upper-cased so case variants converge, and any TZID is stripped
	// (RFC 5545 §3.2.19 scopes TZID to DATE-TIME/TIME values).
	dateOnly := isDateTimeProperty(p.Name) && isValueDate(p.Params)

	// Datetime canonicalization (spec/03 §Datetime resolution): if
	// this is a datetime property carrying a TZID, try to resolve it.
	// On success rewrite the value to UTC form #2 and elide TZID from
	// the params list.
	stripTZID := dateOnly
	if !dateOnly && isDateTimeProperty(p.Name) {
		if tzid, ok := paramValue(p.Params, "TZID"); ok && tzid != "" {
			if t, ok := vstar.ParseTimeWithTZID(p.Value, tzid, cal); ok {
				value = vstar.FormatTime(t.UTC())
				stripTZID = true
			}
		}
	}

	out := vstar.Property{
		Name:  p.Name,
		Value: value,
	}
	if len(p.Params) == 0 {
		return out
	}
	out.Params = make([]vstar.Param, 0, len(p.Params))
	isAttach := strings.EqualFold(p.Name, "ATTACH")
	for _, prm := range p.Params {
		if isAttach {
			if strings.EqualFold(prm.Name, "VALUE") &&
				strings.EqualFold(prm.Value, "BINARY") {
				continue
			}
			if strings.EqualFold(prm.Name, "ENCODING") &&
				strings.EqualFold(prm.Value, "BASE64") {
				continue
			}
		}
		if stripTZID && strings.EqualFold(prm.Name, "TZID") {
			continue
		}
		pv := nfc(prm.Value)
		if dateOnly && strings.EqualFold(prm.Name, "VALUE") {
			pv = strings.ToUpper(pv)
		}
		out.Params = append(out.Params, vstar.Param{
			Name:  prm.Name,
			Value: pv,
		})
	}
	sort.SliceStable(out.Params, func(i, j int) bool {
		return strings.ToUpper(out.Params[i].Name) < strings.ToUpper(out.Params[j].Name)
	})
	return out
}

// paramValue returns the value of the first parameter whose name
// matches name (case-insensitive) along with an ok flag.
func paramValue(params []vstar.Param, name string) (string, bool) {
	for _, prm := range params {
		if strings.EqualFold(prm.Name, name) {
			return prm.Value, true
		}
	}
	return "", false
}

// datetimeProperties is the allow-list of property names whose
// values are RFC 5545 §3.3.5 DATE-TIME and may carry a TZID
// parameter that the canonical form should resolve to UTC
// (spec/03 §Datetime resolution). DTSTAMP is included even though
// RFC 5545 §3.8.7.2 requires it to be UTC-only — defensive
// resolution catches a non-conforming producer rather than emitting
// a TZID-tagged DTSTAMP unchanged.
var datetimeProperties = map[string]struct{}{
	"DTSTAMP":       {},
	"DTSTART":       {},
	"DTEND":         {},
	"DUE":           {},
	"COMPLETED":     {},
	"RECURRENCE-ID": {},
	"CREATED":       {},
	"LAST-MODIFIED": {},
}

// isDateTimeProperty reports whether name is a datetime-typed
// property eligible for TZID-to-UTC resolution.
func isDateTimeProperty(name string) bool {
	_, ok := datetimeProperties[strings.ToUpper(name)]
	return ok
}

// isValueDate reports whether params declares VALUE=DATE, i.e. that
// the property holds an RFC 5545 §3.3.4 calendar date rather than a
// DATE-TIME. Both the parameter name and its registered-token
// argument are compared case-insensitively per RFC 5545 §3.2 /
// §3.2.20.
func isValueDate(params []vstar.Param) bool {
	v, ok := paramValue(params, "VALUE")
	return ok && strings.EqualFold(v, "DATE")
}

// nfc returns the NFC-normalized form of s per spec/03 rule 9. A
// pass-through fast path avoids the allocation when s is already
// in NFC.
func nfc(s string) string {
	if norm.NFC.IsNormalString(s) {
		return s
	}
	return norm.NFC.String(s)
}

// hasProp reports whether props contains a property whose Name
// equals name (case-insensitive).
func hasProp(props []vstar.Property, name string) bool {
	for _, p := range props {
		if strings.EqualFold(p.Name, name) {
			return true
		}
	}
	return false
}

// sortedComponents returns a copy of components sorted per
// spec/03 rule 6:
//
//   - VTIMEZONE components sort by TZID property value.
//   - All other components sort by UID property value.
//   - Components with neither UID nor TZID sort last in stable
//     input order (a producer bug — sort is best-effort).
//   - Sort is stable; equal sort keys preserve relative input
//     order.
func sortedComponents(components []vstar.Component) []vstar.Component {
	out := make([]vstar.Component, len(components))
	copy(out, components)
	sort.SliceStable(out, func(i, j int) bool {
		ki, hasI := componentSortKey(out[i])
		kj, hasJ := componentSortKey(out[j])
		switch {
		case hasI && hasJ:
			return ki < kj
		case hasI && !hasJ:
			return true
		case !hasI && hasJ:
			return false
		default:
			return false
		}
	})
	return out
}

// componentSortKey returns the canonical sort key for a component
// (UID for most components; TZID for VTIMEZONE) along with an
// "ok" flag — false when the component has no usable key.
func componentSortKey(c vstar.Component) (string, bool) {
	if c.Type == vstar.CompTimezone {
		if p, ok := c.Get("TZID"); ok {
			return p.Value, true
		}
	}
	if uid := c.UID(); uid != "" {
		return uid, true
	}
	if p, ok := c.Get("TZID"); ok {
		return p.Value, true
	}
	return "", false
}
