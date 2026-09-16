// SPDX-License-Identifier: MIT

package diff

import (
	"fmt"
	"sort"
	"strings"

	vstar "hop.top/vstar"
)

const (
	// xVstarHashPropertyName is the spec/03 sentinel header excluded
	// from diff input on both sides (matches canonical's exclusion).
	xVstarHashPropertyName = "X-VSTAR-HASH"
)

// OfComponent computes the property-level structural difference
// between two components and returns the resulting ComponentDiff.
//
// Both inputs are first stripped of any X-VSTAR-HASH property
// (matches canonical's exclusion per spec/03 rule 7) so the diff
// never reports hash drift as a content change.
//
// Properties are paired by Name (case-insensitive); the result is
// sorted by Property.Name. Sub-components are paired by
// (Type, UID) when both children carry a UID; sub-components without
// a UID are paired positionally by index. See package doc for the
// implications of positional matching.
//
// Naming note: the brief proposed `Component(a,b) ComponentDiff` but
// Go does not allow a function and a type to share an identifier in
// the same package — the symmetric Equal helper here is also named
// Component, and the data type returned by every diff entrypoint is
// ComponentDiff. The `Of`-prefix variant resolves the collision while
// preserving the type-safe per-shape API.
func OfComponent(a, b vstar.Component) ComponentDiff {
	return componentDiffAt("", a, b)
}

// OfCard computes the property-level structural difference between
// two cards. Cards have no Sub-components; the returned ComponentDiff
// always has empty SubDiffs. Path is "" so callers can use the
// returned value as a rendering root.
func OfCard(a, b vstar.Card) ComponentDiff {
	d := ComponentDiff{}
	d.Properties = diffProperties(filterHashProps(a.Props), filterHashProps(b.Props))
	return d
}

// OfCalendar computes the per-component structural difference
// between two calendars. Components are paired by (Type, UID) — same
// rule as ComponentDiff's Sub matching. Each pair that differs
// becomes one ComponentDiff with Path
// "VCALENDAR.<Type>[uid=<uid>]"; components only on one side become
// all-Added or all-Removed entries.
//
// Returned entries are sorted by Path (alphabetically), which keeps
// rendering deterministic across input orderings.
//
// PRODID is intentionally NOT diffed — the calendar identity (its
// component set) is what V* equality cares about. Use Calendar (the
// Equal variant) when PRODID matters.
//
// Components without a UID at the top level are paired positionally
// by Type (same heuristic as nested Sub). VTIMEZONE entries that
// carry no UID but a TZID are paired positionally; a later version
// may add
// TZID-keyed pairing.
func OfCalendar(a, b vstar.Calendar) []ComponentDiff {
	pairs := pairSubs(a.Components, b.Components)
	var out []ComponentDiff
	for _, pr := range pairs {
		path := subPath("VCALENDAR", pr.label)
		switch {
		case pr.a == nil && pr.b != nil:
			out = append(out, allAddedDiff(path, *pr.b))
		case pr.a != nil && pr.b == nil:
			out = append(out, allRemovedDiff(path, *pr.a))
		case pr.a != nil && pr.b != nil:
			cd := componentDiffAt(path, *pr.a, *pr.b)
			if !cd.Empty() {
				out = append(out, cd)
			}
		}
	}
	return out
}

// componentDiffAt is the recursive worker. path is the rendered
// dotted-path prefix for this level; "" for a top-level call.
func componentDiffAt(path string, a, b vstar.Component) ComponentDiff {
	d := ComponentDiff{Path: path}
	d.Properties = diffProperties(filterHashProps(a.Props), filterHashProps(b.Props))
	d.SubDiffs = diffSubs(path, a, b)
	return d
}

