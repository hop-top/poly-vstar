// SPDX-License-Identifier: MIT

package diff

import (
	"sort"
	"strings"

	vstar "hop.top/vstar"
)

// String renders a ComponentDiff as a unified-diff-ish text block:
//
//	--- <Path>
//	+ NAME[;PARAM=VAL...]:VALUE      // OpAdded
//	- NAME[;PARAM=VAL...]:VALUE      // OpRemoved
//	~ NAME: <oldValue> -> <newValue> // OpChanged
//
// Sub-component diffs are indented two spaces per level. Each
// sub-block opens with its own "--- <Path>" header.
//
// Header convention: "--- " (three dashes + space). The unified-diff
// tradition uses "---" for the original side and "+++" for the new
// side; here we collapse both into a single "---" line because each
// block represents the *changes between* the two sides, not one side
// in isolation. (Returns "" for an Empty diff.)
//
// Format stability: this output is informational, intended for
// humans and AGR's debug CLI. It is NOT a wire format and is not
// covered by V*'s cross-implementation parity guarantees.
func (d ComponentDiff) String() string {
	if d.Empty() {
		return ""
	}
	var sb strings.Builder
	d.writeTo(&sb, 0)
	return sb.String()
}

// writeTo renders d into sb at the given indent depth (in levels of
// two spaces per level).
func (d ComponentDiff) writeTo(sb *strings.Builder, depth int) {
	indent := strings.Repeat("  ", depth)
	sb.WriteString(indent)
	sb.WriteString("--- ")
	sb.WriteString(d.Path)
	sb.WriteByte('\n')
	for _, pd := range d.Properties {
		sb.WriteString(indent)
		sb.WriteString(renderPropertyDiff(pd))
		sb.WriteByte('\n')
	}
	for _, sd := range d.SubDiffs {
		if sd.Empty() {
			continue
		}
		sd.writeTo(sb, depth+1)
	}
}

// renderPropertyDiff returns the single-line representation of one
// PropertyDiff entry. No trailing newline.
func renderPropertyDiff(pd PropertyDiff) string {
	switch pd.Op {
	case OpAdded:
		return "+ " + renderProperty(pd.Property)
	case OpRemoved:
		return "- " + renderProperty(pd.Property)
	case OpChanged:
		return "~ " + pd.Property.Name + ": " + pd.Old.Value + " -> " + pd.Property.Value
	default:
		return "? " + renderProperty(pd.Property)
	}
}

// renderProperty produces a NAME[;PARAM=VAL...]:VALUE wire-style line
// (without folding or CRLF — String is for human display, not codec
// output). Parameters are sorted alphabetically by Name to keep
// output deterministic.
func renderProperty(p vstar.Property) string {
	var sb strings.Builder
	sb.WriteString(p.Name)
	if len(p.Params) > 0 {
		params := make([]vstar.Param, len(p.Params))
		copy(params, p.Params)
		sort.SliceStable(params, func(i, j int) bool {
			return strings.ToUpper(params[i].Name) < strings.ToUpper(params[j].Name)
		})
		for _, pr := range params {
			sb.WriteByte(';')
			sb.WriteString(pr.Name)
			sb.WriteByte('=')
			sb.WriteString(pr.Value)
		}
	}
	sb.WriteByte(':')
	sb.WriteString(p.Value)
	return sb.String()
}
