// SPDX-License-Identifier: MIT

package diff

import (
	"bytes"

	vstar "hop.top/vstar"
	"hop.top/vstar/canonical"
)

// Component reports whether two components are semantically equal:
// canonical.Component(a) and canonical.Component(b) yield identical
// bytes.
//
// Equality therefore ignores property order, parameter order, and
// the X-VSTAR-HASH property (which canonical strips per spec/03
// rule 7). Datetime forms compare equal when canonical resolves
// them to the same UTC representation; for components carrying
// TZID-tagged datetimes that need a sibling VTIMEZONE, prefer
// CalendarInContext-style comparison via Calendar.
//
// Performance: this routes through full canonical-byte comparison
// (correctness > speed).
func Component(a, b vstar.Component) bool {
	return bytes.Equal(canonical.Component(a), canonical.Component(b))
}

// Card reports whether two cards are semantically equal via
// canonical.Card. Same X-VSTAR-HASH / ordering rules apply.
func Card(a, b vstar.Card) bool {
	return bytes.Equal(canonical.Card(a), canonical.Card(b))
}

// Calendar reports whether two calendars are semantically equal via
// canonical.Calendar. This automatically handles top-level component
// reordering (canonical sorts by UID/TZID per spec/03 rule 6) and
// TZID-tagged datetime resolution against the calendar's VTIMEZONE
// registry.
func Calendar(a, b vstar.Calendar) bool {
	return bytes.Equal(canonical.Calendar(a), canonical.Calendar(b))
}
