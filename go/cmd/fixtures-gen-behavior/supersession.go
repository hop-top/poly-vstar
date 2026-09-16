// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"path/filepath"

	"hop.top/vstar/supersession"
)

// supersessionInputs are the conformance-corpus fixtures the
// effective-status family projects. They are NOT regenerated here:
// they are authored by fixtures-gen-supersession under
// spec/v1.0/conformance/supersession/ and carry their own
// .canonical/.hash siblings and hand-written .notes.md.
//
// Reusing them rather than minting parallel inputs keeps one .ics
// per scenario in the repo: a port that already loads the
// conformance corpus for canonicalization gets the supersession
// projection table for free, keyed by the same file name.
var supersessionInputs = []string{
	"linear",
	"multi_step",
	"cross_component",
	"corrupt_mutated",
	"effective_status_vevent",
}

// writeSupersessionFamily writes one <name>.effective.json per
// conformance supersession fixture: a map from component UID to the
// status the ledger projects onto it.
//
// Only components the ledger actually supersedes appear. A UID
// absent from the map means Superseded reported ok=false for it —
// the component stands as written. That is the "corrupt_mutated"
// case: its hash is broken but no supersession entry targets it, so
// its map is empty. Supersession is a projection query, not a
// validator; the hash violation is the validate family's business.
func writeSupersessionFamily(dir string) (int, error) {
	module, err := moduleRoot()
	if err != nil {
		return 0, err
	}
	corpus := filepath.Join(filepath.Dir(module), specDirName, "v1.0", "conformance", "supersession")
	n := 0
	for _, name := range supersessionInputs {
		cal, err := readCalendar(filepath.Join(corpus, name+".ics"))
		if err != nil {
			return 0, err
		}
		effective := map[string]string{}
		for _, c := range cal.Components {
			status, ok := supersession.Superseded(c, cal.Components)
			if !ok {
				continue
			}
			uid := c.UID()
			if uid == "" {
				return 0, fmt.Errorf("%s: superseded component has no UID", name)
			}
			effective[uid] = status
		}
		if err := writeJSON(filepath.Join(dir, name+".effective.json"), effective); err != nil {
			return 0, err
		}
		n++
	}
	return n, nil
}
