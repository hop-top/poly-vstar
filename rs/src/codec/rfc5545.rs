// SPDX-License-Identifier: MIT

//! The iCalendar (VCALENDAR) wire format, RFC 5545.
//!
//! A content-line scanner with §3.1 unfolding, a content-line parser
//! (§3.2), a recursive BEGIN/END block parser (§3.4–§3.6), and an encoder
//! producing CRLF output folded at 75 octets.
//!
//! Parsing preserves property, parameter and component order verbatim and
//! performs no canonicalization — normalization is the `canonical`
//! layer's job, and doing any of it here would make the canonical form
//! depend on which codec produced the model.

use crate::codec::contentline::{
    encode_param_value, find_unquoted, split_unquoted, unquote, write_folded,
};
use crate::model::{Calendar, Component, Param, Property};
use crate::{CompType, Error, Result};
use std::io::{Read, Write};

pub use crate::codec::contentline::Scanner;

/// The only iCalendar `VERSION` value V\* honors at v0.1.
const SUPPORTED_VERSION: &str = "2.0";

/// Builds a scanner over `r`.
pub fn new_scanner(r: impl Read) -> Scanner {
    Scanner::new(r)
}

/// Parses a single VCALENDAR from `r`.
///
/// Errors carry positional context — the offending content line — and one
/// of [`Error::Malformed`], [`Error::UnclosedBlock`] or
/// [`Error::UnsupportedVersion`].
///
/// Trailing content after `END:VCALENDAR` is ignored intentionally:
/// producers concatenate streams and scanners round up trailing
/// whitespace, so the stance is "we got a valid calendar; stop reading".
pub fn parse(r: impl Read) -> Result<Calendar> {
    let mut s = Scanner::new(r);

    let Some(first) = s.next_line()? else {
        return Err(Error::Malformed("empty input".into()));
    };
    if !first.eq_ignore_ascii_case("BEGIN:VCALENDAR") {
        return Err(Error::Malformed(format!(
            "expected BEGIN:VCALENDAR, got {first:?}"
        )));
    }

    let root = parse_block(&mut s, "VCALENDAR")?;

    let mut cal = Calendar::default();
    for p in &root.props {
        if p.name.eq_ignore_ascii_case("VERSION") {
            if p.value != SUPPORTED_VERSION {
                return Err(Error::UnsupportedVersion(format!(
                    "VERSION={:?} (only {SUPPORTED_VERSION:?} supported)",
                    p.value
                )));
            }
        } else if p.name.eq_ignore_ascii_case(PROP_PRODID) {
            cal.prod_id.clone_from(&p.value);
        }
    }
    cal.components = root.sub;
    Ok(cal)
}

/// Consumes content lines until `END:<type_name>`, assembling one
/// component. A mismatched `END` name is [`Error::Malformed`]; end of
/// input before the `END` is [`Error::UnclosedBlock`].
fn parse_block(s: &mut Scanner, type_name: &str) -> Result<Component> {
    // `from_wire` keeps an unregistered name — STANDARD, DAYLIGHT, an
    // X- extension — verbatim, so a VTIMEZONE's sub-blocks survive the
    // round-trip instead of collapsing onto a named variant.
    let ty = CompType::from_wire(type_name);
    let mut out = Component {
        r#type: ty.clone(),
        props: Vec::new(),
        sub: Vec::new(),
    };

    loop {
        let Some(line) = s.next_line()? else {
            return Err(Error::UnclosedBlock(format!(
                "BEGIN:{type_name} never closed"
            )));
        };

        let prop = parse_content_line(&line)?;

        if prop.name.eq_ignore_ascii_case("BEGIN") {
            out.sub.push(parse_block(s, &prop.value)?);
            continue;
        }
        if prop.name.eq_ignore_ascii_case("END") {
            if !ty.eq_fold(&prop.value) {
                return Err(Error::Malformed(format!(
                    "END:{} does not match BEGIN:{type_name}",
                    prop.value
                )));
            }
            return Ok(out);
        }

        // Unescape TEXT-typed values per RFC 5545 §3.3.11 so the model
        // holds raw values. The encoder re-applies escaping symmetrically;
        // that pairing is what makes parse → encode byte-stable.
        let mut prop = prop;
        if is_text_property(&prop.name) {
            prop.value = unescape_text(&prop.value);
        }
        out.props.push(prop);
    }
}

