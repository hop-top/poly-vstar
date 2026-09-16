<?php

declare(strict_types=1);

// SPDX-License-Identifier: MIT

namespace HopTop\Vstar\Canonical;

use HopTop\Vstar\Calendar;
use HopTop\Vstar\Card;
use HopTop\Vstar\Codec\Rfc5545\Encoder;
use HopTop\Vstar\Component;
use HopTop\Vstar\CompType;
use HopTop\Vstar\Hashing\Hashing;
use HopTop\Vstar\Param;
use HopTop\Vstar\Property;
use HopTop\Vstar\Time;
use HopTop\Vstar\Vstar;

/**
 * The deterministic canonical byte form of V* objects, per spec rules
 * 1-12.
 *
 * Two V* documents containing the same logical content MUST produce
 * identical canonical bytes. That is the invariant the whole
 * specification exists to hold, and it is what makes an `X-VSTAR-HASH`
 * comparable across implementations.
 *
 * Every method here returns a **binary string**. PHP strings are byte
 * arrays, which is the right type: the canonical form is a byte sequence,
 * its final CRLF is part of the value, and comparing it as re-decoded
 * text is how a port passes its own tests while emitting wrong bytes. Do
 * not run these results through `trim()`, `mb_convert_encoding()`, a
 * newline normalizer, or any other text transform before comparing.
 *
 * The transforms are applied to one component in this order, which is the
 * order the spec fixes:
 *
 * 1. **Select** -- drop `X-VSTAR-HASH` (rule 7); reduce `ATTACH` to URI
 *    form by stripping `VALUE=BINARY` and `ENCODING=BASE64` (rule 10).
 * 2. **Resolve datetimes** on the rule-5 allow-list against the
 *    calendar's VTIMEZONE registry, re-emitting as UTC form #2 and
 *    dropping the TZID where resolution succeeds. A `VALUE=DATE`
 *    property is governed by rule 11 and is never resolved; `RRULE`
 *    (rule 8) and `DURATION` (rule 12) pass through verbatim.
 * 3. **Normalize to NFC** -- each property value and each parameter
 *    value, individually. Not names (rule 9).
 * 4. **Sort** -- properties by name, each property's parameters by name
 *    (rule 2); top-level components by UID, or TZID for a VTIMEZONE,
 *    byte-wise on UTF-8, stable, key-less last (rule 6). Sub-components
 *    keep input order.
 * 5. **Assemble** each content line with escaping and parameter quoting
 *    (rule 4).
 * 6. **Fold** each assembled line at 75 octets (rule 3).
 * 7. **Terminate** every physical line with CRLF (rule 1).
 *
 * Steps 5-7 belong to the RFC 5545 encoder, which owns folding, CRLF and
 * TEXT escaping so there is exactly one implementation of each.
 *
 * NFC therefore sits **after** datetime resolution and **before** sorting
 * and folding. Before folding matters: normalization changes a string's
 * UTF-8 length -- `e` + U+0301 is three octets, `é` is two -- so folding
 * a pre-normalization string puts the break at the wrong octet.
 *
 * Nothing here mutates its input.
 */
final class Canonical
{
    /**
     * The rule-5 allow-list: property names whose values are RFC 5545
     * §3.3.5 DATE-TIME and may carry a TZID the canonical form resolves.
     *
     * DTSTAMP is here even though RFC 5545 §3.8.7.2 requires it to be UTC:
     * resolving defensively catches a non-conforming producer rather than
     * emitting a TZID-tagged DTSTAMP unchanged.
     */
    private const DATETIME_PROPERTIES = [
        'DTSTAMP' => true,
        'DTSTART' => true,
        'DTEND' => true,
        'DUE' => true,
        'COMPLETED' => true,
        'RECURRENCE-ID' => true,
        'CREATED' => true,
        'LAST-MODIFIED' => true,
    ];

    /**
     * The canonical byte form of a single component, emitting datetimes
     * **verbatim**.
     *
     * This form is for components carrying no TZID-tagged datetimes. The
     * VTIMEZONE registry lives on the Calendar, not on the Component, so
     * this entry point cannot resolve a TZID reference and emits the value
     * with its TZID parameter retained -- output is non-canonical for such
     * a component. Use {@see self::componentInContext()} to thread the
     * parent calendar.
     *
     * The asymmetry is real, not an overload: a component without a parent
     * calendar genuinely has no registry to consult.
     */
    public static function component(Component $c): string
    {
        return Encoder::encodeComponent(self::prepareComponent($c, new Calendar()));
    }

