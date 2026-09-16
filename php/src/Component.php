<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar;

/**
 * A single iCalendar component -- VEVENT, VTODO, VCALENDAR, VTIMEZONE and
 * the rest -- as a typed identifier, a list of properties, and a list of
 * nested sub-components (VTIMEZONE inside VCALENDAR; VALARM inside
 * VEVENT).
 *
 * `$type` is the wire string rather than a {@see CompType} instance
 * because the component vocabulary is open: `STANDARD` and `DAYLIGHT`
 * inside a VTIMEZONE have no CompType case, and the conformance corpus
 * contains both. Compare against `CompType::Event->value`, or narrow with
 * `CompType::tryFrom($c->type)`.
 *
 * `$props` and `$sub` preserve wire order. Nothing in this class sorts:
 * ordering is canonicalization's business, and a model that reordered on
 * read would make parse → encode lossy in a way no round-trip test can
 * see.
 */
final class Component
{
    /** The wire-string component type, e.g. `VEVENT`. */
    public string $type;

    /** @var list<Property> properties in wire order */
    public array $props;

    /** @var list<Component> sub-components in wire order */
    public array $sub;

    /**
     * @param list<Property>  $props
     * @param list<Component> $sub
     */
    public function __construct(
        CompType|string $type = CompType::Calendar,
        array $props = [],
        array $sub = [],
    ) {
        $this->type = $type instanceof CompType ? $type->value : $type;
        $this->props = $props;
        $this->sub = $sub;
    }

    /**
     * The first property whose name matches, case-insensitively per
     * RFC 5545 §3.1. Null when none match.
     */
    public function get(string $name): ?Property
    {
        foreach ($this->props as $p) {
            if (strcasecmp($p->name, $name) === 0) {
                return $p;
            }
        }

        return null;
    }

    /**
     * Every property whose name matches, case-insensitively, in wire
     * order. An empty list when none match.
     *
     * @return list<Property>
     */
    public function getAll(string $name): array
    {
        $out = [];

        foreach ($this->props as $p) {
            if (strcasecmp($p->name, $name) === 0) {
                $out[] = $p;
            }
        }

        return $out;
    }

    /**
     * Replace every property matching `$p`'s name (case-insensitively)
     * with a single copy of `$p`, in the position the first match held.
     * Appends when nothing matches.
     */
    public function set(Property $p): void
    {
        $out = [];
        $replaced = false;

        foreach ($this->props as $existing) {
            if (strcasecmp($existing->name, $p->name) === 0) {
                if (!$replaced) {
                    $out[] = $p;
                    $replaced = true;
                }

                continue;
            }

            $out[] = $existing;
        }

        if (!$replaced) {
            $out[] = $p;
        }

        $this->props = $out;
    }

    /**
     * Append a property without touching existing properties of the same
     * name -- the multi-valued counterpart to {@see self::set()}.
     */
    public function add(Property $p): void
    {
        $this->props[] = $p;
    }

    /**
     * Delete every property matching the name, case-insensitively. A
     * no-op when none match.
     */
    public function remove(string $name): void
    {
        $out = [];

        foreach ($this->props as $p) {
            if (strcasecmp($p->name, $name) === 0) {
                continue;
            }

            $out[] = $p;
        }

        $this->props = $out;
    }

    /**
     * The UID property value, or the empty string when absent --
     * convenience for the universally-required identifier per RFC 5545
     * §3.8.4.7 / RFC 6350 §6.7.6.
     */
    public function uid(): string
    {
        $p = $this->get('UID');

        return $p === null ? '' : $p->value;
    }

    /**
     * The DTSTAMP property value as the raw RFC 5545 wire string, or the
     * empty string when absent.
     */
    public function dtstampRaw(): string
    {
        $p = $this->get('DTSTAMP');

        return $p === null ? '' : $p->value;
    }

    /**
     * Whether the named property is present and carries `VALUE=DATE` --
     * i.e. whether its value is a calendar date rather than an instant.
     *
     * This is the branch point for a caller that does not know the wire
     * form up front. False for an absent property.
     */
    public function isDateOnly(string $name): bool
    {
        $p = $this->get($name);

        return $p !== null && self::hasValueDate($p);
    }

    /**
     * The DTSTART value as a calendar date, when the property carries
     * `VALUE=DATE` (an all-day event or task).
     *
     * Null when DTSTART is absent, is a DATE-TIME, or carries a malformed
     * DATE. A midnight DATE-TIME does **not** surface here -- see
     * {@see VDate} for why the two are not interchangeable.
     */
    public function dtstartDate(): ?VDate
    {
        return $this->dateProp('DTSTART');
    }

    /**
     * The DTEND value as a calendar date; semantics match
     * {@see self::dtstartDate()}.
     *
     * Note RFC 5545 §3.6.1: for an all-day event DTEND is EXCLUSIVE -- a
     * one-day event on the 15th has `DTEND;VALUE=DATE:20260516`. This
     * reports the wire value as written and does not adjust it.
     */
    public function dtendDate(): ?VDate
    {
        return $this->dateProp('DTEND');
    }

    /**
     * The VTODO DUE value as a calendar date -- the all-day-task reader.
     */
    public function dueDate(): ?VDate
    {
        return $this->dateProp('DUE');
    }

    /**
     * The VTODO COMPLETED value as a calendar date.
     *
     * RFC 5545 §3.8.2.1 defines COMPLETED as DATE-TIME only, so a
     * `VALUE=DATE` COMPLETED is non-conforming input. This accessor
     * exists for symmetry with the other three and to let a reader
     * recover such a value rather than lose it.
     */
    public function completedDate(): ?VDate
    {
        return $this->dateProp('COMPLETED');
    }

