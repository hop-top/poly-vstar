// SPDX-License-Identifier: MIT

package supersession_test

import (
	"errors"
	"testing"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/hashing"
	"hop.top/vstar/supersession"
)

// --- T1: constants ----------------------------------------------------------

func TestConstants_MatchSpec02(t *testing.T) {
	t.Parallel()

	if got, want := supersession.CategoryStatusSupersession, "status-supersession"; got != want {
		t.Errorf("CategoryStatusSupersession = %q, want %q (spec/02 example line)", got, want)
	}
	if got, want := supersession.PropEffectiveStatus, "X-VSTAR-EFFECTIVE-STATUS"; got != want {
		t.Errorf("PropEffectiveStatus = %q, want %q (spec/02 example line)", got, want)
	}
}

// --- T2: Supersedes ---------------------------------------------------------

// makeTarget builds a hashed VTODO for use as a Supersedes target.
func makeTarget(t *testing.T, uid string) vstar.Component {
	t.Helper()
	c := vstar.Component{Type: vstar.CompTodo}
	c.Set(vstar.Property{Name: "UID", Value: uid})
	c.Set(vstar.Property{Name: "DTSTAMP", Value: "20260504T120000Z"})
	c.Set(vstar.Property{Name: "SUMMARY", Value: "scaffold app"})
	hashing.SetXVSTAR(&c)
	return c
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, ok := vstar.ParseTime(s)
	if !ok {
		t.Fatalf("ParseTime(%q) failed", s)
	}
	return parsed
}

func TestSupersedes_BasicShape(t *testing.T) {
	t.Parallel()

	target := makeTarget(t, "todo-123")
	at := mustTime(t, "20260504T120000Z")

	got, err := supersession.Supersedes(target, "completed", at)
	if err != nil {
		t.Fatalf("Supersedes returned error: %v", err)
	}

	if got.Type != vstar.CompJournal {
		t.Errorf("Type = %q, want %q", got.Type, vstar.CompJournal)
	}

	wantUID := "journal:status:todo-123:20260504T120000Z"
	if got.UID() != wantUID {
		t.Errorf("UID = %q, want %q", got.UID(), wantUID)
	}

	if got, want := got.DTSTAMPRaw(), "20260504T120000Z"; got != want {
		t.Errorf("DTSTAMP = %q, want %q", got, want)
	}

	rel, ok := got.Get("RELATED-TO")
	if !ok || rel.Value != "todo-123" {
		t.Errorf("RELATED-TO = %q (ok=%v), want %q", rel.Value, ok, "todo-123")
	}

	cat, ok := got.Get("CATEGORIES")
	if !ok || cat.Value != supersession.CategoryStatusSupersession {
		t.Errorf("CATEGORIES = %q (ok=%v), want %q",
			cat.Value, ok, supersession.CategoryStatusSupersession)
	}

	st, ok := got.Get(supersession.PropEffectiveStatus)
	if !ok || st.Value != "completed" {
		t.Errorf("X-VSTAR-EFFECTIVE-STATUS = %q (ok=%v), want %q",
			st.Value, ok, "completed")
	}
}

func TestSupersedes_HashRoundTripsClean(t *testing.T) {
	t.Parallel()

	target := makeTarget(t, "todo-123")
	got, err := supersession.Supersedes(target, "completed",
		mustTime(t, "20260504T120000Z"))
	if err != nil {
		t.Fatalf("Supersedes: %v", err)
	}

	ok, want, stored := hashing.VerifyXVSTAR(got)
	if !ok {
		t.Fatalf("VerifyXVSTAR: ok=false; want=%q got=%q", want, stored)
	}
}

func TestSupersedes_UIDFormatStable(t *testing.T) {
	t.Parallel()

	target := makeTarget(t, "todo-123")
	at := mustTime(t, "20260504T120000Z")

	a, err := supersession.Supersedes(target, "completed", at)
	if err != nil {
		t.Fatalf("Supersedes a: %v", err)
	}
	b, err := supersession.Supersedes(target, "completed", at)
	if err != nil {
		t.Fatalf("Supersedes b: %v", err)
	}

	if a.UID() != b.UID() {
		t.Errorf("UID not deterministic: a=%q b=%q", a.UID(), b.UID())
	}
}

