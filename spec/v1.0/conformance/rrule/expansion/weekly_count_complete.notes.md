# weekly_count_complete

`FREQ=WEEKLY;BYDAY=MO,WE;COUNT=4` from a Monday DTSTART. The limit
(10) exceeds COUNT, so the series ends on its own inside the
bound: four occurrences and `complete: true` — nothing was
truncated.
