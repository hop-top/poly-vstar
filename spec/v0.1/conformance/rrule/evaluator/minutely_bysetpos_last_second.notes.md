# minutely_bysetpos_last_second

`FREQ=MINUTELY;BYSECOND=0,20,40;BYSETPOS=-1` from 10:00:00Z. Each
minute period's expanded set is {:00, :20, :40}; `BYSETPOS=-1`
keeps the last, :40. Base 10:00 → 10:00:40 (after dtstart), then
10:01:40, 10:02:40. An evaluator that skips `BYSETPOS` under
`MINUTELY` reports 10:00:20 first.
