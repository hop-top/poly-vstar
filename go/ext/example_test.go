// SPDX-License-Identifier: MIT

package ext_test

import (
	"fmt"

	vstar "hop.top/vstar"
	"hop.top/vstar/ext"
)

func ExampleIsExtension() {
	fmt.Println(ext.IsExtension("X-AGR-INTENT"))
	fmt.Println(ext.IsExtension("DTSTART"))
	// Output:
	// true
	// false
}

func ExampleScopeOf() {
	fmt.Println(ext.ScopeOf("X-VSTAR-HASH"))
	fmt.Println(ext.ScopeOf("X-AGR-INTENT"))
	fmt.Println(ext.ScopeOf("X-EXP-FOO"))
	fmt.Println(ext.ScopeOf("DTSTART"))
	fmt.Println(ext.ScopeOf("X-"))
	// Output:
	// VStar
	// System
	// Experimental
	// None
	// Unknown
}

func ExampleSystemName() {
	slug, ok := ext.SystemName("X-AGR-INTENT")
	fmt.Println(slug, ok)
	// Output: AGR true
}

func ExampleExtensionsByScope() {
	c := vstar.Component{
		Type: vstar.CompEvent,
		Props: []vstar.Property{
			{Name: "UID", Value: "u@example"},
			{Name: "X-AGR-INTENT", Value: "schedule"},
			{Name: "X-AGR-ROLE", Value: "host"},
			{Name: "X-EXP-FOO", Value: "bar"},
		},
	}
	for _, p := range ext.ExtensionsByScope(c, ext.ScopeSystem) {
		fmt.Println(p.Name)
	}
	// Output:
	// X-AGR-INTENT
	// X-AGR-ROLE
}
