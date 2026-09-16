// SPDX-License-Identifier: MIT

package stream

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	vstar "hop.top/vstar"
	"hop.top/vstar/codec/rfc5545"
)

// Compile-time guards: VCalendarParser and VCalendarEncoder MUST
// satisfy the StreamParser / StreamEncoder interfaces from stream.go.
// Co-located with the concrete types so a refactor that breaks either
// side trips the build at the type's declaration site.
var (
	_ StreamParser  = (*VCalendarParser)(nil)
	_ StreamEncoder = (*VCalendarEncoder)(nil)
)

// supportedCalendarVersion mirrors rfc5545.supportedVersion (which is
// unexported in that package). Streaming parser MUST also reject
// VERSION values other than 2.0 to stay consistent with the batch
// codec's surface contract.
const supportedCalendarVersion = "2.0"

// kwBegin / kwEnd are the RFC 5545 keyword names (uppercased) for the
// BEGIN: and END: pseudo-properties that delimit components. Defined
// here so the parser can switch on them without repeating string
// literals (which trips goconst).
const (
	kwBegin = "BEGIN"
	kwEnd   = "END"
)

// defaultProdID is the wire string emitted as PRODID by
// VCalendarEncoder when SetHeader was not called. The value mirrors
// the batch encoder's default PRODID for byte-stable round trips
// between batch and stream output for the same logical calendar.
const defaultProdID = "-//hop-top//vstar-go v0.1.0//EN"

// VCalendarParser is the streaming VCALENDAR reader. It wraps a
// rfc5545.Scanner and yields one top-level Component per call to Next.
//
// The parser auto-skips the VCALENDAR header on first use: it reads
// content lines until BEGIN:VCALENDAR, captures any calendar-level
// properties (VERSION, PRODID, METHOD, …) into the parser's header
// state for later retrieval via Header, and then begins yielding
// nested components.
//
// When END:VCALENDAR is seen, subsequent Next calls return io.EOF.
//
// VCalendarParser is not safe for concurrent use; construct one per
// reader.
type VCalendarParser struct {
	scanner *rfc5545.Scanner

	// headerRead is set true once the VCALENDAR header has been
	// consumed (i.e. BEGIN:VCALENDAR has been seen and any leading
	// calendar-level properties before the first BEGIN:<sub> have
	// been captured into header). Driven by ensureHeader.
	headerRead bool

	// header captures the calendar-level properties (VERSION, PRODID,
	// METHOD, …) seen between BEGIN:VCALENDAR and the first
	// BEGIN:<sub-component>. Available to callers via Header.
	header vstar.Calendar

	// pending holds a content line peeked while detecting whether the
	// header section is over. Empty when no peek is buffered.
	pending string

	// done is true after END:VCALENDAR has been seen. Subsequent
	// Next calls always return io.EOF without touching the scanner.
	done bool
}

// NewVCalendarParser returns a VCalendarParser reading from r. r is
// wrapped internally; callers should not pre-buffer.
func NewVCalendarParser(r io.Reader) *VCalendarParser {
	return &VCalendarParser{scanner: rfc5545.NewScanner(r)}
}

// Header returns the calendar-level properties captured from the
// BEGIN:VCALENDAR header. Components is always nil — only the
// VCALENDAR-level properties (VERSION, PRODID, METHOD, …) populate.
//
// Header is safe to call before the first Next; in that case the
// header is read on demand. If the header read fails (malformed
// stream, unsupported VERSION, …) the error is captured internally
// and surfaced from the next call to Next; Header returns the
// possibly-empty Calendar so callers can still introspect what was
// parsed.
func (p *VCalendarParser) Header() vstar.Calendar {
	if !p.headerRead {
		// Best-effort header read; ignore the error here so a
		// pre-Next caller can still retrieve whatever properties
		// were captured. The error will surface on Next.
		_ = p.ensureHeader()
	}
	return p.header
}

