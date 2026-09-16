# minutely_sparse_limit_caps

`FREQ=MINUTELY;BYMONTH=1` from 2026-02-01 00:00Z. The rule is
satisfiable — every minute of January 2027 matches — but `BYMONTH`
LIMITS under `MINUTELY`, so every minute base from February through
December fails and the evaluator steps period by period. Reaching
2027-01-01 needs 334 days × 1440 = 480 960 minute periods; the
reference bound is 100 000 periods = 69 d 10 h 40 min, whose last
base is 2026-04-11 10:39Z, never January. The evaluator reports
`ErrIterationCap`, not the eventual occurrence and not
`(zero, false, nil)`.

Assumption this fixture makes: the implementation-defined bound
(spec/03 §Expansion) is below 480 960 minute periods. Every
in-tree implementation documents 100 000; a port that picks a
larger bound will legitimately fail this stem and must say so.
Skipping the base to the next admitted window would lift the
consequence and is a follow-up for every FREQ.

An evaluator that ignores `BYMONTH` under `MINUTELY` reports
00:01 instead of the cap.
