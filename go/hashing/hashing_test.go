// SPDX-License-Identifier: MIT

package hashing_test

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/canonical"
	"hop.top/vstar/hashing"
)

// helperVTODO returns the same fixture used in canonical_test.go so
// the hash is verifiable by recomputing sha256(canonical.Component).
func helperVTODO() vstar.Component {
	return vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "UID", Value: "abc-123"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "SUMMARY", Value: "Buy milk"},
			{Name: "PRIORITY", Value: "3"},
		},
	}
}

// expectedVTODOHash recomputes the expected hash using the public
// canonical helper directly, so the test pins the contract
// "Hash(Component) == sha256(canonical.Component(c))" without
// hard-coding a hex digest that would silently drift if canonical
// changes.
func expectedVTODOHash(t *testing.T) string {
	t.Helper()
	sum := sha256.Sum256(canonical.Component(helperVTODO()))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func TestComponent_FormatPrefixAndLength(t *testing.T) {
	got := hashing.Component(helperVTODO())
	if !strings.HasPrefix(got, "sha256:") {
		t.Fatalf("Component hash missing sha256: prefix; got %q", got)
	}
	hexPart := strings.TrimPrefix(got, "sha256:")
	if len(hexPart) != 64 {
		t.Fatalf("Component hash hex must be 64 chars; got %d (%q)", len(hexPart), hexPart)
	}
	for _, r := range hexPart {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			t.Fatalf("Component hash hex must be lowercase 0-9a-f; got %q", hexPart)
		}
	}
}

func TestComponent_MatchesSha256OfCanonical(t *testing.T) {
	got := hashing.Component(helperVTODO())
	want := expectedVTODOHash(t)
	if got != want {
		t.Fatalf("Component hash mismatch.\n got:  %s\n want: %s", got, want)
	}
}

func TestComponent_PropertyOrderDoesNotMatter(t *testing.T) {
	a := helperVTODO()
	b := vstar.Component{
		Type: vstar.CompTodo,
		Props: []vstar.Property{
			{Name: "PRIORITY", Value: "3"},
			{Name: "SUMMARY", Value: "Buy milk"},
			{Name: "DTSTAMP", Value: "20260504T120000Z"},
			{Name: "UID", Value: "abc-123"},
		},
	}
	ah := hashing.Component(a)
	bh := hashing.Component(b)
	if ah != bh {
		t.Fatalf("Hash should be order-independent.\n a: %s\n b: %s", ah, bh)
	}
}

func TestComponent_ContentChangeYieldsDifferentHash(t *testing.T) {
	base := helperVTODO()
	tampered := helperVTODO()
	tampered.Props[2].Value = "Buy bread"
	if hashing.Component(base) == hashing.Component(tampered) {
		t.Fatal("Hash collision: tampering SUMMARY did not change hash")
	}
}

func TestHash_DeterministicAcross100Runs(t *testing.T) {
	c := helperVTODO()
	first := hashing.Component(c)
	for i := 0; i < 100; i++ {
		got := hashing.Component(c)
		if got != first {
			t.Fatalf("non-deterministic hash on iteration %d: got %s, first %s", i, got, first)
		}
	}
}

// helperCalendar returns a small VCALENDAR carrying one VTODO.
func helperCalendar() vstar.Calendar {
	return vstar.Calendar{
		ProdID:     "-//Test//EN",
		Components: []vstar.Component{helperVTODO()},
	}
}

// helperCard returns a minimal VCARD.
func helperCard() vstar.Card {
	return vstar.Card{
		UID:  "card-1",
		Kind: vstar.KindIndividual,
		Props: []vstar.Property{
			{Name: "FN", Value: "Reza"},
		},
	}
}

func TestCalendar_FormatPrefixAndLength(t *testing.T) {
	got := hashing.Calendar(helperCalendar())
	if !strings.HasPrefix(got, "sha256:") {
		t.Fatalf("Calendar hash missing sha256: prefix; got %q", got)
	}
	if len(strings.TrimPrefix(got, "sha256:")) != 64 {
		t.Fatalf("Calendar hash hex length != 64; got %q", got)
	}
}