/// Parses a single already-unfolded content line into a [`Property`].
///
/// The grammar (RFC 5545 §3.1):
///
/// ```text
/// contentline = name *(";" param) ":" value
/// param       = param-name "=" param-value *("," param-value)
/// param-value = paramtext / quoted-string
/// ```
///
/// Quoted parameter values may contain commas, semicolons and colons;
/// unquoted ones may not. The wire case of names is preserved verbatim —
/// case-insensitive matching belongs to the accessors.
///
/// Returns [`Error::Malformed`] for a missing colon, an empty name, a
/// parameter without `=`, or an unbalanced DQUOTE.
pub fn parse_content_line(line: &str) -> Result<Property> {
    let colon = find_unquoted(line, ':')?
        .ok_or_else(|| Error::Malformed(format!("missing colon in content line {line:?}")))?;
    let head = &line[..colon];
    let value = &line[colon + 1..];

    let segs = split_unquoted(head, ';')?;
    let name = segs.first().copied().unwrap_or("");
    if name.is_empty() {
        return Err(Error::Malformed(format!("empty property name in {line:?}")));
    }

    let mut params = Vec::with_capacity(segs.len().saturating_sub(1));
    for seg in &segs[1..] {
        let eq = seg
            .find('=')
            .filter(|i| *i > 0)
            .ok_or_else(|| Error::Malformed(format!("malformed parameter {seg:?} in {line:?}")))?;
        params.push(Param {
            name: seg[..eq].to_owned(),
            value: unquote(&seg[eq + 1..]).to_owned(),
        });
    }

    Ok(Property {
        name: name.to_owned(),
        params,
        value: value.to_owned(),
    })
}

/// Writes `cal` to `w` in RFC 5545 wire format.
///
/// Output is always CRLF-terminated and folded at 75 octets. Property and
/// component order is preserved verbatim. The VCALENDAR wrapper is always
/// emitted with `VERSION:2.0` and a `PRODID` derived from
/// [`Calendar::prod_id`]; any inline calendar-level `VERSION` / `PRODID`
/// is ignored at this layer.
pub fn encode(mut w: impl Write, cal: &Calendar) -> std::io::Result<()> {
    let mut out = Vec::<u8>::new();
    write_folded(&mut out, "BEGIN:VCALENDAR");
    write_folded(&mut out, &format!("VERSION:{SUPPORTED_VERSION}"));
    write_folded(
        &mut out,
        &encode_content_line(&Property::new(PROP_PRODID, &cal.prod_id)),
    );
    for c in &cal.components {
        push_component(&mut out, c);
    }
    write_folded(&mut out, "END:VCALENDAR");
    w.write_all(&out)
}

/// Writes one [`Component`] — its BEGIN/END wrapper, properties and
/// recursive sub-components — exactly as [`encode`] would emit it inside
/// the VCALENDAR wrapper.
///
/// No wrapper, no `VERSION`, no `PRODID`. This is the building block
/// canonicalization reuses so both share one fold/CRLF implementation.
pub fn encode_component(mut w: impl Write, c: &Component) -> std::io::Result<()> {
    let mut out = Vec::<u8>::new();
    push_component(&mut out, c);
    w.write_all(&out)
}

fn push_component(out: &mut Vec<u8>, c: &Component) {
    let tname = c.r#type.as_str();
    write_folded(out, &format!("BEGIN:{tname}"));
    for p in &c.props {
        write_folded(out, &encode_content_line(p));
    }
    for s in &c.sub {
        push_component(out, s);
    }
    write_folded(out, &format!("END:{tname}"));
}

/// Renders a [`Property`] as one unfolded wire content line:
/// `NAME[;PARAM=val]*:value`.
///
/// Names and parameter names uppercase per RFC 5545 §3.1; values do not.
/// TEXT-typed values are escaped per §3.3.11, everything else emits
/// verbatim.
fn encode_content_line(p: &Property) -> String {
    let mut b = String::with_capacity(p.name.len() + p.value.len() + 16);
    b.push_str(&p.name.to_ascii_uppercase());
    for par in &p.params {
        b.push(';');
        b.push_str(&par.name.to_ascii_uppercase());
        b.push('=');
        b.push_str(&encode_param_value(&par.value));
    }
    b.push(':');
    if is_text_property(&p.name) {
        b.push_str(&escape_text(&p.value));
    } else {
        b.push_str(&p.value);
    }
    b
}

