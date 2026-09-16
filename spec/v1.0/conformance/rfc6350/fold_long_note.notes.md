# fold_long_note

A `NOTE` whose logical line is 365 octets — long enough to fold onto a
first physical line plus **three** continuations.

The continuation count is the whole point. Folding gives the first
physical line a full 75 octets of payload; every continuation after it
spends one of its 75 on the leading SP and so carries 74. An encoder
that alternates those two widths across loop iterations — 75, then 74,
then 75 again — still emits a legal first continuation and only
overruns on the second, at 76 octets. Every other folding fixture in
the corpus stops after one continuation, so the defect reproduced
nowhere until this fixture existed.

The longest line elsewhere in the corpus is 58 octets, below the fold
boundary entirely.

Pair this with `rfc5545/fold_split_utf8.ics`, which pins that the
75-octet boundary counts octets rather than characters. Together they
fix both degrees of freedom: where the cut lands, and how wide each
chunk may be.