    /**
     * The canonical byte form of a single component, resolving TZID-tagged
     * datetimes against `$cal`'s VTIMEZONE registry.
     *
     * For each property on the rule-5 allow-list carrying a TZID: on
     * successful resolution the value is re-emitted as UTC form #2 and the
     * TZID parameter is dropped. On failure -- no matching VTIMEZONE, or
     * one outside the spec's VTIMEZONE subset -- the value AND the TZID pass through
     * verbatim. Canonical bytes are not deterministic across calendars
     * carrying different VTIMEZONE definitions in that branch; a producer
     * is expected to ship coverage inside the subset.
     *
     * A value already in UTC form #2, and one with no TZID at all, pass
     * through unchanged -- there is nothing to resolve.
     *
     * STANDARD and DAYLIGHT children inside a VTIMEZONE carry a wall-clock
     * DTSTART that defines the transition rule itself. Those are
     * deliberately not TZID-tagged and pass through by design.
     */
    public static function componentInContext(Component $c, Calendar $cal): string
    {
        return Encoder::encodeComponent(self::prepareComponent($c, $cal));
    }

    /**
     * The canonical byte form of a full VCALENDAR.
     *
     * Top-level components are sorted per rule 6 and each is prepared
     * against the calendar's own VTIMEZONE registry, so a TZID-bearing
     * datetime in any child resolves against a VTIMEZONE in the same
     * document.
     *
     * The PRODID value is NFC-normalized here; the encoder owns its TEXT
     * escaping, so no pre-escaping happens at this layer.
     */
    public static function calendar(Calendar $c): string
    {
        $prepared = [];

        foreach (self::sortedComponents($c->components) as $sub) {
            $prepared[] = self::prepareComponent($sub, $c);
        }

        return Encoder::encode(new Calendar(self::nfc($c->prodId), $prepared));
    }

    /**
     * The canonical byte form of a single VCARD:
     *
     * ```
     * BEGIN:VCARD
     * VERSION:4.0
     * <properties sorted by name; UID is one of them>
     * END:VCARD
     * ```
     *
     * VERSION is promoted ahead of alphabetical order, because RFC 6350
     * §3.3 requires it immediately after `BEGIN:VCARD`. The card's UID and
     * KIND are emitted as properties, or absorbed when `$c->props` already
     * carries one of the same name.
     *
     * A vCard has no datetime or TZID concerns, so there is no context
     * form.
     */
    public static function card(Card $c): string
    {
        $props = [new Property('VERSION', [], '4.0')];

        if ($c->uid !== '' && !self::hasProp($c->props, 'UID')) {
            $props[] = new Property('UID', [], $c->uid);
        }

        if ($c->kind !== null && !self::hasProp($c->props, 'KIND')) {
            $props[] = new Property('KIND', [], $c->kind->value);
        }

        foreach ($c->props as $p) {
            $props[] = $p;
        }

        // A wire-string component type, so the encoder emits BEGIN:VCARD
        // and END:VCARD. The vCard shares the iCalendar content-line
        // grammar, so it shares the encoder rather than duplicating the
        // fold and escape logic.
        $prepared = self::prepareComponent(new Component('VCARD', $props), new Calendar());

        $prepared->props = self::versionFirst($prepared->props);

        return Encoder::encodeComponent($prepared);
    }

    /**
     * A copy of `$props` with the first VERSION property moved to the
     * front, per RFC 6350 §3.3. Every other property keeps its relative
     * order.
     *
     * @param list<Property> $props
     *
     * @return list<Property>
     */
    private static function versionFirst(array $props): array
    {
        $version = null;
        $rest = [];

        foreach ($props as $p) {
            if ($version === null && strcasecmp($p->name, 'VERSION') === 0) {
                $version = $p;

                continue;
            }

            $rest[] = $p;
        }

        return $version === null ? $rest : array_merge([$version], $rest);
    }