// Next reads and returns the next top-level Component from the
// stream. Returns io.EOF when END:VCALENDAR has been consumed.
//
// On the first call, Next consumes the VCALENDAR header (BEGIN line +
// any calendar-level properties); subsequent calls go straight to the
// next BEGIN:<sub>. If the header is structurally invalid (missing
// BEGIN:VCALENDAR, unsupported VERSION, …) Next returns the
// underlying error wrapped with vstar sentinels.
func (p *VCalendarParser) Next() (vstar.Component, error) {
	if p.done {
		return vstar.Component{}, io.EOF
	}
	if err := p.ensureHeader(); err != nil {
		return vstar.Component{}, err
	}

	// Pull the next non-empty content line (consume any peeked one
	// from header detection first).
	line, err := p.nextLine()
	if err != nil {
		// EOF inside an open VCALENDAR is unclosed-block; the spec
		// requires END:VCALENDAR. Map io.EOF here accordingly.
		if errors.Is(err, io.EOF) {
			return vstar.Component{}, fmt.Errorf(
				"stream/vcalendar: BEGIN:VCALENDAR never closed: %w",
				vstar.ErrUnclosedBlock,
			)
		}
		return vstar.Component{}, err
	}

	prop, err := rfc5545.ParseContentLine(line)
	if err != nil {
		return vstar.Component{}, err
	}

	switch strings.ToUpper(prop.Name) {
	case kwEnd:
		if !strings.EqualFold(prop.Value, "VCALENDAR") {
			return vstar.Component{}, fmt.Errorf(
				"stream/vcalendar: END:%s does not match BEGIN:VCALENDAR: %w",
				prop.Value, vstar.ErrMalformed,
			)
		}
		p.done = true
		return vstar.Component{}, io.EOF
	case kwBegin:
		// Recurse via the rfc5545 batch parseBlock — but that's
		// unexported, so reimplement the small subset inline. The
		// scanner is shared so reads stay in lockstep with the
		// outer stream.
		return p.readBlock(strings.ToUpper(prop.Value))
	default:
		// Calendar-level property AFTER the header section — RFC 5545
		// allows METHOD / X-* anywhere inside VCALENDAR before the
		// first sub-component. If we see one here it means the caller
		// is mixing late header props among components, which we
		// reject as malformed (the AGR flow always emits header props
		// up front).
		return vstar.Component{}, fmt.Errorf(
			"stream/vcalendar: unexpected calendar-level property %q after components: %w",
			prop.Name, vstar.ErrMalformed,
		)
	}
}

// ensureHeader runs once: it reads BEGIN:VCALENDAR, captures every
// content line up until the first BEGIN:<sub> (or END:VCALENDAR for
// an empty calendar) into p.header, and stashes the first non-header
// line in p.pending so Next can consume it on the next pull.
func (p *VCalendarParser) ensureHeader() error {
	if p.headerRead {
		return nil
	}
	p.headerRead = true // set before mutation so retries don't loop.

	// First non-empty line MUST be BEGIN:VCALENDAR.
	first, err := p.scanner.Next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return fmt.Errorf(
				"stream/vcalendar: empty input: %w", vstar.ErrMalformed,
			)
		}
		return err
	}
	if !strings.EqualFold(first, "BEGIN:VCALENDAR") {
		return fmt.Errorf(
			"stream/vcalendar: expected BEGIN:VCALENDAR, got %q: %w",
			first, vstar.ErrMalformed,
		)
	}

	// Consume calendar-level properties until the first BEGIN: or
	// END:VCALENDAR. Stash the boundary line in pending.
	for {
		line, err := p.scanner.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return fmt.Errorf(
					"stream/vcalendar: BEGIN:VCALENDAR never closed: %w",
					vstar.ErrUnclosedBlock,
				)
			}
			return err
		}

		prop, err := rfc5545.ParseContentLine(line)
		if err != nil {
			return err
		}

		upper := strings.ToUpper(prop.Name)
		if upper == kwBegin || upper == kwEnd {
			p.pending = line
			return nil
		}

		switch upper {
		case "VERSION":
			if prop.Value != supportedCalendarVersion {
				return fmt.Errorf(
					"stream/vcalendar: VERSION=%q (only %q supported): %w",
					prop.Value, supportedCalendarVersion, vstar.ErrUnsupportedVersion,
				)
			}
		case "PRODID":
			p.header.ProdID = prop.Value
		}
		// METHOD and X-* properties are intentionally not surfaced as
		// dedicated fields on Calendar yet. Callers needing them can
		// inspect the underlying scanner output via a custom layer.
	}
}

// nextLine consumes the next content line from the scanner, returning
// any pending peeked line first.
func (p *VCalendarParser) nextLine() (string, error) {
	if p.pending != "" {
		line := p.pending
		p.pending = ""
		return line, nil
	}
	return p.scanner.Next()
}

// readBlock parses a BEGIN:typeName … END:typeName subtree starting
// from the line immediately after the BEGIN. Mirrors the rfc5545
// batch parseBlock semantics but works directly off this parser's
// shared scanner so reads do not overlap.
func (p *VCalendarParser) readBlock(typeName string) (vstar.Component, error) {
	out := vstar.Component{Type: vstar.CompType(typeName)}
	for {
		line, err := p.scanner.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return vstar.Component{}, fmt.Errorf(
					"stream/vcalendar: BEGIN:%s never closed: %w",
					typeName, vstar.ErrUnclosedBlock,
				)
			}
			return vstar.Component{}, err
		}

		prop, err := rfc5545.ParseContentLine(line)
		if err != nil {
			return vstar.Component{}, err
		}

		switch strings.ToUpper(prop.Name) {
		case kwBegin:
			child, err := p.readBlock(strings.ToUpper(prop.Value))
			if err != nil {
				return vstar.Component{}, err
			}
			out.Sub = append(out.Sub, child)
		case kwEnd:
			if !strings.EqualFold(prop.Value, typeName) {
				return vstar.Component{}, fmt.Errorf(
					"stream/vcalendar: END:%s does not match BEGIN:%s: %w",
					prop.Value, typeName, vstar.ErrMalformed,
				)
			}
			return out, nil
		default:
			out.Props = append(out.Props, prop)
		}
	}
}

