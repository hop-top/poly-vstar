<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Rrule;

use HopTop\Vstar\Exception\MalformedException;
use HopTop\Vstar\Exception\UnsupportedRRuleException;
use HopTop\Vstar\Param;
use HopTop\Vstar\Property;
use HopTop\Vstar\Time;

/**
 * The typed form of a RECURRENCE-ID property (RFC 5545 §3.8.4.4): the
 * instant identifying which instance of a series a component overrides,
 * plus the RANGE parameter.
 *
 * This is parsing and typed access only. Applying overrides -- taking a
 * base component plus its RECURRENCE-ID siblings and producing the
 * effective series -- needs component-level semantics that sit above
 * this layer and are outside its scope.
 */
final class RecurrenceId
{
    /**
     * @param \DateTimeImmutable $time  the identified instance's *original* start
     *                                  instant -- what the base series expands to for
     *                                  it, not the overriding component's own
     *                                  (possibly moved) DTSTART
     * @param RecurrenceRange    $range the RANGE parameter; the default when absent
     */
    public function __construct(
        public readonly \DateTimeImmutable $time,
        public readonly RecurrenceRange $range = RecurrenceRange::ThisInstance,
    ) {
    }

    /**
     * Extract a recurrence id from a RECURRENCE-ID property.
     *
     * `ErrMalformed` for a property that is not RECURRENCE-ID, a value
     * outside form #2, or a RANGE other than `THISANDFUTURE` -- RFC 5545
     * §3.2.13 defines exactly the one token. `ErrUnsupportedRRule` for
     * `VALUE=DATE` or `TZID`, for the same reasons as EXDATE and RDATE.
     */
    public static function parse(Property $p): self
    {
        if (strcasecmp($p->name, 'RECURRENCE-ID') !== 0) {
            throw new MalformedException(
                'rrule: property "' . $p->name . '" is not RECURRENCE-ID',
            );
        }

        $range = RecurrenceRange::ThisInstance;

        foreach ($p->params as $param) {
            switch (strtoupper($param->name)) {
                case 'RANGE':
                    if (strcasecmp($param->value, 'THISANDFUTURE') !== 0) {
                        throw new MalformedException(sprintf(
                            'rrule: RECURRENCE-ID RANGE="%s" invalid (RFC 5545 §3.2.13 defines THISANDFUTURE only)',
                            $param->value,
                        ));
                    }

                    $range = RecurrenceRange::ThisAndFuture;

                    break;

                case 'VALUE':
                    if (strcasecmp($param->value, 'DATE-TIME') !== 0) {
                        throw new UnsupportedRRuleException(sprintf(
                            'rrule: RECURRENCE-ID VALUE=%s is outside the supported value types (DATE-TIME only)',
                            $param->value,
                        ));
                    }

                    break;

                case 'TZID':
                    throw new UnsupportedRRuleException(sprintf(
                        'rrule: RECURRENCE-ID TZID=%s requires VTIMEZONE resolution unavailable at property scope',
                        $param->value,
                    ));

                default:
                    break;
            }
        }

        $t = Time::parseTime($p->value);

        if ($t === null) {
            throw new MalformedException(
                'rrule: RECURRENCE-ID "' . $p->value . '" is not an RFC 5545 form #2 date-time',
            );
        }

        return new self($t, $range);
    }

    /**
     * Render back to wire form.
     *
     * RANGE is emitted only for `THISANDFUTURE`: the default is
     * expressed by omitting the parameter, so emitting a token for it
     * would change the bytes.
     */
    public function toProperty(): Property
    {
        $params = $this->range === RecurrenceRange::ThisAndFuture
            ? [new Param('RANGE', $this->range->value)]
            : [];

        return new Property('RECURRENCE-ID', $params, Time::formatTime($this->time));
    }
}
