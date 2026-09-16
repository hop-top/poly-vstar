// SPDX-License-Identifier: MIT

package canonical_test

import (
	"bytes"
	"testing"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/canonical"
	"hop.top/vstar/hashing"
)

// dateTodo builds a VTODO whose DUE is date-only, with params in the
// supplied order so tests can vary input shape.
func dateTodo(params []vstar.Param, value string) vstar.Component {
	return vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "UID", Value: "todo-1"},
			{Name: "DTSTAMP", Value: "20260504T183045Z"},
			{Name: "DUE", Params: params, Value: value},
		},
	}
}

// TestCanonical_DateValuePreserved verifies a DATE value survives
// canonicalization verbatim and keeps its VALUE=DATE parameter. The
// parameter is load-bearing: strip it and the 8-octet value becomes a
// malformed DATE-TIME, so canonical MUST NOT elide it the way it
// elides a resolved TZID.
func TestCanonical_DateValuePreserved(t *testing.T) {
	c := dateTodo([]vstar.Param{{Name: "VALUE", Value: "DATE"}}, "20260515")
	got := canonical.Component(c)
	if !bytes.Contains(got, []byte("DUE;VALUE=DATE:20260515\r\n")) {
		t.Errorf("canonical bytes missing DUE;VALUE=DATE:20260515\ngot:\n%s", got)
	}
}

// TestCanonical_DateValueCaseNormalized verifies the VALUE=DATE
// parameter canonicalizes to upper case. Two producers writing
// `VALUE=DATE` and `value=date` express the same logical content and
// MUST hash identically (spec/03 principle: hash equality follows
// logical equality).
func TestCanonical_DateValueCaseNormalized(t *testing.T) {
	upper := dateTodo([]vstar.Param{{Name: "VALUE", Value: "DATE"}}, "20260515")
	lower := dateTodo([]vstar.Param{{Name: "value", Value: "date"}}, "20260515")
	if a, b := canonical.Component(upper), canonical.Component(lower); !bytes.Equal(a, b) {
		t.Errorf("case variants canonicalize differently:\n%s\nvs\n%s", a, b)
	}
	if a, b := hashing.Component(upper), hashing.Component(lower); a != b {
		t.Errorf("case variants hash differently: %s vs %s", a, b)
	}
}

// TestCanonical_DateStripsTZID verifies a TZID parameter on a DATE
// value is dropped. RFC 5545 §3.2.19 scopes TZID to DATE-TIME and
// TIME values; on a DATE it is meaningless, and leaving it in would
// make canonical bytes depend on a producer bug.
func TestCanonical_DateStripsTZID(t *testing.T) {
	withTZ := dateTodo([]vstar.Param{
		{Name: "VALUE", Value: "DATE"},
		{Name: "TZID", Value: "America/Montreal"},
	}, "20260515")
	without := dateTodo([]vstar.Param{{Name: "VALUE", Value: "DATE"}}, "20260515")

	got := canonical.Component(withTZ)
	if bytes.Contains(got, []byte("TZID")) {
		t.Errorf("TZID survived canonicalization of a DATE value:\n%s", got)
	}
	if want := canonical.Component(without); !bytes.Equal(got, want) {
		t.Errorf("TZID-bearing DATE canonicalizes differently:\n%s\nvs\n%s", got, want)
	}
}

// TestCanonical_DateNotConvertedToUTC verifies canonical does NOT
// promote a DATE to a midnight DATE-TIME. The two are semantically
// distinct and must not collapse into the same bytes.
func TestCanonical_DateNotConvertedToUTC(t *testing.T) {
	dateOnly := dateTodo([]vstar.Param{{Name: "VALUE", Value: "DATE"}}, "20260515")
	midnight := dateTodo(nil, "20260515T000000Z")

	a := canonical.Component(dateOnly)
	b := canonical.Component(midnight)
	if bytes.Equal(a, b) {
		t.Errorf("DATE and midnight DATE-TIME canonicalize identically:\n%s", a)
	}
	if hashing.Component(dateOnly) == hashing.Component(midnight) {
		t.Error("DATE and midnight DATE-TIME hash identically")
	}
}

// TestCanonical_DateDeterministic verifies repeated canonicalization
// and hashing of the same logical date is byte-stable, including
// across differing input parameter order.
func TestCanonical_DateDeterministic(t *testing.T) {
	a := dateTodo([]vstar.Param{
		{Name: "VALUE", Value: "DATE"},
		{Name: "X-Z", Value: "1"},
	}, "20260515")
	b := dateTodo([]vstar.Param{
		{Name: "X-Z", Value: "1"},
		{Name: "VALUE", Value: "DATE"},
	}, "20260515")

	first := canonical.Component(a)
	for i := range 8 {
		if got := canonical.Component(a); !bytes.Equal(got, first) {
			t.Fatalf("iteration %d differs:\n%s\nvs\n%s", i, got, first)
		}
	}
	if got := canonical.Component(b); !bytes.Equal(got, first) {
		t.Errorf("param order affects canonical bytes:\n%s\nvs\n%s", got, first)
	}
}

// TestCanonical_DateInCalendarContext verifies a DATE value inside a
// full VCALENDAR — where TZID resolution machinery is live — is still
// left alone. There is no time to convert and no zone to resolve.
func TestCanonical_DateInCalendarContext(t *testing.T) {
	cal := vstar.Calendar{
		ProdID:     "-//test//EN",
		Components: []vstar.Component{dateTodo([]vstar.Param{{Name: "VALUE", Value: "DATE"}}, "20260515")},
	}
	got := canonical.Calendar(cal)
	if !bytes.Contains(got, []byte("DUE;VALUE=DATE:20260515\r\n")) {
		t.Errorf("calendar canonical bytes missing the DATE property:\n%s", got)
	}
}

// TestCanonical_DateRoundTripsThroughAccessors verifies a component
// built via the typed setters canonicalizes to the same bytes as the
// hand-written wire shape — the two paths must not diverge.
func TestCanonical_DateRoundTripsThroughAccessors(t *testing.T) {
	built := vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "UID", Value: "todo-1"},
			{Name: "DTSTAMP", Value: "20260504T183045Z"},
		},
	}
	built.SetDUEDate(vstar.Date{Year: 2026, Month: time.May, Day: 15})

	want := canonical.Component(dateTodo([]vstar.Param{{Name: "VALUE", Value: "DATE"}}, "20260515"))
	if got := canonical.Component(built); !bytes.Equal(got, want) {
		t.Errorf("setter-built component canonicalizes differently:\n%s\nvs\n%s", got, want)
	}
}
