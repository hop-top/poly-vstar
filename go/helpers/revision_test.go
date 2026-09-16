// SPDX-License-Identifier: MIT

package helpers_test

import (
	"testing"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/hashing"
	"hop.top/vstar/helpers"
)

// newTodoT builds a hash-stamped VTODO for mutator tests.
func newTodoT(t *testing.T, uid string) vstar.Component {
	t.Helper()
	c, err := helpers.NewTodo(uid, time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewTodo: %v", err)
	}
	return c
}

// newEventT builds a hash-stamped VEVENT for mutator tests.
func newEventT(t *testing.T, uid string) vstar.Component {
	t.Helper()
	start := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	c, err := helpers.NewEvent(uid, start, start.Add(time.Hour))
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	return c
}

// newJournalT builds a hash-stamped VJOURNAL for mutator tests.
func newJournalT(t *testing.T, uid string) vstar.Component {
	t.Helper()
	c, err := helpers.NewJournal(uid, time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewJournal: %v", err)
	}
	return c
}

// assertHashFresh fails when the stored X-VSTAR-HASH does not match
// the recomputed digest — i.e. when a setter refreshed the hash
// before its other mutations instead of last.
func assertHashFresh(t *testing.T, c vstar.Component, what string) {
	t.Helper()
	ok, want, got := hashing.VerifyXVSTAR(c)
	if !ok {
		t.Errorf("%s: VerifyXVSTAR ok=false, want=%q got=%q (hash refresh not last?)", what, want, got)
	}
}

// ---------- SEQUENCE ----------

func TestSequence_falseWhenAbsent(t *testing.T) {
	c := vstar.Component{Type: vstar.CompEvent}
	if n, ok := helpers.Sequence(c); ok {
		t.Errorf("Sequence = %d true, want false", n)
	}
}

func TestSequence_falseWhenUnparseable(t *testing.T) {
	for _, bad := range []string{"", "abc", "1.5", "-1", " 3"} {
		c := vstar.Component{Type: vstar.CompEvent}
		c.Set(vstar.Property{Name: "SEQUENCE", Value: bad})
		if n, ok := helpers.Sequence(c); ok {
			t.Errorf("Sequence(%q) = %d true, want false", bad, n)
		}
	}
}

func TestSetSequence_roundTripAndHash(t *testing.T) {
	for _, want := range []int{0, 1, 7, 65535} {
		c := newEventT(t, "uid-seq")
		helpers.SetSequence(&c, want)
		got, ok := helpers.Sequence(c)
		if !ok || got != want {
			t.Errorf("Sequence = %d %v, want %d true", got, ok, want)
		}
		assertHashFresh(t, c, "SetSequence")
	}
}

func TestSetSequence_refreshesHash(t *testing.T) {
	c := newEventT(t, "uid-seq-h")
	pre, _ := c.Get("X-VSTAR-HASH")
	helpers.SetSequence(&c, 3)
	post, _ := c.Get("X-VSTAR-HASH")
	if pre.Value == post.Value {
		t.Errorf("X-VSTAR-HASH unchanged after SetSequence")
	}
}

func TestSetSequence_negativeIsNoOp(t *testing.T) {
	c := newEventT(t, "uid-seq-neg")
	pre, _ := c.Get("X-VSTAR-HASH")
	helpers.SetSequence(&c, -1)
	if _, ok := c.Get("SEQUENCE"); ok {
		t.Errorf("SEQUENCE written for negative input, want no-op")
	}
	post, _ := c.Get("X-VSTAR-HASH")
	if pre.Value != post.Value {
		t.Errorf("X-VSTAR-HASH refreshed on no-op SetSequence")
	}
}

func TestSetSequence_appliesToEventTodoJournal(t *testing.T) {
	for _, c := range []vstar.Component{
		newEventT(t, "e"), newTodoT(t, "t"), newJournalT(t, "j"),
	} {
		helpers.SetSequence(&c, 5)
		if got, ok := helpers.Sequence(c); !ok || got != 5 {
			t.Errorf("%s: Sequence = %d %v, want 5 true", c.Type, got, ok)
		}
	}
}

