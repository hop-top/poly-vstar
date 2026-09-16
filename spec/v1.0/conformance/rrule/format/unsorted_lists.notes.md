# unsorted_lists

A rule whose `BYDAY` and `BYMONTHDAY` list values are authored out of
ascending order, to pin that the wire form keeps them as authored.

Spec §RRULE wire form fixes the order of the rule-*parts* but says list
values "keep their authored order; lists are not sorted". Nothing else
in the corpus can tell the two apart: `scrambled_parts` spells
`BYDAY=MO,WE`, which is already ascending, so a port that sorts list
values on parse reproduces every other `.formatted` fixture exactly.

Three ports independently confirmed the gap — each had to write a
hand-rolled assertion because the corpus walk passed under a
sort-on-parse mutation.
