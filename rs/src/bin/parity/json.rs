// SPDX-License-Identifier: MIT

//! A minimal JSON reader and writer for the parity emitter.
//!
//! The crate ships no `serde` dependency and the emitter does not add
//! one. `serde_json` cannot be a plain `[dependencies]` entry — the
//! library itself has no JSON needs, and every downstream consumer
//! would resolve it. It cannot usefully be feature-gated either:
//! `tools/parity/parity.py` runs this binary as `cargo run --quiet
//! --bin parity`, with no `--features`, so a gated dependency leaves
//! the binary uncompilable under the harness' own invocation.
//!
//! What is left is the subset of JSON the contract actually uses. The
//! reader handles the sidecar shapes in `spec/behavior/` — arrays of
//! flat objects whose values are strings, integers, booleans or
//! `null`. The writer emits the document in
//! `tools/parity/README.md`'s form: two-space indentation, object keys
//! sorted, matching what Go's `encoding/json` produces with
//! `SetIndent("", "  ")`.
//!
//! `null` is load-bearing on both sides and is modelled explicitly
//! rather than folded into "absent": `"utc": null` is a different
//! assertion from the key being missing.

use std::collections::BTreeMap;
use std::fmt::Write as _;

/// One JSON value.
///
/// `Object` is a `BTreeMap`, which is what makes the writer's key
/// ordering automatic: Go's `encoding/json` sorts map keys, so a
/// container that sorts on insert removes any chance of an emitted
/// document differing from the reference by key order alone.
#[derive(Debug, Clone, PartialEq)]
pub enum Json {
    /// A JSON `null`.
    Null,
    /// `true` or `false`.
    Bool(bool),
    /// An integer. The document carries no fractional numbers: the one
    /// numeric column is `duration/parse`'s `seconds`, a signed second
    /// count, and Go emits it from an `int64`.
    Int(i64),
    /// A string with its escapes resolved.
    Str(String),
    /// An array, in document order.
    Array(Vec<Json>),
    /// An object, keyed in sorted order.
    Object(BTreeMap<String, Json>),
}

impl Json {
    /// A `Str` from anything string-like, so call sites read as data
    /// rather than as conversions.
    pub fn str(s: impl Into<String>) -> Json {
        Json::Str(s.into())
    }

    /// An `Object` built from pairs, for the fixed-shape entries every
    /// family emits.
    pub fn obj<const N: usize>(pairs: [(&str, Json); N]) -> Json {
        Json::Object(
            pairs
                .into_iter()
                .map(|(k, v)| (k.to_owned(), v))
                .collect::<BTreeMap<_, _>>(),
        )
    }

    /// The array's elements, or `None` when this is another shape.
    pub fn as_array(&self) -> Option<&[Json]> {
        match self {
            Json::Array(v) => Some(v),
            _ => None,
        }
    }

    /// The value at `key`, or `None` when absent or when this is not an
    /// object.
    ///
    /// A key present with a `null` value returns `Some(Json::Null)`.
    pub fn get(&self, key: &str) -> Option<&Json> {
        match self {
            Json::Object(m) => m.get(key),
            _ => None,
        }
    }

    /// The string at `key`, or `None` when absent, `null`, or another
    /// shape.
    pub fn str_field(&self, key: &str) -> Option<&str> {
        match self.get(key) {
            Some(Json::Str(s)) => Some(s),
            _ => None,
        }
    }

    /// The integer at `key`, or `None` when absent, `null`, or another
    /// shape.
    pub fn int_field(&self, key: &str) -> Option<i64> {
        match self.get(key) {
            Some(Json::Int(n)) => Some(*n),
            _ => None,
        }
    }
}

/// Renders `value` the way Go's `encoding/json` does with
/// `SetIndent("", "  ")`, including the trailing newline `Encode`
/// appends.
pub fn render(value: &Json) -> String {
    let mut out = String::new();
    write_value(&mut out, value, 0);
    out.push('\n');
    out
}

fn write_value(out: &mut String, value: &Json, depth: usize) {
    match value {
        Json::Null => out.push_str("null"),
        Json::Bool(true) => out.push_str("true"),
        Json::Bool(false) => out.push_str("false"),
        Json::Int(n) => {
            let _ = write!(out, "{n}");
        }
        Json::Str(s) => write_string(out, s),
        Json::Array(items) => write_seq(out, depth, '[', ']', items.len(), |out, i, depth| {
            write_value(out, &items[i], depth);
        }),
        Json::Object(map) => {
            let entries: Vec<(&String, &Json)> = map.iter().collect();
            write_seq(out, depth, '{', '}', entries.len(), |out, i, depth| {
                let (key, val) = entries[i];
                write_string(out, key);
                out.push_str(": ");
                write_value(out, val, depth);
            });
        }
    }
}

