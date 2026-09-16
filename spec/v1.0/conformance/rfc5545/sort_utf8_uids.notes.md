# sort_utf8_uids

Four top-level `VEVENT`s whose `UID`s must sort by **UTF-8 byte order**
per rule 6, authored in reverse so canonicalization has to reorder them.

The keys are chosen so UTF-8 byte order and UTF-16 code-unit order
disagree:

| UID contains | UTF-8 lead | UTF-16 code unit |
|---|---|---|
| `a` (U+0061) | `61` | `0061` |
| U+00FF | `C3` | `00FF` |
| U+FF21 FULLWIDTH LATIN CAPITAL A | `EF` | `FF21` |
| U+1D400 MATHEMATICAL BOLD CAPITAL A | `F0` | `D835 DC00` |

U+1D400 is astral, so in UTF-16 it begins with a surrogate (`D835`)
that compares **below** U+FF21 — the reverse of its UTF-8 ordering.
A port comparing strings with a language operator that works on UTF-16
code units (JavaScript `<`, and any UTF-16-based `compare`) therefore
sorts this document differently from the reference and produces
different canonical bytes and a different hash.

This fixture exists because every other UID in the corpus is ASCII,
where the two orders coincide. The TypeScript port needed a dedicated
byte-wise comparator to pass; nothing in the corpus proved it was
required. Python `str`, Rust `str` and PHP byte strings already compare
in code-point / byte order and agree with the reference.