func TestSupersedes_NonUTCTime_NormalizedToUTC(t *testing.T) {
	t.Parallel()

	target := makeTarget(t, "todo-456")
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("tz data not available: %v", err)
	}
	// 2026-05-04 08:00 EDT == 12:00:00 UTC (EDT is UTC-4 in May).
	local := time.Date(2026, 5, 4, 8, 0, 0, 0, loc)

	got, err := supersession.Supersedes(target, "completed", local)
	if err != nil {
		t.Fatalf("Supersedes: %v", err)
	}

	if got, want := got.DTSTAMPRaw(), "20260504T120000Z"; got != want {
		t.Errorf("DTSTAMP not normalised to UTC: got %q want %q", got, want)
	}
	if got, want := got.UID(), "journal:status:todo-456:20260504T120000Z"; got != want {
		t.Errorf("UID timestamp not normalised to UTC: got %q want %q", got, want)
	}
}

func TestSupersedes_TargetWithoutHash_Accepted(t *testing.T) {
	t.Parallel()

	target := vstar.Component{Type: vstar.CompTodo}
	target.Set(vstar.Property{Name: "UID", Value: "todo-no-hash"})
	target.Set(vstar.Property{Name: "DTSTAMP", Value: "20260504T120000Z"})

	got, err := supersession.Supersedes(target, "completed",
		mustTime(t, "20260504T120000Z"))
	if err != nil {
		t.Fatalf("Supersedes should accept hash-less target, got err: %v", err)
	}
	if got.UID() != "journal:status:todo-no-hash:20260504T120000Z" {
		t.Errorf("UID = %q", got.UID())
	}
}

func TestSupersedes_DoesNotMutateTarget(t *testing.T) {
	t.Parallel()

	target := makeTarget(t, "todo-immutable")
	originalHash, _ := hashing.GetXVSTAR(target)
	originalProps := len(target.Props)

	_, err := supersession.Supersedes(target, "completed",
		mustTime(t, "20260504T120000Z"))
	if err != nil {
		t.Fatalf("Supersedes: %v", err)
	}

	if got, _ := hashing.GetXVSTAR(target); got != originalHash {
		t.Errorf("target hash mutated: was %q now %q", originalHash, got)
	}
	if len(target.Props) != originalProps {
		t.Errorf("target prop count mutated: was %d now %d", originalProps, len(target.Props))
	}
}

// --- T4: integrity check ----------------------------------------------------

func TestSupersedes_RejectsCorruptedTarget(t *testing.T) {
	t.Parallel()

	target := makeTarget(t, "todo-corrupted")
	// Corrupt the target post-hash by mutating SUMMARY without rehash.
	target.Set(vstar.Property{Name: "SUMMARY", Value: "TAMPERED"})

	got, err := supersession.Supersedes(target, "completed",
		mustTime(t, "20260504T120000Z"))
	if !errors.Is(err, supersession.ErrTargetCorrupted) {
		t.Fatalf("err = %v, want ErrTargetCorrupted", err)
	}
	if got.Type != "" || len(got.Props) != 0 {
		t.Errorf("expected zero Component on error, got %+v", got)
	}
}

// --- T3: Superseded ---------------------------------------------------------

// makeSupersessionEntry builds a supersession VJOURNAL by hand (rather
// than via Supersedes) to keep T3 tests independent of T2.
func makeSupersessionEntry(t *testing.T, targetUID, status, dtstamp string) vstar.Component {
	t.Helper()
	c := vstar.Component{Type: vstar.CompJournal}
	c.Set(vstar.Property{Name: "UID", Value: "journal:status:" + targetUID + ":" + dtstamp})
	c.Set(vstar.Property{Name: "DTSTAMP", Value: dtstamp})
	c.Set(vstar.Property{Name: "RELATED-TO", Value: targetUID})
	c.Set(vstar.Property{Name: "CATEGORIES", Value: supersession.CategoryStatusSupersession})
	c.Set(vstar.Property{Name: supersession.PropEffectiveStatus, Value: status})
	return c
}

