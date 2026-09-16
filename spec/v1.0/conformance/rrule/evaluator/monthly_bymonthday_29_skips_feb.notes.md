# monthly_bymonthday_29_skips_feb

`BYMONTHDAY=29` under MONTHLY: 2026 is not a leap year so Feb
has 28 days; the parser SKIPS that occurrence and lands on Mar 29.
This is the canonical leap-day edge case from spec/03 §RRULE
parsing scope (evaluator behaviour).
