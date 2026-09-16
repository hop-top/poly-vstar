# freq_minutely

Bare `FREQ=MINUTELY` — minimum-fields minutely recurrence. Parser
populates `Interval=1`, `WeekStart=MO`; everything else nil.
`MINUTELY` joined the accepted scope in v0.2 (spec/03 §RRULE
parsing scope); `SECONDLY` and `RSCALE` remain deferred.