func TestSuperseded_EmptyLedger(t *testing.T) {
	t.Parallel()

	target := makeTarget(t, "todo-1")
	status, ok := supersession.Superseded(target, nil)
	if ok {
		t.Errorf("ok = true, want false (status=%q)", status)
	}
	if status != "" {
		t.Errorf("status = %q, want empty", status)
	}
}

func TestSuperseded_SingleEntry(t *testing.T) {
	t.Parallel()

	target := makeTarget(t, "todo-1")
	ledger := []vstar.Component{
		makeSupersessionEntry(t, "todo-1", "completed", "20260504T120000Z"),
	}
	status, ok := supersession.Superseded(target, ledger)
	if !ok || status != "completed" {
		t.Errorf("(status,ok) = (%q,%v), want (completed,true)", status, ok)
	}
}

func TestSuperseded_LatestByDTSTAMPWins(t *testing.T) {
	t.Parallel()

	target := makeTarget(t, "todo-1")
	// Out-of-order ledger to confirm we sort by DTSTAMP, not by index.
	ledger := []vstar.Component{
		makeSupersessionEntry(t, "todo-1", "in-process", "20260504T120000Z"),
		makeSupersessionEntry(t, "todo-1", "completed", "20260504T180000Z"),
		makeSupersessionEntry(t, "todo-1", "needs-action", "20260504T100000Z"),
	}
	status, ok := supersession.Superseded(target, ledger)
	if !ok || status != "completed" {
		t.Errorf("(status,ok) = (%q,%v), want (completed,true) — latest DTSTAMP must win",
			status, ok)
	}
}

func TestSuperseded_DifferentTargetIgnored(t *testing.T) {
	t.Parallel()

	target := makeTarget(t, "todo-1")
	ledger := []vstar.Component{
		makeSupersessionEntry(t, "todo-2", "completed", "20260504T120000Z"),
		makeSupersessionEntry(t, "todo-3", "abandoned", "20260504T180000Z"),
	}
	status, ok := supersession.Superseded(target, ledger)
	if ok {
		t.Errorf("ok = true, want false (status=%q)", status)
	}
}

func TestSuperseded_MissingCategoryIgnored(t *testing.T) {
	t.Parallel()

	target := makeTarget(t, "todo-1")
	entry := makeSupersessionEntry(t, "todo-1", "completed", "20260504T120000Z")
	entry.Remove("CATEGORIES")
	ledger := []vstar.Component{entry}

	status, ok := supersession.Superseded(target, ledger)
	if ok {
		t.Errorf("entry without CATEGORIES must be ignored; got status=%q ok=true", status)
	}
}

func TestSuperseded_NonJournalIgnored(t *testing.T) {
	t.Parallel()

	target := makeTarget(t, "todo-1")
	entry := makeSupersessionEntry(t, "todo-1", "completed", "20260504T120000Z")
	entry.Type = vstar.CompEvent
	ledger := []vstar.Component{entry}

	status, ok := supersession.Superseded(target, ledger)
	if ok {
		t.Errorf("non-VJOURNAL entry must be ignored; got status=%q ok=true", status)
	}
}

func TestSuperseded_MissingEffectiveStatusIgnored(t *testing.T) {
	t.Parallel()

	target := makeTarget(t, "todo-1")
	entry := makeSupersessionEntry(t, "todo-1", "completed", "20260504T120000Z")
	entry.Remove(supersession.PropEffectiveStatus)
	ledger := []vstar.Component{entry}

	status, ok := supersession.Superseded(target, ledger)
	if ok {
		t.Errorf("entry without X-VSTAR-EFFECTIVE-STATUS must be ignored; got status=%q ok=true", status)
	}
}

