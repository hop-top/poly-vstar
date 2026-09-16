# minutely_bysecond

`FREQ=MINUTELY;BYSECOND=0,30` from 10:00:00Z. `BYSECOND` EXPANDS
under `MINUTELY` (RFC 5545 §3.3.10 table): each minute base emits
one candidate per listed second. Base 10:00 → 10:00:00 (= dtstart,
not after) and 10:00:30; base 10:01 → 10:01:00, 10:01:30; base
10:02 → 10:02:00. An evaluator that applies `BYSECOND` as a limit
on the base's second never emits :30 and fails here.