    /**
     * Write an all-day DTSTART: the value in RFC 5545 §3.3.4 DATE form
     * plus the required `VALUE=DATE` parameter. The zero VDate removes
     * the property.
     */
    public function setDtstartDate(VDate $d): void
    {
        $this->setOrClearDate('DTSTART', $d);
    }

    /**
     * Write an all-day DTEND; semantics match
     * {@see self::setDtstartDate()}.
     *
     * Per RFC 5545 §3.6.1 the all-day DTEND is EXCLUSIVE: to express a
     * one-day event on the 15th, pass the 16th. This writes what it is
     * given and does not adjust.
     */
    public function setDtendDate(VDate $d): void
    {
        $this->setOrClearDate('DTEND', $d);
    }

    /**
     * Write an all-day DUE -- the all-day-task writer.
     */
    public function setDueDate(VDate $d): void
    {
        $this->setOrClearDate('DUE', $d);
    }

    /**
     * Write a date-only COMPLETED. RFC 5545 §3.8.2.1 mandates DATE-TIME
     * for COMPLETED, so this emits non-conforming output; it exists for
     * symmetry.
     */
    public function setCompletedDate(VDate $d): void
    {
        $this->setOrClearDate('COMPLETED', $d);
    }

    /**
     * The DTSTART value as an instant, resolving a TZID-bearing local time
     * against `$cal`'s VTIMEZONE registry.
     *
     * The calendar argument is not optional plumbing: the registry lives
     * on the Calendar, not here. Null for an absent property, a
     * `VALUE=DATE` property (read those with {@see self::dtstartDate()}),
     * or a value neither form #2 nor resolvable.
     */
    public function dtstart(Calendar $cal): ?\DateTimeImmutable
    {
        return Time::dtstart($this, $cal);
    }

    /**
     * The DTEND value as an instant; semantics match
     * {@see self::dtstart()}.
     */
    public function dtend(Calendar $cal): ?\DateTimeImmutable
    {
        return Time::dtend($this, $cal);
    }

    /**
     * The VTODO DUE value as an instant.
     */
    public function due(Calendar $cal): ?\DateTimeImmutable
    {
        return Time::due($this, $cal);
    }

    /**
     * The COMPLETED value as an instant.
     */
    public function completed(Calendar $cal): ?\DateTimeImmutable
    {
        return Time::completed($this, $cal);
    }

    /**
     * The DTSTAMP value as an instant.
     *
     * No calendar argument: RFC 5545 §3.8.7.2 requires DTSTAMP to be UTC,
     * so there is never a zone to resolve.
     */
    public function dtstamp(): ?\DateTimeImmutable
    {
        return Time::dtstamp($this);
    }

    /**
     * Write DTSTART as a UTC form #2 instant, dropping any parameters the
     * property carried.
     */
    public function setDtstart(\DateTimeInterface $t): void
    {
        Time::setDtstart($this, $t);
    }

    /**
     * Write DTEND as a UTC form #2 instant.
     */
    public function setDtend(\DateTimeInterface $t): void
    {
        Time::setDtend($this, $t);
    }

    /**
     * Write DUE as a UTC form #2 instant.
     */
    public function setDue(\DateTimeInterface $t): void
    {
        Time::setDue($this, $t);
    }

    /**
     * Write COMPLETED as a UTC form #2 instant.
     */
    public function setCompleted(\DateTimeInterface $t): void
    {
        Time::setCompleted($this, $t);
    }

    /**
     * Whether a property carries `VALUE=DATE`.
     *
     * Both halves fold case: parameter names are case-insensitive per
     * RFC 5545 §3.2, and the VALUE parameter's argument is a registered
     * value-type token (§3.2.20), likewise case-insensitive.
     */
    private static function hasValueDate(Property $p): bool
    {
        $v = $p->param(Vstar::VALUE_PARAM);

        return $v !== null && strcasecmp($v, Vstar::VALUE_DATE) === 0;
    }

    /**
     * Parse a date-bearing property's value. Null for a missing property,
     * one that does not declare `VALUE=DATE`, or a malformed DATE.
     *
     * The `VALUE=DATE` requirement is deliberate, not merely defensive.
     * An untagged `20260515` declares itself DATE-TIME by default and is
     * simply torn data; promoting it to a VDate would be exactly the
     * silent coercion the parsers exist to prevent. A producer that means
     * DATE must say so.
     */
    private function dateProp(string $name): ?VDate
    {
        $p = $this->get($name);

        if ($p === null || !self::hasValueDate($p)) {
            return null;
        }

        return VDate::parse($p->value);
    }

    /**
     * Write a date-only value, or remove the property when `$d` is the
     * zero date.
     *
     * The written property carries exactly one parameter, `VALUE=DATE`,
     * and nothing else. Dropping pre-existing parameters is not tidiness
     * but a requirement: a stale TZID from a prior local-time form would
     * be meaningless on a DATE (RFC 5545 §3.2.19 scopes TZID to DATE-TIME
     * and TIME values), and a stale parameter set would make the canonical
     * bytes depend on the property's edit history.
     */
    private function setOrClearDate(string $name, VDate $d): void
    {
        if ($d->isZero()) {
            $this->remove($name);

            return;
        }

        $this->set(new Property(
            $name,
            [new Param(Vstar::VALUE_PARAM, Vstar::VALUE_DATE)],
            VDate::format($d),
        ));
    }
}
