// SPDX-License-Identifier: MIT

package vstar

import "strings"

// Component is a single iCalendar component (VEVENT, VTODO, VCARD,
// VCALENDAR, etc.) in struct form: a typed identifier, a list of
// properties, and a list of nested sub-components (e.g. VTIMEZONE
// inside VCALENDAR; VALARM inside VEVENT).
type Component struct {
	Type  CompType
	Props []Property
	Sub   []Component
}

// Get returns the first property whose Name matches name (case-
// insensitive per RFC 5545 §3.1). Returns the zero Property and
// ok=false when none match.
func (c *Component) Get(name string) (Property, bool) {
	for _, p := range c.Props {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return Property{}, false
}

// GetAll returns all properties whose Name matches name (case-
// insensitive). Returns nil when none match (not an empty slice),
// so callers can treat the zero return as "no such property".
func (c *Component) GetAll(name string) []Property {
	var out []Property
	for _, p := range c.Props {
		if strings.EqualFold(p.Name, name) {
			out = append(out, p)
		}
	}
	return out
}

// Set replaces every property matching p.Name (case-insensitive)
// with a single copy of p. If no property matches, p is appended.
func (c *Component) Set(p Property) {
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

// Add appends p to the component's property list without touching
// existing properties of the same name.
func (c *Component) Add(p Property) {
	c.Props = append(c.Props, p)
}

// Remove deletes every property matching name (case-insensitive).
// No-op when none match.
func (c *Component) Remove(name string) {
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

// UID returns the component's UID property value, or "" when absent.
// Convenience for the universally-required identifier per RFC 5545
// §3.8.4.7 / RFC 6350 §6.7.6.
func (c *Component) UID() string {
	if p, ok := c.Get("UID"); ok {
		return p.Value
	}
	return ""
}

// DTSTAMPRaw returns the component's DTSTAMP property value as the
// raw RFC 5545 wire string, or "" when absent. For the parsed
// time.Time form (the ergonomic primary), use DTSTAMP.
func (c *Component) DTSTAMPRaw() string {
	if p, ok := c.Get("DTSTAMP"); ok {
		return p.Value
	}
	return ""
}
