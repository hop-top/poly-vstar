// SPDX-License-Identifier: MIT

package helpers

import (
	"strings"

	vstar "hop.top/vstar"
	"hop.top/vstar/hashing"
)

// relatedToProp is the wire name for the RFC 5545 §3.8.4.5
// RELATED-TO property.
const relatedToProp = "RELATED-TO"

// relTypeParam is the wire name for the RFC 5545 §3.2.15 RELTYPE
// parameter. Default value when omitted is "PARENT" per the RFC.
const relTypeParam = "RELTYPE"

// RelatedRef is one parsed RELATED-TO property: the referenced UID
// (the property value) and the RELTYPE. RelType carries a registered
// value from vstar's RelType vocabulary (PARENT, CHILD, SIBLING,
// DEPENDS-ON, the RFC 9253 temporal types) folded to canonical case,
// or an unregistered value — an X-prefixed extension, say — verbatim.
// Validation of RelType against the spec vocabulary is the validate
// track's job; this layer accepts any non-empty value.
type RelatedRef struct {
	UID     string
	RelType vstar.RelType
}

// RelatedTo returns every RELATED-TO property on c parsed into a
// RelatedRef. Order matches the property list order on c. Returns
// nil when no RELATED-TO properties are present.
//
// RELTYPE is read case-insensitively per RFC 5545 §3.2 and folded to
// its canonical spelling; an absent RELTYPE param defaults to PARENT
// per RFC 5545 §3.2.15. Unregistered values pass through verbatim.
func RelatedTo(c vstar.Component) []RelatedRef {
	props := c.GetAll(relatedToProp)
	if len(props) == 0 {
		return nil
	}
	out := make([]RelatedRef, 0, len(props))
	for _, p := range props {
		ref := RelatedRef{UID: p.Value, RelType: vstar.DefaultRelType}
		for _, par := range p.Params {
			if strings.EqualFold(par.Name, relTypeParam) && par.Value != "" {
				ref.RelType, _ = vstar.ParseRelType(par.Value)
				break
			}
		}
		out = append(out, ref)
	}
	return out
}

// AddRelatedTo appends a RELATED-TO property with value=uid and
// RELTYPE=reltype. Empty reltype omits the param (consumers will see
// the RFC default PARENT via RelatedTo). reltype accepts the vstar
// RelType constants or any string literal, including X-prefixed
// extensions.
//
// No-op when c is nil or uid is empty. Refreshes X-VSTAR-HASH last
// on success.
func AddRelatedTo(c *vstar.Component, uid string, reltype vstar.RelType) {
	if c == nil || uid == "" {
		return
	}
	prop := vstar.Property{Name: relatedToProp, Value: uid}
	if reltype != "" {
		prop.Params = []vstar.Param{{Name: relTypeParam, Value: string(reltype)}}
	}
	c.Add(prop)
	hashing.SetXVSTAR(c)
}
