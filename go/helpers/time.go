// SPDX-License-Identifier: MIT

package helpers

import (
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/hashing"
)

// Due is a thin wrapper over Component.DUE that funnels callers
// through the helpers package so the symmetric SetDue counterpart
// has a matching reader. Equivalent to c.DUE(cal).
func Due(c vstar.Component, cal vstar.Calendar) (time.Time, bool) {
	return c.DUE(cal)
}

// SetDue writes DUE in UTC form #2 and refreshes X-VSTAR-HASH last.
// No-op when c is nil. Passing the zero time removes the property
// (delegated through Component.SetDUE).
func SetDue(c *vstar.Component, t time.Time) {
	if c == nil {
		return
	}
	c.SetDUE(t)
	hashing.SetXVSTAR(c)
}
