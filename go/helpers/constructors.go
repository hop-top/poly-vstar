// SPDX-License-Identifier: MIT

// Package helpers provides convenience constructors and high-level
// mutators that sit above the bare property API in the vstar package.
//
// Every helper that mutates state refreshes the X-VSTAR-HASH property
// last (via hashing.SetXVSTAR) so callers receive components whose
// stored hash matches their canonical bytes.
//
// # Methods vs functions
//
// The helpers in this package are package-level functions that take
// vstar.Component / vstar.Card / vstar.Calendar (value or pointer)
// rather than methods on those types. Methods would require editing
// the model package. Free functions in helpers/ keep the model
// package untouched while delivering the same call ergonomics:
//
//	helpers.SetStatus(&c, vstar.TodoCompleted)
//
// instead of
//
//	c.SetStatus(vstar.TodoCompleted)
//
// All other semantics — hash refresh discipline, no-op on type
// mismatch, validation of enum values — match the original method
// shape spelled out in the design doc.
package helpers

import (
	"fmt"
	"time"

	vstar "hop.top/vstar"
	"hop.top/vstar/hashing"
)

// defaultProdID is the PRODID emitted when callers pass an empty
// string to NewCalendar. PRODID survives canonicalization and is
// hashed, so the default carries neither a version nor a language:
// a document built by any release of any port hashes the same.
// Callers seeking a custom identifier supply their own PRODID.
const defaultProdID = "-//hop-top//vstar//EN"

// Property names used repeatedly across constructors. Centralized so
// linters (goconst) stay quiet and renames stay sound.
const (
	propUID     = "UID"
	propDTSTAMP = "DTSTAMP"
)

// stampUID seeds c with UID and a fresh DTSTAMP=now (UTC). Returns
// the component value-style so callers can chain further mutations
// before the X-VSTAR-HASH refresh.
func stampUID(c vstar.Component, uid string) vstar.Component {
	c.Set(vstar.Property{Name: propUID, Value: uid})
	c.Set(vstar.Property{Name: propDTSTAMP, Value: vstar.FormatTime(time.Now().UTC())})
	return c
}

// NewTodo returns a fresh VTODO with UID, DTSTAMP=now (UTC), and DUE
// set to due. The X-VSTAR-HASH property is computed and stored last.
//
// Returns ErrMissingUID when uid is empty.
func NewTodo(uid string, due time.Time) (vstar.Component, error) {
	if uid == "" {
		return vstar.Component{}, fmt.Errorf("helpers.NewTodo: %w", vstar.ErrMissingUID)
	}
	c := stampUID(vstar.Component{Type: vstar.CompTodo}, uid)
	c.SetDUE(due)
	hashing.SetXVSTAR(&c)
	return c, nil
}

// NewJournal returns a fresh VJOURNAL with UID, DTSTAMP=now (UTC),
// and DTSTART set to dtstart. The X-VSTAR-HASH property is computed
// and stored last.
//
// Returns ErrMissingUID when uid is empty.
func NewJournal(uid string, dtstart time.Time) (vstar.Component, error) {
	if uid == "" {
		return vstar.Component{}, fmt.Errorf("helpers.NewJournal: %w", vstar.ErrMissingUID)
	}
	c := stampUID(vstar.Component{Type: vstar.CompJournal}, uid)
	c.SetDTSTART(dtstart)
	hashing.SetXVSTAR(&c)
	return c, nil
}

// NewEvent returns a fresh VEVENT with UID, DTSTAMP=now (UTC),
// DTSTART, and DTEND set. The X-VSTAR-HASH property is computed and
// stored last.
//
// Returns ErrMissingUID when uid is empty.
func NewEvent(uid string, dtstart, dtend time.Time) (vstar.Component, error) {
	if uid == "" {
		return vstar.Component{}, fmt.Errorf("helpers.NewEvent: %w", vstar.ErrMissingUID)
	}
	c := stampUID(vstar.Component{Type: vstar.CompEvent}, uid)
	c.SetDTSTART(dtstart)
	c.SetDTEND(dtend)
	hashing.SetXVSTAR(&c)
	return c, nil
}

// NewFreeBusy returns a fresh VFREEBUSY with UID, DTSTAMP=now (UTC),
// DTSTART, and DTEND set. The X-VSTAR-HASH property is computed and
// stored last.
//
// Returns ErrMissingUID when uid is empty.
func NewFreeBusy(uid string, dtstart, dtend time.Time) (vstar.Component, error) {
	if uid == "" {
		return vstar.Component{}, fmt.Errorf("helpers.NewFreeBusy: %w", vstar.ErrMissingUID)
	}
	c := stampUID(vstar.Component{Type: vstar.CompFreeBusy}, uid)
	c.SetDTSTART(dtstart)
	c.SetDTEND(dtend)
	hashing.SetXVSTAR(&c)
	return c, nil
}

// NewAlarm returns a fresh VALARM with UID, DTSTAMP=now (UTC),
// ACTION=action, and TRIGGER=trigger. The X-VSTAR-HASH property is
// computed and stored last.
//
// VALARM is the one component type whose RFC-5545 schema does not
// require UID, but V* requires UID on every persisted component
// (spec/02). Callers should still supply a stable identifier;
// ErrMissingUID is returned when uid is empty.
func NewAlarm(uid, action, trigger string) (vstar.Component, error) {
	if uid == "" {
		return vstar.Component{}, fmt.Errorf("helpers.NewAlarm: %w", vstar.ErrMissingUID)
	}
	c := stampUID(vstar.Component{Type: vstar.CompAlarm}, uid)
	c.Set(vstar.Property{Name: "ACTION", Value: action})
	c.Set(vstar.Property{Name: "TRIGGER", Value: trigger})
	hashing.SetXVSTAR(&c)
	return c, nil
}

// NewCalendar returns a fresh VCALENDAR with VERSION:2.0,
// METHOD:PUBLISH, and the supplied PRODID. Empty prodID resolves to
// the package default ("-//hop-top//vstar-go vX.Y.Z//EN").
//
// VCALENDAR is the top-level container; VEVENT/VTODO/etc. components
// are appended via Calendar.Append. The fixed-property bag here lives
// on the Calendar itself in the spec, but the in-memory model keeps
// PRODID as a struct field and routes VERSION/METHOD as Sub-component
// metadata is not used at this layer.
func NewCalendar(prodID string) vstar.Calendar {
	if prodID == "" {
		prodID = defaultProdID
	}
	return vstar.Calendar{ProdID: prodID}
}

// NewCard returns a fresh VCARD with VERSION:4.0, KIND, and UID
// properties set. A zero kind defaults to KindIndividual.
//
// vCards are not subject to the X-VSTAR-HASH discipline at the
// constructor layer at v1.0: the hashing.Card helper exists for
// callers that need a card-level digest, but Card has no
// X-VSTAR-HASH property of its own and this constructor does not
// stamp one.
func NewCard(uid string, kind vstar.Kind) vstar.Card {
	if kind == "" {
		kind = vstar.KindIndividual
	}
	card := vstar.Card{UID: uid, Kind: kind}
	card.Set(vstar.Property{Name: "VERSION", Value: "4.0"})
	card.Set(vstar.Property{Name: "KIND", Value: string(kind)})
	card.Set(vstar.Property{Name: propUID, Value: uid})
	return card
}
