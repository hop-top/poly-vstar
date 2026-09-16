# fold_split_utf8

A `SUMMARY` whose value places a two-octet UTF-8 sequence (U+00E9)
directly across the 75-octet fold boundary.

Folding counts octets, not characters (RFC 5545 §3.1, spec rule 3), so
the sequence is split: the physical line ending the first chunk is not
valid UTF-8 on its own, and neither is the continuation's first octet.
Unfolding rejoins them and the logical line decodes cleanly.

This fixture exists because nothing else in the corpus reaches a fold
point with non-ASCII content. Without it a port can fold at any width,
or retreat the cut to a character boundary, and still reproduce every
other `.canonical` and `.hash` file byte for byte. A port that decodes
physical lines before unfolding rejects this document outright.
