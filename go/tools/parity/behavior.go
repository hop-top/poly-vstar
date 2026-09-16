// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/rfc5545"
	"hop.top/vstar/diff"
	"hop.top/vstar/supersession"
	"hop.top/vstar/validate"
)

// The six behavior families read their inputs from spec/behavior/
// (and, for supersession and time, from the conformance corpus the
// behavior fixtures reuse). Keys are the input's path relative to
// spec/behavior without extension, so a port keys its own table by
// the same file names it loaded.

// diagnosticEntry is one validate.Diagnostic's stable surface.
// Message is deliberately absent: it is prose that rewords between
// versions without the behavior changing.
type diagnosticEntry struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Path     string `json:"path"`
}

// emitValidate runs validate.Validate over every <name>.ics under
// spec/behavior/validate and emits the diagnostics it raises,
// sorted by (path, code).
//
// The sort is the contract, not the reference's emission order:
// checks run in an order that is an implementation detail, and a
// port orders its own checks differently. Sorting both sides makes
// the comparison about which diagnostics were raised.
func emitValidate(dir string) (map[string]any, error) {
	root := filepath.Dir(dir)
	out := map[string]any{}
	err := eachFixture(dir, extICS, func(path, stem string) error {
		cal, err := readCalendar(path)
		if err != nil {
			return err
		}
		return record(out, root, stem, diagnosticEntries(validate.Validate(cal)))
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// diagnosticEntries projects diagnostics onto the emitted shape.
// Always a non-nil slice: a clean document emits [] rather than
// null, so every port's decoder handles one shape.
func diagnosticEntries(ds []validate.Diagnostic) []diagnosticEntry {
	out := make([]diagnosticEntry, 0, len(ds))
	for _, d := range ds {
		out = append(out, diagnosticEntry{
			Code:     d.Code,
			Severity: d.Severity.String(),
			Path:     d.Path,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Code < out[j].Code
	})
	return out
}

// componentDiffEntry is the change set for one paired component.
// UID is lifted out of Path so the common case needs no path
// parsing, and is empty for a component addressed positionally.
type componentDiffEntry struct {
	UID  string               `json:"uid"`
	Path string               `json:"path"`
	Ops  []opEntry            `json:"ops"`
	Subs []componentDiffEntry `json:"subs"`
}

// opEntry is one property-level change. Before is null on an add,
// After null on a remove. Params travel apart from the value
// because a parameter-only change is a "change" op whose values are
// equal and whose parameter lists differ.
type opEntry struct {
	Op           string       `json:"op"`
	Property     string       `json:"property"`
	Before       *string      `json:"before"`
	After        *string      `json:"after"`
	BeforeParams []paramEntry `json:"before_params"`
	AfterParams  []paramEntry `json:"after_params"`
}

// paramEntry is one property parameter, in the order the property
// carries it.
type paramEntry struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// emitDiff runs diff.OfCalendar over every <name>.a.ics /
// <name>.b.ics pair under spec/behavior/diff. The key is the stem
// with the side suffix dropped: "diff/value_changed".
//
// No sorting happens here. diff.OfCalendar emits components in
// pairing order and properties sorted by name case-insensitively,
// and that ordering is the contract a port reproduces — sorting it
// again would hide an ordering divergence rather than catch it.
func emitDiff(dir string) (map[string]any, error) {
	root := filepath.Dir(dir)
	out := map[string]any{}
	err := eachFixture(dir, ".a"+extICS, func(path, stem string) error {
		a, err := readCalendar(path)
		if err != nil {
			return err
		}
		b, err := readCalendar(stem + ".b" + extICS)
		if err != nil {
			return err
		}
		return record(out, root, stem, componentDiffEntries(diff.OfCalendar(a, b)))
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func componentDiffEntries(ds []diff.ComponentDiff) []componentDiffEntry {
	out := make([]componentDiffEntry, 0, len(ds))
	for _, d := range ds {
		out = append(out, componentDiffEntry{
			UID:  uidFromPath(d.Path),
			Path: d.Path,
			Ops:  opEntries(d.Properties),
			Subs: componentDiffEntries(changedOnly(d.SubDiffs)),
		})
	}
	return out
}

// changedOnly drops sub-diffs that record no change. OfCalendar
// filters these at the top level but carries them nested; the
// document reports only real changes so a port need not reproduce
// the reference's internal bookkeeping.
func changedOnly(ds []diff.ComponentDiff) []diff.ComponentDiff {
	out := make([]diff.ComponentDiff, 0, len(ds))
	for _, d := range ds {
		if !d.Empty() {
			out = append(out, d)
		}
	}
	return out
}

func opEntries(pds []diff.PropertyDiff) []opEntry {
	out := make([]opEntry, 0, len(pds))
	for _, pd := range pds {
		e := opEntry{
			Op:           opToken(pd.Op),
			Property:     propertyName(pd),
			BeforeParams: []paramEntry{},
			AfterParams:  []paramEntry{},
		}
		switch pd.Op {
		case diff.OpAdded:
			e.After = strPtr(pd.Property.Value)
			e.AfterParams = paramEntries(pd.Property.Params)
		case diff.OpRemoved:
			e.Before = strPtr(pd.Property.Value)
			e.BeforeParams = paramEntries(pd.Property.Params)
		case diff.OpChanged:
			e.Before = strPtr(pd.Old.Value)
			e.After = strPtr(pd.Property.Value)
			e.BeforeParams = paramEntries(pd.Old.Params)
			e.AfterParams = paramEntries(pd.Property.Params)
		}
		out = append(out, e)
	}
	return out
}

// The lowercase op tokens the document records. DiffOp.String() is
// capitalized for human display; one flat convention here spares
// every port a case mapping.
const (
	opAdd     = "add"
	opRemove  = "remove"
	opChange  = "change"
	opUnknown = "unknown"
)

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

// propertyName reports the name the op is about, taking it from
// whichever side of the pair carries it.
func propertyName(pd diff.PropertyDiff) string {
	if pd.Property.Name != "" {
		return pd.Property.Name
	}
	return pd.Old.Name
}

func paramEntries(params []vstar.Param) []paramEntry {
	out := make([]paramEntry, 0, len(params))
	for _, p := range params {
		out = append(out, paramEntry{Name: p.Name, Value: p.Value})
	}
	return out
}

// uidFromPath lifts the uid out of a rendered path such as
// "VCALENDAR.VTODO[uid=todo-1]". Empty for a positional path
// ("VCALENDAR.VALARM[#0]"), which has no UID to key on.
func uidFromPath(path string) string {
	const marker = "[uid="
	i := strings.LastIndex(path, marker)
	if i < 0 || !strings.HasSuffix(path, "]") {
		return ""
	}
	return path[i+len(marker) : len(path)-1]
}

// emitSupersession projects the effective status each supersession
// ledger imposes, keyed by component UID.
//
// The inputs are the conformance corpus' supersession fixtures; the
// behavior tree holds only the <name>.effective.json sidecars, so
// the sidecar names which .ics to load. A UID absent from the map
// is not superseded — supersession is a projection query, not a
// validator.
func emitSupersession(conformance, dir string) (map[string]any, error) {
	root := filepath.Dir(dir)
	inputs := filepath.Join(conformance, "supersession")
	out := map[string]any{}
	err := eachFixture(dir, ".effective.json", func(_, stem string) error {
		name := filepath.Base(stem)
		cal, err := readCalendar(filepath.Join(inputs, name+".ics"))
		if err != nil {
			return err
		}
		effective := map[string]string{}
		for _, c := range cal.Components {
			status, ok := supersession.Superseded(c, cal.Components)
			if !ok {
				continue
			}
			uid := c.UID()
			if uid == "" {
				return fmt.Errorf("%s: superseded component has no UID", name)
			}
			effective[uid] = status
		}
		return record(out, root, stem, effective)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// readCalendar parses an .ics file into a Calendar.
func readCalendar(path string) (vstar.Calendar, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return vstar.Calendar{}, fmt.Errorf("read %s: %w", path, err)
	}
	cal, err := rfc5545.Parse(bytes.NewReader(raw))
	if err != nil {
		return vstar.Calendar{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return cal, nil
}

func strPtr(s string) *string { return &s }
