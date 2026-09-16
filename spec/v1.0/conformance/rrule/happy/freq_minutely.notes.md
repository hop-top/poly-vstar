# freq_minutely

Bare `FREQ=MINUTELY` — minimum-fields minutely recurrence. Parser
populates `Interval=1`, `WeekStart=MO`; everything else nil.
`MINUTELY` is in the accepted scope (spec/03 §RRULE parsing
scope); `SECONDLY` and `RSCALE` remain deferred.
