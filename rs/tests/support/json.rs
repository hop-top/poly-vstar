// SPDX-License-Identifier: MIT

//! A minimal JSON reader for the behavior fixtures.
//!
//! The crate ships no `serde` dependency and does not want one: `serde`
//! and `serde_json` would appear in the published crate's dependency
//! graph for the sake of a test loader, and the behavior fixtures use a
//! tiny subset of JSON — arrays of flat objects whose values are
//! strings, integers, booleans or `null`.
//!
//! `null` is load-bearing and is therefore modelled explicitly rather
//! than folded into "absent": `spec/behavior/time/tzid.json` spells a
//! not-ok resolution as `"utc": null`, which is a different assertion
//! from the key being missing.

#![allow(dead_code)]

use std::collections::BTreeMap;
use std::fmt;

/// One JSON value, restricted to the shapes the behavior corpus uses.
#[derive(Debug, Clone, PartialEq)]
pub enum Json {
    /// A JSON `null`.
    Null,
    /// `true` or `false`.
    Bool(bool),
    /// Any JSON number, kept as `f64` so an integer and a fraction
    /// share one variant.
    Number(f64),
    /// A string with its escapes resolved.
    String(String),
    /// An array, in document order.
    Array(Vec<Json>),
    /// An object. `BTreeMap` rather than a `Vec` of pairs: the corpus
    /// never carries a duplicate key, and lookup is what every caller
    /// wants.
    Object(BTreeMap<String, Json>),
}

impl Json {
    /// The array's elements, or a panic naming the actual shape.
    pub fn array(&self) -> &[Json] {
        match self {
            Json::Array(v) => v,
            other => panic!("expected a JSON array, got {other:?}"),
        }
    }

    /// The value at `key`, or `None` when the key is absent.
    ///
    /// A key present with a `null` value returns `Some(Json::Null)` —
    /// the distinction the `tzid.json` gate rests on.
    pub fn get(&self, key: &str) -> Option<&Json> {
        match self {
            Json::Object(m) => m.get(key),
            other => panic!("expected a JSON object, got {other:?}"),
        }
    }

    /// The string at `key`, or `None` when the key is absent or `null`.
    pub fn str_field(&self, key: &str) -> Option<&str> {
        match self.get(key) {
            None | Some(Json::Null) => None,
            Some(Json::String(s)) => Some(s),
            Some(other) => panic!("field {key:?} is not a string: {other:?}"),
        }
    }

    /// The number at `key` as an `i64`, or `None` when absent or `null`.
    pub fn i64_field(&self, key: &str) -> Option<i64> {
        match self.get(key) {
            None | Some(Json::Null) => None,
            #[allow(clippy::cast_possible_truncation)]
            Some(Json::Number(n)) => Some(*n as i64),
            Some(other) => panic!("field {key:?} is not a number: {other:?}"),
        }
    }

    /// The boolean at `key`, or `None` when absent or `null`.
    pub fn bool_field(&self, key: &str) -> Option<bool> {
        match self.get(key) {
            None | Some(Json::Null) => None,
            Some(Json::Bool(b)) => Some(*b),
            Some(other) => panic!("field {key:?} is not a boolean: {other:?}"),
        }
    }
}

/// A parse failure, carrying the byte offset it was detected at.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct JsonError {
    /// Byte offset into the input.
    pub at: usize,
    /// What went wrong.
    pub message: String,
}

impl fmt::Display for JsonError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "JSON error at byte {}: {}", self.at, self.message)
    }
}

/// Decodes a complete JSON document, rejecting trailing content.
pub fn parse(src: &str) -> Result<Json, JsonError> {
    let mut p = Parser {
        b: src.as_bytes(),
        i: 0,
    };
    p.skip_ws();
    let v = p.value()?;
    p.skip_ws();
    if p.i != p.b.len() {
        return Err(p.err("trailing content after the document"));
    }
    Ok(v)
}

struct Parser<'a> {
    b: &'a [u8],
    i: usize,
}