    /**
     * A copy of `$c` with every canonicalization transform applied except
     * assembly, folding and CRLF, which the encoder owns.
     *
     * Sub-components are prepared recursively and are NOT sorted: they
     * have no natural sort key, so rule 6 preserves their input order.
     */
    private static function prepareComponent(Component $c, Calendar $cal): Component
    {
        $props = [];

        foreach ($c->props as $p) {
            // Rule 7: the hash property never contributes to the bytes its
            // own value is computed over.
            if (strcasecmp($p->name, Hashing::X_VSTAR_HASH_PROPERTY) === 0) {
                continue;
            }

            $props[] = self::prepareProperty($p, $cal);
        }

        $sub = [];

        foreach ($c->sub as $s) {
            $sub[] = self::prepareComponent($s, $cal);
        }

        return new Component($c->type, self::stableSortByUpperName($props), $sub);
    }

    /**
     * A copy of `$p` with the value and parameter transforms applied.
     *
     * TEXT escaping is deliberately absent: the RFC 5545 encoder owns the
     * single authoritative escape pass on emit, using its own allow-list
     * of TEXT-typed property names. Escaping here would double it.
     */
    private static function prepareProperty(Property $p, Calendar $cal): Property
    {
        $value = self::nfc($p->value);

        // Rule 11: a DATE value has no time to convert and no zone to
        // resolve, so it is emitted verbatim and the resolution registry
        // is never consulted. VALUE=DATE is RETAINED -- unlike a resolved
        // TZID it is load-bearing, since the default value type for these
        // properties is DATE-TIME and an untagged eight-octet value is a
        // malformed DATE-TIME, not a DATE.
        $isDatetime = self::isDatetimeProperty($p->name);
        $dateOnly = $isDatetime && self::isValueDate($p);

        // Rule 5: a datetime property carrying a TZID resolves to UTC form
        // #2 where the calendar's registry allows, and the TZID is then
        // dropped.
        $stripTzid = $dateOnly;

        if (!$dateOnly && $isDatetime) {
            $tzid = $p->param('TZID');

            if ($tzid !== null && $tzid !== '') {
                $at = Time::parseTimeWithTzid($p->value, $tzid, $cal);

                if ($at !== null) {
                    $value = Time::formatTime($at);
                    $stripTzid = true;
                }
            }
        }

        $isAttach = strcasecmp($p->name, 'ATTACH') === 0;
        $params = [];

        foreach ($p->params as $prm) {
            $name = strtoupper($prm->name);

            // Rule 10: ATTACH is reference-only on emit. The value itself
            // is untouched -- canonical form is best-effort for a
            // malformed URI.
            if ($isAttach && $name === 'VALUE' && strcasecmp($prm->value, 'BINARY') === 0) {
                continue;
            }

            if ($isAttach && $name === 'ENCODING' && strcasecmp($prm->value, 'BASE64') === 0) {
                continue;
            }

            // A TZID on a DATE is a producer bug (RFC 5545 §3.2.19 scopes
            // TZID to DATE-TIME and TIME) and must not leak into the
            // canonical bytes; a resolved TZID is redundant with the UTC
            // value that replaced it.
            if ($stripTzid && $name === 'TZID') {
                continue;
            }

            $pv = self::nfc($prm->value);

            // Rule 11 upper-cases the VALUE argument so `VALUE=date` and
            // `VALUE=DATE` converge. General case-folding of other VALUE
            // tokens is deferred to a later version, so this is scoped to the DATE
            // branch.
            if ($dateOnly && $name === Vstar::VALUE_PARAM) {
                $pv = strtoupper($pv);
            }

            $params[] = new Param($prm->name, $pv);
        }

        return new Property($p->name, self::stableSortByUpperName($params), $value);
    }

    /**
     * A stable copy of `$items` sorted by upper-cased name.
     *
     * The comparison is on the upper-cased ASCII name, so the byte-order
     * question that governs the component sort does not arise: property
     * and parameter names are ASCII by RFC 5545 §3.1 / RFC 6350 §3.3.
     *
     * @template T of Param|Property
     *
     * @param list<T> $items
     *
     * @return list<T>
     */
    private static function stableSortByUpperName(array $items): array
    {
        // usort has been stable since PHP 8.0, but the index tiebreak is
        // kept explicit: the stability of equal keys is a spec requirement
        // (rule 2 and rule 6 both say so), not an implementation detail to
        // inherit silently from the sort's current guarantees.
        $indexed = [];

        foreach ($items as $i => $item) {
            $indexed[] = ['item' => $item, 'i' => $i];
        }

        usort(
            $indexed,
            static function (array $a, array $b): int {
                /** @var array{item: Param|Property, i: int} $a */
                /** @var array{item: Param|Property, i: int} $b */
                $cmp = strcmp(strtoupper($a['item']->name), strtoupper($b['item']->name));

                return $cmp !== 0 ? $cmp : $a['i'] <=> $b['i'];
            },
        );

        $out = [];

        foreach ($indexed as $entry) {
            /** @var T $item */
            $item = $entry['item'];
            $out[] = $item;
        }

        return $out;
    }

