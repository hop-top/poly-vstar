# minutely_count_terminates

`FREQ=MINUTELY;COUNT=3` from 12:00Z. dtstart is occurrence #1,
12:01 is #2, 12:02 is #3; the series then terminates. The sidecar
lists the two occurrences after dtstart; termination after the
third is pinned cross-port by the parity harness (`next_complete`).
