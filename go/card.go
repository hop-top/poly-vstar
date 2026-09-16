// SPDX-License-Identifier: MIT

package vstar

import "strings"

// Card is a top-level VCARD object per RFC 6350: a UID identifier,
// a Kind discriminator (individual / group / org / location), and the
// vCard property list. Card is structurally analogous to Component
// but distinct because vCards do not nest sub-components and do not
// carry CompType.
type Card struct {
	UID   string
	Kind  Kind
	Props []Property
}

// Get returns the first property whose Name matches name (case-
// insensitive per RFC 6350 §3.3). Returns the zero Property and
// ok=false when none match.
func (c *Card) Get(name string) (Property, bool) {
	for _, p := range c.Props {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return Property{}, false
}

// GetAll returns all properties whose Name matches name (case-
// insensitive). Returns nil when none match.
func (c *Card) GetAll(name string) []Property {
	var out []Property
	for _, p := range c.Props {
		if strings.EqualFold(p.Name, name) {
			out = append(out, p)
		}
	}
	return out
}

// Set replaces every property matching p.Name (case-insensitive) with
// a single copy of p. If no property matches, p is appended.
func (c *Card) Set(p Property) {
	out := make([]Property, 0, len(c.Props))
	replaced := false
	for _, existing := range c.Props {
		if strings.EqualFold(existing.Name, p.Name) {
			if !replaced {
				out = append(out, p)
				replaced = true
			}
			continue
		}
		out = append(out, existing)
	}
	if !replaced {
		out = append(out, p)
	}
	c.Props = out
}

// Add appends p to the card's property list without touching existing
// properties of the same name.
func (c *Card) Add(p Property) {
	c.Props = append(c.Props, p)
}

// Remove deletes every property matching name (case-insensitive).
// No-op when none match.
func (c *Card) Remove(name string) {
	out := make([]Property, 0, len(c.Props))
	for _, p := range c.Props {
		if strings.EqualFold(p.Name, name) {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		c.Props = nil
		return
	}
	c.Props = out
}