    /**
     * A copy of `$components` sorted per rule 6: by UID, or by TZID for a
     * VTIMEZONE; components with neither key sort last; the sort is
     * stable, so equal keys -- a producer bug -- keep their relative input
     * order.
     *
     * The key comparison is `strcmp`, which compares PHP strings byte by
     * byte. Since every value here is UTF-8, byte order **is** the
     * UTF-8-lexicographic order rule 6 specifies, so no custom comparator
     * is needed. Languages whose strings are UTF-16 need one, because
     * their code-unit order puts astral characters below U+E000; PHP's
     * does not have that problem.
     *
     * @param list<Component> $components
     *
     * @return list<Component>
     */
    private static function sortedComponents(array $components): array
    {
        $indexed = [];

        foreach ($components as $i => $c) {
            $indexed[] = ['c' => $c, 'i' => $i, 'key' => self::componentSortKey($c)];
        }

        usort(
            $indexed,
            static function (array $a, array $b): int {
                /** @var array{c: Component, i: int, key: string|null} $a */
                /** @var array{c: Component, i: int, key: string|null} $b */
                if ($a['key'] !== null && $b['key'] !== null) {
                    $cmp = strcmp($a['key'], $b['key']);

                    return $cmp !== 0 ? $cmp : $a['i'] <=> $b['i'];
                }

                if ($a['key'] !== null) {
                    return -1;
                }

                if ($b['key'] !== null) {
                    return 1;
                }

                return $a['i'] <=> $b['i'];
            },
        );

        $out = [];

        foreach ($indexed as $entry) {
            /** @var Component $c */
            $c = $entry['c'];
            $out[] = $c;
        }

        return $out;
    }

    /**
     * The rule-6 sort key: TZID for a VTIMEZONE, otherwise UID then TZID.
     * Null for a component carrying neither, which rule 6 sorts last.
     */
    private static function componentSortKey(Component $c): ?string
    {
        if ($c->type === CompType::Timezone->value) {
            $tzid = $c->get('TZID');

            if ($tzid !== null) {
                return $tzid->value;
            }
        }

        $uid = $c->get('UID');

        if ($uid !== null && $uid->value !== '') {
            return $uid->value;
        }

        $tzid = $c->get('TZID');

        return $tzid === null ? null : $tzid->value;
    }

    /**
     * Whether `$props` already carries a property named `$name`.
     *
     * @param list<Property> $props
     */
    private static function hasProp(array $props, string $name): bool
    {
        foreach ($props as $p) {
            if (strcasecmp($p->name, $name) === 0) {
                return true;
            }
        }

        return false;
    }

    /**
     * Whether `$name` is on the rule-5 datetime allow-list.
     */
    private static function isDatetimeProperty(string $name): bool
    {
        return isset(self::DATETIME_PROPERTIES[strtoupper($name)]);
    }

    /**
     * Whether `$p` declares `VALUE=DATE`. Both the parameter name and its
     * registered-token argument compare case-insensitively per RFC 5545
     * §3.2 / §3.2.20.
     */
    private static function isValueDate(Property $p): bool
    {
        $v = $p->param(Vstar::VALUE_PARAM);

        return $v !== null && strcasecmp($v, Vstar::VALUE_DATE) === 0;
    }

    /**
     * The NFC form of `$s`, per rule 9.
     *
     * The fast path matters: `Normalizer::normalize` allocates
     * unconditionally and the overwhelming majority of values are already
     * normalized, so `isNormalized` is checked first -- which is what the
     * Go reference does, for the same reason.
     *
     * A string the normalizer rejects (invalid UTF-8, which the codec
     * layer does not guarantee against) is returned unchanged rather than
     * replaced by `false`: canonical form is best-effort for malformed
     * input, and losing the value outright would be worse than emitting
     * the producer's own bytes.
     */
    private static function nfc(string $s): string
    {
        if ($s === '' || \Normalizer::isNormalized($s, \Normalizer::FORM_C)) {
            return $s;
        }

        $normalized = \Normalizer::normalize($s, \Normalizer::FORM_C);

        return $normalized === false ? $s : $normalized;
    }
}
