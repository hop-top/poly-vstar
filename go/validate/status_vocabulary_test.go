// SPDX-License-Identifier: MIT

package validate_test

import (
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/validate"
)

// withStatus builds a minimally-valid component of type ct carrying
// STATUS=value, so the only interesting diagnostic is VS044.
func withStatus(ct vstar.CompType, value string) vstar.Component {
	props := append(commonProps("status-1"), vstar.Property{Name: "STATUS", Value: value})
	switch ct {
	case vstar.CompTodo:
		props = append(props, vstar.Property{Name: "DUE", Value: "20260601T000000Z"})
	case vstar.CompEvent:
		props = append(props, vstar.Property{Name: "DTSTART", Value: "20260601T000000Z"})
	}
	return hashed(vstar.Component{Type: ct, Props: props})
}

func TestStatusVocabulary_validValuesAreClean(t *testing.T) {
	cases := []struct {
		ct    vstar.CompType
		value string
	}{
		{vstar.CompEvent, string(vstar.EventTentative)},
		{vstar.CompEvent, string(vstar.EventConfirmed)},
		{vstar.CompEvent, string(vstar.EventCancelled)},
		{vstar.CompTodo, string(vstar.TodoNeedsAction)},
		{vstar.CompTodo, string(vstar.TodoInProcess)},
		{vstar.CompTodo, string(vstar.TodoCancelled)},
		{vstar.CompJournal, string(vstar.JournalDraft)},
		{vstar.CompJournal, string(vstar.JournalFinal)},
		{vstar.CompJournal, string(vstar.JournalCancelled)},
	}
	for _, tc := range cases {
		t.Run(string(tc.ct)+"/"+tc.value, func(t *testing.T) {
			c := withStatus(tc.ct, tc.value)
			if hasCode(validate.ValidateComponent(c), validate.CodeStatusNotInVocabulary) {
				t.Errorf("%s STATUS=%s must be clean", tc.ct, tc.value)
			}
		})
	}
}

// TestStatusVocabulary_crossTypeValueIsVS044 is the whole point:
// each value below is legal RFC 5545 STATUS text, but not for the
// component type carrying it.
func TestStatusVocabulary_crossTypeValueIsVS044(t *testing.T) {
	cases := []struct {
		ct    vstar.CompType
		value string
	}{
		{vstar.CompEvent, string(vstar.JournalDraft)},
		{vstar.CompEvent, string(vstar.TodoNeedsAction)},
		{vstar.CompTodo, string(vstar.EventTentative)},
		{vstar.CompTodo, string(vstar.JournalFinal)},
		{vstar.CompJournal, string(vstar.EventConfirmed)},
		{vstar.CompJournal, string(vstar.TodoInProcess)},
	}
	for _, tc := range cases {
		t.Run(string(tc.ct)+"/"+tc.value, func(t *testing.T) {
			c := withStatus(tc.ct, tc.value)
			d, ok := findCode(validate.ValidateComponent(c), validate.CodeStatusNotInVocabulary)
			if !ok {
				t.Fatalf("%s STATUS=%s must yield VS044", tc.ct, tc.value)
			}
			if d.Severity != validate.SeverityError {
				t.Errorf("VS044 severity = %v, want Error", d.Severity)
			}
		})
	}
}

func TestStatusVocabulary_unknownValueIsVS044(t *testing.T) {
	c := withStatus(vstar.CompEvent, "FROBNICATED")
	if !hasCode(validate.ValidateComponent(c), validate.CodeStatusNotInVocabulary) {
		t.Errorf("unknown STATUS value must yield VS044")
	}
}

// TestStatusVocabulary_caseInsensitive — RFC 5545 §3.1 property
// values for enumerated types are compared case-insensitively.
func TestStatusVocabulary_caseInsensitive(t *testing.T) {
	c := withStatus(vstar.CompEvent, "confirmed")
	if hasCode(validate.ValidateComponent(c), validate.CodeStatusNotInVocabulary) {
		t.Errorf("lowercase CONFIRMED on VEVENT must be accepted")
	}
}

// TestStatusVocabulary_absentStatusIsClean — STATUS is optional on
// every component type that admits it.
func TestStatusVocabulary_absentStatusIsClean(t *testing.T) {
	c := hashed(vstar.Component{
		Type:  vstar.CompEvent,
		Props: append(commonProps("no-status"), vstar.Property{Name: "DTSTART", Value: "20260601T000000Z"}),
	})
	if hasCode(validate.ValidateComponent(c), validate.CodeStatusNotInVocabulary) {
		t.Errorf("absent STATUS must be clean")
	}
}

