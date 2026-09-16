// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"path/filepath"
	"strings"

	vstar "hop.top/vstar"
	"hop.top/vstar/diff"
)

// componentDiffFixture is one entry of a <name>.diff.json file:
// the changes diff.OfCalendar reports for one paired component.
//
// UID is the identity a port keys on; it is lifted out of Path
// (which the reference renders as "VCALENDAR.VTODO[uid=x]") so the
// common case needs no path parsing. Path is carried too because a
// component with no UID is addressed positionally ("VALARM[#0]")
// and UID is then empty.
//
// Subs holds the same shape recursively for changed sub-components
// (a VALARM inside a VEVENT). Ops is always present, empty list and
// all, so an identical pair renders as `"ops": []` rather than
// null.
type componentDiffFixture struct {
	UID  string                 `json:"uid"`
	Path string                 `json:"path"`
	Ops  []opFixture            `json:"ops"`
	Subs []componentDiffFixture `json:"subs,omitempty"`
}

// opFixture is one property-level change.
//
// Before/After are nullable: an add has no before, a remove has no
// after. Params carry the property's parameters as an ordered list
// of name/value pairs — a parameter-only change (same value, new
// TZID) is a "change" op whose before and after values are equal
// and whose param lists differ.
//
// No message field: the reference's rendering of a diff is prose
// and not part of the contract.
type opFixture struct {
	Op           string         `json:"op"`
	Property     string         `json:"property"`
	Before       *string        `json:"before"`
	After        *string        `json:"after"`
	BeforeParams []paramFixture `json:"before_params,omitempty"`
	AfterParams  []paramFixture `json:"after_params,omitempty"`
}

// paramFixture is one property parameter.
type paramFixture struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// paramTZID and tzMontreal build the parameter-only change case.
const (
	paramTZID  = "TZID"
	tzMontreal = "America/Montreal"
)

// diffCase is one generated diff fixture: two calendars and the op
// kinds the pair is authored to produce.
type diffCase struct {
	name string
	ops  []string
	a    vstar.Calendar
	b    vstar.Calendar
}

// writeDiffFamily writes <name>.a.ics, <name>.b.ics and
// <name>.diff.json per case and returns the file count.
func writeDiffFamily(dir string) (int, error) {
	n := 0
	for _, tc := range diffCases() {
		got := diff.OfCalendar(tc.a, tc.b)
		fixtures := componentDiffFixtures(got)
		if err := assertDiffOps(tc, fixtures); err != nil {
			return 0, err
		}
		if err := writeCalendar(filepath.Join(dir, tc.name+".a.ics"), tc.a); err != nil {
			return 0, err
		}
		if err := writeCalendar(filepath.Join(dir, tc.name+".b.ics"), tc.b); err != nil {
			return 0, err
		}
		if err := writeJSON(filepath.Join(dir, tc.name+".diff.json"), fixtures); err != nil {
			return 0, err
		}
		n += 3
	}
	return n, nil
}

// componentDiffFixtures projects the reference's diff onto the
// fixture shape. No sorting happens here: diff.OfCalendar already
// emits components in pairing order and properties sorted by name
// (case-insensitive), and that ordering is the documented contract
// a port reproduces.
func componentDiffFixtures(ds []diff.ComponentDiff) []componentDiffFixture {
	out := make([]componentDiffFixture, 0, len(ds))
	for _, d := range ds {
		subs := nonEmpty(d.SubDiffs)
		var nested []componentDiffFixture
		if len(subs) > 0 {
			nested = componentDiffFixtures(subs)
		}
		out = append(out, componentDiffFixture{
			UID:  uidFromPath(d.Path),
			Path: d.Path,
			Ops:  opFixtures(d.Properties),
			Subs: nested,
		})
	}
	return out
}

// nonEmpty drops sub-diffs that record no change. diff.OfCalendar
// filters these at the top level but carries them nested; a port
// comparing against the fixture should see only real changes.
//
// Returns nil, not an empty slice, when nothing survives: `subs`
// carries omitempty, so an empty slice would be dropped from the
// JSON and come back nil on decode. Producing nil directly keeps
// the fixture round-trippable — encode, decode, compare.
func nonEmpty(ds []diff.ComponentDiff) []diff.ComponentDiff {
	var out []diff.ComponentDiff
	for _, d := range ds {
		if !d.Empty() {
			out = append(out, d)
		}
	}
	return out
}

func opFixtures(pds []diff.PropertyDiff) []opFixture {
	out := make([]opFixture, 0, len(pds))
	for _, pd := range pds {
		f := opFixture{
			Op:       opToken(pd.Op),
			Property: propertyName(pd),
		}
		switch pd.Op {
		case diff.OpAdded:
			f.After = strPtr(pd.Property.Value)
			f.AfterParams = paramFixtures(pd.Property.Params)
		case diff.OpRemoved:
			f.Before = strPtr(pd.Property.Value)
			f.BeforeParams = paramFixtures(pd.Property.Params)
		case diff.OpChanged:
			f.Before = strPtr(pd.Old.Value)
			f.After = strPtr(pd.Property.Value)
			f.BeforeParams = paramFixtures(pd.Old.Params)
			f.AfterParams = paramFixtures(pd.Property.Params)
		}
		out = append(out, f)
	}
	return out
}

// propertyName reports the name the op is about. For a removal the
// b-side property is zero, so the name lives on Property for adds
// and changes and on Property for removals too (the reference puts
// the surviving side there in every case).
func propertyName(pd diff.PropertyDiff) string {
	if pd.Property.Name != "" {
		return pd.Property.Name
	}
	return pd.Old.Name
}

