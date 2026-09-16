# last_weekday_of_month

The hero BYSETPOS use case: BYDAY=MO,TU,WE,TH,FR enumerates every
weekday of the month; BYSETPOS=-1 selects the last one. Verifies
the positional filter resolves correctly across months whose last
day falls on a different weekday (Jan 30 = Fri, Feb 27 = Fri, Mar
31 = Tue, Apr 30 = Thu).
