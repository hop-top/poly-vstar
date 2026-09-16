# minutely_interval

`BYSECOND=0,30;INTERVAL=15;FREQ=MINUTELY` authored out of order.
The emitter renders `FREQ` first, then `INTERVAL` (non-default, so
kept), then `BYSECOND` in the spec/03 §RRULE wire form order:
`FREQ=MINUTELY;INTERVAL=15;BYSECOND=0,30`. `MINUTELY` renders as
its RFC 5545 token.
