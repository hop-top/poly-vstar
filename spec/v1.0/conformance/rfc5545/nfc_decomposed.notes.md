# nfc_decomposed

Values written in decomposed form that rule 9 must recompose to NFC.

`SUMMARY` combines `e` + U+0301 COMBINING ACUTE ACCENT (which NFC folds
to U+00E9) with U+212B ANGSTROM SIGN (which NFC folds to U+00C5 LATIN
CAPITAL LETTER A WITH RING ABOVE) and a precomposed U+00F6 that must be
left alone. `LOCATION` carries `u` + U+0308 COMBINING DIAERESIS in both
its **parameter value** and its property value, because rule 9 covers
parameters as well and a port can easily normalize only the value.

Rule 9 does NOT normalize property or parameter *names*; those are
ASCII and uppercased separately.

This fixture exists because nothing else in the corpus carries a
non-NFC value. Without it a port can skip normalization entirely and
still reproduce every other `.canonical` and `.hash` byte for byte —
verified by disabling NFC in a port and watching all 26 byte-identity
fixtures still pass.
