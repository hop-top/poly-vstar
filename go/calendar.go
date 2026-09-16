// SPDX-License-Identifier: MIT

package vstar

// Calendar is the top-level VCALENDAR container: the PRODID identifying
// the producing system plus the list of contained components (VEVENT,
// VTODO, VJOURNAL, VFREEBUSY, VTIMEZONE, …) per RFC 5545 §3.4.
//
// Calendar is intentionally small at this layer; canonicalization,
// validation, and supersession projection live in their own packages.
type Calendar struct {
	ProdID     string
	Components []Component
}

// Find returns the first component whose UID property matches uid.
// UID comparison is case-sensitive per RFC 5545 §3.8.4.7 (UIDs are
// opaque identifiers, not user-facing text). Returns the zero
// Component and ok=false when none match.
func (c *Calendar) Find(uid string) (Component, bool) {
	for _, comp := range c.Components {
		if comp.UID() == uid {
			return comp, true
		}
	}
	return Component{}, false
}

// Append adds comp to the calendar's component list.
func (c *Calendar) Append(comp Component) {
	c.Components = append(c.Components, comp)
}

// Filter returns every component of the requested type. Returns nil
// when none match (callers can treat nil as "no such components").
// CompType comparison is case-sensitive: components carry the wire
// string verbatim and constants are uppercase per RFC 5545 §3.6.
func (c *Calendar) Filter(t CompType) []Component {
	var out []Component
	for _, comp := range c.Components {
		if comp.Type == t {
			out = append(out, comp)
		}
	}
	return out
}
