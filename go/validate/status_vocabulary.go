// SPDX-License-Identifier: MIT

package validate

import (
	"strings"

	vstar "hop.top/vstar"
)

// statusVocabularies maps each component type that admits STATUS to
// the wire values RFC 5545 §3.8.1.11 allows for it. Types absent
// from the map have no STATUS vocabulary and are skipped.
//
// Built from the package's own wire constants rather than read out of
// the generated StatusVocabulary, deliberately. These are the values
// the codec encodes against, so a table built from them cannot
// disagree with what this library actually writes — a guarantee a
// lookup into a generated table would give up. The registry's
// cross-language copy is reconciled against this one by
// TestStatusVocabulary_matchesRegistry, which is what keeps the five
// ports agreeing without any of them losing the codec linkage.
var statusVocabularies = map[vstar.CompType][]string{
	vstar.CompEvent: {
		string(vstar.EventTentative),
		string(vstar.EventConfirmed),
		string(vstar.EventCancelled),
	},
	vstar.CompTodo: {
		string(vstar.TodoNeedsAction),
		string(vstar.TodoInProcess),
		string(vstar.TodoCompleted),
		string(vstar.TodoCancelled),
	},
	vstar.CompJournal: {
		string(vstar.JournalDraft),
		string(vstar.JournalFinal),
		string(vstar.JournalCancelled),
	},
}

// checkStatusVocabulary emits VS044 when c carries a STATUS whose
// value is not in its component type's vocabulary.
//
// Comparison is case-insensitive per RFC 5545 §3.1 (enumerated
// property values are not case-sensitive). An absent STATUS is
// clean — the property is optional on every type that admits it.
func checkStatusVocabulary(c vstar.Component, path string) []Diagnostic {
	allowed, scoped := statusVocabularies[c.Type]
	if !scoped {
		return nil
	}
	p, ok := c.Get("STATUS")
	if !ok {
		return nil
	}
	for _, want := range allowed {
		if strings.EqualFold(p.Value, want) {
			return nil
		}
	}
	return []Diagnostic{{
		Severity: SeverityError,
		Code:     CodeStatusNotInVocabulary,
		Message: "STATUS value " + p.Value + " is not valid for " + string(c.Type) +
			"; allowed: " + strings.Join(allowed, ", ") + " (RFC 5545 §3.8.1.11)",
		Path: path + ".STATUS",
	}}
}