func TestSuperseded_TargetWithoutUID(t *testing.T) {
	t.Parallel()

	target := vstar.Component{Type: vstar.CompTodo}
	ledger := []vstar.Component{
		makeSupersessionEntry(t, "todo-1", "completed", "20260504T120000Z"),
	}
	status, ok := supersession.Superseded(target, ledger)
	if ok {
		t.Errorf("target without UID must return false; got status=%q ok=true", status)
	}
}

func TestSuperseded_UnparseableDTSTAMPSortsToZero(t *testing.T) {
	t.Parallel()

	target := makeTarget(t, "todo-1")
	bad := makeSupersessionEntry(t, "todo-1", "garbage-status", "20260504T120000Z")
	bad.Set(vstar.Property{Name: "DTSTAMP", Value: "not-a-time"})

	good := makeSupersessionEntry(t, "todo-1", "completed", "20260504T120000Z")

	ledger := []vstar.Component{bad, good}
	status, ok := supersession.Superseded(target, ledger)
	if !ok || status != "completed" {
		t.Errorf("(status,ok) = (%q,%v), want (completed,true) — bad DTSTAMP must sort to zero",
			status, ok)
	}
}

func TestSuperseded_CategoriesCommaSeparated(t *testing.T) {
	t.Parallel()

	target := makeTarget(t, "todo-1")
	entry := makeSupersessionEntry(t, "todo-1", "completed", "20260504T120000Z")
	entry.Set(vstar.Property{
		Name:  "CATEGORIES",
		Value: "audit, status-supersession ,foo",
	})
	ledger := []vstar.Component{entry}

	status, ok := supersession.Superseded(target, ledger)
	if !ok || status != "completed" {
		t.Errorf("(status,ok) = (%q,%v), want (completed,true) — comma-list match required",
			status, ok)
	}
}

func TestSuperseded_CategoriesCaseInsensitive(t *testing.T) {
	t.Parallel()

	target := makeTarget(t, "todo-1")
	entry := makeSupersessionEntry(t, "todo-1", "completed", "20260504T120000Z")
	entry.Set(vstar.Property{
		Name:  "CATEGORIES",
		Value: "STATUS-SUPERSESSION",
	})
	ledger := []vstar.Component{entry}

	status, ok := supersession.Superseded(target, ledger)
	if !ok || status != "completed" {
		t.Errorf("(status,ok) = (%q,%v), want (completed,true) — case-insensitive match required",
			status, ok)
	}
}

func TestSuperseded_TieBreakLastInLedgerWins(t *testing.T) {
	t.Parallel()

	// Two entries with identical DTSTAMP — the LATER ledger position
	// wins under stable scan order.
	target := makeTarget(t, "todo-1")
	ledger := []vstar.Component{
		makeSupersessionEntry(t, "todo-1", "first", "20260504T120000Z"),
		makeSupersessionEntry(t, "todo-1", "second", "20260504T120000Z"),
	}
	status, ok := supersession.Superseded(target, ledger)
	if !ok || status != "second" {
		t.Errorf("(status,ok) = (%q,%v), want (second,true) — tie must resolve to later index",
			status, ok)
	}
}

// --- Doc UID example (matches the report) -----------------------------------

func TestSupersedes_UIDExample_FromReport(t *testing.T) {
	t.Parallel()

	target := vstar.Component{Type: vstar.CompTodo}
	target.Set(vstar.Property{Name: "UID", Value: "todo-123"})
	target.Set(vstar.Property{Name: "DTSTAMP", Value: "20260504T120000Z"})
	// No X-VSTAR-HASH on target — exercise hash-less acceptance path.

	got, err := supersession.Supersedes(target, "completed",
		mustTime(t, "20260504T120000Z"))
	if err != nil {
		t.Fatalf("Supersedes: %v", err)
	}
	if got, want := got.UID(), "journal:status:todo-123:20260504T120000Z"; got != want {
		t.Errorf("UID = %q, want %q", got, want)
	}
}
