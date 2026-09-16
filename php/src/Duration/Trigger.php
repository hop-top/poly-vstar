<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Duration;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\Exception\MalformedException;
use HopTop\Vstar\Exception\NoAnchorException;
use HopTop\Vstar\Param;
use HopTop\Vstar\Property;
use HopTop\Vstar\Time;
use HopTop\Vstar\Vstar;

/**
 * A parsed RFC 5545 §3.8.6.3 TRIGGER property: either a relative offset
 * from one end of the parent component, or an absolute instant.
 *
 * Exactly one form is populated. When {@see self::$relative} is true,
 * {@see self::$duration} and {@see self::$related} carry the offset and
 * its anchor and {@see self::$absolute} is null; otherwise
 * {@see self::$absolute} carries the instant and {@see self::$duration} is
 * zero-length.
 *
 * `$absolute` is nullable rather than carrying an in-band "no instant"
 * sentinel. A sentinel epoch would be a real, representable instant that a
 * conformant document may legitimately name, so a caller could not tell
 * "absent" from "1970-01-01T00:00:00Z".
 */
final class Trigger
{
    private const PROP_TRIGGER = 'TRIGGER';

    private const PARAM_RELATED = 'RELATED';

    private const VALUE_DURATION = 'DURATION';

    private const VALUE_DATE_TIME = 'DATE-TIME';

    /**
     * @param bool               $relative which of the two forms this is
     * @param VDuration          $duration the offset, meaningful only when `$relative` is true;
     *                                     a negative duration fires before the anchor
     * @param Related            $related  the anchor, meaningful only when `$relative` is true
     * @param ?\DateTimeImmutable $absolute the firing instant, meaningful only when `$relative`
     *                                      is false
     */
    public function __construct(
        public readonly bool $relative = false,
        public readonly VDuration $duration = new VDuration(),
        public readonly Related $related = Related::Start,
        public readonly ?\DateTimeImmutable $absolute = null,
    ) {
    }

    /**
     * Decode a TRIGGER property.
     *
     * The form is chosen as follows:
     *
     * - `VALUE=DURATION`, or no VALUE parameter with a value that parses
     *   as a duration -> relative.
     * - `VALUE=DATE-TIME`, or no VALUE parameter with a value that parses
     *   as an RFC 5545 form #2 instant -> absolute.
     *
     * An explicit VALUE parameter is **authoritative**: a value that
     * contradicts it throws `ErrMalformed` rather than being silently
     * re-read as the other form. With no VALUE parameter the two shapes
     * are unambiguous, so the value itself decides -- producers in the
     * wild routinely omit the parameter.
     *
     * RELATED is honoured on relative triggers only; RFC 5545 §3.2.14
     * scopes it to DURATION-valued triggers, so RELATED on an absolute
     * trigger is rejected. Parameter names and values match
     * case-insensitively per RFC 5545 §3.2.
     */
    public static function parse(Property $p): self
    {
        $declared = $p->param(Vstar::VALUE_PARAM);
        $t = self::fromValue($p, $declared);

        $related = $p->param(self::PARAM_RELATED);

        if ($related === null) {
            return $t;
        }

        if (!$t->relative) {
            throw new MalformedException(
                'trigger: RELATED is meaningful only on a relative trigger (RFC 5545 §3.2.14)',
            );
        }

        $anchor = Related::tryFrom(strtoupper($related));

        if ($anchor === null) {
            throw new MalformedException(
                "trigger: unknown RELATED={$related} (want START or END)",
            );
        }

        return new self(relative: true, duration: $t->duration, related: $anchor);
    }

    /**
     * Render back to the wire property.
     *
     * A relative trigger emits its duration as the value, adding
     * `RELATED=END` only when the anchor is the end -- `RELATED=START` is
     * the RFC default and is left implicit. An absolute trigger emits the
     * UTC form #2 instant and carries `VALUE=DATE-TIME` explicitly, so a
     * consumer never has to infer the form.
     */
    public function toProperty(): Property
    {
        if (!$this->relative) {
            return new Property(
                self::PROP_TRIGGER,
                [new Param(Vstar::VALUE_PARAM, self::VALUE_DATE_TIME)],
                $this->absolute === null ? '' : Time::formatTime($this->absolute),
            );
        }

        return new Property(
            self::PROP_TRIGGER,
            $this->related === Related::End
                ? [new Param(self::PARAM_RELATED, Related::End->value)]
                : [],
            (string) $this->duration,
        );
    }

