// SPDX-License-Identifier: MIT

package vstar

import "testing"

func TestCompType_WireStrings(t *testing.T) {
	cases := []struct {
		got  CompType
		want string
	}{
		{CompCalendar, "VCALENDAR"},
		{CompTodo, "VTODO"},
		{CompJournal, "VJOURNAL"},
		{CompEvent, "VEVENT"},
		{CompFreeBusy, "VFREEBUSY"},
		{CompTimezone, "VTIMEZONE"},
		{CompAlarm, "VALARM"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if string(tc.got) != tc.want {
				t.Errorf("CompType = %q, want %q", string(tc.got), tc.want)
			}
		})
	}
}

func TestKind_WireStrings(t *testing.T) {
	cases := []struct {
		got  Kind
		want string
	}{
		{KindIndividual, "individual"},
		{KindOrg, "org"},
		{KindGroup, "group"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if string(tc.got) != tc.want {
				t.Errorf("Kind = %q, want %q", string(tc.got), tc.want)
			}
		})
	}
}

func TestTodoStatus_WireStrings(t *testing.T) {
	cases := []struct {
		got  TodoStatus
		want string
	}{
		{TodoNeedsAction, "NEEDS-ACTION"},
		{TodoInProcess, "IN-PROCESS"},
		{TodoCompleted, "COMPLETED"},
		{TodoCancelled, "CANCELLED"}, //nolint:misspell // RFC 5545 wire string.
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if string(tc.got) != tc.want {
				t.Errorf("TodoStatus = %q, want %q", string(tc.got), tc.want)
			}
		})
	}
}

func TestRelType_WireStrings(t *testing.T) {
	cases := []struct {
		got  RelType
		want string
	}{
		{RelParent, "PARENT"},
		{RelChild, "CHILD"},
		{RelSibling, "SIBLING"},
		{RelDependsOn, "DEPENDS-ON"},
		{RelFinishToStart, "FINISHTOSTART"},
		{RelFinishToFinish, "FINISHTOFINISH"},
		{RelStartToFinish, "STARTTOFINISH"},
		{RelStartToStart, "STARTTOSTART"},
		{RelFirst, "FIRST"},
		{RelNext, "NEXT"},
		{RelConcept, "CONCEPT"},
		{RelRefID, "REFID"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if string(tc.got) != tc.want {
				t.Errorf("RelType = %q, want %q", string(tc.got), tc.want)
			}
		})
	}
}

func TestDefaultRelType_IsParent(t *testing.T) {
	// RFC 5545 §3.2.15: an omitted RELTYPE means PARENT.
	if DefaultRelType != RelParent {
		t.Errorf("DefaultRelType = %q, want %q", DefaultRelType, RelParent)
	}
}

