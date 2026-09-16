# daily_count_terminates

`FREQ=DAILY;COUNT=3` — DTSTART is occurrence #1; only #2
and #3 are returned by NextOccurrence after that. The 4th call would
return `(zero, false, nil)`.