impl Parser<'_> {
    fn err(&self, message: &str) -> JsonError {
        JsonError {
            at: self.i,
            message: message.to_owned(),
        }
    }

    fn skip_ws(&mut self) {
        while self
            .b
            .get(self.i)
            .is_some_and(|c| matches!(c, b' ' | b'\t' | b'\n' | b'\r'))
        {
            self.i += 1;
        }
    }

    fn eat(&mut self, lit: &str) -> bool {
        if self.b[self.i..].starts_with(lit.as_bytes()) {
            self.i += lit.len();
            return true;
        }
        false
    }

    fn value(&mut self) -> Result<Json, JsonError> {
        match self.b.get(self.i) {
            None => Err(self.err("unexpected end of input")),
            Some(b'n') if self.eat("null") => Ok(Json::Null),
            Some(b't') if self.eat("true") => Ok(Json::Bool(true)),
            Some(b'f') if self.eat("false") => Ok(Json::Bool(false)),
            Some(b'"') => self.string().map(Json::String),
            Some(b'[') => self.array(),
            Some(b'{') => self.object(),
            Some(c) if *c == b'-' || c.is_ascii_digit() => self.number(),
            Some(_) => Err(self.err("unexpected character")),
        }
    }

    fn array(&mut self) -> Result<Json, JsonError> {
        self.i += 1; // '['
        let mut out = Vec::new();
        self.skip_ws();
        if self.b.get(self.i) == Some(&b']') {
            self.i += 1;
            return Ok(Json::Array(out));
        }
        loop {
            self.skip_ws();
            out.push(self.value()?);
            self.skip_ws();
            match self.b.get(self.i) {
                Some(b',') => self.i += 1,
                Some(b']') => {
                    self.i += 1;
                    return Ok(Json::Array(out));
                }
                _ => return Err(self.err("expected ',' or ']' in an array")),
            }
        }
    }

    fn object(&mut self) -> Result<Json, JsonError> {
        self.i += 1; // '{'
        let mut out = BTreeMap::new();
        self.skip_ws();
        if self.b.get(self.i) == Some(&b'}') {
            self.i += 1;
            return Ok(Json::Object(out));
        }
        loop {
            self.skip_ws();
            let key = self.string()?;
            self.skip_ws();
            if self.b.get(self.i) != Some(&b':') {
                return Err(self.err("expected ':' after an object key"));
            }
            self.i += 1;
            self.skip_ws();
            let value = self.value()?;
            out.insert(key, value);
            self.skip_ws();
            match self.b.get(self.i) {
                Some(b',') => self.i += 1,
                Some(b'}') => {
                    self.i += 1;
                    return Ok(Json::Object(out));
                }
                _ => return Err(self.err("expected ',' or '}' in an object")),
            }
        }
    }

    fn number(&mut self) -> Result<Json, JsonError> {
        let start = self.i;
        if self.b.get(self.i) == Some(&b'-') {
            self.i += 1;
        }
        while self
            .b
            .get(self.i)
            .is_some_and(|c| c.is_ascii_digit() || matches!(c, b'.' | b'e' | b'E' | b'+' | b'-'))
        {
            self.i += 1;
        }
        let text = std::str::from_utf8(&self.b[start..self.i])
            .map_err(|_| self.err("number is not valid UTF-8"))?;
        text.parse::<f64>()
            .map(Json::Number)
            .map_err(|_| self.err("malformed number"))
    }

    fn string(&mut self) -> Result<String, JsonError> {
        if self.b.get(self.i) != Some(&b'"') {
            return Err(self.err("expected a string"));
        }
        self.i += 1;
        let mut out = String::new();
        loop {
            let c = *self
                .b
                .get(self.i)
                .ok_or_else(|| self.err("unterminated string"))?;
            self.i += 1;
            match c {
                b'"' => return Ok(out),
                b'\\' => {
                    let esc = *self
                        .b
                        .get(self.i)
                        .ok_or_else(|| self.err("unterminated escape"))?;
                    self.i += 1;
                    match esc {
                        b'"' => out.push('"'),
                        b'\\' => out.push('\\'),
                        b'/' => out.push('/'),
                        b'b' => out.push('\u{8}'),
                        b'f' => out.push('\u{c}'),
                        b'n' => out.push('\n'),
                        b'r' => out.push('\r'),
                        b't' => out.push('\t'),
                        b'u' => out.push(self.unicode_escape()?),
                        _ => return Err(self.err("unknown escape")),
                    }
                }
                _ => {
                    // Copy the whole UTF-8 sequence: the fixtures carry
                    // non-ASCII text, and pushing one byte at a time
                    // would tear it.
                    let len = utf8_len(c);
                    let end = self.i - 1 + len;
                    let s = std::str::from_utf8(&self.b[self.i - 1..end])
                        .map_err(|_| self.err("invalid UTF-8 in a string"))?;
                    out.push_str(s);
                    self.i = end;
                }
            }
        }
    }

    /// Decodes a `\uXXXX` escape, pairing surrogates when one follows.
    fn unicode_escape(&mut self) -> Result<char, JsonError> {
        let hi = self.hex4()?;
        if !(0xD800..0xDC00).contains(&hi) {
            return char::from_u32(hi).ok_or_else(|| self.err("escape is not a scalar value"));
        }
        if !self.eat("\\u") {
            return Err(self.err("high surrogate without a low surrogate"));
        }
        let lo = self.hex4()?;
        if !(0xDC00..0xE000).contains(&lo) {
            return Err(self.err("expected a low surrogate"));
        }
        let cp = 0x1_0000 + ((hi - 0xD800) << 10) + (lo - 0xDC00);
        char::from_u32(cp).ok_or_else(|| self.err("surrogate pair is not a scalar value"))
    }

    fn hex4(&mut self) -> Result<u32, JsonError> {
        let end = self.i + 4;
        if end > self.b.len() {
            return Err(self.err("truncated \\u escape"));
        }
        let text =
            std::str::from_utf8(&self.b[self.i..end]).map_err(|_| self.err("bad \\u escape"))?;
        let v = u32::from_str_radix(text, 16).map_err(|_| self.err("bad \\u escape"))?;
        self.i = end;
        Ok(v)
    }
}

/// The byte length of the UTF-8 sequence starting with `lead`.
fn utf8_len(lead: u8) -> usize {
    match lead {
        0x00..=0x7F => 1,
        0xC0..=0xDF => 2,
        0xE0..=0xEF => 3,
        _ => 4,
    }
}

#[cfg(test)]
mod tests {
    use super::{parse, Json};

    #[test]
    fn reads_the_shapes_the_behavior_corpus_uses() {
        let v = parse(r#"[{"a":"x","n":-3,"b":true,"z":null}]"#).expect("parses");
        let row = &v.array()[0];
        assert_eq!(row.str_field("a"), Some("x"));
        assert_eq!(row.i64_field("n"), Some(-3));
        assert_eq!(row.bool_field("b"), Some(true));
        assert_eq!(row.get("z"), Some(&Json::Null));
        assert_eq!(row.str_field("z"), None);
        assert_eq!(row.get("missing"), None);
    }

    #[test]
    fn decodes_escapes_and_non_ascii() {
        let v = parse(r#"["café — ok\n", "😀"]"#).expect("parses");
        assert_eq!(v.array()[0], Json::String("café — ok\n".to_owned()));
        assert_eq!(v.array()[1], Json::String("😀".to_owned()));
    }

    #[test]
    fn rejects_trailing_content() {
        assert!(parse("[] []").is_err());
    }
}
