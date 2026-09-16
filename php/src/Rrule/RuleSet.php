<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Rrule;

use HopTop\Vstar\Component;
use HopTop\Vstar\Exception\MalformedException;
use HopTop\Vstar\Exception\UnboundedExpansionException;
use HopTop\Vstar\Exception\UnsupportedRRuleException;
use HopTop\Vstar\Property;
use HopTop\Vstar\Time;

/**
 * A complete recurrence definition for one component (RFC 5545 §3.8.5):
 * the DTSTART anchor, an optional RRULE, and the explicit RDATE
 * additions and EXDATE removals layered on top.
 *
 * # The evaluation order is the contract
 *
 * `spec/v1.0/03-canonicalization.md` §Recurrence sets fixes it, and
 * every step is load-bearing:
 *
 * 1. DTSTART is the first occurrence.
 * 2. RRULE, if present, expands from DTSTART.
 * 3. RDATE values are merged in.
 * 4. EXDATE values are removed **last**, so an excluded instant stays
 *    excluded even when an RDATE names it.
 * 5. The result is sorted and de-duplicated by instant.
 *
 * Applying EXDATE before RDATE would resurrect an instant the producer
 * cancelled, which is exactly what `rrule/set/exdate_after_rdate`
 * exists to catch.
 *
 * A set with RDATE and no RRULE is legal and finite. A set with neither
 * is a single non-recurring occurrence at DTSTART.
 *
 * The reference calls this `Set`; the API mapping renames it for every
 * port, where a bare `Set` collides with a builtin -- and in PHP reads
 * as a mutator.
 */
final class RuleSet
{
    /** @var list<\DateTimeImmutable> explicit additions, sorted and de-duplicated */
    public readonly array $rdate;

    /** @var list<\DateTimeImmutable> explicit exclusions, sorted and de-duplicated */
    public readonly array $exdate;

    /**
     * @param \DateTimeImmutable        $dtstart the anchor, and occurrence #1
     * @param ?Rule                     $rrule   the rule, or null for an RDATE-only set
     * @param list<\DateTimeImmutable>  $rdate
     * @param list<\DateTimeImmutable>  $exdate
     */
    public function __construct(
        public readonly \DateTimeImmutable $dtstart,
        public readonly ?Rule $rrule = null,
        array $rdate = [],
        array $exdate = [],
    ) {
        $this->rdate = Rrule::sortDedupe($rdate);
        $this->exdate = Rrule::sortDedupe($exdate);
    }

    /**
     * Build a set from a component's DTSTART, RRULE, RDATE and EXDATE
     * properties.
     *
     * EXDATE and RDATE may each appear several times and may each carry
     * several comma-separated values; every value accumulates.
     *
     * Recurrence sets are UTC form #2 only by spec, so a `VALUE=DATE` or
     * `TZID` EXDATE/RDATE is `ErrUnsupportedRRule`. Failing closed is
     * deliberate: silently dropping an unparseable EXDATE would surface
     * an occurrence the producer explicitly cancelled.
     */
    public static function fromComponent(Component $c): self
    {
        $dtstart = new \DateTimeImmutable('@0');
        $rrule = null;
        $rdate = [];
        $exdate = [];

        $start = $c->get('DTSTART');

        if ($start !== null) {
            $t = Time::parseTime($start->value);

            if ($t === null) {
                throw new MalformedException(
                    'rrule: DTSTART "' . $start->value . '" is not RFC 5545 form #2',
                );
            }

            $dtstart = $t;
        }

        foreach ($c->props as $p) {
            switch (strtoupper($p->name)) {
                case 'RRULE':
                    $rrule = Rrule::parse($p->value);

                    break;

                case 'RDATE':
                    foreach (self::parseDateListProperty($p) as $t) {
                        $rdate[] = $t;
                    }

                    break;

                case 'EXDATE':
                    foreach (self::parseDateListProperty($p) as $t) {
                        $exdate[] = $t;
                    }

                    break;

                default:
                    break;
            }
        }

        return new self($dtstart, $rrule, $rdate, $exdate);
    }