func TestSetSequence_wrongTypeIsNoOp(t *testing.T) {
	for _, ct := range []vstar.CompType{vstar.CompFreeBusy, vstar.CompAlarm, vstar.CompTimezone, vstar.CompCalendar} {
		c := vstar.Component{Type: ct}
		c.Set(vstar.Property{Name: "UID", Value: "u"})
		helpers.SetSequence(&c, 2)
		if _, ok := c.Get("SEQUENCE"); ok {
			t.Errorf("SEQUENCE written on %s, want no-op", ct)
		}
	}
}

func TestSetSequence_nilIsNoOp(t *testing.T) {
	helpers.SetSequence(nil, 1)
}

func TestIncrementSequence_absentBecomesOne(t *testing.T) {
	c := newEventT(t, "uid-inc")
	helpers.IncrementSequence(&c)
	if got, ok := helpers.Sequence(c); !ok || got != 1 {
		t.Errorf("Sequence after increment from absent = %d %v, want 1 true", got, ok)
	}
	assertHashFresh(t, c, "IncrementSequence")
}

func TestIncrementSequence_bumpsExisting(t *testing.T) {
	c := newEventT(t, "uid-inc2")
	helpers.SetSequence(&c, 4)
	helpers.IncrementSequence(&c)
	helpers.IncrementSequence(&c)
	if got, ok := helpers.Sequence(c); !ok || got != 6 {
		t.Errorf("Sequence = %d %v, want 6 true", got, ok)
	}
	assertHashFresh(t, c, "IncrementSequence")
}

func TestIncrementSequence_unparseableRestartsAtOne(t *testing.T) {
	c := newEventT(t, "uid-inc3")
	c.Set(vstar.Property{Name: "SEQUENCE", Value: "garbage"})
	helpers.IncrementSequence(&c)
	if got, ok := helpers.Sequence(c); !ok || got != 1 {
		t.Errorf("Sequence = %d %v, want 1 true", got, ok)
	}
}

func TestIncrementSequence_wrongTypeIsNoOp(t *testing.T) {
	c := vstar.Component{Type: vstar.CompFreeBusy}
	c.Set(vstar.Property{Name: "UID", Value: "u"})
	helpers.IncrementSequence(&c)
	if _, ok := c.Get("SEQUENCE"); ok {
		t.Errorf("SEQUENCE written on VFREEBUSY, want no-op")
	}
}

func TestIncrementSequence_nilIsNoOp(t *testing.T) {
	helpers.IncrementSequence(nil)
}

// ---------- PRIORITY ----------

func TestPriority_falseWhenAbsent(t *testing.T) {
	c := vstar.Component{Type: vstar.CompTodo}
	if n, ok := helpers.Priority(c); ok {
		t.Errorf("Priority = %d true, want false", n)
	}
}

func TestPriority_zeroIsPresentAndDistinctFromAbsent(t *testing.T) {
	c := newTodoT(t, "uid-p0")
	helpers.SetPriority(&c, 0)
	got, ok := helpers.Priority(c)
	if !ok || got != 0 {
		t.Errorf("Priority = %d %v, want 0 true (explicit undefined)", got, ok)
	}
	assertHashFresh(t, c, "SetPriority(0)")
}

func TestPriority_falseWhenOutOfRangeOnWire(t *testing.T) {
	for _, bad := range []string{"10", "-1", "abc", "", "999"} {
		c := vstar.Component{Type: vstar.CompTodo}
		c.Set(vstar.Property{Name: "PRIORITY", Value: bad})
		if n, ok := helpers.Priority(c); ok {
			t.Errorf("Priority(%q) = %d true, want false", bad, n)
		}
	}
}

func TestSetPriority_roundTripAllValid(t *testing.T) {
	for want := 0; want <= 9; want++ {
		c := newTodoT(t, "uid-pr")
		helpers.SetPriority(&c, want)
		got, ok := helpers.Priority(c)
		if !ok || got != want {
			t.Errorf("Priority = %d %v, want %d true", got, ok, want)
		}
		assertHashFresh(t, c, "SetPriority")
	}
}

