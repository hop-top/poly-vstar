// SPDX-License-Identifier: MIT

package helpers_test

import (
	"testing"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/helpers"
)

func TestDue_returnsParsedTime(t *testing.T) {
	want := time.Date(2026, 5, 4, 17, 30, 0, 0, time.UTC)
	c, _ := helpers.NewTodo("u", want)
	got, ok := helpers.Due(c, vstar.Calendar{})
	if !ok || !got.Equal(want) {
		t.Errorf("Due = %v %v, want %v", got, ok, want)
	}
}

func TestDue_falseWhenAbsent(t *testing.T) {
	c := vstar.Component{Type: vstar.CompTodo}
	if _, ok := helpers.Due(c, vstar.Calendar{}); ok {
		t.Errorf("Due ok = true, want false")
	}
}

func TestSetDue_writesAndRefreshesHash(t *testing.T) {
	c, _ := helpers.NewTodo("u", time.Now().UTC())
	pre, _ := c.Get("X-VSTAR-HASH")
	when := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	helpers.SetDue(&c, when)
	got, ok := helpers.Due(c, vstar.Calendar{})
	if !ok || !got.Equal(when) {
		t.Errorf("Due = %v %v", got, ok)
	}
	post, _ := c.Get("X-VSTAR-HASH")
	if pre.Value == post.Value {
		t.Errorf("X-VSTAR-HASH unchanged")
	}
}

func TestSetDue_nilIsNoOp(t *testing.T) {
	helpers.SetDue(nil, time.Now())
}
