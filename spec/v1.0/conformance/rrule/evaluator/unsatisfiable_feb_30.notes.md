# unsatisfiable_feb_30

`FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30` parses cleanly — the parser
stays permissive on unsatisfiable BY-* combinations — but February
never has a 30th, so no period ever yields. The evaluator walks its
full iteration bound and reports `ErrIterationCap` instead of
`(zero, false, nil)`: the search was abandoned, the rule did not
terminate. Consumers must not read a cap hit as series end.