func TestSetPriority_refreshesHash(t *testing.T) {
	c := newTodoT(t, "uid-pr-h")
	pre, _ := c.Get("X-VSTAR-HASH")
	helpers.SetPriority(&c, 1)
	post, _ := c.Get("X-VSTAR-HASH")
	if pre.Value == post.Value {
		t.Errorf("X-VSTAR-HASH unchanged after SetPriority")
	}
}

func TestSetPriority_outOfRangeIsNoOp(t *testing.T) {
	for _, bad := range []int{-1, 10, 100} {
		c := newTodoT(t, "uid-pr-bad")
		pre, _ := c.Get("X-VSTAR-HASH")
		helpers.SetPriority(&c, bad)
		if _, ok := c.Get("PRIORITY"); ok {
			t.Errorf("PRIORITY written for %d, want no-op", bad)
		}
		post, _ := c.Get("X-VSTAR-HASH")
		if pre.Value != post.Value {
			t.Errorf("X-VSTAR-HASH refreshed on no-op SetPriority(%d)", bad)
		}
	}
}

func TestSetPriority_outOfRangeDoesNotClobberExisting(t *testing.T) {
	c := newTodoT(t, "uid-pr-keep")
	helpers.SetPriority(&c, 3)
	helpers.SetPriority(&c, 42)
	if got, ok := helpers.Priority(c); !ok || got != 3 {
		t.Errorf("Priority = %d %v, want 3 true (out-of-range must not clobber)", got, ok)
	}
}

func TestSetPriority_appliesToTodoAndEvent(t *testing.T) {
	for _, c := range []vstar.Component{newTodoT(t, "t"), newEventT(t, "e")} {
		helpers.SetPriority(&c, 2)
		if got, ok := helpers.Priority(c); !ok || got != 2 {
			t.Errorf("%s: Priority = %d %v, want 2 true", c.Type, got, ok)
		}
	}
}

func TestSetPriority_wrongTypeIsNoOp(t *testing.T) {
	for _, ct := range []vstar.CompType{vstar.CompJournal, vstar.CompFreeBusy, vstar.CompAlarm} {
		c := vstar.Component{Type: ct}
		c.Set(vstar.Property{Name: "UID", Value: "u"})
		helpers.SetPriority(&c, 1)
		if _, ok := c.Get("PRIORITY"); ok {
			t.Errorf("PRIORITY written on %s, want no-op", ct)
		}
	}
}

func TestSetPriority_nilIsNoOp(t *testing.T) {
	helpers.SetPriority(nil, 1)
}

func TestRemovePriority_dropsPropertyAndRefreshesHash(t *testing.T) {
	c := newTodoT(t, "uid-pr-rm")
	helpers.SetPriority(&c, 5)
	helpers.RemovePriority(&c)
	if _, ok := helpers.Priority(c); ok {
		t.Errorf("Priority still present after RemovePriority")
	}
	if _, ok := c.Get("PRIORITY"); ok {
		t.Errorf("PRIORITY property still on component")
	}
	assertHashFresh(t, c, "RemovePriority")
}

func TestRemovePriority_nilIsNoOp(t *testing.T) {
	helpers.RemovePriority(nil)
}

// ---------- PERCENT-COMPLETE ----------

func TestPercentComplete_falseWhenAbsent(t *testing.T) {
	c := vstar.Component{Type: vstar.CompTodo}
	if n, ok := helpers.PercentComplete(c); ok {
		t.Errorf("PercentComplete = %d true, want false", n)
	}
}

func TestPercentComplete_falseWhenOutOfRangeOnWire(t *testing.T) {
	for _, bad := range []string{"101", "-1", "abc", "", "1000"} {
		c := vstar.Component{Type: vstar.CompTodo}
		c.Set(vstar.Property{Name: "PERCENT-COMPLETE", Value: bad})
		if n, ok := helpers.PercentComplete(c); ok {
			t.Errorf("PercentComplete(%q) = %d true, want false", bad, n)
		}
	}
}

func TestSetPercentComplete_roundTripAndHash(t *testing.T) {
	for _, want := range []int{0, 1, 40, 99, 100} {
		c := newTodoT(t, "uid-pc")
		helpers.SetPercentComplete(&c, want)
		got, ok := helpers.PercentComplete(c)
		if !ok || got != want {
			t.Errorf("PercentComplete = %d %v, want %d true", got, ok, want)
		}
		assertHashFresh(t, c, "SetPercentComplete")
	}
}

