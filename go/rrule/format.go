// SPDX-License-Identifier: MIT

package rrule

import (
	"strconv"
	"strings"

	vstar "hop.top/vstar"
)

// String renders the Rule as an RFC 5545 §3.3.10 RRULE property
// value with no "RRULE:" prefix, e.g.
// "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE".
//
// # Rule-part order
//
// RFC 5545 §3.3.10 imposes NO order on rule-parts — a producer may
// emit them any way it likes and the value means the same thing.
// spec/03 §RRULE wire form does impose one on emitters, so that
// identical logical content produces byte-identical output. String
// follows it, and the order is part of the API contract, not an
// implementation detail:
//
//	FREQ, INTERVAL, UNTIL, COUNT, BYMONTH, BYWEEKNO, BYYEARDAY,
//	BYMONTHDAY, BYDAY, BYHOUR, BYMINUTE, BYSECOND, BYSETPOS, WKST
//
// The order is the one RFC 5545 §3.3.10's own `recur` ABNF lists
// the rule-parts in, which makes it the least surprising choice
// and matches what most producers already emit. Reading it
// left-to-right also tracks decreasing period length — FREQ and
// its modifiers, then the termination bound, then the BY-* filters
// from coarsest (month) to finest (second) — with BYSETPOS last
// because it is applied last, as a positional filter over the
// fully expanded set, and WKST last of all because it is a
// modifier on the whole rule rather than a filter.
//
// # Defaults
//
// Rule-parts equal to their RFC default are omitted: INTERVAL=1
// and WKST=MO (spec/03 §RRULE wire form). This keeps the wire form
// minimal and means two Rules that differ only in whether the
// producer spelled out a default render identically.
//
// List values keep the order the Rule holds them in — RFC 5545
// gives BY-* lists no ordering semantics, and preserving producer
// order round-trips faithfully through ParseRRule. Callers wanting
// order-insensitive equality should compare parsed Rules, not
// strings.
//
// String returns "" for a Rule that cannot produce a valid RRULE
// value (FREQ unset), rather than emitting a partial value that
// would fail to re-parse.
func (r Rule) String() string {
	if r.Freq == FreqInvalid {
		return ""
	}

	var b strings.Builder
	b.WriteString("FREQ=")
	b.WriteString(r.Freq.String())

	// INTERVAL — omitted at its RFC default of 1.
	if r.Interval > 1 {
		b.WriteString(";INTERVAL=")
		b.WriteString(strconv.Itoa(r.Interval))
	}
	// UNTIL and COUNT are mutually exclusive per RFC 5545; emit
	// whichever is set.
	if !r.Until.IsZero() {
		b.WriteString(";UNTIL=")
		b.WriteString(vstar.FormatTime(r.Until))
	}
	if r.Count > 0 {
		b.WriteString(";COUNT=")
		b.WriteString(strconv.Itoa(r.Count))
	}

	writeIntList(&b, "BYMONTH", r.ByMonth)
	writeIntList(&b, "BYWEEKNO", r.ByWeekNo)
	writeIntList(&b, "BYYEARDAY", r.ByYearDay)
	writeIntList(&b, "BYMONTHDAY", r.ByMonthDay)
	writeByDayList(&b, r.ByDay)
	writeIntList(&b, "BYHOUR", r.ByHour)
	writeIntList(&b, "BYMINUTE", r.ByMinute)
	writeIntList(&b, "BYSECOND", r.BySecond)
	writeIntList(&b, "BYSETPOS", r.BySetPos)

	// WKST — omitted at its RFC default of MO.
	if r.WeekStart != MO {
		b.WriteString(";WKST=")
		b.WriteString(r.WeekStart.String())
	}

	return b.String()
}

// writeIntList appends ";NAME=v1,v2,..." when the list is
// non-empty, and writes nothing otherwise.
func writeIntList(b *strings.Builder, name string, vals []int) {
	if len(vals) == 0 {
		return
	}
	b.WriteByte(';')
	b.WriteString(name)
	b.WriteByte('=')
	for i, v := range vals {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Itoa(v))
	}
}

// writeByDayList appends ";BYDAY=[<ordinal>]<weekday>,..." when the
// list is non-empty. A zero ordinal means "every weekday of this
// kind" and is rendered with no numeric prefix (per RFC 5545, the
// explicit "0" prefix is invalid).
func writeByDayList(b *strings.Builder, vals []ByDay) {
	if len(vals) == 0 {
		return
	}
	b.WriteString(";BYDAY=")
	for i, bd := range vals {
		if i > 0 {
			b.WriteByte(',')
		}
		if bd.Ordinal != 0 {
			b.WriteString(strconv.Itoa(bd.Ordinal))
		}
		b.WriteString(bd.Weekday.String())
	}
}

// Property renders the Rule as a complete vstar.Property ready to
// attach to a Component. Returns the zero Property when the Rule
// cannot produce a valid value (see String).
func (r Rule) Property() vstar.Property {
	v := r.String()
	if v == "" {
		return vstar.Property{}
	}
	return vstar.Property{Name: "RRULE", Value: v}
}