// TestStatusVocabulary_untypedComponentIsSkipped — component types
// with no STATUS vocabulary in RFC 5545 (VFREEBUSY, VALARM, …) are
// not this rule's business.
func TestStatusVocabulary_untypedComponentIsSkipped(t *testing.T) {
	for _, ct := range []vstar.CompType{vstar.CompFreeBusy, vstar.CompAlarm, vstar.CompTimezone} {
		c := hashed(vstar.Component{
			Type: ct,
			Props: append(
				commonProps("x"),
				vstar.Property{Name: "STATUS", Value: "WHATEVER"},
				vstar.Property{Name: "DTSTART", Value: "20260601T000000Z"},
				vstar.Property{Name: "DTEND", Value: "20260601T010000Z"},
			),
		})
		if hasCode(validate.ValidateComponent(c), validate.CodeStatusNotInVocabulary) {
			t.Errorf("%s must be skipped by the STATUS vocabulary rule", ct)
		}
	}
}

// TestStatusVocabulary_matchesRegistry reconciles the table this
// package validates against with the registry's cross-language copy.
//
// The two are built from different sources on purpose.
// statusVocabularies is projected from the wire constants the codec
// encodes against, so it cannot disagree with what the library
// writes; validate.StatusVocabulary is rendered from
// spec/registry/status-vocabulary.json, so it cannot disagree with the
// other four ports. This test is the join: it is what lets both
// guarantees hold at once. Without it, generating the table would buy
// cross-language agreement by giving up the codec linkage, and
// deriving it would buy the codec linkage by giving up cross-language
// agreement.
//
// Order matters, not just membership: the VS044 message joins the
// allowed values, so a reordering is a user-visible change that the
// behavior fixtures pin.
func TestStatusVocabulary_matchesRegistry(t *testing.T) {
	// Projected from the wire constants, exactly as
	// statusVocabularies is — not read back out of the package, which
	// would make the assertion vacuous.
	fromConstants := map[string][]string{
		string(vstar.CompEvent): {
			string(vstar.EventTentative),
			string(vstar.EventConfirmed),
			string(vstar.EventCancelled),
		},
		string(vstar.CompTodo): {
			string(vstar.TodoNeedsAction),
			string(vstar.TodoInProcess),
			string(vstar.TodoCompleted),
			string(vstar.TodoCancelled),
		},
		string(vstar.CompJournal): {
			string(vstar.JournalDraft),
			string(vstar.JournalFinal),
			string(vstar.JournalCancelled),
		},
	}

	if len(validate.StatusVocabulary) != len(fromConstants) {
		t.Fatalf("registry scopes STATUS to %d component type(s), the wire constants to %d",
			len(validate.StatusVocabulary), len(fromConstants))
	}
	for ct, want := range fromConstants {
		got, ok := validate.StatusVocabulary[ct]
		if !ok {
			t.Errorf("registry has no STATUS vocabulary for %s", ct)
			continue
		}
		if len(got) != len(want) {
			t.Errorf("%s: registry has %v, wire constants have %v", ct, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s[%d]: registry has %q, wire constants have %q", ct, i, got[i], want[i])
			}
		}
	}
}

// TestClassTranspRelTypeVocabulary_matchesRegistry does for the flat
// vocabularies what TestStatusVocabulary_matchesRegistry does for
// STATUS: pins the generated cross-language table against the wire
// constants this module encodes against.
//
// CLASS and TRANSP are closed vocabularies. RELTYPE is not — RFC 5545
// §3.2.15 admits IANA and X- values — so the assertion is that the
// registry lists exactly the *registered* set the package names, not
// that no other value may appear on the wire.
func TestClassTranspRelTypeVocabulary_matchesRegistry(t *testing.T) {
	cases := []struct {
		name     string
		registry []string
		want     []string
	}{
		{
			name:     "CLASS",
			registry: validate.ClassVocabulary,
			want: []string{
				string(vstar.ClassPublic),
				string(vstar.ClassPrivate),
				string(vstar.ClassConfidential),
			},
		},
		{
			name:     "TRANSP",
			registry: validate.TranspVocabulary,
			want: []string{
				string(vstar.TranspOpaque),
				string(vstar.TranspTransparent),
			},
		},
		{
			name:     "RELTYPE",
			registry: validate.RelTypeVocabulary,
			want: []string{
				string(vstar.RelParent),
				string(vstar.RelChild),
				string(vstar.RelSibling),
				string(vstar.RelFinishToStart),
				string(vstar.RelFinishToFinish),
				string(vstar.RelStartToFinish),
				string(vstar.RelStartToStart),
				string(vstar.RelDependsOn),
				string(vstar.RelFirst),
				string(vstar.RelNext),
				string(vstar.RelConcept),
				string(vstar.RelRefID),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.registry) != len(tc.want) {
				t.Fatalf("registry has %v, wire constants have %v", tc.registry, tc.want)
			}
			for i := range tc.want {
				if tc.registry[i] != tc.want[i] {
					t.Errorf("[%d]: registry has %q, wire constants have %q",
						i, tc.registry[i], tc.want[i])
				}
			}
		})
	}
}
