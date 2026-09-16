# leap_day

`BYYEARDAY=366` under FREQ=YEARLY: only fires in leap years (the
366th day = Dec 31 of a leap year). Non-leap years have 365 days
so the entry resolves out-of-range and is silently dropped per
RFC 5545 §3.3.10.
