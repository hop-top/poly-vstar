// SPDX-License-Identifier: MIT

package helpers_test

import (
	"testing"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/helpers"
)

func TestStatus_returnsValueWhenSet(t *testing.T) {
	c := vstar.Component{Type: vstar.CompTodo}
	c.Set(vstar.Property{Name: "STATUS", Value: string(vstar.TodoInProcess)})
	got, ok := helpers.Status(c)
	if !ok || got != vstar.TodoInProcess {
		t.Errorf("Status = %q %v, want %q true", got, ok, vstar.TodoInProcess)
	}
}

func TestStatus_returnsFalseWhenAbsent(t *testing.T) {
	c := vstar.Component{Type: vstar.CompTodo}
	got, ok := helpers.Status(c)
	if ok {
		t.Errorf("Status ok = true, want false (got %q)", got)
	}
}

func TestStatus_returnsFalseForInvalidValue(t *testing.T) {
	c := vstar.Component{Type: vstar.CompTodo}
	c.Set(vstar.Property{Name: "STATUS", Value: "FROBNICATED"})
	if _, ok := helpers.Status(c); ok {
		t.Errorf("Status ok = true for invalid value, want false")
	}
}

func TestSetStatus_writesAndRefreshesHash(t *testing.T) {
	todo, _ := helpers.NewTodo("uid-s", time.Now().UTC())
	pre, _ := todo.Get("X-VSTAR-HASH")
	helpers.SetStatus(&todo, vstar.TodoCompleted)
	post, _ := todo.Get("X-VSTAR-HASH")
	if pre.Value == post.Value {
		t.Errorf("X-VSTAR-HASH unchanged after SetStatus (%q)", pre.Value)
	}
	got, ok := helpers.Status(todo)
	if !ok || got != vstar.TodoCompleted {
		t.Errorf("Status = %q %v", got, ok)
	}
}

func TestSetStatus_roundTrip(t *testing.T) {
	for _, want := range []vstar.TodoStatus{
		vstar.TodoNeedsAction,
		vstar.TodoInProcess,
		vstar.TodoCompleted,
		vstar.TodoCancelled,
	} {
		c := vstar.Component{Type: vstar.CompTodo}
		c.Set(vstar.Property{Name: "UID", Value: "u"})
		helpers.SetStatus(&c, want)
		got, ok := helpers.Status(c)
		if !ok || got != want {
			t.Errorf("round-trip %q: got %q ok=%v", want, got, ok)
		}
	}
}

func TestSetStatus_invalidValueIsNoOp(t *testing.T) {
	c := vstar.Component{Type: vstar.CompTodo}
	c.Set(vstar.Property{Name: "UID", Value: "u"})
	helpers.SetStatus(&c, vstar.TodoStatus("BOGUS"))
	if _, ok := c.Get("STATUS"); ok {
		t.Errorf("STATUS written for invalid value, want no-op")
	}
}

func TestSetStatus_nonTodoIsNoOp(t *testing.T) {
	c := vstar.Component{Type: vstar.CompEvent}
	c.Set(vstar.Property{Name: "UID", Value: "u"})
	helpers.SetStatus(&c, vstar.TodoCompleted)
	if _, ok := c.Get("STATUS"); ok {
		t.Errorf("STATUS written on non-VTODO, want no-op")
	}
}

func TestComplete_setsStatusAndCompletedAndPercent(t *testing.T) {
	todo, _ := helpers.NewTodo("uid-c", time.Now().UTC())
	pre, _ := todo.Get("X-VSTAR-HASH")
	when := time.Date(2026, 5, 4, 18, 0, 0, 0, time.UTC)
	helpers.Complete(&todo, when)

	got, ok := helpers.Status(todo)
	if !ok || got != vstar.TodoCompleted {
		t.Errorf("Status = %q %v", got, ok)
	}
	cmp, ok := todo.COMPLETED(vstar.Calendar{})
	if !ok || !cmp.Equal(when) {
		t.Errorf("COMPLETED = %v %v, want %v", cmp, ok, when)
	}
	pct, ok := todo.Get("PERCENT-COMPLETE")
	if !ok || pct.Value != "100" {
		t.Errorf("PERCENT-COMPLETE = %v %v", pct, ok)
	}
	post, _ := todo.Get("X-VSTAR-HASH")
	if pre.Value == post.Value {
		t.Errorf("X-VSTAR-HASH unchanged")
	}
}

func TestComplete_nonTodoIsNoOp(t *testing.T) {
	c := vstar.Component{Type: vstar.CompEvent}
	c.Set(vstar.Property{Name: "UID", Value: "u"})
	helpers.Complete(&c, time.Now())
	if _, ok := c.Get("STATUS"); ok {
		t.Errorf("STATUS written on non-VTODO")
	}
	if _, ok := c.Get("COMPLETED"); ok {
		t.Errorf("COMPLETED written on non-VTODO")
	}
	if _, ok := c.Get("PERCENT-COMPLETE"); ok {
		t.Errorf("PERCENT-COMPLETE written on non-VTODO")
	}
}