    /**
     * The instant at which this trigger fires.
     *
     * An absolute trigger returns its instant and ignores both arguments.
     * A relative trigger resolves its anchor from `$parent` -- DTSTART for
     * `RELATED=START`; for `RELATED=END` a VTODO's DUE, else the end
     * {@see Duration::eventEnd()} computes -- then offsets it.
     *
     * `$cal` supplies the VTIMEZONE registry used to resolve a
     * TZID-bearing anchor.
     *
     * Throws `ErrNoAnchor` when the anchor the RELATED parameter selects
     * is absent or unparseable. That is reported rather than silently
     * resolving against the epoch, which would place every such alarm in
     * 1970.
     */
    public function resolve(Component $parent, Calendar $cal): \DateTimeImmutable
    {
        if (!$this->relative) {
            if ($this->absolute === null) {
                throw new NoAnchorException(
                    'duration: absolute trigger carries no instant',
                );
            }

            return $this->absolute;
        }

        $at = $this->anchor($parent, $cal);

        if ($at === null) {
            throw new NoAnchorException(sprintf(
                'duration: %s has no %s anchor for a relative trigger',
                $parent->type,
                $this->related->value,
            ));
        }

        return $this->duration->addTo($at);
    }

    /**
     * The trigger implied by `$p`'s value and its declared VALUE
     * parameter, before RELATED is applied.
     */
    private static function fromValue(Property $p, ?string $declared): self
    {
        if ($declared !== null && strcasecmp($declared, self::VALUE_DURATION) === 0) {
            return new self(relative: true, duration: self::parseOrRethrow($p->value));
        }

        if ($declared !== null && strcasecmp($declared, self::VALUE_DATE_TIME) === 0) {
            $at = Time::parseTime($p->value);

            if ($at === null) {
                throw new MalformedException(sprintf(
                    'trigger: VALUE=DATE-TIME but value "%s" is not an RFC 5545 form #2 instant',
                    $p->value,
                ));
            }

            return new self(absolute: $at);
        }

        if ($declared !== null) {
            throw new MalformedException(
                "trigger: unsupported VALUE={$declared} (want DURATION or DATE-TIME)",
            );
        }

        // No VALUE parameter -- infer from the value's own shape.
        if (Duration::valid($p->value)) {
            return new self(relative: true, duration: Duration::parse($p->value));
        }

        $at = Time::parseTime($p->value);

        if ($at === null) {
            throw new MalformedException(sprintf(
                'trigger: value "%s" is neither a DURATION nor a DATE-TIME',
                $p->value,
            ));
        }

        return new self(absolute: $at);
    }

    /**
     * Parse a duration value, re-wording the failure for the VALUE
     * context so the message names the contradiction rather than the
     * grammar detail.
     */
    private static function parseOrRethrow(string $value): VDuration
    {
        try {
            return Duration::parse($value);
        } catch (MalformedException $cause) {
            throw new MalformedException(
                'trigger: VALUE=DURATION but value is not a duration',
                0,
                $cause,
            );
        }
    }

    /**
     * The parent instant this trigger is measured from.
     */
    private function anchor(Component $parent, Calendar $cal): ?\DateTimeImmutable
    {
        if ($this->related === Related::Start) {
            return Time::dtstart($parent, $cal);
        }

        // RELATED=END: a VTODO ends at DUE; everything else at DTEND or
        // DTSTART + DURATION.
        if ($parent->type === CompType::Todo->value) {
            $at = Time::due($parent, $cal);

            if ($at !== null) {
                return $at;
            }
        }

        return Duration::eventEnd($parent, $cal);
    }
}
