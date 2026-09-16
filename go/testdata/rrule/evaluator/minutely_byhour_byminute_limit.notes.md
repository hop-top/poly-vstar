# minutely_byhour_byminute_limit

`FREQ=MINUTELY;BYHOUR=9;BYMINUTE=0,30` from 08:00Z. Under
`MINUTELY` both `BYHOUR` and `BYMINUTE` LIMIT (RFC 5545 §3.3.10
table): a minute base fires only when its hour is 9 and its minute
is 0 or 30. From 08:00 the bases 08:00..08:59 fail `BYHOUR` —
dtstart is therefore not occurrence #1 — 09:00 passes, 09:01..09:29
fail `BYMINUTE`, 09:30 passes, 09:31 through 08:59 the next day
fail, then 2026-04-02 09:00 and 09:30. An evaluator that ignores
`BYHOUR` reports 08:00/08:30 first; one that ignores `BYMINUTE`
reports 09:01 second.