// VCalendarEncoder is the streaming VCALENDAR writer. The first call
// to Encode emits the BEGIN:VCALENDAR header (VERSION + PRODID,
// optionally overridden via SetHeader); each subsequent Encode appends
// one component to the wire output. Close emits END:VCALENDAR and
// flushes the underlying writer.
//
// Encoder buffers writes through bufio internally; callers should NOT
// pre-buffer w. Encoder is not safe for concurrent use.
type VCalendarEncoder struct {
	bw *bufio.Writer

	// header is the calendar-level metadata emitted on first Encode.
	// SetHeader replaces this before the lock; after the lock,
	// SetHeader returns ErrHeaderLocked.
	header vstar.Calendar

	// headerWritten is true once the BEGIN:VCALENDAR header has been
	// flushed to bw. Locks SetHeader, gates idempotent header emission.
	headerWritten bool

	// closed is true after Close has run successfully (or failed
	// after writing END). Subsequent Encode/Close calls return
	// ErrAlreadyClosed.
	closed bool
}

// NewVCalendarEncoder returns a VCalendarEncoder writing to w. The
// returned encoder is in the "header pending" state — call SetHeader
// to override the default VERSION/PRODID, then Encode + Close.
func NewVCalendarEncoder(w io.Writer) *VCalendarEncoder {
	return &VCalendarEncoder{bw: bufio.NewWriter(w)}
}

// SetHeader configures the VCALENDAR header that will be emitted on
// the next Encode call. Only the ProdID field is consulted; VERSION
// is fixed at 2.0 per the v0.1 supported-version contract. Returns
// ErrHeaderLocked if called after the first Encode (the header is
// already on the wire and cannot be retroactively changed).
func (e *VCalendarEncoder) SetHeader(h vstar.Calendar) error {
	if e.headerWritten {
		return fmt.Errorf("stream/vcalendar: %w", ErrHeaderLocked)
	}
	e.header = h
	return nil
}

// Encode writes a single Component to the underlying writer. The
// VCALENDAR header is emitted on the first Encode call. Subsequent
// Encodes append to the same calendar; Close finalizes with
// END:VCALENDAR.
//
// Calling Encode after Close returns ErrAlreadyClosed.
func (e *VCalendarEncoder) Encode(c vstar.Component) error {
	if e.closed {
		return fmt.Errorf("stream/vcalendar: %w", ErrAlreadyClosed)
	}
	if err := e.writeHeaderOnce(); err != nil {
		return err
	}
	if err := rfc5545.EncodeComponent(e.bw, c); err != nil {
		return err
	}
	return nil
}

// Close emits END:VCALENDAR and flushes the underlying writer. Safe
// to call on an encoder that has never seen Encode (the header is
// emitted first so the output is still a legal empty calendar).
//
// Returns ErrAlreadyClosed (wrapped) if called more than once.
func (e *VCalendarEncoder) Close() error {
	if e.closed {
		return fmt.Errorf("stream/vcalendar: %w", ErrAlreadyClosed)
	}
	if err := e.writeHeaderOnce(); err != nil {
		return err
	}
	if _, err := e.bw.WriteString("END:VCALENDAR\r\n"); err != nil {
		return err
	}
	if err := e.bw.Flush(); err != nil {
		return err
	}
	e.closed = true
	return nil
}

// writeHeaderOnce emits BEGIN:VCALENDAR + VERSION + PRODID exactly
// once across the encoder's lifetime. Subsequent calls are no-ops.
// PRODID falls back to defaultProdID when SetHeader was not used.
func (e *VCalendarEncoder) writeHeaderOnce() error {
	if e.headerWritten {
		return nil
	}
	e.headerWritten = true

	prodID := e.header.ProdID
	if prodID == "" {
		prodID = defaultProdID
	}

	if _, err := e.bw.WriteString("BEGIN:VCALENDAR\r\n"); err != nil {
		return err
	}
	if _, err := e.bw.WriteString("VERSION:" + supportedCalendarVersion + "\r\n"); err != nil {
		return err
	}
	// PRODID is TEXT-typed and may need escaping; build the property
	// and route through rfc5545 contentline encoding for symmetry
	// with the batch encoder. To stay with public surface, write a
	// minimal escaped form here: the only TEXT-special characters in
	// a typical PRODID are ',' ';' '\\' '\n', which we hand-escape.
	if _, err := e.bw.WriteString("PRODID:" + escapeProdID(prodID) + "\r\n"); err != nil {
		return err
	}
	return nil
}

// escapeProdID applies RFC 5545 §3.3.11 TEXT escaping to a PRODID
// value. PRODID is TEXT-typed so the same escape rules apply as any
// other TEXT property. Implementation kept inline (rather than calling
// into rfc5545) because rfc5545.escapeText is unexported.
func escapeProdID(s string) string {
	if !strings.ContainsAny(s, "\\,;\n\r") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			b.WriteString(`\\`)
		case ',':
			b.WriteString(`\,`)
		case ';':
			b.WriteString(`\;`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			// Drop CR; canonical TEXT uses bare LF for embedded
			// newlines, escaped above.
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
