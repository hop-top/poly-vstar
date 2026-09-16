// SPDX-License-Identifier: MIT

// Package rrule implements a generic RFC 5545 §3.3.10 RRULE parser
// (ParseRRule), boundary check (ValidateRRule), and forward
// evaluator (NextOccurrence) for the scope fixed by
// spec/03 §RRULE parsing scope (BYSETPOS, BYWEEKNO, and BYYEARDAY
// are in scope; FREQ=SECONDLY and RSCALE are not).
//
// Scope (spec/03 §RRULE parsing scope):
//
//   - FREQ values: MINUTELY, HOURLY, DAILY, WEEKLY, MONTHLY, YEARLY.
//   - INTERVAL, UNTIL (form #2 / UTC only), COUNT.
//   - BYDAY, BYMONTH, BYMONTHDAY, BYHOUR, BYMINUTE, BYSECOND.
//   - BYYEARDAY, BYWEEKNO (FREQ=YEARLY only per RFC).
//   - BYSETPOS (positional filter on the expanded BY-set; requires
//     at least one other BY-* clause).
//   - WKST (default MO).
//
// All RFC 5545 §3.3.10 BY-clauses are now supported. Out of scope
// (parser returns ErrUnsupportedRRule):
//
//   - FREQ=SECONDLY.
//   - RSCALE (RFC 7529 — non-Gregorian calendars).
//
// Syntactic violations return vstar.ErrMalformed wrapped via %w.
// Out-of-scope-but-syntactically-valid features return
// ErrUnsupportedRRule (spec/03 §RRULE parsing scope).
//
// The package depends only on the standard library plus the root
// vstar package (for ErrMalformed and ParseTime). It is independent
// of every other vstar subpackage.
package rrule

import (
	"errors"
	"strconv"
	"time"
)

// freqXxx wire tokens for FREQ rule-part values, pulled to
// constants so parse.go and rrule.go agree on the canonical
// spelling and goconst stays quiet.
const (
	freqMinutely = "MINUTELY"
	freqHourly   = "HOURLY"
	freqDaily    = "DAILY"
	freqWeekly   = "WEEKLY"
	freqMonthly  = "MONTHLY"
	freqYearly   = "YEARLY"
	freqSecondly = "SECONDLY"
)

// ErrUnsupportedRRule signals an RRULE that parses syntactically
// but uses a feature spec/03 §RRULE parsing scope explicitly
// defers: FREQ=SECONDLY and RSCALE.
//
// Consumers MUST match with errors.Is — ParseRRule wraps this with
// %w to add the offending rule-part name.
var ErrUnsupportedRRule = errors.New("rrule: feature outside the RRULE parsing scope")

// Freq is the RRULE FREQ value as a typed enum. The zero value is
// FreqInvalid so a Rule literal without an explicit Freq is
// detectable as "unset" by callers.
type Freq int

// Freq values per RFC 5545 §3.3.10, in RFC order (increasing
// period length). SECONDLY is deliberately omitted from the
// accepted set (spec/03 §RRULE parsing scope); the parser returns
// ErrUnsupportedRRule when it encounters it. The wire contract is
// String(), not the integer: nothing compares Freq values ordinally.
const (
	// FreqInvalid is the zero value; a Rule without an explicit
	// Freq is FreqInvalid. ParseRRule never returns a Rule with
	// FreqInvalid — FREQ is required and missing FREQ produces
	// ErrMalformed.
	FreqInvalid Freq = iota
	FreqMinutely
	FreqHourly
	FreqDaily
	FreqWeekly
	FreqMonthly
	FreqYearly
)

// String renders the Freq as its RFC 5545 wire token (e.g.
// "DAILY"). FreqInvalid renders "INVALID"; unknown values render
// "Freq(N)" for debugging.
func (f Freq) String() string {
	switch f {
	case FreqInvalid:
		return "INVALID"
	case FreqMinutely:
		return freqMinutely
	case FreqHourly:
		return freqHourly
	case FreqDaily:
		return freqDaily
	case FreqWeekly:
		return freqWeekly
	case FreqMonthly:
		return freqMonthly
	case FreqYearly:
		return freqYearly
	default:
		return "Freq(" + strconv.Itoa(int(f)) + ")"
	}
}