func TestParseRelType_CaseInsensitive(t *testing.T) {
	cases := []struct {
		in   string
		want RelType
		ok   bool
	}{
		{"PARENT", RelParent, true},
		{"parent", RelParent, true},
		{"Parent", RelParent, true},
		{"pArEnT", RelParent, true},
		{"CHILD", RelChild, true},
		{"child", RelChild, true},
		{"SIBLING", RelSibling, true},
		{"sibling", RelSibling, true},
		{"DEPENDS-ON", RelDependsOn, true},
		{"depends-on", RelDependsOn, true},
		{"FINISHTOSTART", RelFinishToStart, true},
		{"finishtostart", RelFinishToStart, true},
		{"FINISHTOFINISH", RelFinishToFinish, true},
		{"STARTTOFINISH", RelStartToFinish, true},
		{"STARTTOSTART", RelStartToStart, true},
		{"starttostart", RelStartToStart, true},
		{"FIRST", RelFirst, true},
		{"first", RelFirst, true},
		{"NEXT", RelNext, true},
		{"next", RelNext, true},
		{"CONCEPT", RelConcept, true},
		{"concept", RelConcept, true},
		{"REFID", RelRefID, true},
		{"refid", RelRefID, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := ParseRelType(tc.in)
			if ok != tc.ok {
				t.Fatalf("ParseRelType(%q) ok = %v, want %v", tc.in, ok, tc.ok)
			}
			if got != tc.want {
				t.Errorf("ParseRelType(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseRelType_EmptyDefaultsToParent(t *testing.T) {
	// RFC 5545 §3.2.15: absent RELTYPE means PARENT.
	got, ok := ParseRelType("")
	if !ok {
		t.Fatalf("ParseRelType(\"\") ok = false, want true")
	}
	if got != RelParent {
		t.Errorf("ParseRelType(\"\") = %q, want %q", got, RelParent)
	}
}

func TestParseRelType_UnknownPreservesValue(t *testing.T) {
	// Unregistered / X-prefixed values round-trip verbatim with ok=false.
	for _, in := range []string{"X-BLOCKS", "x-blocks", "NOPE"} {
		got, ok := ParseRelType(in)
		if ok {
			t.Errorf("ParseRelType(%q) ok = true, want false", in)
		}
		if string(got) != in {
			t.Errorf("ParseRelType(%q) = %q, want verbatim %q", in, string(got), in)
		}
	}
}

func TestRelType_EqualFold(t *testing.T) {
	if !RelParent.EqualFold("parent") {
		t.Error("RelParent.EqualFold(\"parent\") = false, want true")
	}
	if !RelDependsOn.EqualFold("depends-on") {
		t.Error("RelDependsOn.EqualFold(\"depends-on\") = false, want true")
	}
	if RelParent.EqualFold("CHILD") {
		t.Error("RelParent.EqualFold(\"CHILD\") = true, want false")
	}
}

func TestEventStatus_WireStrings(t *testing.T) {
	cases := []struct {
		got  EventStatus
		want string
	}{
		{EventTentative, "TENTATIVE"},
		{EventConfirmed, "CONFIRMED"},
		{EventCancelled, "CANCELLED"}, //nolint:misspell // RFC 5545 wire string.
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if string(tc.got) != tc.want {
				t.Errorf("EventStatus = %q, want %q", string(tc.got), tc.want)
			}
		})
	}
}

func TestJournalStatus_WireStrings(t *testing.T) {
	cases := []struct {
		got  JournalStatus
		want string
	}{
		{JournalDraft, "DRAFT"},
		{JournalFinal, "FINAL"},
		{JournalCancelled, "CANCELLED"}, //nolint:misspell // RFC 5545 wire string.
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if string(tc.got) != tc.want {
				t.Errorf("JournalStatus = %q, want %q", string(tc.got), tc.want)
			}
		})
	}
}

func TestClass_WireStrings(t *testing.T) {
	cases := []struct {
		got  Class
		want string
	}{
		{ClassPublic, "PUBLIC"},
		{ClassPrivate, "PRIVATE"},
		{ClassConfidential, "CONFIDENTIAL"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if string(tc.got) != tc.want {
				t.Errorf("Class = %q, want %q", string(tc.got), tc.want)
			}
		})
	}
}

func TestTransp_WireStrings(t *testing.T) {
	cases := []struct {
		got  Transp
		want string
	}{
		{TranspOpaque, "OPAQUE"},
		{TranspTransparent, "TRANSPARENT"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if string(tc.got) != tc.want {
				t.Errorf("Transp = %q, want %q", string(tc.got), tc.want)
			}
		})
	}
}

// TestStatusVocabularies_CancelledIsDistinctPerType pins the
// deliberate overlap: the cancellation value is legal wire text for
// VEVENT, VTODO and VJOURNAL alike, but each vocabulary owns its Go
// constant so a VEVENT status cannot be assigned to a VTODO field.

func TestStatusVocabularies_CancelledIsDistinctPerType(t *testing.T) {
	if string(EventCancelled) != string(TodoCancelled) {
		t.Errorf("EventCancelled and TodoCancelled must share the wire spelling")
	}
	if string(JournalCancelled) != string(TodoCancelled) {
		t.Errorf("JournalCancelled and TodoCancelled must share the wire spelling")
	}
}