func TestCalendar_MatchesSha256OfCanonical(t *testing.T) {
	cal := helperCalendar()
	got := hashing.Calendar(cal)
	sum := sha256.Sum256(canonical.Calendar(cal))
	want := "sha256:" + hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("Calendar hash mismatch.\n got:  %s\n want: %s", got, want)
	}
}

func TestCalendar_DeterministicAcross100Runs(t *testing.T) {
	cal := helperCalendar()
	first := hashing.Calendar(cal)
	for i := 0; i < 100; i++ {
		if got := hashing.Calendar(cal); got != first {
			t.Fatalf("Calendar hash not deterministic at i=%d", i)
		}
	}
}

func TestCard_FormatPrefixAndLength(t *testing.T) {
	got := hashing.Card(helperCard())
	if !strings.HasPrefix(got, "sha256:") {
		t.Fatalf("Card hash missing sha256: prefix; got %q", got)
	}
	if len(strings.TrimPrefix(got, "sha256:")) != 64 {
		t.Fatalf("Card hash hex length != 64; got %q", got)
	}
}

func TestCard_MatchesSha256OfCanonical(t *testing.T) {
	card := helperCard()
	got := hashing.Card(card)
	sum := sha256.Sum256(canonical.Card(card))
	want := "sha256:" + hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("Card hash mismatch.\n got:  %s\n want: %s", got, want)
	}
}

func TestCard_DeterministicAcross100Runs(t *testing.T) {
	card := helperCard()
	first := hashing.Card(card)
	for i := 0; i < 100; i++ {
		if got := hashing.Card(card); got != first {
			t.Fatalf("Card hash not deterministic at i=%d", i)
		}
	}
}

func TestSetXVSTAR_AddsPropertyToFreshComponent(t *testing.T) {
	c := helperVTODO()
	if _, ok := c.Get("X-VSTAR-HASH"); ok {
		t.Fatal("precondition: helperVTODO must not carry X-VSTAR-HASH")
	}
	hashing.SetXVSTAR(&c)
	p, ok := c.Get("X-VSTAR-HASH")
	if !ok {
		t.Fatal("SetXVSTAR did not add X-VSTAR-HASH property")
	}
	if !strings.HasPrefix(p.Value, "sha256:") {
		t.Fatalf("X-VSTAR-HASH value missing sha256: prefix; got %q", p.Value)
	}
}

func TestSetXVSTAR_ReplacesExistingNotDuplicate(t *testing.T) {
	c := helperVTODO()
	hashing.SetXVSTAR(&c)
	hashing.SetXVSTAR(&c)
	props := c.GetAll("X-VSTAR-HASH")
	if len(props) != 1 {
		t.Fatalf("expected single X-VSTAR-HASH after two SetXVSTAR calls, got %d", len(props))
	}
}

func TestSetXVSTAR_IsIdempotent(t *testing.T) {
	c := helperVTODO()
	hashing.SetXVSTAR(&c)
	first, _ := hashing.GetXVSTAR(c)
	hashing.SetXVSTAR(&c)
	second, _ := hashing.GetXVSTAR(c)
	if first != second {
		t.Fatalf("SetXVSTAR not idempotent: first=%s second=%s", first, second)
	}
}

func TestSetXVSTAR_NilSafe(t *testing.T) {
	hashing.SetXVSTAR(nil) // must not panic
}

func TestGetXVSTAR_ReturnsStoredValue(t *testing.T) {
	c := helperVTODO()
	hashing.SetXVSTAR(&c)
	stored, ok := hashing.GetXVSTAR(c)
	if !ok {
		t.Fatal("GetXVSTAR ok=false on component carrying X-VSTAR-HASH")
	}
	want := hashing.Component(c)
	if stored != want {
		t.Fatalf("stored hash != recomputed hash; stored=%s want=%s", stored, want)
	}
}

func TestGetXVSTAR_FalseWhenAbsent(t *testing.T) {
	c := helperVTODO()
	if v, ok := hashing.GetXVSTAR(c); ok || v != "" {
		t.Fatalf("GetXVSTAR on bare component must return (\"\", false); got (%q, %v)", v, ok)
	}
}