// filterHashProps returns a copy of props with any X-VSTAR-HASH
// property removed.
func filterHashProps(props []vstar.Property) []vstar.Property {
	out := make([]vstar.Property, 0, len(props))
	for _, p := range props {
		if strings.EqualFold(p.Name, xVstarHashPropertyName) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// diffProperties pairs props by Name (case-insensitive) and emits
// PropertyDiff entries sorted by Name.
func diffProperties(a, b []vstar.Property) []PropertyDiff {
	// Group properties by upper-cased Name to keep multi-valued
	// properties (e.g. multiple ATTENDEE) intact.
	groupBy := func(props []vstar.Property) map[string][]vstar.Property {
		m := make(map[string][]vstar.Property)
		for _, p := range props {
			key := strings.ToUpper(p.Name)
			m[key] = append(m[key], p)
		}
		return m
	}
	ga := groupBy(a)
	gb := groupBy(b)

	keys := make(map[string]struct{}, len(ga)+len(gb))
	for k := range ga {
		keys[k] = struct{}{}
	}
	for k := range gb {
		keys[k] = struct{}{}
	}
	sortedKeys := make([]string, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	var out []PropertyDiff
	for _, k := range sortedKeys {
		out = append(out, diffPropertyGroup(ga[k], gb[k])...)
	}
	return out
}

// diffPropertyGroup emits PropertyDiff entries for one property name
// across the two sides. Multiple instances are matched in order:
// the i-th from a pairs with the i-th from b. Surplus on either side
// becomes Added/Removed.
func diffPropertyGroup(a, b []vstar.Property) []PropertyDiff {
	var out []PropertyDiff
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		switch {
		case i >= len(a):
			out = append(out, PropertyDiff{Op: OpAdded, Property: b[i]})
		case i >= len(b):
			out = append(out, PropertyDiff{Op: OpRemoved, Property: a[i]})
		case !vstar.Equal(a[i], b[i]):
			out = append(out, PropertyDiff{Op: OpChanged, Property: b[i], Old: a[i]})
		}
	}
	return out
}

// diffSubs pairs sub-components and recurses, returning only
// non-empty sub-diffs.
func diffSubs(parentPath string, a, b vstar.Component) []ComponentDiff {
	pairs := pairSubs(a.Sub, b.Sub)
	var out []ComponentDiff
	for _, pr := range pairs {
		path := subPath(parentPath, pr.label)
		switch {
		case pr.a == nil && pr.b != nil:
			out = append(out, allAddedDiff(path, *pr.b))
		case pr.a != nil && pr.b == nil:
			out = append(out, allRemovedDiff(path, *pr.a))
		case pr.a != nil && pr.b != nil:
			cd := componentDiffAt(path, *pr.a, *pr.b)
			if !cd.Empty() {
				out = append(out, cd)
			}
		}
	}
	return out
}

// subPair carries a paired set of sub-components (either side may be
// nil for added/removed) plus the human-readable label segment used
// to render the diff path.
type subPair struct {
	a, b  *vstar.Component
	label string // "VALARM[uid=…]" or "VALARM[#0]"
}

// pairSubs matches sub-components by (Type, UID). Sub-components
// without a UID are paired positionally by index within the same
// Type bucket. The returned order is:
//
//  1. (Type, UID) pairs in canonical order (Type ASC, UID ASC).
//  2. UID-less buckets in Type order, position-paired entries first
//     (in input order), then surplus from a then b.
//  3. UID-only-on-b additions ordered by Type then UID.
//  4. UID-only-on-a removals ordered by Type then UID.
func pairSubs(aSub, bSub []vstar.Component) []subPair {
	type key struct{ typ, uid string }

	// Index UID-bearing children.
	aByKey := map[key]vstar.Component{}
	bByKey := map[key]vstar.Component{}

	// Index UID-less children by Type, preserving input order.
	aByType := map[string][]vstar.Component{}
	bByType := map[string][]vstar.Component{}

	collect := func(subs []vstar.Component, byKey map[key]vstar.Component, byType map[string][]vstar.Component) {
		for _, s := range subs {
			uid := s.UID()
			typ := string(s.Type)
			if uid == "" {
				byType[typ] = append(byType[typ], s)
				continue
			}
			byKey[key{typ: typ, uid: uid}] = s
		}
	}
	collect(aSub, aByKey, aByType)
	collect(bSub, bByKey, bByType)

	// Union of UID-bearing keys, sorted.
	keys := map[key]struct{}{}
	for k := range aByKey {
		keys[k] = struct{}{}
	}
	for k := range bByKey {
		keys[k] = struct{}{}
	}
	sortedKeys := make([]key, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Slice(sortedKeys, func(i, j int) bool {
		if sortedKeys[i].typ != sortedKeys[j].typ {
			return sortedKeys[i].typ < sortedKeys[j].typ
		}
		return sortedKeys[i].uid < sortedKeys[j].uid
	})

	var out []subPair
	for _, k := range sortedKeys {
		label := fmt.Sprintf("%s[uid=%s]", k.typ, k.uid)
		var aPtr, bPtr *vstar.Component
		if v, ok := aByKey[k]; ok {
			aPtr = ptrComponent(v)
		}
		if v, ok := bByKey[k]; ok {
			bPtr = ptrComponent(v)
		}
		out = append(out, subPair{a: aPtr, b: bPtr, label: label})
	}

	// UID-less children: union of types, sorted; pair positionally.
	typeSet := map[string]struct{}{}
	for t := range aByType {
		typeSet[t] = struct{}{}
	}
	for t := range bByType {
		typeSet[t] = struct{}{}
	}
	sortedTypes := make([]string, 0, len(typeSet))
	for t := range typeSet {
		sortedTypes = append(sortedTypes, t)
	}
	sort.Strings(sortedTypes)
	for _, t := range sortedTypes {
		as := aByType[t]
		bs := bByType[t]
		n := len(as)
		if len(bs) > n {
			n = len(bs)
		}
		for i := 0; i < n; i++ {
			label := fmt.Sprintf("%s[#%d]", t, i)
			var aPtr, bPtr *vstar.Component
			if i < len(as) {
				aPtr = ptrComponent(as[i])
			}
			if i < len(bs) {
				bPtr = ptrComponent(bs[i])
			}
			out = append(out, subPair{a: aPtr, b: bPtr, label: label})
		}
	}
	return out
}

// ptrComponent returns a pointer to a copy of c. Used so subPair
// can distinguish "no entry" (nil) from "zero-valued component"
// (non-nil pointer to zero value) — important when comparing zero
// components.
func ptrComponent(c vstar.Component) *vstar.Component {
	cp := c
	return &cp
}

// subPath joins a parent path with a sub-component label.
func subPath(parent, label string) string {
	if parent == "" {
		return label
	}
	return parent + "." + label
}

// allAddedDiff renders an entire component as Added: every property
// (except X-VSTAR-HASH) becomes a PropertyDiff{OpAdded}; recurses
// into Sub the same way.
func allAddedDiff(path string, c vstar.Component) ComponentDiff {
	d := ComponentDiff{Path: path}
	for _, p := range filterHashProps(c.Props) {
		d.Properties = append(d.Properties, PropertyDiff{Op: OpAdded, Property: p})
	}
	sortPropertyDiffs(d.Properties)
	for _, s := range c.Sub {
		label := subLabel(s)
		d.SubDiffs = append(d.SubDiffs, allAddedDiff(subPath(path, label), s))
	}
	return d
}

// allRemovedDiff is the symmetric counterpart of allAddedDiff.
func allRemovedDiff(path string, c vstar.Component) ComponentDiff {
	d := ComponentDiff{Path: path}
	for _, p := range filterHashProps(c.Props) {
		d.Properties = append(d.Properties, PropertyDiff{Op: OpRemoved, Property: p})
	}
	sortPropertyDiffs(d.Properties)
	for _, s := range c.Sub {
		label := subLabel(s)
		d.SubDiffs = append(d.SubDiffs, allRemovedDiff(subPath(path, label), s))
	}
	return d
}

// subLabel renders the label segment for an unpaired sub-component.
// Mirrors the format chosen in pairSubs for symmetry.
func subLabel(c vstar.Component) string {
	uid := c.UID()
	if uid == "" {
		return fmt.Sprintf("%s[#0]", c.Type)
	}
	return fmt.Sprintf("%s[uid=%s]", c.Type, uid)
}

// sortPropertyDiffs sorts in place by Property.Name (case-
// insensitive). diffProperties already emits sorted output for the
// pair-diff path, but allAddedDiff / allRemovedDiff need this.
func sortPropertyDiffs(pds []PropertyDiff) {
	sort.SliceStable(pds, func(i, j int) bool {
		return strings.ToUpper(pds[i].Property.Name) < strings.ToUpper(pds[j].Property.Name)
	})
}
