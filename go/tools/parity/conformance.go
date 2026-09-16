// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	vstar "hop.top/vstar"
	"hop.top/vstar/canonical"
	"hop.top/vstar/codec/rfc5545"
	"hop.top/vstar/codec/rfc6350"
	"hop.top/vstar/hashing"
)

// conformanceEntry is one parseable fixture's wire contract: the
// content hash the reference computes, and a digest of the
// canonical bytes it hashed.
//
// CanonicalSHA256 is emitted rather than the canonical bytes
// themselves so the document stays small and diffable: a port that
// produces one wrong byte shows a different digest, which is the
// signal the harness needs. The bytes are on disk in the
// <stem>.canonical sibling for whoever has to debug the mismatch.
//
// The digest is taken over the LF form on disk, not the CRLF form
// canonical.Calendar returns — see canonicalDigest.
type conformanceEntry struct {
	Hash            string `json:"hash"`
	CanonicalSHA256 string `json:"canonical_sha256"`
}

// calendarDirs are the conformance subdirectories whose .ics files
// parse into a Calendar and carry .canonical/.hash siblings.
var calendarDirs = []string{"rfc5545", "supersession"}

// emitConformance walks the conformance corpus and keys every
// fixture by its path relative to the corpus root, without
// extension: "rfc5545/world", "malformed/malformed".
//
// Parseable fixtures emit a conformanceEntry. Fixtures under
// malformed/ emit an errorEntry naming the sentinel their .error
// sibling pins — reproduced by actually failing, never by copying
// the sidecar: the emitter states what the implementation does.
//
// time/ fixtures are VTIMEZONE registries consumed by the time
// family and carry no .canonical/.hash siblings, so they are not
// part of this family. fuzz-seed/ is fuzz-target input, not a
// conformance contract.
func emitConformance(corpus string) (map[string]any, error) {
	out := map[string]any{}

	for _, dir := range calendarDirs {
		if err := eachFixture(filepath.Join(corpus, dir), extICS, func(path, stem string) error {
			entry, err := calendarEntry(path)
			if err != nil {
				return err
			}
			return record(out, corpus, stem, entry)
		}); err != nil {
			return nil, err
		}
	}

	if err := eachFixture(filepath.Join(corpus, "rfc6350"), extVCF, func(path, stem string) error {
		entry, err := cardEntry(path)
		if err != nil {
			return err
		}
		return record(out, corpus, stem, entry)
	}); err != nil {
		return nil, err
	}

	malformed := filepath.Join(corpus, "malformed")
	for _, ext := range []string{extICS, extVCF} {
		if err := eachFixture(malformed, ext, func(path, stem string) error {
			token, err := malformedToken(path, ext)
			if err != nil {
				return err
			}
			return record(out, corpus, stem, errorEntry{Error: token})
		}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// calendarEntry parses an .ics fixture, canonicalizes and hashes
// it, then self-checks the hash against the committed .hash
// sibling.
//
// The self-check is what makes the reference emitter falsifiable on
// its own. Cross-language diffing cannot catch a reference that
// emits a wrong value — with nothing to compare against, a wrong
// reference is simply the new truth. Anchoring to the committed
// corpus gives the harness an independent expectation for the one
// family that has one.
func calendarEntry(path string) (conformanceEntry, error) {
	input, err := os.ReadFile(path)
	if err != nil {
		return conformanceEntry{}, fmt.Errorf("read %s: %w", path, err)
	}
	cal, err := rfc5545.Parse(bytes.NewReader(input))
	if err != nil {
		return conformanceEntry{}, fmt.Errorf("parse %s: %w", path, err)
	}
	entry := conformanceEntry{
		Hash:            hashing.Calendar(cal),
		CanonicalSHA256: canonicalDigest(canonical.Calendar(cal)),
	}
	return entry, checkHashSibling(path, extICS, entry.Hash)
}

// cardEntry is calendarEntry for a .vcf fixture. The corpus holds
// exactly one card per file; more would mean the fixture changed
// shape under the emitter and the contract no longer says which
// card the hash belongs to.
func cardEntry(path string) (conformanceEntry, error) {
	input, err := os.ReadFile(path)
	if err != nil {
		return conformanceEntry{}, fmt.Errorf("read %s: %w", path, err)
	}
	cards, err := rfc6350.New().Parse(bytes.NewReader(input))
	if err != nil {
		return conformanceEntry{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(cards) != 1 {
		return conformanceEntry{}, fmt.Errorf("%s: expected exactly one card, got %d", path, len(cards))
	}
	entry := conformanceEntry{
		Hash:            hashing.Card(cards[0]),
		CanonicalSHA256: canonicalDigest([]byte(canonical.Card(cards[0]))),
	}
	return entry, checkHashSibling(path, extVCF, entry.Hash)
}

// checkHashSibling compares a computed hash against the committed
// <stem>.hash file. A mismatch aborts the emitter: the reference
// disagreeing with the corpus it is the reference for is not a
// parity mismatch to report downstream, it is a broken reference.
func checkHashSibling(path, ext, got string) error {
	sibling := strings.TrimSuffix(path, ext) + ".hash"
	raw, err := os.ReadFile(sibling)
	if err != nil {
		return fmt.Errorf("read %s: %w", sibling, err)
	}
	want := strings.TrimRight(string(raw), "\r\n")
	if got != want {
		return fmt.Errorf("%s: hash %s does not match committed %s", sibling, got, want)
	}
	return nil
}

// canonicalDigest hashes canonical bytes after folding CRLF to LF.
//
// The canonical form per spec/03 is CRLF, but the corpus stores it
// LF for diff-friendliness and every port reads the same LF file.
// Digesting the LF form keeps the emitted value comparable to what
// a port computes from the on-disk sibling, and removes line-ending
// handling as a source of false mismatches between ports.
func canonicalDigest(b []byte) string {
	sum := sha256.Sum256(crlfToLF(b))
	return hex.EncodeToString(sum[:])
}

func crlfToLF(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
}

// sentinel pairs a cross-language token with the Go error it
// names. Tables of these are ordered lists, not maps: classify
// returns the first match, so a specific sentinel must be tried
// before a general one. ErrMalformed is the general case every
// codec layer may additionally wrap, and so is always last.
type sentinel struct {
	token string
	err   error
}

// malformedSentinels is the vocabulary a malformed/*.error sibling
// may name, most specific first. The emitter reports which one the
// fixture actually produced, so the token is derived from behavior;
// this list only bounds what behavior is recognized.
var malformedSentinels = []sentinel{
	{"ErrUnsupportedVersion", vstar.ErrUnsupportedVersion},
	{"ErrUnclosedBlock", vstar.ErrUnclosedBlock},
	{"ErrMissingUID", vstar.ErrMissingUID},
	{classMalformed, vstar.ErrMalformed},
}

// malformedToken runs a malformed fixture through the codec and
// returns the sentinel token it produced.
//
// Most fixtures fail at parse time. ErrMissingUID is encoder-only
// at v1.0 — the rfc6350 parser accepts a UID-less VCARD and the
// encoder refuses it — so a fixture that parses is re-encoded and
// the encode failure classified instead. That two-stage shape is
// the documented contract (see malformed/README.md); a port
// reproduces both stages.
func malformedToken(path, ext string) (string, error) {
	input, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	var parseErr, encodeErr error
	switch ext {
	case extICS:
		_, parseErr = rfc5545.Parse(bytes.NewReader(input))
	case extVCF:
		var cards []vstar.Card
		cards, parseErr = rfc6350.New().Parse(bytes.NewReader(input))
		if parseErr == nil {
			encodeErr = encodeCards(cards)
		}
	}
	if token, ok := classify(parseErr, malformedSentinels); ok {
		return token, nil
	}
	if token, ok := classify(encodeErr, malformedSentinels); ok {
		return token, nil
	}
	if parseErr == nil && encodeErr == nil {
		return "", fmt.Errorf("%s: expected a failure, parse and encode both succeeded", path)
	}
	return "", fmt.Errorf("%s: failure matches no known sentinel: %w", path, errors.Join(parseErr, encodeErr))
}

// encodeCards re-encodes every parsed card and returns the first
// failure, which is how an encoder-only sentinel surfaces.
func encodeCards(cards []vstar.Card) error {
	if len(cards) == 0 {
		return errors.New("parse returned no cards and no error")
	}
	for _, c := range cards {
		var sb strings.Builder
		if err := rfc6350.New().Encode(&sb, c); err != nil {
			return err
		}
	}
	return nil
}

// eachFixture calls fn for every non-directory entry of dir with
// the given extension, in sorted order, passing the full path and
// the extension-stripped path. A missing dir is a silent no-op so
// the emitter tolerates a corpus that has yet to grow a family.
func eachFixture(dir, ext string, fn func(path, stem string) error) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ext) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		full := filepath.Join(dir, name)
		if err := fn(full, strings.TrimSuffix(full, ext)); err != nil {
			return err
		}
	}
	return nil
}

// record keys value by stem's slash-separated path relative to
// root, refusing a collision. Two fixtures colliding on one key
// would silently drop a case from the document and shrink the
// harness' coverage without failing it.
func record(out map[string]any, root, stem string, value any) error {
	key, err := fixtureKey(root, stem)
	if err != nil {
		return err
	}
	if _, dup := out[key]; dup {
		return fmt.Errorf("duplicate fixture key %q", key)
	}
	out[key] = value
	return nil
}

// fixtureKey renders stem as a slash-separated path relative to
// root: the document's key space is filesystem layout with the
// extension dropped, identical on every platform a port runs on.
func fixtureKey(root, stem string) (string, error) {
	rel, err := filepath.Rel(root, stem)
	if err != nil {
		return "", fmt.Errorf("relativize %s against %s: %w", stem, root, err)
	}
	return filepath.ToSlash(rel), nil
}

// classify reports the token of the first sentinel in table that
// err wraps. Reports ok=false for a nil error and for a failure
// matching nothing in the table, which the caller turns into a
// diagnostic rather than an emitted token no port would recognize.
//
// First-match, not best-match: the tables are ordered specific to
// general, so a failure wrapping both a specific sentinel and
// ErrMalformed reports the specific one.
func classify(err error, table []sentinel) (string, bool) {
	if err == nil {
		return "", false
	}
	for _, s := range table {
		if errors.Is(err, s.err) {
			return s.token, true
		}
	}
	return "", false
}