// The op tokens a <name>.diff.json records. The reference's
// DiffOp.String() is capitalized for human display; the fixtures use
// these lowercase forms so ports need no case convention of their
// own.
const (
	opAdd     = "add"
	opRemove  = "remove"
	opChange  = "change"
	opUnknown = "unknown"
)

// opToken renders a DiffOp as its lowercase fixture token.
func opToken(op diff.DiffOp) string {
	switch op {
	case diff.OpAdded:
		return opAdd
	case diff.OpRemoved:
		return opRemove
	case diff.OpChanged:
		return opChange
	default:
		return opUnknown
	}
}

func paramFixtures(params []vstar.Param) []paramFixture {
	if len(params) == 0 {
		return nil
	}
	out := make([]paramFixture, 0, len(params))
	for _, p := range params {
		out = append(out, paramFixture{Name: p.Name, Value: p.Value})
	}
	return out
}

func strPtr(s string) *string { return &s }

// uidFromPath lifts the uid out of a rendered diff path such as
// "VCALENDAR.VTODO[uid=todo-1]". Returns "" for a positional path
// ("VCALENDAR.VALARM[#0]") — the component has no UID to key on.
func uidFromPath(path string) string {
	const marker = "[uid="
	i := strings.LastIndex(path, marker)
	if i < 0 || !strings.HasSuffix(path, "]") {
		return ""
	}
	return path[i+len(marker) : len(path)-1]
}

// assertDiffOps verifies the case produced the op kinds it was
// authored for, so a refactor cannot quietly hollow out a fixture.
// An empty want list asserts the whole diff is empty.
func assertDiffOps(tc diffCase, fixtures []componentDiffFixture) error {
	seen := map[string]bool{}
	var walk func([]componentDiffFixture)
	walk = func(ds []componentDiffFixture) {
		for _, d := range ds {
			for _, o := range d.Ops {
				seen[o.Op] = true
			}
			walk(d.Subs)
		}
	}
	walk(fixtures)
	if len(tc.ops) == 0 {
		if len(fixtures) != 0 {
			return fmt.Errorf("diff case %q: expected no changes, got %d component diff(s)", tc.name, len(fixtures))
		}
		return nil
	}
	for _, want := range tc.ops {
		if !seen[want] {
			return fmt.Errorf("diff case %q: expected a %q op, got %v", tc.name, want, seen)
		}
	}
	return nil
}

// diffCases enumerates the generated pairs. Each isolates one
// change kind so a failing port sees exactly one thing wrong.
func diffCases() []diffCase {
	base := func(uid string, props ...vstar.Property) vstar.Component {
		return hashed(withProps(
			bare(vstar.CompTodo),
			append([]vstar.Property{
				prop("UID", uid),
				prop("DTSTAMP", stampTime),
				prop("SUMMARY", "Ship the fixtures"),
			}, props...)...,
		))
	}
	return []diffCase{
		{
			name: "property_added",
			ops:  []string{opAdd},
			a:    oneComponent("Diff-PropertyAdded-A", base("todo-add")),
			b: oneComponent("Diff-PropertyAdded-B",
				base("todo-add", prop("DUE", "20260101T000000Z"))),
		},
		{
			name: "property_removed",
			ops:  []string{opRemove},
			a: oneComponent("Diff-PropertyRemoved-A",
				base("todo-remove", prop("DUE", "20260101T000000Z"))),
			b: oneComponent("Diff-PropertyRemoved-B", base("todo-remove")),
		},
		{
			name: "value_changed",
			ops:  []string{opChange},
			a: oneComponent("Diff-ValueChanged-A",
				base("todo-change", prop("DUE", "20260101T000000Z"))),
			b: oneComponent("Diff-ValueChanged-B",
				base("todo-change", prop("DUE", "20260202T000000Z"))),
		},
		{
			name: "param_changed",
			ops:  []string{opChange},
			// Same value, different parameter: the change is
			// entirely in the params, which is why the fixture
			// format carries them separately from the value.
			a: oneComponent("Diff-ParamChanged-A",
				base("todo-param", vstar.Property{
					Name:   "DUE",
					Params: []vstar.Param{{Name: paramTZID, Value: tzMontreal}},
					Value:  "20260101T000000",
				})),
			b: oneComponent("Diff-ParamChanged-B",
				base("todo-param", vstar.Property{
					Name:   "DUE",
					Params: []vstar.Param{{Name: paramTZID, Value: "Europe/Paris"}},
					Value:  "20260101T000000",
				})),
		},
		{
			name: "component_added",
			ops:  []string{opAdd},
			a:    oneComponent("Diff-ComponentAdded-A", base("todo-keep")),
			b: twoComponents("Diff-ComponentAdded-B",
				base("todo-keep"),
				base("todo-new", prop("DUE", "20260301T000000Z"))),
		},
		{
			name: "component_removed",
			ops:  []string{opRemove},
			a: twoComponents("Diff-ComponentRemoved-A",
				base("todo-keep"),
				base("todo-gone", prop("DUE", "20260301T000000Z"))),
			b: oneComponent("Diff-ComponentRemoved-B", base("todo-keep")),
		},
		{
			name: "identical",
			ops:  nil,
			a:    oneComponent("Diff-Identical-A", base("todo-same", prop("DUE", "20260101T000000Z"))),
			b:    oneComponent("Diff-Identical-B", base("todo-same", prop("DUE", "20260101T000000Z"))),
		},
	}
}

// twoComponents wraps a pair of components in one calendar.
func twoComponents(slug string, a, b vstar.Component) vstar.Calendar {
	cal := oneComponent(slug, a)
	cal.Append(b)
	return cal
}
