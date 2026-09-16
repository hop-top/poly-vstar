# defaults_elided

`INTERVAL=1` and `WKST=MO` are the RFC 5545 defaults; the emitter
omits both, so the wire form is the bare `FREQ=DAILY`. Canonical
form still preserves the authored value verbatim (rule 8) — this
fixture is about what an emitter writes, not what canonical hashes.
