// SPDX-License-Identifier: MIT

package validate_test

import (
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/validate"
)

// commonProps wraps a typed component with UID / DTSTAMP so the
// type-specific tests below don't trip §1 noise.
func commonProps(uid string) []vstar.Property {
	return []vstar.Property{
		{Name: "UID", Value: uid},
		{Name: "DTSTAMP", Value: "20260504T120000Z"},
	}
}

func TestVTODO_DUE_OK(t *testing.T) {
	c := vstar.Component{
		Type:  vstar.CompTodo,
		Props: append(commonProps("todo-1"), vstar.Property{Name: "DUE", Value: "20260601T000000Z"}),
	}
	c = hashed(c)
	if hasCode(validate.ValidateComponent(c), validate.CodeVTODOMissingDue) {
		t.Errorf("VTODO with DUE should be clean")
	}
}

func TestVTODO_CompletedRoute_OK(t *testing.T) {
	c := vstar.Component{
		Type: vstar.CompTodo,
		Props: append(
			commonProps("todo-1"),
			vstar.Property{Name: "STATUS", Value: string(vstar.TodoCompleted)},
			vstar.Property{Name: "COMPLETED", Value: "20260601T120000Z"},
		),
	}
	c = hashed(c)
	if hasCode(validate.ValidateComponent(c), validate.CodeVTODOMissingDue) {
		t.Errorf("VTODO with STATUS=COMPLETED + COMPLETED should be clean")
	}
}

func TestVTODO_CompletedWithoutCOMPLETED_VS040(t *testing.T) {
	c := vstar.Component{
		Type: vstar.CompTodo,
		Props: append(
			commonProps("todo-1"),
			vstar.Property{Name: "STATUS", Value: string(vstar.TodoCompleted)},
		),
	}
	c = hashed(c)
	if !hasCode(validate.ValidateComponent(c), validate.CodeVTODOMissingDue) {
		t.Errorf("VTODO with STATUS=COMPLETED but no COMPLETED prop must yield VS040")
	}
}

func TestVTODO_NeedsActionMissingDUE_VS040(t *testing.T) {
	c := vstar.Component{
		Type:  vstar.CompTodo,
		Props: append(commonProps("todo-1"), vstar.Property{Name: "STATUS", Value: string(vstar.TodoNeedsAction)}),
	}
	c = hashed(c)
	d, ok := findCode(validate.ValidateComponent(c), validate.CodeVTODOMissingDue)
	if !ok {
		t.Fatalf("expected VS040 on VTODO with STATUS=NEEDS-ACTION and no DUE")
	}
	if d.Severity != validate.SeverityError {
		t.Errorf("VS040 must be Error; got %v", d.Severity)
	}
}

func TestVEVENT_DTSTART_OK(t *testing.T) {
	c := vstar.Component{
		Type:  vstar.CompEvent,
		Props: append(commonProps("evt-1"), vstar.Property{Name: "DTSTART", Value: "20260601T100000Z"}),
	}
	c = hashed(c)
	if hasCode(validate.ValidateComponent(c), validate.CodeVEVENTMissingDTSTART) {
		t.Errorf("VEVENT with DTSTART should be clean")
	}
}

func TestVEVENT_NoDTSTART_VS041(t *testing.T) {
	c := vstar.Component{
		Type:  vstar.CompEvent,
		Props: commonProps("evt-1"),
	}
	c = hashed(c)
	d, ok := findCode(validate.ValidateComponent(c), validate.CodeVEVENTMissingDTSTART)
	if !ok {
		t.Fatalf("expected VS041 on VEVENT without DTSTART")
	}
	if d.Severity != validate.SeverityError {
		t.Errorf("VS041 must be Error; got %v", d.Severity)
	}
	if d.Path != "VEVENT[uid=evt-1].DTSTART" {
		t.Errorf("expected path ...DTSTART; got %q", d.Path)
	}
}

func TestVFREEBUSY_BothPresent_OK(t *testing.T) {
	c := vstar.Component{
		Type: vstar.CompFreeBusy,
		Props: append(
			commonProps("fb-1"),
			vstar.Property{Name: "DTSTART", Value: "20260601T100000Z"},
			vstar.Property{Name: "DTEND", Value: "20260601T110000Z"},
		),
	}
	c = hashed(c)
	if hasCode(validate.ValidateComponent(c), validate.CodeVFREEBUSYMissingTimes) {
		t.Errorf("VFREEBUSY with DTSTART+DTEND should be clean")
	}
}

func TestVFREEBUSY_MissingDTEND_VS042(t *testing.T) {
	c := vstar.Component{
		Type:  vstar.CompFreeBusy,
		Props: append(commonProps("fb-1"), vstar.Property{Name: "DTSTART", Value: "20260601T100000Z"}),
	}
	c = hashed(c)
	d, ok := findCode(validate.ValidateComponent(c), validate.CodeVFREEBUSYMissingTimes)
	if !ok {
		t.Fatalf("expected VS042 on VFREEBUSY without DTEND")
	}
	if d.Severity != validate.SeverityError {
		t.Errorf("VS042 must be Error")
	}
}

func TestVFREEBUSY_MissingBoth_VS042(t *testing.T) {
	c := vstar.Component{
		Type:  vstar.CompFreeBusy,
		Props: commonProps("fb-1"),
	}
	c = hashed(c)
	if !hasCode(validate.ValidateComponent(c), validate.CodeVFREEBUSYMissingTimes) {
		t.Errorf("expected VS042 on VFREEBUSY missing both DTSTART and DTEND")
	}
}

func TestVCARDComponent_OK(t *testing.T) {
	c := vstar.Component{
		Type: "VCARD",
		Props: []vstar.Property{
			{Name: "UID", Value: "card-1"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "VERSION", Value: "4.0"},
		},
	}
	c = hashed(c)
	if hasCode(validate.ValidateComponent(c), validate.CodeVCARDMissingRequired) {
		t.Errorf("VCARD with VERSION+UID should be clean")
	}
}

func TestVCARDComponent_MissingVERSION_VS043(t *testing.T) {
	c := vstar.Component{
		Type: "VCARD",
		Props: []vstar.Property{
			{Name: "UID", Value: "card-1"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
		},
	}
	c = hashed(c)
	if !hasCode(validate.ValidateComponent(c), validate.CodeVCARDMissingRequired) {
		t.Errorf("expected VS043 on VCARD missing VERSION")
	}
}

func TestNonRulesType_NoTypeSpecificDiagnostic(t *testing.T) {
	// VJOURNAL has no extra MUST in spec/05 §5; it should yield
	// none of the type-specific codes when minimally complete.
	c := vstar.Component{
		Type:  vstar.CompJournal,
		Props: commonProps("j-1"),
	}
	c = hashed(c)
	got := validate.ValidateComponent(c)
	for _, code := range []string{
		validate.CodeVTODOMissingDue,
		validate.CodeVEVENTMissingDTSTART,
		validate.CodeVFREEBUSYMissingTimes,
		validate.CodeVCARDMissingRequired,
	} {
		if hasCode(got, code) {
			t.Errorf("VJOURNAL must not produce %s; got %v", code, got)
		}
	}
}
