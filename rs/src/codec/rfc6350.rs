// SPDX-License-Identifier: MIT

//! The vCard 4.0 wire format, RFC 6350.
//!
//! Line unfolding (§3.2, which defers to RFC 5545 §3.1), content-line
//! parsing including `group.` prefixes (§3.3), TEXT escaping (§3.4),
//! `BEGIN:VCARD…END:VCARD` framing, and the symmetric encoder with
//! 75-octet folding and CRLF terminators.
//!
//! `VERSION:4.0` is the only version accepted; `VERSION:3.0`
//! yields [`Error::UnsupportedVersion`].
//!
//! # UID handling is asymmetric
//!
//! [`parse`] **accepts** a VCARD with no `UID` — the resulting
//! [`Card::uid`] is `""`. [`encode`] **refuses** an empty `Card::uid`
//! with [`Error::MissingUid`]. Validation reports the missing `UID` at
//! the semantic layer. The parser is deliberately permissive: adopters
//! wanting strict-on-read wrap it with validation before storing.
//!
//! This asymmetry is easy to half-implement. A port that raises the error
//! only on the parse side passes the `malformed/missing_uid.vcf` fixture
//! — the fixture fails either way — while being wrong about both halves.

use crate::codec::contentline::{find_unquoted, unquote, write_folded, Scanner};
use crate::model::{Card, Param, Property};
use crate::{Error, Kind, Result};
use std::io::{Read, Write};

/// The only vCard `VERSION` value accepted.
const SUPPORTED_VERSION: &str = "4.0";

/// Parses zero or more `BEGIN:VCARD … END:VCARD` blocks from `r`.
///
/// Returns a **`Vec`**, not one card: a vCard stream is a sequence of
/// self-contained blocks with no enclosing wrapper, so a file with three
/// cards parses to three [`Card`] values. A port whose `parse` returns a
/// single card passes every single-card fixture and fails the rest.
///
/// Empty input yields an empty `Vec` rather than an error — callers MUST
/// treat zero cards as "no cards".
///
/// Errors: [`Error::Malformed`] for syntactic defects (missing `VERSION`,
/// a stray `END`, a nested `BEGIN`), [`Error::UnsupportedVersion`] when
/// `VERSION` is present but not `4.0`, and [`Error::UnclosedBlock`] when
/// input ends inside an open VCARD.
pub fn parse(r: impl Read) -> Result<Vec<Card>> {
    let lines = Scanner::new(r).collect_lines()?;

    let mut cards = Vec::new();
    let mut open = false;
    let mut cur = Card::default();
    let mut version = String::new();

    for (i, line) in lines.iter().enumerate() {
        let n = i + 1;
        if line.is_empty() {
            continue;
        }

        if line.eq_ignore_ascii_case("BEGIN:VCARD") {
            if open {
                return Err(Error::Malformed(format!("line {n}: nested BEGIN:VCARD")));
            }
            open = true;
            cur = Card::default();
            version.clear();
            continue;
        }

        if line.eq_ignore_ascii_case("END:VCARD") {
            if !open {
                return Err(Error::Malformed(format!("line {n}: stray END:VCARD")));
            }
            if version.is_empty() {
                return Err(Error::Malformed("VCARD missing VERSION".into()));
            }
            if version != SUPPORTED_VERSION {
                return Err(Error::UnsupportedVersion(format!("VERSION:{version}")));
            }
            cards.push(std::mem::take(&mut cur));
            open = false;
            version.clear();
            continue;
        }

        if !open {
            // Real vCards never carry content outside BEGIN/END.
            return Err(Error::Malformed(format!("line {n}: content outside VCARD")));
        }

        let prop =
            parse_content_line(line).map_err(|e| Error::Malformed(format!("line {n}: {e}")))?;

        // VERSION, UID and KIND are lifted into the Card's own fields
        // rather than kept in props, so the encoder can emit them in a
        // fixed order.
        if prop.name.eq_ignore_ascii_case("VERSION") {
            if !version.is_empty() {
                return Err(Error::Malformed(format!("line {n}: duplicate VERSION")));
            }
            version = prop.value;
            continue;
        }
        if prop.name.eq_ignore_ascii_case("UID") {
            cur.uid = prop.value;
            continue;
        }
        if prop.name.eq_ignore_ascii_case("KIND") {
            // RFC 6350 §6.1.4 registers lowercase values; the wire form
            // is case-insensitive on read.
            cur.kind = Kind::parse(&prop.value);
            continue;
        }
        cur.props.push(prop);
    }

    if open {
        return Err(Error::UnclosedBlock("VCARD never closed".into()));
    }
    Ok(cards)
}

/// Decomposes one unfolded vCard content line into a [`Property`] per
/// RFC 6350 §3.3 / §3.4.
///
/// Format: `[group "."] name *(";" param) ":" value`. The group prefix,
/// when present, is preserved verbatim in [`Property::name`] (e.g.
/// `home.TEL`), because the group's case is part of the round-trip.
pub fn parse_content_line(line: &str) -> Result<Property> {
    if line.is_empty() {
        return Err(Error::Malformed("empty line".into()));
    }

    let colon = find_unquoted(line, ':')?
        .ok_or_else(|| Error::Malformed("missing value separator".into()))?;
    let head = &line[..colon];
    let raw_value = &line[colon + 1..];

    let (name, param_tok) = match find_unquoted(head, ';')? {
        Some(i) => (&head[..i], &head[i + 1..]),
        None => (head, ""),
    };
    if name.is_empty() {
        return Err(Error::Malformed("empty property name".into()));
    }

    Ok(Property {
        name: name.to_owned(),
        params: parse_params(param_tok)?,
        value: unescape_text(raw_value),
    })
}

