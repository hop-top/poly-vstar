// SPDX-License-Identifier: MIT

package validate

import (
	"strings"

	vstar "hop.top/vstar"
	"hop.top/vstar/supersession"
)

// checkSupersessionDiscipline emits VS030 / VS031 diagnostics for
// every supersession VJOURNAL in cal that violates spec/05 §4.
//
// The check is calendar-scoped (not component-scoped) because VS031
// requires cross-component RELATED-TO resolution. ValidateComponent
// (single-component entry point) cannot run VS031 — it only sees
// one component — and so emits only VS030 from this check.
//
// Calendar-level invocation is wired in Validate; component-level
// invocation is wired in validateComponentAt with a nil ledger so
// only VS030 fires.
func checkSupersessionDiscipline(c vstar.Component, ledger []vstar.Component, path string) []Diagnostic {
	if c.Type != vstar.CompJournal {
		return nil
	}
	if !categoriesContainSupersession(c) {
		return nil
	}

	var out []Diagnostic

	// VS030 — RELATED-TO must be present.
	relProp, hasRel := c.Get("RELATED-TO")
	if !hasRel || strings.TrimSpace(relProp.Value) == "" {
		out = append(out, Diagnostic{
			Severity: SeverityError,
			Code:     CodeSupersessionMissingProps,
			Message:  "supersession VJOURNAL missing RELATED-TO (spec/02, spec/05 §4)",
			Path:     path + ".RELATED-TO",
		})
	}

	// VS030 — X-VSTAR-EFFECTIVE-STATUS must be present.
	statusProp, hasStatus := c.Get(supersession.PropEffectiveStatus)
	if !hasStatus || strings.TrimSpace(statusProp.Value) == "" {
		out = append(out, Diagnostic{
			Severity: SeverityError,
			Code:     CodeSupersessionMissingProps,
			Message:  "supersession VJOURNAL missing " + supersession.PropEffectiveStatus + " (spec/02, spec/05 §4)",
			Path:     path + "." + supersession.PropEffectiveStatus,
		})
	}

	// VS031 — RELATED-TO must resolve to a component in the same
	// Calendar. Skip when ledger is nil (single-component
	// ValidateComponent caller has no cross-component context).
	if hasRel && ledger != nil {
		target := strings.TrimSpace(relProp.Value)
		if target != "" && !ledgerContainsUID(ledger, target) {
			out = append(out, Diagnostic{
				Severity: SeverityError,
				Code:     CodeSupersessionOrphan,
				Message:  "supersession VJOURNAL RELATED-TO=" + target + " has no matching component in calendar (spec/02, spec/05 §4)",
				Path:     path + ".RELATED-TO",
			})
		}
	}

	return out
}

// categoriesContainSupersession reports whether c carries a
// CATEGORIES property whose value contains
// supersession.CategoryStatusSupersession (comma-separated, case-
// insensitive). Mirrors the helper in the supersession package
// (which is unexported there).
func categoriesContainSupersession(c vstar.Component) bool {
	for _, p := range c.GetAll("CATEGORIES") {
		for _, tok := range strings.Split(p.Value, ",") {
			if strings.EqualFold(strings.TrimSpace(tok), supersession.CategoryStatusSupersession) {
				return true
			}
		}
	}
	return false
}

// ledgerContainsUID reports whether any component in ledger has a
// UID property equal to uid. Comparison is case-sensitive per RFC
// 5545 §3.8.4.7 (UIDs are opaque identifiers).
func ledgerContainsUID(ledger []vstar.Component, uid string) bool {
	for i := range ledger {
		if ledger[i].UID() == uid {
			return true
		}
	}
	return false
}
