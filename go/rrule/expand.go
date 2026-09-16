// SPDX-License-Identifier: MIT

package rrule

import (
	"errors"
	"fmt"
	"iter"
	"slices"
	"time"
)

// ErrUnboundedExpansion signals an expansion request that has no
// terminating condition: neither the rule (UNTIL/COUNT), nor a
// caller-supplied limit, nor a finite window bounds the result.
//
// Expansion into a slice MUST be bounded — a FREQ=DAILY rule with
// no UNTIL and no COUNT has infinitely many occurrences. Callers
// that genuinely want an open-ended series use All, which is lazy.
//
// Consumers match with errors.Is.
var ErrUnboundedExpansion = errors.New("rrule: unbounded expansion request")

// All returns a lazy iterator over every occurrence of rule
// starting at dtstart, in chronological order.
//
// Termination contract:
//
//   - The sequence ends on its own when the rule terminates (UNTIL
//     reached — inclusive per RFC 5545 — or COUNT exhausted).
//   - A rule with neither UNTIL nor COUNT is INFINITE. Ranging
//     over All without breaking will not return. This is the
//     documented, intended behavior of a lazy iterator; use
//     Occurrences or Between when you need a bounded slice.
//   - The sequence also ends when the evaluator's iteration bound
//     is reached: MaxIterations consecutive FREQ periods without an
//     occurrence (an unsatisfiable BY-* combination such as
//     FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30). A range-over-func
//     iterator has nowhere to put an error, so All cannot tell
//     this apart from termination; Occurrences and Between report
//     it as ErrIterationCap.
//   - An invalid rule (FREQ unset, INTERVAL < 1) yields nothing.
//     Use Occurrences or Between if you need that reported as an
//     error.
//
// The first occurrence is dtstart itself whenever dtstart
// satisfies the rule's BY-* filters, per RFC 5545 §3.3.10.
//
// Time-zone semantics match NextOccurrence: candidates carry
// dtstart's location, so a zoned dtstart preserves wall-clock time
// across DST transitions and the UTC offset shifts accordingly.
func All(rule Rule, dtstart time.Time) iter.Seq[time.Time] {
	return func(yield func(time.Time) bool) {
		walk(rule, dtstart, yield)
	}
}

// Occurrences returns up to limit occurrences of rule starting at
// dtstart, in chronological order.
//
// The complete return value distinguishes the two ways expansion
// stops:
//
//   - complete == true: the rule itself terminated (UNTIL reached
//     or COUNT exhausted) within the limit, so the returned slice
//     is the ENTIRE series. Nothing was truncated.
//   - complete == false: the limit was reached first, so more
//     occurrences exist beyond the last element returned.
//
// This is the distinguishable termination signal that a bare
// stepping loop cannot provide: a caller that stops after N steps
// never learns whether N was the whole series or just the first N.
//
// limit must be >= 0. A limit of 0 returns (nil, false, nil) — no
// occurrences, and no claim that the series ended.
//
// Errors:
//
//   - wrapped ErrIterationCap when the evaluator walked
//     MaxIterations consecutive FREQ periods without an occurrence
//     and abandoned the search. The rule has not terminated; the
//     evaluator gave up, and complete is false.
//   - wrapped ErrUnboundedExpansion for a negative limit.
//   - wrapped ErrUnsupportedRRule when rule is not evaluable (FREQ
//     unset or INTERVAL < 1), which happens only for Rule literals
//     that did not go through ParseRRule.
func Occurrences(rule Rule, dtstart time.Time, limit int) (occs []time.Time, complete bool, err error) {
	if limit < 0 {
		return nil, false, fmt.Errorf("rrule: Occurrences: limit must be >= 0, got %d: %w", limit, ErrUnboundedExpansion)
	}
	if err := checkExpandable(rule); err != nil {
		return nil, false, err
	}
	if limit == 0 {
		return nil, false, nil
	}

	out := make([]time.Time, 0, limit)
	truncated := false
	capped := walk(rule, dtstart, func(occ time.Time) bool {
		if len(out) == limit {
			// The walk produced one more than we asked for, so the
			// series definitively continues past the limit.
			truncated = true
			return false
		}
		out = append(out, occ)
		return true
	})
	if capped {
		return nil, false, iterationCapError("Occurrences", rule)
	}
	if len(out) == 0 {
		return nil, !truncated, nil
	}
	return out, !truncated, nil
}

