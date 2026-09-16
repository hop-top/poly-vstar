# hourly_byhour_byminute

`FREQ=HOURLY;BYHOUR=9;BYMINUTE=0,30` from 08:00Z — limit and expand
in one hour. `BYHOUR` LIMITS: base 08:00 fails, so nothing fires
in the 08:00 hour. Base 09:00 passes and `BYMINUTE` EXPANDS it to
09:00:00 and 09:30:00. Bases 10:00 through 08:00 the next day fail;
2026-04-02 09:00 expands to 09:00 and 09:30. An evaluator that
ignores `BYHOUR` emits 08:00 and 08:30 from the first base and
reports 08:30 first.
