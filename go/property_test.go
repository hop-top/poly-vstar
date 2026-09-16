// SPDX-License-Identifier: MIT

package vstar

import "testing"

func TestProperty_Equal(t *testing.T) {
	tests := []struct {
		name string
		a, b Property
		want bool
	}{
		{
			name: "identical properties are equal",
			a:    Property{Name: "UID", Value: "abc"},
			b:    Property{Name: "UID", Value: "abc"},
			want: true,
		},
		{
			name: "name case differs but equal",
			a:    Property{Name: "UID", Value: "abc"},
			b:    Property{Name: "uid", Value: "abc"},
			want: true,
		},
		{
			name: "params reordered are equal",
			a: Property{
				Name:   "ATTENDEE",
				Params: []Param{{Name: "CN", Value: "Jad"}, {Name: "ROLE", Value: "REQ-PARTICIPANT"}},
				Value:  "mailto:jad@example.com",
			},
			b: Property{
				Name:   "ATTENDEE",
				Params: []Param{{Name: "ROLE", Value: "REQ-PARTICIPANT"}, {Name: "CN", Value: "Jad"}},
				Value:  "mailto:jad@example.com",
			},
			want: true,
		},
		{
			name: "param value differs not equal",
			a: Property{
				Name:   "ATTENDEE",
				Params: []Param{{Name: "ROLE", Value: "REQ-PARTICIPANT"}},
				Value:  "mailto:jad@example.com",
			},
			b: Property{
				Name:   "ATTENDEE",
				Params: []Param{{Name: "ROLE", Value: "OPT-PARTICIPANT"}},
				Value:  "mailto:jad@example.com",
			},
			want: false,
		},
		{
			name: "value differs not equal",
			a:    Property{Name: "SUMMARY", Value: "hello"},
			b:    Property{Name: "SUMMARY", Value: "world"},
			want: false,
		},
		{
			name: "param name differs not equal",
			a: Property{
				Name:   "ATTENDEE",
				Params: []Param{{Name: "CN", Value: "Jad"}},
				Value:  "mailto:jad@example.com",
			},
			b: Property{
				Name:   "ATTENDEE",
				Params: []Param{{Name: "ROLE", Value: "Jad"}},
				Value:  "mailto:jad@example.com",
			},
			want: false,
		},
		{
			name: "param count differs not equal",
			a: Property{
				Name:   "ATTENDEE",
				Params: []Param{{Name: "CN", Value: "Jad"}},
				Value:  "mailto:jad@example.com",
			},
			b: Property{
				Name:   "ATTENDEE",
				Params: []Param{{Name: "CN", Value: "Jad"}, {Name: "ROLE", Value: "REQ-PARTICIPANT"}},
				Value:  "mailto:jad@example.com",
			},
			want: false,
		},
		{
			name: "name differs not equal",
			a:    Property{Name: "UID", Value: "abc"},
			b:    Property{Name: "SUMMARY", Value: "abc"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Equal(tt.a, tt.b); got != tt.want {
				t.Errorf("Equal(%+v, %+v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestProperty_Equal_DoesNotMutate(t *testing.T) {
	a := Property{
		Name:   "ATTENDEE",
		Params: []Param{{Name: "ROLE", Value: "REQ-PARTICIPANT"}, {Name: "CN", Value: "Jad"}},
		Value:  "mailto:jad@example.com",
	}
	b := Property{
		Name:   "ATTENDEE",
		Params: []Param{{Name: "CN", Value: "Jad"}, {Name: "ROLE", Value: "REQ-PARTICIPANT"}},
		Value:  "mailto:jad@example.com",
	}
	origA := []Param{a.Params[0], a.Params[1]}
	origB := []Param{b.Params[0], b.Params[1]}
	_ = Equal(a, b)
	for i := range origA {
		if a.Params[i] != origA[i] {
			t.Errorf("Equal mutated a.Params: index %d became %+v, want %+v", i, a.Params[i], origA[i])
		}
		if b.Params[i] != origB[i] {
			t.Errorf("Equal mutated b.Params: index %d became %+v, want %+v", i, b.Params[i], origB[i])
		}
	}
}