// Weekday is an RFC 5545 §3.3.10 weekday. Numbering is SU=0..SA=6
// per the RFC (which happens to match the Go time.Weekday
// numbering exactly — see Weekday.ToTime if you need explicit
// conversion to the standard library type).
type Weekday int

// Weekday symbols per RFC 5545 §3.3.10.
const (
	SU Weekday = iota
	MO
	TU
	WE
	TH
	FR
	SA
)

// String renders the Weekday as its two-letter RFC 5545 wire
// symbol. Unknown values render "Weekday(N)" for debugging.
func (w Weekday) String() string {
	switch w {
	case SU:
		return "SU"
	case MO:
		return "MO"
	case TU:
		return "TU"
	case WE:
		return "WE"
	case TH:
		return "TH"
	case FR:
		return "FR"
	case SA:
		return "SA"
	default:
		return "Weekday(" + strconv.Itoa(int(w)) + ")"
	}
}

// ToTime converts the rrule.Weekday to time.Weekday. Both use the
// same numbering (Sunday=0..Saturday=6) so the conversion is a
// straight cast, exposed here as a typed helper to keep the
// boundary explicit.
func (w Weekday) ToTime() time.Weekday {
	return time.Weekday(w)
}

// ByDay is one entry in an RRULE BYDAY list: an optional ordinal
// and a weekday. Ordinal=0 means "every weekday of this kind in the
// containing FREQ period" (e.g. BYDAY=MO under FREQ=MONTHLY = "every
// Monday of the month"). Ordinal in -53..-1 or 1..53 picks the Nth
// weekday; ordinal 0 with explicit "0" prefix is invalid per RFC.
type ByDay struct {
	// Ordinal is the BYDAY prefix integer, or 0 when omitted.
	// Valid range when non-zero: -53..-1 or 1..53.
	Ordinal int
	// Weekday is the BYDAY weekday symbol.
	Weekday Weekday
}

// Rule is the structured form of an RFC 5545 §3.3.10 RRULE value
// for the accepted subset (spec/03 §RRULE parsing scope).
//
// Field semantics match RFC 5545 directly. Order of rule-parts on
// the wire is irrelevant on parse; struct field order is for
// ergonomic display.
//
// Slices are nil when the corresponding rule-part is absent;
// callers should treat nil and empty equivalently.
type Rule struct {
	// Freq is the FREQ value (required on parse).
	Freq Freq
	// Interval is the INTERVAL value, default 1 when absent. Must
	// be >= 1 — INTERVAL=0 or negative is ErrMalformed.
	Interval int
	// Until is the UNTIL value (RFC 5545 form #2, UTC). Zero when
	// absent. Mutually exclusive with Count.
	Until time.Time
	// Count is the COUNT value, default 0 when absent. Mutually
	// exclusive with Until.
	Count int
	// ByDay is the BYDAY list.
	ByDay []ByDay
	// ByMonth is the BYMONTH list (1..12).
	ByMonth []int
	// ByMonthDay is the BYMONTHDAY list (-31..-1 or 1..31; 0 is
	// rejected by ParseRRule).
	ByMonthDay []int
	// ByHour is the BYHOUR list (0..23).
	ByHour []int
	// ByMinute is the BYMINUTE list (0..59).
	ByMinute []int
	// BySecond is the BYSECOND list (0..60; 60 retained for leap
	// seconds per RFC).
	BySecond []int
	// ByYearDay is the BYYEARDAY list (-366..-1 or 1..366; 0 is
	// rejected by ParseRRule). RFC 5545 §3.3.10 restricts
	// BYYEARDAY to FREQ=YEARLY; combinations with other FREQ
	// values are rejected at parse time.
	ByYearDay []int
	// ByWeekNo is the BYWEEKNO list (-53..-1 or 1..53; 0 is
	// rejected by ParseRRule). ISO 8601 week-numbering. RFC 5545
	// §3.3.10 restricts BYWEEKNO to FREQ=YEARLY; combinations with
	// other FREQ values are rejected at parse time.
	ByWeekNo []int
	// BySetPos is the BYSETPOS list (-366..-1 or 1..366; 0 is
	// rejected by ParseRRule). Positional filter applied AFTER
	// every other BY-* expansion within the FREQ period. RFC 5545
	// requires at least one other BY-* clause when BYSETPOS is
	// used; BYSETPOS-alone is rejected at parse time.
	BySetPos []int
	// WeekStart is the WKST value, default MO when absent.
	WeekStart Weekday
}
