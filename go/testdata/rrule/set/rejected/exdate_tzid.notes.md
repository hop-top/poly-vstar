# exdate_tzid

The calendar parses, but the recurrence set is rejected: v0.1
EXDATE/RDATE values must be UTC form #2. EXDATE is not on the
datetime resolution allow-list, so a TZID-bearing value reaches
set evaluation unresolved and is reported as `ErrUnsupportedRRule`
rather than silently dropped — dropping an exclusion would
resurrect a canceled occurrence.