func TestSetPercentComplete_refreshesHash(t *testing.T) {
	c := newTodoT(t, "uid-pc-h")
	pre, _ := c.Get("X-VSTAR-HASH")
	helpers.SetPercentComplete(&c, 40)
	post, _ := c.Get("X-VSTAR-HASH")
	if pre.Value == post.Value {
		t.Errorf("X-VSTAR-HASH unchanged after SetPercentComplete")
	}
}

func TestSetPercentComplete_outOfRangeIsNoOp(t *testing.T) {
	for _, bad := range []int{-1, 101, 1000} {
		c := newTodoT(t, "uid-pc-bad")
		pre, _ := c.Get("X-VSTAR-HASH")
		helpers.SetPercentComplete(&c, bad)
		if _, ok := c.Get("PERCENT-COMPLETE"); ok {
			t.Errorf("PERCENT-COMPLETE written for %d, want no-op", bad)
		}
		post, _ := c.Get("X-VSTAR-HASH")
		if pre.Value != post.Value {
			t.Errorf("X-VSTAR-HASH refreshed on no-op SetPercentComplete(%d)", bad)
		}
	}
}

func TestSetPercentComplete_nonTodoIsNoOp(t *testing.T) {
	for _, ct := range []vstar.CompType{vstar.CompEvent, vstar.CompJournal, vstar.CompFreeBusy} {
		c := vstar.Component{Type: ct}
		c.Set(vstar.Property{Name: "UID", Value: "u"})
		helpers.SetPercentComplete(&c, 50)
		if _, ok := c.Get("PERCENT-COMPLETE"); ok {
			t.Errorf("PERCENT-COMPLETE written on %s, want no-op", ct)
		}
	}
}

func TestSetPercentComplete_nilIsNoOp(t *testing.T) {
	helpers.SetPercentComplete(nil, 50)
}

func TestRemovePercentComplete_dropsPropertyAndRefreshesHash(t *testing.T) {
	c := newTodoT(t, "uid-pc-rm")
	helpers.SetPercentComplete(&c, 40)
	helpers.RemovePercentComplete(&c)
	if _, ok := helpers.PercentComplete(c); ok {
		t.Errorf("PercentComplete still present after RemovePercentComplete")
	}
	assertHashFresh(t, c, "RemovePercentComplete")
}

func TestRemovePercentComplete_nilIsNoOp(t *testing.T) {
	helpers.RemovePercentComplete(nil)
}

// ---------- Complete invariants (behavior must not change) ----------

func TestComplete_percentReadableAsHundred(t *testing.T) {
	c := newTodoT(t, "uid-complete")
	helpers.Complete(&c, time.Date(2026, 5, 4, 18, 0, 0, 0, time.UTC))
	if got, ok := helpers.PercentComplete(c); !ok || got != 100 {
		t.Errorf("PercentComplete after Complete = %d %v, want 100 true", got, ok)
	}
	assertHashFresh(t, c, "Complete")
}

func TestComplete_hashMatchesManualEquivalent(t *testing.T) {
	when := time.Date(2026, 5, 4, 18, 0, 0, 0, time.UTC)

	got := newTodoT(t, "uid-eq")
	helpers.Complete(&got, when)

	// Manual equivalent: the exact property writes Complete
	// promises, in the documented order, hash last.
	want := newTodoT(t, "uid-eq")
	want.Set(vstar.Property{Name: "STATUS", Value: string(vstar.TodoCompleted)})
	want.SetCOMPLETED(when)
	want.Set(vstar.Property{Name: "PERCENT-COMPLETE", Value: "100"})
	hashing.SetXVSTAR(&want)

	if len(got.Props) != len(want.Props) {
		t.Fatalf("prop count = %d, want %d", len(got.Props), len(want.Props))
	}
	for i := range got.Props {
		if !vstar.Equal(got.Props[i], want.Props[i]) {
			t.Errorf("prop[%d] = %+v, want %+v", i, got.Props[i], want.Props[i])
		}
	}
}