/// Write a bracketed, indented sequence. An empty one renders as `[]`
/// or `{}` on one line, which is what `encoding/json` does.
fn write_seq(
    out: &mut String,
    depth: usize,
    open: char,
    close: char,
    len: usize,
    mut item: impl FnMut(&mut String, usize, usize),
) {
    out.push(open);
    if len == 0 {
        out.push(close);
        return;
    }
    for i in 0..len {
        if i > 0 {
            out.push(',');
        }
        out.push('\n');
        indent(out, depth + 1);
        item(out, i, depth + 1);
    }
    out.push('\n');
    indent(out, depth);
    out.push(close);
}

fn indent(out: &mut String, depth: usize) {
    for _ in 0..depth {
        out.push_str("  ");
    }
}

/// Escape a string the way Go's `encoding/json` does.
///
/// Go's encoder escapes `<`, `>` and `&` as `<`, `>` and
/// `&` unless `SetEscapeHTML(false)` is called, which the
/// reference emitter does not call. Reproducing that is what keeps the
/// two documents byte-identical should a fixture ever grow one of
/// those characters — today none do, so this is a latent contract
/// rather than an observed one.
fn write_string(out: &mut String, s: &str) {
    out.push('"');
    for c in s.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            '<' => out.push_str("\\u003c"),
            '>' => out.push_str("\\u003e"),
            '&' => out.push_str("\\u0026"),
            c if (c as u32) < 0x20 => {
                let _ = write!(out, "\\u{:04x}", c as u32);
            }
            c => out.push(c),
        }
    }
    out.push('"');
}

/// A parse failure, carrying the byte offset it was detected at.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct JsonError {
    /// Byte offset into the input.
    pub at: usize,
    /// What went wrong.
    pub message: String,
}

impl std::fmt::Display for JsonError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "JSON error at byte {}: {}", self.at, self.message)
    }
}

/// Decode a complete JSON document, rejecting trailing content.
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
            Some(b'"') => self.string().map(Json::Str),
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
        while self.b.get(self.i).is_some_and(u8::is_ascii_digit) {
            self.i += 1;
        }
        // The sidecars carry only integers. A fractional or exponent
        // form would be a corpus change this emitter should refuse
        // loudly rather than silently truncate.
        if self
            .b
            .get(self.i)
            .is_some_and(|c| matches!(c, b'.' | b'e' | b'E'))
        {
            return Err(self.err("only integer numbers are supported"));
        }
        let text = std::str::from_utf8(&self.b[start..self.i])
            .map_err(|_| self.err("number is not valid UTF-8"))?;
        text.parse::<i64>()
            .map(Json::Int)
            .map_err(|_| self.err("number does not fit in an i64"))
    }

    fn string(&mut self) -> Result<String, JsonError> {
        if self.b.get(self.i) != Some(&b'"') {
            return Err(self.err("expected a string"));
        }
        self.i += 1;
        let mut out = String::new();
        loop {
            match self.b.get(self.i) {
                None => return Err(self.err("unterminated string")),
                Some(b'"') => {
                    self.i += 1;
                    return Ok(out);
                }
                Some(b'\\') => {
                    self.i += 1;
                    let esc = *self
                        .b
                        .get(self.i)
                        .ok_or_else(|| self.err("dangling escape"))?;
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
                Some(_) => {
                    let rest = std::str::from_utf8(&self.b[self.i..])
                        .map_err(|_| self.err("string is not valid UTF-8"))?;
                    let c = rest
                        .chars()
                        .next()
                        .ok_or_else(|| self.err("empty string tail"))?;
                    out.push(c);
                    self.i += c.len_utf8();
                }
            }
        }
    }

    /// Decode a `\uXXXX` escape, joining a surrogate pair when one
    /// follows. The corpus uses no escapes today; handling them keeps
    /// the reader honest about the format it claims to read.
    fn unicode_escape(&mut self) -> Result<char, JsonError> {
        let hi = self.hex4()?;
        if (0xD800..0xDC00).contains(&hi) {
            if !self.eat("\\u") {
                return Err(self.err("high surrogate without a low surrogate"));
            }
            let lo = self.hex4()?;
            if !(0xDC00..0xE000).contains(&lo) {
                return Err(self.err("expected a low surrogate"));
            }
            let cp = 0x1_0000 + ((hi - 0xD800) << 10) + (lo - 0xDC00);
            return char::from_u32(cp).ok_or_else(|| self.err("invalid surrogate pair"));
        }
        char::from_u32(hi).ok_or_else(|| self.err("invalid code point"))
    }

    fn hex4(&mut self) -> Result<u32, JsonError> {
        let end = self.i + 4;
        if end > self.b.len() {
            return Err(self.err("truncated \\u escape"));
        }
        let text =
            std::str::from_utf8(&self.b[self.i..end]).map_err(|_| self.err("bad \\u escape"))?;
        let value = u32::from_str_radix(text, 16).map_err(|_| self.err("bad \\u escape"))?;
        self.i = end;
        Ok(value)
    }
}
