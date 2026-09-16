// SPDX-License-Identifier: MIT

package helpers_test

import (
	"reflect"
	"testing"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/helpers"
)

func TestCategories_emptyWhenAbsent(t *testing.T) {
	c := vstar.Component{Type: vstar.CompTodo}
	if got := helpers.Categories(c); len(got) != 0 {
		t.Errorf("Categories = %v, want empty", got)
	}
}

func TestCategories_parsesCommaSeparated(t *testing.T) {
	c := vstar.Component{Type: vstar.CompTodo}
	c.Set(vstar.Property{Name: "CATEGORIES", Value: "work,urgent,review"})
	got := helpers.Categories(c)
	want := []string{"work", "urgent", "review"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Categories = %v, want %v", got, want)
	}
}

func TestCategories_acceptsWhitespaceAfterComma(t *testing.T) {
	c := vstar.Component{Type: vstar.CompTodo}
	c.Set(vstar.Property{Name: "CATEGORIES", Value: "work, urgent, review"})
	got := helpers.Categories(c)
	want := []string{"work", "urgent", "review"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Categories = %v, want %v", got, want)
	}
}

func TestCategories_dropsEmptyTokens(t *testing.T) {
	c := vstar.Component{Type: vstar.CompTodo}
	c.Set(vstar.Property{Name: "CATEGORIES", Value: "work,,urgent, ,review"})
	got := helpers.Categories(c)
	want := []string{"work", "urgent", "review"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Categories = %v, want %v", got, want)
	}
}

func TestSetCategories_writesNoSpaceJoin(t *testing.T) {
	c, _ := helpers.NewTodo("u", time.Now().UTC())
	pre, _ := c.Get("X-VSTAR-HASH")
	helpers.SetCategories(&c, []string{"work", "urgent", "review"})
	post, _ := c.Get("X-VSTAR-HASH")
	if pre.Value == post.Value {
		t.Errorf("X-VSTAR-HASH unchanged")
	}
	p, ok := c.Get("CATEGORIES")
	if !ok || p.Value != "work,urgent,review" {
		t.Errorf("CATEGORIES = %q %v, want \"work,urgent,review\"", p.Value, ok)
	}
}

func TestSetCategories_emptyRemovesProperty(t *testing.T) {
	c := vstar.Component{Type: vstar.CompTodo}
	c.Set(vstar.Property{Name: "UID", Value: "u"})
	c.Set(vstar.Property{Name: "CATEGORIES", Value: "work,urgent"})
	helpers.SetCategories(&c, nil)
	if _, ok := c.Get("CATEGORIES"); ok {
		t.Errorf("CATEGORIES still present after SetCategories(nil)")
	}
}

func TestSetCategories_dedupesWhileWriting(t *testing.T) {
	c := vstar.Component{Type: vstar.CompTodo}
	c.Set(vstar.Property{Name: "UID", Value: "u"})
	helpers.SetCategories(&c, []string{"work", "urgent", "work", "Urgent"})
	got := helpers.Categories(c)
	// Dedupe is case-sensitive (CATEGORIES values are user-facing
	// labels per RFC 5545 §3.8.1.2, not registry tokens).
	want := []string{"work", "urgent", "Urgent"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Categories = %v, want %v", got, want)
	}
}

func TestAddCategory_appendsWithoutDuplicates(t *testing.T) {
	c, _ := helpers.NewTodo("u", time.Now().UTC())
	helpers.AddCategory(&c, "work")
	helpers.AddCategory(&c, "urgent")
	helpers.AddCategory(&c, "work") // dup, should be skipped
	got := helpers.Categories(c)
	want := []string{"work", "urgent"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Categories = %v, want %v", got, want)
	}
}

func TestAddCategory_refreshesHash(t *testing.T) {
	c, _ := helpers.NewTodo("u", time.Now().UTC())
	pre, _ := c.Get("X-VSTAR-HASH")
	helpers.AddCategory(&c, "work")
	post, _ := c.Get("X-VSTAR-HASH")
	if pre.Value == post.Value {
		t.Errorf("X-VSTAR-HASH unchanged after AddCategory")
	}
}

func TestAddCategory_emptyIsNoOp(t *testing.T) {
	c, _ := helpers.NewTodo("u", time.Now().UTC())
	pre, _ := c.Get("X-VSTAR-HASH")
	helpers.AddCategory(&c, "")
	post, _ := c.Get("X-VSTAR-HASH")
	if pre.Value != post.Value {
		t.Errorf("X-VSTAR-HASH changed after empty AddCategory")
	}
	if got := helpers.Categories(c); len(got) != 0 {
		t.Errorf("Categories = %v, want empty", got)
	}
}

func TestAddCategory_nilIsNoOp(t *testing.T) {
	helpers.AddCategory(nil, "work")
}

func TestSetCategories_nilIsNoOp(t *testing.T) {
	helpers.SetCategories(nil, []string{"x"})
}
