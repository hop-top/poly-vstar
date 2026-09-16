// SPDX-License-Identifier: MIT

package vstar

import (
	"sort"
	"strings"
)

// Param is a single property parameter (e.g. CN=Jad on ATTENDEE).
//
// Parameter Name comparisons are case-insensitive per
// RFC 5545 §3.2 / RFC 6350 §5; Value comparisons are
// case-sensitive at this layer (the codec normalises where required).
type Param struct {
	Name  string
	Value string
}

// Property is a single iCalendar/vCard content line in struct form:
// a name, zero or more parameters, and a value. The exact wire format
// is the codec's responsibility — this layer is pure data.
type Property struct {
	Name   string
	Params []Param
	Value  string
}

// Equal reports whether two properties are semantically equal:
// case-insensitive Name + Param.Name comparisons, case-sensitive
// Value comparisons, and Param order normalised alphabetically by
// Param.Name before compare. Inputs are not mutated.
func Equal(a, b Property) bool {
	if !strings.EqualFold(a.Name, b.Name) {
		return false
	}
	if a.Value != b.Value {
		return false
	}
	if len(a.Params) != len(b.Params) {
		return false
	}
	ap := sortedParams(a.Params)
	bp := sortedParams(b.Params)
	for i := range ap {
		if !strings.EqualFold(ap[i].Name, bp[i].Name) {
			return false
		}
		if ap[i].Value != bp[i].Value {
			return false
		}
	}
	return true
}

// sortedParams returns a copy of params sorted alphabetically by
// upper-cased Name. The input slice is not mutated.
func sortedParams(params []Param) []Param {
	out := make([]Param, len(params))
	copy(out, params)
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToUpper(out[i].Name) < strings.ToUpper(out[j].Name)
	})
	return out
}