// Between returns every occurrence of rule in the half-open window
// [start, end) — start inclusive, end exclusive — in chronological
// order.
//
// Termination contract: the window itself bounds the result, so
// Between always terminates even for a rule with no UNTIL and no
// COUNT. Expansion stops at the earlier of `end` and the rule's own
// termination.
//
// end must be non-zero and strictly after start; a zero or
// inverted window is a caller error (wrapped ErrUnboundedExpansion)
// rather than a silent empty result — an unbounded window would be
// an infinite expansion request.
//
// The window bounds the result, not the search: a rule that never
// yields never reaches `end`, so the evaluator's iteration bound
// stops it and Between reports a wrapped ErrIterationCap.
//
// Occurrences before dtstart never exist, so a window entirely
// before dtstart returns an empty slice.
func Between(rule Rule, dtstart, start, end time.Time) ([]time.Time, error) {
	if end.IsZero() {
		return nil, fmt.Errorf("rrule: Between: end must be non-zero (an open-ended window is an infinite expansion; use All): %w", ErrUnboundedExpansion)
	}
	if !end.After(start) {
		return nil, fmt.Errorf("rrule: Between: end %s must be after start %s: %w", end, start, ErrUnboundedExpansion)
	}
	if err := checkExpandable(rule); err != nil {
		return nil, err
	}

	out := []time.Time{}
	capped := walk(rule, dtstart, func(occ time.Time) bool {
		if !occ.Before(end) {
			return false
		}
		if !occ.Before(start) {
			out = append(out, occ)
		}
		return true
	})
	if capped {
		return nil, iterationCapError("Between", rule)
	}
	return out, nil
}

// walk is the shared generator behind All, Occurrences, Between and
// Set. It yields every occurrence of rule from dtstart in
// chronological order and reports how the walk ended: capped is
// true when MaxIterations consecutive FREQ periods produced no
// occurrence and the search was abandoned; false when the rule
// terminated (UNTIL, COUNT, or no further period) or yield stopped
// it.
//
// The bound is a starvation guard, not a total-occurrence limit:
// the empty-period counter resets whenever a period yields, so a
// rule that fires regularly runs indefinitely (bounded only by the
// caller's limit, window, or break) while one whose BY-* filters
// can never match still stops. It shares MaxIterations with
// NextOccurrence so the package documents one bound.
//
// An invalid rule (FREQ unset, INTERVAL < 1) yields nothing and is
// not a cap hit; callers that need it reported run checkExpandable
// first.
func walk(rule Rule, dtstart time.Time, yield func(time.Time) bool) (capped bool) {
	if checkExpandable(rule) != nil {
		return false
	}
	emitted := 0
	empty := 0
	current := dtstart

	for empty < MaxIterations {
		occs := periodOccurrences(rule, current, dtstart)
		produced := false
		for _, occ := range occs {
			if occ.Before(dtstart) {
				continue
			}
			// UNTIL is inclusive per RFC 5545 §3.3.10.
			if !rule.Until.IsZero() && occ.After(rule.Until) {
				return false
			}
			produced = true
			emitted++
			if !yield(occ) {
				return false
			}
			if rule.Count > 0 && emitted >= rule.Count {
				return false
			}
		}
		if produced {
			empty = 0
		} else {
			empty++
		}
		next, ok := advance(rule, current)
		if !ok {
			return false
		}
		current = next
	}
	return true
}

// iterationCapError wraps ErrIterationCap with the entry point and
// rule context, mirroring NextOccurrence's own message shape.
func iterationCapError(op string, rule Rule) error {
	return fmt.Errorf("rrule: %s: no occurrence within %d consecutive %s periods: %w",
		op, MaxIterations, rule.Freq, ErrIterationCap)
}

// checkExpandable rejects Rule values the evaluator cannot walk.
// ParseRRule guarantees both conditions hold; a hand-built Rule
// literal may not.
func checkExpandable(rule Rule) error {
	if rule.Freq == FreqInvalid {
		return fmt.Errorf("rrule: FREQ is required: %w", ErrUnsupportedRRule)
	}
	if rule.Interval < 1 {
		return fmt.Errorf("rrule: INTERVAL must be >= 1, got %d: %w", rule.Interval, ErrUnsupportedRRule)
	}
	return nil
}

// periodOccurrences expands one FREQ period into its concrete
// occurrences, sorted chronologically and with BYSETPOS applied.
//
// It is the shared kernel behind walk (and therefore All,
// Occurrences, Between, and Set): the per-period expansion plus
// ordering plus positional filtering that RFC 5545 §3.3.10
// specifies, factored out so the iteration strategy above it can
// vary.
func periodOccurrences(rule Rule, base, dtstart time.Time) []time.Time {
	occs := expand(rule, base, dtstart)
	slices.SortFunc(occs, func(a, b time.Time) int { return a.Compare(b) })
	if len(rule.BySetPos) > 0 {
		occs = applyBySetPos(occs, rule.BySetPos)
	}
	return occs
}
