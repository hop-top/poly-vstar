# exdate_after_rdate

Evaluation order: DTSTART, RRULE (`FREQ=DAILY;COUNT=3` → 04-01,
04-02, 04-03), RDATE merged (04-10 added; 04-03 coincides with a
rule occurrence and de-duplicates), then EXDATE removed last
(04-02 from the rule, 04-10 even though an RDATE named it). Two
occurrences remain, sorted, and the set is `complete`.