func TestVerifyXVSTAR_TrueOnConsistentComponent(t *testing.T) {
	c := helperVTODO()
	hashing.SetXVSTAR(&c)
	ok, want, got := hashing.VerifyXVSTAR(c)
	if !ok {
		t.Fatalf("VerifyXVSTAR false on freshly-stamped component; want=%s got=%s", want, got)
	}
	if want != got {
		t.Fatalf("VerifyXVSTAR ok=true but want != got (want=%s got=%s)", want, got)
	}
}

func TestVerifyXVSTAR_FalseOnTampered(t *testing.T) {
	c := helperVTODO()
	hashing.SetXVSTAR(&c)
	// Tamper with a non-hash property; the stored hash now no
	// longer matches the canonical bytes.
	c.Set(vstar.Property{Name: "SUMMARY", Value: "Buy bread"})
	ok, want, got := hashing.VerifyXVSTAR(c)
	if ok {
		t.Fatal("VerifyXVSTAR ok=true on tampered component")
	}
	if want == got {
		t.Fatalf("VerifyXVSTAR returned matching want/got despite tamper (%s)", want)
	}
	if got == "" {
		t.Fatal("VerifyXVSTAR got empty stored value despite property being set")
	}
}

// TestHash_ExcludesXVSTARHashProperty is the keystone hash-exclusion
// test per spec/03 §7 and the vstar-go-hashing T4 contract: a
// Component carrying X-VSTAR-HASH MUST hash to the same value as
// the same Component without X-VSTAR-HASH. Otherwise the stored
// hash would feed back into its own digest and Verify would never
// converge.
func TestHash_ExcludesXVSTARHashProperty(t *testing.T) {
	without := helperVTODO()
	with := helperVTODO()
	with.Add(vstar.Property{Name: "X-VSTAR-HASH", Value: "sha256:deadbeef"})

	hWithout := hashing.Component(without)
	hWith := hashing.Component(with)

	if hWithout != hWith {
		t.Fatalf("hash MUST exclude X-VSTAR-HASH from input.\n without: %s\n with:    %s", hWithout, hWith)
	}
}

// TestHash_ExclusionIsCaseInsensitive locks in that the strip pass
// matches X-VSTAR-HASH regardless of the wire case the producer
// emitted (RFC 5545 §3.1 names are case-insensitive).
func TestHash_ExclusionIsCaseInsensitive(t *testing.T) {
	base := helperVTODO()
	tagged := helperVTODO()
	tagged.Add(vstar.Property{Name: "x-vstar-hash", Value: "sha256:beefdead"})

	if hashing.Component(base) != hashing.Component(tagged) {
		t.Fatal("hash exclusion must be case-insensitive on property name")
	}
}

// TestHash_ExclusionDoesNotMutateInput pins that filtering happens
// on a copy — callers see no change to their Component.Props.
func TestHash_ExclusionDoesNotMutateInput(t *testing.T) {
	c := helperVTODO()
	c.Add(vstar.Property{Name: "X-VSTAR-HASH", Value: "sha256:cafebabe"})
	beforeLen := len(c.Props)
	_ = hashing.Component(c)
	if len(c.Props) != beforeLen {
		t.Fatalf("hashing.Component mutated input Props (was %d, now %d)", beforeLen, len(c.Props))
	}
	if _, ok := c.Get("X-VSTAR-HASH"); !ok {
		t.Fatal("hashing.Component mutated input — X-VSTAR-HASH gone from caller's Component")
	}
}

func TestVerifyXVSTAR_FalseWhenAbsentReturnsRecomputed(t *testing.T) {
	c := helperVTODO()
	ok, want, got := hashing.VerifyXVSTAR(c)
	if ok {
		t.Fatal("VerifyXVSTAR ok=true on component without X-VSTAR-HASH")
	}
	if got != "" {
		t.Fatalf("VerifyXVSTAR got=%q, want empty when no stored hash", got)
	}
	if !strings.HasPrefix(want, "sha256:") {
		t.Fatalf("VerifyXVSTAR want must be the recomputed hash even when absent; got %q", want)
	}
}
