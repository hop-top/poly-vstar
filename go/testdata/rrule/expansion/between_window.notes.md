# between_window

Bare `FREQ=DAILY` bounded by a half-open window `[start, end)`.
`start` (04-03) is included; `end` (04-06) is excluded, so three
occurrences come back. The window bounds the result without any
COUNT or UNTIL on the rule.

Windowed expansion has its own sidecar, `.between.json`, rather
than extra fields on `.expand.json`: one sidecar per evaluator
entry point keeps each contract's inputs explicit.