const PROP_PRODID: &str = "PRODID";

/// The allow-list of TEXT-typed property names, RFC 5545 §3.3.11 /
/// §3.7–§3.8 and RFC 6350 §3.4.
///
/// Only TEXT values are backslash-escaped on emit: escaping a comma in a
/// URI would corrupt the address. Custom properties (`X-` extensions and
/// unknown names) are deliberately NOT treated as TEXT.
///
/// This list is shared with canonicalization by contract — the two
/// callers MUST agree on what counts as TEXT, or canonical output and raw
/// encoder output diverge for the same input.
const TEXT_PROPERTIES: [&str; 23] = [
    // RFC 5545 calendar TEXT properties.
    "CATEGORIES",
    "CLASS",
    "COMMENT",
    "CONTACT",
    "DESCRIPTION",
    "LOCATION",
    PROP_PRODID,
    "RELATED-TO",
    "RESOURCES",
    "STATUS",
    "SUMMARY",
    "TRANSP",
    "TZID",
    "TZNAME",
    "UID",
    // RFC 6350 vCard TEXT properties.
    "FN",
    "N",
    "NICKNAME",
    "NOTE",
    "ORG",
    "TITLE",
    "ROLE",
    "KIND",
];

/// Reports whether `name` is TEXT-typed per the allow-list,
/// case-insensitively.
pub(crate) fn is_text_property(name: &str) -> bool {
    TEXT_PROPERTIES.iter().any(|t| name.eq_ignore_ascii_case(t))
}

/// Reverses RFC 5545 §3.3.11 TEXT escaping.
///
/// `\\` → `\`, `\,` → `,`, `\;` → `;`, `\n` and `\N` → LF.
///
/// A trailing solitary backslash is preserved verbatim, mirroring
/// real-world parser leniency and keeping the function total. An unknown
/// two-character escape drops the backslash and keeps the character,
/// matching common practice.
pub(crate) fn unescape_text(s: &str) -> String {
    if !s.contains('\\') {
        return s.to_owned();
    }
    let mut out = String::with_capacity(s.len());
    let mut chars = s.chars();
    while let Some(c) = chars.next() {
        if c != '\\' {
            out.push(c);
            continue;
        }
        match chars.next() {
            Some('\\') => out.push('\\'),
            Some(',') => out.push(','),
            Some(';') => out.push(';'),
            Some('n' | 'N') => out.push('\n'),
            Some(other) => out.push(other),
            // A trailing backslash with nothing after it.
            None => out.push('\\'),
        }
    }
    out
}

/// Applies RFC 5545 §3.3.11 TEXT escaping for encoding.
///
/// `\` → `\\`, `,` → `\,`, `;` → `\;`, LF → `\n`. CR is dropped, so a
/// literal CRLF collapses to a single escaped `\n`.
///
/// Not idempotent — a literal backslash re-escapes on a second pass. The
/// encoder calls this exactly once per emit, paired with
/// [`unescape_text`] on the parse side so the model holds raw values.
pub(crate) fn escape_text(s: &str) -> String {
    if !s.contains(['\\', ',', ';', '\n', '\r']) {
        return s.to_owned();
    }
    let mut out = String::with_capacity(s.len() + 4);
    for c in s.chars() {
        match c {
            '\\' => out.push_str("\\\\"),
            ',' => out.push_str("\\,"),
            ';' => out.push_str("\\;"),
            '\n' => out.push_str("\\n"),
            // Canonical TEXT uses a bare LF for embedded newlines.
            '\r' => {}
            other => out.push(other),
        }
    }
    out
}

/// The RFC 5545 codec: parse and encode composed into one value.
///
/// Stateless, so a single instance is safe to share.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct Rfc5545Codec;

impl Rfc5545Codec {
    /// Parses a VCALENDAR from `r`.
    pub fn parse(&self, r: impl Read) -> Result<Calendar> {
        parse(r)
    }

    /// Writes `cal` to `w`.
    pub fn encode(&self, w: impl Write, cal: &Calendar) -> std::io::Result<()> {
        encode(w, cal)
    }
}

/// A fresh codec instance.
pub fn new_codec() -> Rfc5545Codec {
    Rfc5545Codec
}

/// The shared default codec. Stateless, so this and [`new_codec`] are
/// interchangeable.
pub fn default_codec() -> Rfc5545Codec {
    Rfc5545Codec
}
