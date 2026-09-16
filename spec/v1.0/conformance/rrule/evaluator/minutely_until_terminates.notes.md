# minutely_until_terminates

`FREQ=MINUTELY;UNTIL=20260401T120200Z` from 12:00Z. `UNTIL` is
inclusive: 12:01 and 12:02 fire, the 12:03 base is past `UNTIL`
and the series terminates.
