# scrambled_parts

Rule-parts authored out of order, with the WKST default spelled
out. Parsing is order-insensitive; the emitter must re-emit in the
fixed wire order FREQ, INTERVAL, BYDAY and drop `WKST=MO`. The
BYDAY list keeps its authored order (`MO,WE`).
