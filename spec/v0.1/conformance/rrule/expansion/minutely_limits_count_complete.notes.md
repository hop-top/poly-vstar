# minutely_limits_count_complete

`FREQ=MINUTELY;BYHOUR=9;BYMINUTE=0,30;COUNT=3` from 08:00Z, limit
10. `BYHOUR` and `BYMINUTE` LIMIT under `MINUTELY`: from 08:00 the
minute bases 08:00..08:59 fail `BYHOUR`, 09:00 passes, 09:01..09:29
fail `BYMINUTE`, 09:30 passes, then nothing until 2026-04-02 09:00.
Three emissions reach `COUNT=3` inside the limit: `complete: true`.

Why this stem is an `.expand.json` and not a `.next.json`: `COUNT`
counts every emission. An evaluator that EXPANDS `BYHOUR`×`BYMINUTE`
inside each minute period (the daily-and-coarser cross-product
shape) re-emits 09:00 and 09:30 from every base that passes its
limits, exhausts `COUNT` on the duplicates and yields
`[09:00, 09:30, 09:00]`. `NextOccurrence` filters by `after`, so a
`.next.json` sees the same 09:00, 09:30, next-day 09:00 under that
mistake and cannot catch it; the bounded expansion can.
