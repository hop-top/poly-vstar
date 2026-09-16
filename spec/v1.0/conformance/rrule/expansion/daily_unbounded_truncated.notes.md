# daily_unbounded_truncated

Bare `FREQ=DAILY` has neither UNTIL nor COUNT: the series is
unbounded and only the caller's limit stops it. Five occurrences
come back and `complete: false` — the limit truncated an ongoing
series, which a bare stepping loop could not report.
