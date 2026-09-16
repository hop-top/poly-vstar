// SPDX-License-Identifier: MIT

package vstar

import "errors"

// Error sentinels exposed by every vstar layer. Callers MUST use
// errors.Is to detect these — codecs and validators wrap them with
// fmt.Errorf("...: %w", ...) to add line/offset context.
//
//   - ErrUnsupportedVersion: a vCard or vCalendar VERSION property is
//     present but does not match the supported set (vCard 4.0,
//     iCalendar 2.0).
//   - ErrMalformed: the input is structurally invalid (bad escape,
//     bad parameter syntax, unparseable value, etc.). Always wrapped
//     by codec layers with positional context.
//   - ErrUnclosedBlock: a BEGIN line lacks its matching END before EOF.
//   - ErrMissingUID: a component that requires UID (per spec/02
//     "required common properties") was found without one.
var (
	ErrUnsupportedVersion = errors.New("unsupported version")
	ErrMalformed          = errors.New("malformed")
	ErrUnclosedBlock      = errors.New("unclosed block")
	ErrMissingUID         = errors.New("missing UID")
)
