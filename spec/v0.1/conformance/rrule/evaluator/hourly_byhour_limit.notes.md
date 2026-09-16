# hourly_byhour_limit

`FREQ=HOURLY;BYHOUR=9,17` from 08:00Z. `BYHOUR` LIMITS under
`HOURLY` (RFC 5545 §3.3.10 table): an hourly base fires only when
its hour is listed. Base 08:00 fails (dtstart is not occurrence #1);
09:00 fires at the base's minute and second (00:00); 10:00 through
16:00 fail; 17:00 fires; 18:00 through 08:00 the next day fail; then
2026-04-02 09:00 and 17:00. An evaluator that treats `BYHOUR` as a
no-op under `HOURLY` reports 09:00, 10:00, 11:00, …