    /**
     * Up to `$limit` occurrences of the set, with the same `complete`
     * contract as {@see Rrule::occurrences()}.
     *
     * EXDATE removals do not consume limit slots: the limit bounds
     * *returned* occurrences, so a set whose first hundred rule
     * occurrences are all excluded still yields the hundred-and-first.
     *
     * @return array{occurrences: list<\DateTimeImmutable>, complete: bool}
     */
    public function occurrences(int $limit): array
    {
        if ($limit < 0) {
            throw new UnboundedExpansionException(
                'rrule: RuleSet::occurrences: limit must be >= 0, got ' . $limit,
            );
        }

        if ($this->rrule !== null) {
            Evaluator::checkExpandable($this->rrule);
        }

        if ($limit === 0) {
            return ['occurrences' => [], 'complete' => false];
        }

        $explicit = $this->explicit();
        $out = [];
        $ei = 0;
        $truncated = false;

        // EXDATE is consulted here, at emit -- after the rule stream and
        // the explicit stream have been merged. That is what makes the
        // removal last, and an RDATE cannot undo it.
        $emit = function (\DateTimeImmutable $t) use ($limit, &$out): bool {
            if ($this->isExcluded($t)) {
                return true;
            }

            if (count($out) === $limit) {
                return false;
            }

            $out[] = $t;

            return true;
        };

        if ($this->rrule !== null) {
            $capped = Evaluator::walk(
                $this->rrule,
                $this->dtstart,
                static function (\DateTimeImmutable $occ) use ($explicit, $emit, &$ei, &$truncated): bool {
                    // Drain every explicit occurrence sorting before this
                    // one, so the merged stream stays chronological.
                    while ($ei < count($explicit) && $explicit[$ei] < $occ) {
                        if (!$emit($explicit[$ei])) {
                            $truncated = true;

                            return false;
                        }

                        ++$ei;
                    }

                    // The same instant from both streams is one
                    // occurrence.
                    if ($ei < count($explicit) && $explicit[$ei] == $occ) {
                        ++$ei;
                    }

                    if (!$emit($occ)) {
                        $truncated = true;

                        return false;
                    }

                    return true;
                },
            );

            if ($capped) {
                throw Evaluator::iterationCap('RuleSet::occurrences', $this->rrule);
            }

            if ($truncated) {
                return ['occurrences' => $out, 'complete' => false];
            }
        }

        for (; $ei < count($explicit); ++$ei) {
            if (!$emit($explicit[$ei])) {
                return ['occurrences' => $out, 'complete' => false];
            }
        }

        return ['occurrences' => $out, 'complete' => true];
    }

    /**
     * Every occurrence of the set in the half-open window
     * `[$start, $end)`, with RDATE merged and EXDATE removed last.
     *
     * @return list<\DateTimeImmutable>
     */
    public function between(\DateTimeImmutable $start, ?\DateTimeImmutable $end): array
    {
        if ($end === null) {
            throw new UnboundedExpansionException('rrule: RuleSet::between: end must be present');
        }

        if ($end <= $start) {
            throw new UnboundedExpansionException(sprintf(
                'rrule: RuleSet::between: end %s must be after start %s',
                Time::formatTime($end),
                Time::formatTime($start),
            ));
        }

        $merged = [];

        if ($this->rrule !== null) {
            Evaluator::checkExpandable($this->rrule);

            $capped = Evaluator::walk(
                $this->rrule,
                $this->dtstart,
                static function (\DateTimeImmutable $occ) use ($end, &$merged): bool {
                    if ($occ >= $end) {
                        return false;
                    }

                    $merged[] = $occ;

                    return true;
                },
            );

            if ($capped) {
                throw Evaluator::iterationCap('RuleSet::between', $this->rrule);
            }
        }

        foreach ($this->explicit() as $t) {
            $merged[] = $t;
        }

        $windowed = [];

        foreach ($merged as $t) {
            if ($t >= $start && $t < $end && !$this->isExcluded($t)) {
                $windowed[] = $t;
            }
        }

        return Rrule::sortDedupe($windowed);
    }

    /**
     * DTSTART plus every RDATE, sorted and de-duplicated -- the
     * occurrences that exist independently of any rule.
     *
     * @return list<\DateTimeImmutable>
     */
    private function explicit(): array
    {
        return Rrule::sortDedupe([$this->dtstart, ...$this->rdate]);
    }

    /**
     * Whether an EXDATE names this instant.
     *
     * Comparison is by instant, not object identity: two
     * `DateTimeImmutable` values naming the same second are the same
     * occurrence, and `in_array` with strict comparison would miss that.
     */
    private function isExcluded(\DateTimeImmutable $t): bool
    {
        foreach ($this->exdate as $x) {
            if ($x == $t) {
                return true;
            }
        }

        return false;
    }

    /**
     * Validate an EXDATE/RDATE property's parameters, then parse its
     * list.
     *
     * @return list<\DateTimeImmutable>
     */
    private static function parseDateListProperty(Property $p): array
    {
        $name = strtoupper($p->name);

        foreach ($p->params as $param) {
            switch (strtoupper($param->name)) {
                case 'VALUE':
                    if (strcasecmp($param->value, 'DATE-TIME') !== 0) {
                        // A date-only value would need a time-of-day
                        // guessed for it.
                        throw new UnsupportedRRuleException(sprintf(
                            'rrule: %s VALUE=%s is outside the supported value types (DATE-TIME only)',
                            $name,
                            $param->value,
                        ));
                    }

                    break;

                case 'TZID':
                    // EXDATE and RDATE are not on the
                    // datetime-resolution allow-list, so a zoned value
                    // arrives here unresolved and would need the
                    // calendar's VTIMEZONE registry -- which a
                    // component-scoped constructor cannot reach.
                    throw new UnsupportedRRuleException(sprintf(
                        'rrule: %s TZID=%s requires VTIMEZONE resolution unavailable at component scope',
                        $name,
                        $param->value,
                    ));

                default:
                    break;
            }
        }

        return Rrule::parseDateTimeList($p->value);
    }
}