/// Parses the parameter portion of a content-line head.
///
/// Each parameter is `NAME=VALUE`; the value may be DQUOTE-wrapped to
/// embed `,`, `:` or `;`.
fn parse_params(s: &str) -> Result<Vec<Param>> {
    if s.is_empty() {
        return Ok(Vec::new());
    }
    let mut params = Vec::new();
    let mut rest = s;
    while !rest.is_empty() {
        let (tok, next) = match find_unquoted(rest, ';')? {
            Some(i) => (&rest[..i], &rest[i + 1..]),
            None => (rest, ""),
        };
        rest = next;

        let eq = find_unquoted(tok, '=')?
            .ok_or_else(|| Error::Malformed(format!("param {tok:?} missing '='")))?;
        let name = &tok[..eq];
        if name.is_empty() {
            return Err(Error::Malformed("empty param name".into()));
        }
        params.push(Param {
            name: name.to_owned(),
            value: unquote(&tok[eq + 1..]).to_owned(),
        });
    }
    Ok(params)
}

/// Writes one [`Card`] to `w` as a `BEGIN:VCARD…END:VCARD` block.
///
/// `VERSION:4.0` is always emitted. `UID` is **required**: an empty
/// [`Card::uid`] returns [`Error::MissingUid`], which is the encoder half
/// of the asymmetry described in the [module docs](self).
///
/// Emission order is fixed for byte-stable output:
///
/// 1. `UID`
/// 2. `KIND`
/// 3. [`Card::props`] in input order
///
/// Bare property names uppercase per RFC 6350 §3.3; group prefixes keep
/// their original case. Lines are CRLF-terminated and folded at 75
/// octets.
pub fn encode(mut w: impl Write, c: &Card) -> Result<()> {
    if c.uid.is_empty() {
        return Err(Error::MissingUid("a VCARD requires a UID to encode".into()));
    }

    let mut out = Vec::<u8>::new();
    write_folded(&mut out, "BEGIN:VCARD");
    write_folded(&mut out, &format!("VERSION:{SUPPORTED_VERSION}"));
    write_folded(&mut out, &format!("UID:{}", escape_text(&c.uid)));
    if let Some(kind) = c.kind {
        write_folded(&mut out, &format!("KIND:{}", kind.as_str()));
    }
    for p in &c.props {
        write_folded(&mut out, &format_property(p));
    }
    write_folded(&mut out, "END:VCARD");

    w.write_all(&out)
        .map_err(|e| Error::Malformed(format!("write: {e}")))
}

/// Serializes one [`Property`] to its wire form, excluding the terminator.
fn format_property(p: &Property) -> String {
    let mut sb = String::with_capacity(p.name.len() + p.value.len() + 16);
    let (group, name) = split_group(&p.name);
    if !group.is_empty() {
        sb.push_str(group);
        sb.push('.');
    }
    sb.push_str(&name.to_ascii_uppercase());

    for prm in &p.params {
        sb.push(';');
        sb.push_str(&prm.name.to_ascii_uppercase());
        sb.push('=');
        sb.push_str(&format_param_value(&prm.value));
    }
    sb.push(':');
    sb.push_str(&escape_text(&p.value));
    sb
}

/// Separates a `group.NAME` identifier into its group and bare name.
fn split_group(s: &str) -> (&str, &str) {
    match s.find('.') {
        Some(i) => (&s[..i], &s[i + 1..]),
        None => ("", s),
    }
}

/// DQUOTE-wraps a parameter value that contains `,`, `;` or `:`.
fn format_param_value(v: &str) -> String {
    if v.contains([',', ';', ':']) {
        format!("\"{v}\"")
    } else {
        v.to_owned()
    }
}

/// Reverses RFC 6350 §3.4 TEXT escaping.
///
/// Unlike the RFC 5545 side this applies to **every** property value:
/// RFC 6350 has no TEXT allow-list, so the codec escapes uniformly.
/// An unknown escape keeps both the backslash and the character.
fn unescape_text(s: &str) -> String {
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
            Some(other) => {
                out.push('\\');
                out.push(other);
            }
            None => out.push('\\'),
        }
    }
    out
}

/// Applies RFC 6350 §3.4 TEXT escaping for encoding.
fn escape_text(s: &str) -> String {
    if !s.contains(['\\', ',', ';', '\n']) {
        return s.to_owned();
    }
    let mut out = String::with_capacity(s.len() + 4);
    for c in s.chars() {
        match c {
            '\\' => out.push_str("\\\\"),
            ',' => out.push_str("\\,"),
            ';' => out.push_str("\\;"),
            '\n' => out.push_str("\\n"),
            other => out.push(other),
        }
    }
    out
}

/// The RFC 6350 codec: parse and encode composed into one value.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct Rfc6350Codec;

impl Rfc6350Codec {
    /// Parses every VCARD in `r`.
    pub fn parse(&self, r: impl Read) -> Result<Vec<Card>> {
        parse(r)
    }

    /// Writes one card to `w`.
    ///
    /// Encoding a list means calling this once per card and
    /// concatenating, which is what the stream encoder will do.
    pub fn encode(&self, w: impl Write, c: &Card) -> Result<()> {
        encode(w, c)
    }
}

/// A fresh codec instance.
pub fn new_codec() -> Rfc6350Codec {
    Rfc6350Codec
}

/// A fresh parser-only handle. Stateless, so identical to [`new_codec`].
pub fn new_parser() -> Rfc6350Codec {
    Rfc6350Codec
}

/// A fresh encoder-only handle. Stateless, so identical to [`new_codec`].
pub fn new_encoder() -> Rfc6350Codec {
    Rfc6350Codec
}

/// The shared default codec.
pub fn default_codec() -> Rfc6350Codec {
    Rfc6350Codec
}
