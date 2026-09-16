// SPDX-License-Identifier: MIT

package rfc5545_test

import (
	"bytes"
	"strings"
	"testing"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/rfc5545"
)

func TestNew_ReturnsCodecImplementation(t *testing.T) {
	t.Parallel()

	c := rfc5545.New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	// Structurally satisfies all three interfaces. Each helper takes
	// its interface as its parameter type, forcing a compile-time
	// implementability check that staticcheck cannot replace.
	useCodec(t, c)
	useParser(t, c)
	useEncoder(t, c)
}

func useCodec(t *testing.T, _ vstar.Codec)     { t.Helper() }
func useParser(t *testing.T, _ vstar.Parser)   { t.Helper() }
func useEncoder(t *testing.T, _ vstar.Encoder) { t.Helper() }

func TestDefault_NotNil(t *testing.T) {
	t.Parallel()

	if rfc5545.Default == nil {
		t.Fatal("Default is nil; package init failed")
	}
}

func TestNew_RoundTrip(t *testing.T) {
	t.Parallel()

	src := strings.Join([]string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//V*//Codec//EN",
		"BEGIN:VTODO",
		"UID:codec-1",
		"DTSTAMP:20260504T120000Z",
		"SUMMARY:via codec",
		"END:VTODO",
		"END:VCALENDAR",
	}, "\r\n") + "\r\n"

	c := rfc5545.New()
	cal, err := c.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var buf bytes.Buffer
	if err := c.Encode(&buf, cal); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.Contains(buf.String(), "BEGIN:VTODO\r\n") {
		t.Errorf("encoded output missing VTODO:\n%s", buf.String())
	}
}

func TestDefault_RoundTrip(t *testing.T) {
	t.Parallel()

	src := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:p\r\nEND:VCALENDAR\r\n"
	cal, err := rfc5545.Default.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Default.Parse: %v", err)
	}
	if cal.ProdID != "p" {
		t.Errorf("ProdID = %q", cal.ProdID)
	}
}
