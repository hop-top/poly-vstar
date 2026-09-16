# minutely_byday_across_midnight

`FREQ=MINUTELY;INTERVAL=30;BYDAY=TU` from Tuesday 2026-04-07 23:00Z.
`BYDAY` LIMITS under `MINUTELY` (ordinals ignored, as under
`HOURLY`/`DAILY`). Base 23:30 is still Tuesday → fires. Base
2026-04-08 00:00 is Wednesday → fails, as does every 30-minute base
through Monday 2026-04-13 23:30. Tuesday 2026-04-14 00:00 and 00:30
fire. The walk crosses midnight and a six-day gap: 289 periods from
23:30 to the next Tuesday, well under the iteration bound.
