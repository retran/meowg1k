// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The `@std//` module table.
//!
//! One table, built once, and every consumer takes a module from it. That is
//! `[R-STAR-010]`, and it is the fix for the defect that cost v0.2.x the most:
//! `ctx_run.go` and `module_llm.go` each assembled a context by hand, they
//! drifted on UI nesting depth, and a module added to one was silently missing
//! from tools running inside an agent loop. There is nowhere here for a second
//! copy to live, which is also why `[R-STAR-011]` holds without a mechanism of
//! its own.
//!
//! Every module resolves in both phases. What differs is what its builtins do:
//! a call made while `.meow/` is being evaluated fails with
//! [`StarError::ModuleUnavailable`], except in `env`, which `[R-STAR-084]`
//! carves out because credentials are resolved in a declaration file.
//!
//! [`StarError::ModuleUnavailable`]: crate::error::StarError::ModuleUnavailable

use std::collections::BTreeMap;
use std::path::{Path, PathBuf};

use starlark::environment::{FrozenModule, Globals, GlobalsBuilder, Module};
use starlark::eval::Evaluator;
use starlark::starlark_module;
use starlark::values::Value as StarValue;
use starlark::values::none::{NoneOr, NoneType};

use crate::error::{Result, StarError};
use crate::run::running;

/// The modules that exist, in the order `meow doctor` should list them.
pub const NAMES: &[&str] = &[
    "csv", "env", "fs", "git", "index", "json", "path", "re", "search", "shell", "text", "time",
    "toml", "xml", "yaml",
];

/// Every `@std//` module, built once per load.
#[derive(Debug)]
pub struct Modules(BTreeMap<String, FrozenModule>);

impl Modules {
    /// Build the table.
    ///
    /// # Errors
    ///
    /// [`StarError::Starlark`] if a module will not freeze, which would be a
    /// defect in this file rather than in anything a user wrote.
    pub fn build() -> Result<Self> {
        let mut table = BTreeMap::new();
        table.insert("csv".to_owned(), freeze(csv_module)?);
        table.insert("env".to_owned(), freeze(env_module)?);
        table.insert("fs".to_owned(), freeze(crate::capability::fs_module)?);
        table.insert("git".to_owned(), freeze(crate::capability_git::git_module)?);
        table.insert("index".to_owned(), freeze(index_module)?);
        table.insert("json".to_owned(), freeze(json_module)?);
        table.insert("path".to_owned(), freeze(path_module)?);
        table.insert("re".to_owned(), freeze(re_module)?);
        table.insert("search".to_owned(), freeze(search_module)?);
        table.insert("shell".to_owned(), freeze(crate::capability::shell_module)?);
        table.insert("store".to_owned(), freeze(store_module)?);
        table.insert("text".to_owned(), freeze(text_module)?);
        table.insert("time".to_owned(), freeze(time_module)?);
        table.insert("toml".to_owned(), freeze(toml_module)?);
        table.insert("xml".to_owned(), freeze(xml_module)?);
        table.insert("yaml".to_owned(), freeze(yaml_module)?);
        Ok(Self(table))
    }

    /// One module, if it exists.
    pub fn get(&self, name: &str) -> Option<&FrozenModule> {
        self.0.get(name)
    }

    /// Every name in the table.
    pub fn names(&self) -> impl Iterator<Item = &String> {
        self.0.keys()
    }
}

/// Turn a builder into a module a `load` can return.
fn freeze(build: impl Fn(&mut GlobalsBuilder)) -> Result<FrozenModule> {
    let globals: Globals = GlobalsBuilder::new().with(build).build();
    Module::with_temp_heap(|module| {
        module.frozen_heap().add_reference(globals.heap());
        for (name, value) in globals.iter() {
            module.set(name, value.to_value());
        }
        module
            .freeze()
            .map_err(|e| StarError::Starlark(format!("{e:?}")))
    })
}

fn oops(message: impl std::fmt::Display) -> starlark::Error {
    starlark::Error::new_other(anyhow::anyhow!("{message}"))
}

/// `@std//env`: the environment, readable in both phases.
#[starlark_module]
fn env_module(builder: &mut GlobalsBuilder) {
    /// Read a variable, or a default when it is not set.
    fn get(
        #[starlark(require = pos)] name: String,
        #[starlark(require = pos)] default: Option<String>,
    ) -> starlark::Result<NoneOr<String>> {
        Ok(match std::env::var(&name).ok().or(default) {
            Some(value) => NoneOr::Other(value),
            None => NoneOr::None,
        })
    }

    /// Read a variable that has to be set.
    ///
    /// The error names the variable, because "missing credentials" without a
    /// name is a support ticket.
    fn require(#[starlark(require = pos)] name: String) -> starlark::Result<String> {
        std::env::var(&name).map_err(|_| oops(format!("`{name}` is not set in the environment")))
    }
}

/// `@std//json`: parse and encode.
#[starlark_module]
fn json_module(builder: &mut GlobalsBuilder) {
    /// Parse JSON text into Starlark values.
    fn parse<'v>(
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        running(eval, "json.parse")?;
        let value: serde_json::Value = serde_json::from_str(&text).map_err(oops)?;
        Ok(eval.heap().alloc(value))
    }

    /// Encode Starlark values as JSON text.
    fn encode<'v>(
        #[starlark(require = pos)] value: StarValue<'v>,
        #[starlark(require = named, default = false)] indent: bool,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "json.encode")?;
        let value = value.to_json_value().map_err(oops)?;
        if indent {
            serde_json::to_string_pretty(&value).map_err(oops)
        } else {
            serde_json::to_string(&value).map_err(oops)
        }
    }
}

/// `@std//yaml`: parse and encode.
///
/// `[R-STAR-016]`: the value this produces is the value `json.parse` produces
/// for the same data, so a handler can read one format and write another
/// without knowing which it read.
#[starlark_module]
fn yaml_module(builder: &mut GlobalsBuilder) {
    /// Parse YAML text into Starlark values.
    fn parse<'v>(
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        running(eval, "yaml.parse")?;
        let value: serde_json::Value = serde_yaml_ng::from_str(&text)
            .map_err(|error| oops(format!("this is not YAML: {error}")))?;
        Ok(eval.heap().alloc(value))
    }

    /// Encode Starlark values as YAML text.
    fn encode<'v>(
        #[starlark(require = pos)] value: StarValue<'v>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "yaml.encode")?;
        let value = value.to_json_value().map_err(oops)?;
        serde_yaml_ng::to_string(&as_yaml(&value)?).map_err(oops)
    }
}

/// `@std//toml`: parse and encode.
#[starlark_module]
fn toml_module(builder: &mut GlobalsBuilder) {
    /// Parse TOML text into Starlark values.
    fn parse<'v>(
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        running(eval, "toml.parse")?;
        let value: serde_json::Value =
            toml::from_str(&text).map_err(|error| oops(format!("this is not TOML: {error}")))?;
        Ok(eval.heap().alloc(value))
    }

    /// Encode Starlark values as TOML text.
    ///
    /// TOML has no top-level array and no top-level scalar, so a value that is
    /// not a table is refused here rather than serialised into something the
    /// format cannot express.
    fn encode<'v>(
        #[starlark(require = pos)] value: StarValue<'v>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "toml.encode")?;
        let value = value.to_json_value().map_err(oops)?;
        if !value.is_object() {
            return Err(oops(
                "TOML's top level is a table, so `encode` needs a dict here",
            ));
        }
        toml::to_string_pretty(&as_toml(&value)?).map_err(oops)
    }
}

/// Rebuild a JSON value as a YAML one.
///
/// `starlark` turns on `serde_json`'s `arbitrary_precision`, and features are
/// additive, so every number in this workspace is stored as text behind a
/// private marker. `serde_json`'s own serialiser understands the marker and
/// every other serialiser writes it out verbatim, which is how a `3` becomes a
/// table named `$serde_json::private::Number`. Converting explicitly is the
/// only way across.
fn as_yaml(value: &serde_json::Value) -> starlark::Result<serde_yaml_ng::Value> {
    use serde_yaml_ng::Value as Y;
    Ok(match value {
        serde_json::Value::Null => Y::Null,
        serde_json::Value::Bool(b) => Y::Bool(*b),
        serde_json::Value::Number(n) => match number(n) {
            Number::Int(i) => Y::Number(i.into()),
            Number::Float(f) => Y::Number(f.into()),
        },
        serde_json::Value::String(text) => Y::String(text.clone()),
        serde_json::Value::Array(items) => Y::Sequence(
            items
                .iter()
                .map(as_yaml)
                .collect::<std::result::Result<_, starlark::Error>>()?,
        ),
        serde_json::Value::Object(map) => {
            let mut out = serde_yaml_ng::Mapping::new();
            for (key, value) in map {
                out.insert(Y::String(key.clone()), as_yaml(value)?);
            }
            Y::Mapping(out)
        }
    })
}

/// Rebuild a JSON value as a TOML one, for the reason `as_yaml` gives.
fn as_toml(value: &serde_json::Value) -> starlark::Result<toml::Value> {
    use toml::Value as T;
    Ok(match value {
        // TOML has no null. Dropping the key would silently lose it, so say
        // so: a handler can encode `""` or omit the key itself.
        serde_json::Value::Null => {
            return Err(oops(
                "TOML has no null, so a key with no value cannot be encoded",
            ));
        }
        serde_json::Value::Bool(b) => T::Boolean(*b),
        serde_json::Value::Number(n) => match number(n) {
            Number::Int(i) => T::Integer(i),
            Number::Float(f) => T::Float(f),
        },
        serde_json::Value::String(text) => T::String(text.clone()),
        serde_json::Value::Array(items) => T::Array(
            items
                .iter()
                .map(as_toml)
                .collect::<std::result::Result<_, starlark::Error>>()?,
        ),
        serde_json::Value::Object(map) => {
            let mut out = toml::map::Map::new();
            for (key, value) in map {
                out.insert(key.clone(), as_toml(value)?);
            }
            T::Table(out)
        }
    })
}

/// A number, once it is out from behind the marker.
enum Number {
    Int(i64),
    Float(f64),
}

/// Read a number whatever representation it arrived in.
fn number(n: &serde_json::Number) -> Number {
    if let Some(i) = n.as_i64() {
        return Number::Int(i);
    }
    // An unsigned value too large for `i64` and anything fractional both land
    // here. Neither is exact as `f64`, and neither is expressible in TOML or
    // YAML any other way.
    Number::Float(n.as_f64().unwrap_or(f64::NAN))
}

/// `@std//csv`: records, with or without a header.
#[starlark_module]
fn csv_module(builder: &mut GlobalsBuilder) {
    /// Parse CSV text.
    ///
    /// With a header, a list of dicts keyed by column name; without one, a
    /// list of lists - `[R-STAR-017]`.
    fn parse<'v>(
        #[starlark(require = pos)] text: String,
        #[starlark(require = named, default = true)] header: bool,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        running(eval, "csv.parse")?;
        let mut reader = csv::ReaderBuilder::new()
            .has_headers(header)
            .flexible(true)
            .from_reader(text.as_bytes());

        let columns: Vec<String> = if header {
            reader
                .headers()
                .map_err(|error| oops(format!("this is not CSV: {error}")))?
                .iter()
                .map(str::to_owned)
                .collect()
        } else {
            Vec::new()
        };

        let heap = eval.heap();
        let mut rows: Vec<StarValue<'v>> = Vec::new();
        for (index, record) in reader.records().enumerate() {
            let record = record.map_err(|error| oops(format!("this is not CSV: {error}")))?;
            if header {
                // `[R-STAR-017]`: a short or long record is a defect in the
                // file, and saying which record it was is the difference
                // between a fixable report and "this is not CSV".
                if record.len() != columns.len() {
                    return Err(oops(format!(
                        "record {} has {} fields and the header names {}",
                        index + 1,
                        record.len(),
                        columns.len()
                    )));
                }
                let pairs: Vec<(&str, StarValue<'v>)> = columns
                    .iter()
                    .zip(record.iter())
                    .map(|(name, field)| (name.as_str(), heap.alloc(field)))
                    .collect();
                rows.push(heap.alloc(starlark::values::dict::AllocDict(pairs)));
            } else {
                let fields: Vec<StarValue<'v>> =
                    record.iter().map(|field| heap.alloc(field)).collect();
                rows.push(heap.alloc(fields));
            }
        }

        Ok(heap.alloc(rows))
    }

    /// Encode rows as CSV text.
    ///
    /// A list of dicts writes a header from the first row's keys; a list of
    /// lists writes no header.
    fn encode<'v>(
        #[starlark(require = pos)] rows: StarValue<'v>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "csv.encode")?;

        // Read the Starlark values rather than converting to JSON first. A
        // Starlark dict keeps insertion order and `serde_json::Map` sorts, so
        // the round trip would quietly rename column 1 to whichever name
        // sorts first.
        let rows = starlark::values::list::ListRef::from_value(rows)
            .ok_or_else(|| oops("`encode` takes a list of rows"))?;

        let mut writer = csv::Writer::from_writer(Vec::new());
        let mut columns: Option<Vec<String>> = None;

        for (index, row) in rows.iter().enumerate() {
            if let Some(map) = starlark::values::dict::DictRef::from_value(row) {
                let mut names = Vec::with_capacity(map.len());
                let mut fields = Vec::with_capacity(map.len());
                for (key, value) in map.iter() {
                    names.push(key.to_str());
                    fields.push(field(value));
                }
                match &columns {
                    None => {
                        writer.write_record(&names).map_err(oops)?;
                        columns = Some(names.clone());
                    }
                    // The header is written once, so a later row with
                    // different keys would silently land under the wrong
                    // columns.
                    Some(first) if *first == names => {}
                    Some(first) => {
                        return Err(oops(format!(
                            "row {} has keys {:?} and the first row had {:?}",
                            index + 1,
                            names,
                            first
                        )));
                    }
                }
                writer.write_record(&fields).map_err(oops)?;
            } else if let Some(list) = starlark::values::list::ListRef::from_value(row) {
                let fields: Vec<String> = list.iter().map(field).collect();
                writer.write_record(&fields).map_err(oops)?;
            } else {
                return Err(oops(format!(
                    "row {} is {row}, and a row is a dict or a list",
                    index + 1
                )));
            }
        }

        let bytes = writer.into_inner().map_err(oops)?;
        String::from_utf8(bytes).map_err(oops)
    }
}

/// One CSV field: a string stays itself, and anything else is written the way
/// it prints, because a CSV field has no types to lose.
fn field(value: StarValue<'_>) -> String {
    value
        .unpack_str()
        .map_or_else(|| value.to_string(), str::to_owned)
}

/// The same, for the JSON values `xml.encode` walks.
fn scalar(value: &serde_json::Value) -> String {
    match value {
        serde_json::Value::String(text) => text.clone(),
        serde_json::Value::Null => String::new(),
        other => other.to_string(),
    }
}

/// `@std//xml`: a tree, because that is what XML is.
///
/// `[R-STAR-018]`. Every mapping of XML onto the shape JSON has must decide
/// what to do with an element that carries both attributes and children, or
/// with two children sharing a tag, and every such decision is wrong for some
/// document. So this returns the four things an element has and lets a handler
/// that knows its own document build whatever it wants from them.
#[starlark_module]
fn xml_module(builder: &mut GlobalsBuilder) {
    /// Parse XML text into a tree of elements.
    ///
    /// Each element is a struct with `tag`, `attrs`, `children`, and `text`.
    fn parse<'v>(
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        running(eval, "xml.parse")?;
        let root = read_xml(&text)?;
        let heap = eval.heap();
        Ok(alloc_element(&heap, &root))
    }

    /// Encode a tree of elements as XML text.
    ///
    /// Takes what `parse` returned, and equally a tree of dicts carrying the
    /// same four keys. `parse` returns structs because `root.tag` reads better
    /// than `root["tag"]`, and a handler building a document from nothing has
    /// only dicts to build it from, so `encode` accepts both.
    fn encode<'v>(
        #[starlark(require = pos)] element: StarValue<'v>,
        #[starlark(require = named, default = false)] declaration: bool,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "xml.encode")?;
        let value = element.to_json_value().map_err(oops)?;
        let mut out = String::new();
        if declaration {
            out.push_str("<?xml version=\"1.0\" encoding=\"UTF-8\"?>");
        }
        write_xml(&value, &mut out)?;
        Ok(out)
    }
}

/// An XML element, in the four parts `[R-STAR-018]` names.
#[derive(Debug, Default)]
struct Element {
    tag: String,
    attrs: Vec<(String, String)>,
    children: Vec<Element>,
    text: String,
}

/// Read one document, keeping child order and joining an element's own text.
fn read_xml(text: &str) -> starlark::Result<Element> {
    use quick_xml::events::Event;

    let mut reader = quick_xml::Reader::from_str(text);
    reader.config_mut().trim_text(true);

    let mut stack: Vec<Element> = Vec::new();
    let mut root: Option<Element> = None;

    loop {
        match reader
            .read_event()
            .map_err(|error| oops(format!("this is not XML: {error}")))?
        {
            Event::Eof => break,
            Event::Start(start) => stack.push(open(&start)?),
            Event::Empty(start) => {
                let element = open(&start)?;
                match stack.last_mut() {
                    Some(parent) => parent.children.push(element),
                    None => root = Some(element),
                }
            }
            Event::End(_) => {
                let Some(element) = stack.pop() else {
                    return Err(oops("this is not XML: a closing tag with nothing open"));
                };
                match stack.last_mut() {
                    Some(parent) => parent.children.push(element),
                    None => root = Some(element),
                }
            }
            Event::Text(body) => {
                if let Some(element) = stack.last_mut() {
                    let piece = body
                        .decode()
                        .map_err(|error| oops(format!("this is not XML: {error}")))?;
                    element.text.push_str(&piece);
                }
            }
            _ => {}
        }
    }

    if !stack.is_empty() {
        return Err(oops("this is not XML: a tag was left open"));
    }
    root.ok_or_else(|| oops("this is not XML: the document has no element"))
}

/// The tag and attributes of a start tag.
fn open(start: &quick_xml::events::BytesStart<'_>) -> starlark::Result<Element> {
    let tag = String::from_utf8(start.name().as_ref().to_vec())
        .map_err(|error| oops(format!("this is not XML: {error}")))?;
    let mut attrs = Vec::new();
    for attr in start.attributes() {
        let attr = attr.map_err(|error| oops(format!("this is not XML: {error}")))?;
        let key = String::from_utf8(attr.key.as_ref().to_vec())
            .map_err(|error| oops(format!("this is not XML: {error}")))?;
        // Attribute-value normalisation, as XML 1.0 defines it. `Implicit1_0`
        // is what a document with no declaration is, and it is what almost
        // every document a handler will meet is.
        let value = attr
            .normalized_value(quick_xml::XmlVersion::Implicit1_0)
            .map_err(|error| oops(format!("this is not XML: {error}")))?
            .into_owned();
        attrs.push((key, value));
    }
    Ok(Element {
        tag,
        attrs,
        ..Element::default()
    })
}

/// One element as a Starlark struct, children first so the tree is complete
/// before the parent that holds it.
fn alloc_element<'v>(heap: &starlark::values::Heap<'v>, element: &Element) -> StarValue<'v> {
    let children: Vec<StarValue<'v>> = element
        .children
        .iter()
        .map(|child| alloc_element(heap, child))
        .collect();
    let attrs: Vec<(&str, StarValue<'v>)> = element
        .attrs
        .iter()
        .map(|(key, value)| (key.as_str(), heap.alloc(value.as_str())))
        .collect();
    heap.alloc(starlark::values::structs::AllocStruct([
        ("tag", heap.alloc(element.tag.as_str())),
        (
            "attrs",
            heap.alloc(starlark::values::dict::AllocDict(attrs)),
        ),
        ("children", heap.alloc(children)),
        ("text", heap.alloc(element.text.as_str())),
    ]))
}

/// Write one element and everything under it.
fn write_xml(value: &serde_json::Value, out: &mut String) -> starlark::Result<()> {
    let serde_json::Value::Object(map) = value else {
        return Err(oops(format!(
            "an element is a struct with `tag`, and this is {value}"
        )));
    };
    let Some(serde_json::Value::String(tag)) = map.get("tag") else {
        return Err(oops("an element needs a `tag`, and a tag is a string"));
    };

    out.push('<');
    out.push_str(tag);
    if let Some(serde_json::Value::Object(attrs)) = map.get("attrs") {
        for (key, value) in attrs {
            out.push(' ');
            out.push_str(key);
            out.push_str("=\"");
            escape(&scalar(value), out);
            out.push('"');
        }
    }

    let children = match map.get("children") {
        Some(serde_json::Value::Array(children)) => children.as_slice(),
        _ => &[],
    };
    let text = match map.get("text") {
        Some(serde_json::Value::String(text)) => text.as_str(),
        _ => "",
    };

    if children.is_empty() && text.is_empty() {
        out.push_str("/>");
        return Ok(());
    }

    out.push('>');
    escape(text, out);
    for child in children {
        write_xml(child, out)?;
    }
    out.push_str("</");
    out.push_str(tag);
    out.push('>');
    Ok(())
}

/// `[R-STAR-018]`: text and attribute values are escaped, so a document built
/// from a model's output cannot close a tag the handler did not write.
fn escape(text: &str, out: &mut String) {
    for ch in text.chars() {
        match ch {
            '<' => out.push_str("&lt;"),
            '>' => out.push_str("&gt;"),
            '&' => out.push_str("&amp;"),
            '"' => out.push_str("&quot;"),
            '\'' => out.push_str("&apos;"),
            other => out.push(other),
        }
    }
}

/// `@std//path`: paths as text, with no disk behind them.
#[starlark_module]
fn path_module(builder: &mut GlobalsBuilder) {
    /// Join segments.
    fn join<'v>(
        #[starlark(args)] parts: starlark::values::tuple::UnpackTuple<String>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "path.join")?;
        let mut out = PathBuf::new();
        for part in parts.items {
            out.push(part);
        }
        Ok(out.to_string_lossy().into_owned())
    }

    /// Everything before the last separator.
    fn dir<'v>(
        #[starlark(require = pos)] path: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "path.dir")?;
        Ok(Path::new(&path)
            .parent()
            .map(|p| p.to_string_lossy().into_owned())
            .unwrap_or_default())
    }

    /// Everything after the last separator.
    fn base<'v>(
        #[starlark(require = pos)] path: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "path.base")?;
        Ok(Path::new(&path)
            .file_name()
            .map(|p| p.to_string_lossy().into_owned())
            .unwrap_or_default())
    }

    /// The extension, without its dot.
    fn ext<'v>(
        #[starlark(require = pos)] path: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "path.ext")?;
        Ok(Path::new(&path)
            .extension()
            .map(|p| p.to_string_lossy().into_owned())
            .unwrap_or_default())
    }

    /// A path relative to a base.
    ///
    /// Purely lexical, like the rest of this module: it does not ask the disk
    /// what anything resolves to, so it behaves the same whether or not the
    /// file exists.
    fn rel<'v>(
        #[starlark(require = pos)] path: String,
        #[starlark(require = pos)] base: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "path.rel")?;
        Ok(Path::new(&path)
            .strip_prefix(Path::new(&base))
            .map(|p| p.to_string_lossy().into_owned())
            .unwrap_or(path))
    }

    /// A path against the workspace root.
    fn abs<'v>(
        #[starlark(require = pos)] path: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        let state = running(eval, "path.abs")?;
        let given = Path::new(&path);
        let out = if given.is_absolute() {
            given.to_path_buf()
        } else {
            state.runtime.workspace().root().join(given)
        };
        Ok(out.to_string_lossy().into_owned())
    }
}

/// `@std//search`: asking the index a question.
#[starlark_module]
fn search_module(builder: &mut GlobalsBuilder) {
    /// Rank the workspace against a question, by meaning.
    ///
    /// Returns a list of structs carrying the path, the line range, the text,
    /// and the score, so a result can be cited rather than only read.
    fn code<'v>(
        #[starlark(require = pos)] query: String,
        #[starlark(require = named, default = 10)] limit: u32,
        #[starlark(require = named)] paths: Option<StarValue<'v>>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        let state = running(eval, "search.code")?;

        let paths = match paths {
            None => Vec::new(),
            Some(value) if value.is_none() => Vec::new(),
            Some(value) => match value.to_json_value().map_err(oops)? {
                serde_json::Value::Array(items) => items
                    .into_iter()
                    .filter_map(|item| item.as_str().map(str::to_owned))
                    .collect(),
                other => return Err(oops(format!("`paths` must be a list, and is {other}"))),
            },
        };

        // `[R-STAR-081]`: the call blocks this thread. The index reads a file
        // and may reach a provider, and neither is something Starlark can
        // wait on itself.
        let found = state
            .runtime
            .search()
            .code(&query, limit as usize, &paths)
            .map_err(oops)?;

        let heap = eval.heap();
        let results: Vec<StarValue<'v>> = found
            .into_iter()
            .map(|hit| {
                heap.alloc(starlark::values::structs::AllocStruct([
                    ("path", heap.alloc(hit.path)),
                    ("first_line", heap.alloc(hit.first_line as u32)),
                    ("last_line", heap.alloc(hit.last_line as u32)),
                    ("text", heap.alloc(hit.text)),
                    ("score", heap.alloc(f64::from(hit.score))),
                ]))
            })
            .collect();

        Ok(heap.alloc(results))
    }

    /// Find text in the workspace, by what it says rather than what it means.
    ///
    /// `[R-STAR-019]`: no index is involved, so this works in a workspace
    /// where `meow index build` has never run. The walk is the index's walk,
    /// so one `.gitignore` decides what is searchable however a handler
    /// searches.
    fn text<'v>(
        #[starlark(require = pos)] pattern: String,
        #[starlark(require = named, default = false)] regex: bool,
        #[starlark(require = named, default = 50)] limit: u32,
        #[starlark(require = named, default = NoneOr::None)] paths: NoneOr<StarValue<'v>>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        let state = running(eval, "search.text")?;
        let paths = path_list(paths.into_option(), "paths")?;
        let found = state
            .runtime
            .search()
            .text(&pattern, regex, limit as usize, &paths)
            .map_err(oops)?;
        let heap = eval.heap();
        Ok(heap.alloc(hits(&heap, found)))
    }

    /// Every file the walk reaches whose path matches a glob.
    fn files<'v>(
        #[starlark(require = pos)] pattern: String,
        #[starlark(require = named, default = 500)] limit: u32,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        let state = running(eval, "search.files")?;
        let found = state
            .runtime
            .search()
            .files(&pattern, limit as usize)
            .map_err(oops)?;
        let heap = eval.heap();
        let paths: Vec<StarValue<'v>> = found.into_iter().map(|p| heap.alloc(p)).collect();
        Ok(heap.alloc(paths))
    }
}

/// `@std//index`: the index a workspace declares, from a handler.
///
/// `[R-STAR-024]`. `meow index build` and `index.build()` do the same work
/// through the same port, so a handler that indexes before it searches does
/// not have to shell out to the binary that is running it.
#[starlark_module]
fn index_module(builder: &mut GlobalsBuilder) {
    /// Walk, record what changed, and embed what has no embedding yet.
    fn build<'v>(eval: &mut Evaluator<'v, '_, '_>) -> starlark::Result<StarValue<'v>> {
        let state = running(eval, "index.build")?;
        let done = state.runtime.search().build().map_err(oops)?;
        let heap = eval.heap();
        Ok(alloc_indexed(&heap, &done))
    }

    /// Walk and record what changed, without reaching a provider.
    ///
    /// The cheap half of `build`, for a handler that wants to know whether
    /// anything moved before it spends on embeddings.
    fn update<'v>(eval: &mut Evaluator<'v, '_, '_>) -> starlark::Result<StarValue<'v>> {
        let state = running(eval, "index.update")?;
        let done = state.runtime.search().update().map_err(oops)?;
        let heap = eval.heap();
        Ok(alloc_indexed(&heap, &done))
    }

    /// How much is indexed, and by which model.
    fn stats<'v>(eval: &mut Evaluator<'v, '_, '_>) -> starlark::Result<StarValue<'v>> {
        let state = running(eval, "index.stats")?;
        let stats = state.runtime.search().stats().map_err(oops)?;
        let heap = eval.heap();
        Ok(heap.alloc(starlark::values::structs::AllocStruct([
            ("chunks", heap.alloc(stats.chunks as u32)),
            ("embedded", heap.alloc(stats.embedded as u32)),
            (
                "model",
                match stats.model {
                    Some(name) => heap.alloc(name),
                    None => StarValue::new_none(),
                },
            ),
        ])))
    }

    /// Rank the workspace against a question, with the knobs.
    fn query<'v>(
        #[starlark(require = pos)] question: String,
        #[starlark(require = named, default = 10)] limit: u32,
        // `UnpackFloat` rather than a float type, so `min_score = 0` and
        // `min_score = 0.4` both work. A handler that wrote the integer and
        // got a type error would be right to be annoyed.
        #[starlark(require = named, default = NoneOr::None)] min_score: NoneOr<
            starlark::values::float::UnpackFloat,
        >,
        #[starlark(require = named, default = NoneOr::None)] paths: NoneOr<StarValue<'v>>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        let state = running(eval, "index.query")?;
        let paths = path_list(paths.into_option(), "paths")?;
        let found = state
            .runtime
            .search()
            .query(
                &question,
                limit as usize,
                &paths,
                min_score.into_option().map_or(0.0, |f| f.0 as f32),
            )
            .map_err(oops)?;
        let heap = eval.heap();
        Ok(heap.alloc(hits(&heap, found)))
    }
}

/// A list of paths from an optional argument.
fn path_list(value: Option<StarValue<'_>>, name: &str) -> starlark::Result<Vec<String>> {
    let Some(value) = value else {
        return Ok(Vec::new());
    };
    if value.is_none() {
        return Ok(Vec::new());
    }
    match value.to_json_value().map_err(oops)? {
        serde_json::Value::Array(items) => Ok(items
            .into_iter()
            .filter_map(|item| item.as_str().map(str::to_owned))
            .collect()),
        other => Err(oops(format!("`{name}` must be a list, and is {other}"))),
    }
}

/// Hits as structs, the one shape every search in this table returns.
fn hits<'v>(
    heap: &starlark::values::Heap<'v>,
    found: Vec<crate::port::Found>,
) -> Vec<StarValue<'v>> {
    found
        .into_iter()
        .map(|hit| {
            heap.alloc(starlark::values::structs::AllocStruct([
                ("path", heap.alloc(hit.path)),
                ("first_line", heap.alloc(hit.first_line as u32)),
                ("last_line", heap.alloc(hit.last_line as u32)),
                ("text", heap.alloc(hit.text)),
                ("score", heap.alloc(f64::from(hit.score))),
            ]))
        })
        .collect()
}

/// What a walk changed, as counts - `[R-STAR-024]`.
fn alloc_indexed<'v>(
    heap: &starlark::values::Heap<'v>,
    done: &crate::port::Indexed,
) -> StarValue<'v> {
    heap.alloc(starlark::values::structs::AllocStruct([
        ("files", heap.alloc(done.files as u32)),
        ("added", heap.alloc(done.added as u32)),
        ("changed", heap.alloc(done.changed as u32)),
        ("removed", heap.alloc(done.removed as u32)),
        ("embedded", heap.alloc(done.embedded as u32)),
    ]))
}

/// `@std//re`: regular expressions.
///
/// Every call compiles its pattern, which costs microseconds and buys the
/// property that a bad pattern is reported at the call that wrote it rather
/// than at a load that mentioned it.
#[starlark_module]
fn re_module(builder: &mut GlobalsBuilder) {
    /// The first match, as a list of groups, or `None`.
    ///
    /// Group 0 is the whole match and the rest follow in the order the pattern
    /// opens them. A group that took part in no match is `None`, which is the
    /// only way to tell "matched empty" from "did not match" - `[R-STAR-013]`.
    fn r#match<'v>(
        #[starlark(require = pos)] pattern: String,
        #[starlark(require = pos)] subject: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        running(eval, "re.match")?;
        let re = compile(&pattern)?;
        let heap = eval.heap();
        match re.captures(&subject) {
            None => Ok(StarValue::new_none()),
            Some(caps) => Ok(heap.alloc(groups(&heap, &caps))),
        }
    }

    /// Every match, each as its own list of groups.
    fn find_all<'v>(
        #[starlark(require = pos)] pattern: String,
        #[starlark(require = pos)] subject: String,
        #[starlark(require = named, default = 0)] limit: u32,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        running(eval, "re.find_all")?;
        let re = compile(&pattern)?;
        let heap = eval.heap();
        // A pattern that can match empty makes `captures_iter` unbounded in
        // the number of matches it reports for a long subject, so `limit`
        // exists to put a ceiling on it. Zero means no ceiling, which is what
        // a caller who has not thought about it wants.
        let cap = if limit == 0 {
            usize::MAX
        } else {
            limit as usize
        };
        let found: Vec<StarValue<'v>> = re
            .captures_iter(&subject)
            .take(cap)
            .map(|caps| heap.alloc(groups(&heap, &caps)))
            .collect();
        Ok(heap.alloc(found))
    }

    /// Replace every match.
    ///
    /// `$1` and `${name}` in the replacement refer to groups, per the `regex`
    /// crate's syntax. A `$` that means itself is written `$$`.
    fn replace<'v>(
        #[starlark(require = pos)] pattern: String,
        #[starlark(require = pos)] subject: String,
        #[starlark(require = pos)] replacement: String,
        #[starlark(require = named, default = false)] first_only: bool,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "re.replace")?;
        let re = compile(&pattern)?;
        Ok(if first_only {
            re.replace(&subject, replacement.as_str()).into_owned()
        } else {
            re.replace_all(&subject, replacement.as_str()).into_owned()
        })
    }

    /// Split on every match.
    fn split<'v>(
        #[starlark(require = pos)] pattern: String,
        #[starlark(require = pos)] subject: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        running(eval, "re.split")?;
        let re = compile(&pattern)?;
        let heap = eval.heap();
        let parts: Vec<StarValue<'v>> = re.split(&subject).map(|part| heap.alloc(part)).collect();
        Ok(heap.alloc(parts))
    }
}

/// Compile a pattern, saying what was wrong with it rather than that something
/// was.
fn compile(pattern: &str) -> starlark::Result<regex::Regex> {
    regex::Regex::new(pattern)
        .map_err(|error| oops(format!("`{pattern}` is not a regular expression: {error}")))
}

/// One match's groups, with a group that did not participate as `None`.
fn groups<'v>(heap: &starlark::values::Heap<'v>, caps: &regex::Captures<'_>) -> Vec<StarValue<'v>> {
    caps.iter()
        .map(|group| match group {
            Some(m) => heap.alloc(m.as_str()),
            None => StarValue::new_none(),
        })
        .collect()
}

/// `@std//time`: one scale, and it is seconds in UTC.
///
/// `[R-STAR-014]`. A handler that measures a duration gets the same number on
/// every machine, and a zone enters only at `format`, where a human is about
/// to read the result.
#[starlark_module]
fn time_module(builder: &mut GlobalsBuilder) {
    /// Now, in seconds since the Unix epoch.
    fn now<'v>(eval: &mut Evaluator<'v, '_, '_>) -> starlark::Result<i64> {
        running(eval, "time.now")?;
        Ok(jiff::Timestamp::now().as_second())
    }

    /// Read an RFC 3339 timestamp, in seconds since the epoch.
    fn parse<'v>(
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<i64> {
        running(eval, "time.parse")?;
        let stamp: jiff::Timestamp = text
            .parse()
            .map_err(|error| oops(format!("`{text}` is not an RFC 3339 timestamp: {error}")))?;
        Ok(stamp.as_second())
    }

    /// Write seconds since the epoch as text.
    ///
    /// The default is RFC 3339 in UTC, which is what another program should
    /// be given. `layout` takes `strftime` fields for the case where a person
    /// is going to read it.
    fn format<'v>(
        #[starlark(require = pos)] seconds: i64,
        #[starlark(require = named, default = NoneOr::None)] layout: NoneOr<String>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "time.format")?;
        let stamp = jiff::Timestamp::from_second(seconds)
            .map_err(|error| oops(format!("{seconds} is not a timestamp: {error}")))?;
        match layout.into_option() {
            None => Ok(stamp.to_string()),
            Some(layout) => jiff::fmt::strtime::format(&layout, stamp)
                .map_err(|error| oops(format!("`{layout}` is not a layout: {error}"))),
        }
    }

    /// How many seconds have passed since an instant.
    ///
    /// Negative if the instant is in the future, which is a fact about the
    /// argument rather than an error.
    fn since<'v>(
        #[starlark(require = pos)] seconds: i64,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<i64> {
        running(eval, "time.since")?;
        Ok(jiff::Timestamp::now().as_second().saturating_sub(seconds))
    }
}

/// `@std//store`: what a handler keeps between runs.
///
/// `[R-STAR-026]`. The table belongs to the workspace rather than to a
/// session, so `meow session gc` leaves it alone. That is the difference from
/// `ctx.session`, which is the right place for what one run decided and the
/// wrong place for what every run should remember.
#[starlark_module]
fn store_module(builder: &mut GlobalsBuilder) {
    /// Read a key, or the default when it has never been written.
    fn get<'v>(
        #[starlark(require = pos)] key: String,
        #[starlark(require = named, default = NoneOr::None)] default: NoneOr<StarValue<'v>>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        let state = running(eval, "store.get")?;
        match state.runtime.keep().get(&key).map_err(oops)? {
            Some(value) => Ok(eval.heap().alloc(value)),
            // `[R-STAR-027]`: an absent key gives what the caller asked for,
            // and `None` when it asked for nothing.
            None => Ok(default.into_option().unwrap_or_else(StarValue::new_none)),
        }
    }

    /// Write a key, replacing whatever was there.
    fn put<'v>(
        #[starlark(require = pos)] key: String,
        #[starlark(require = pos)] value: StarValue<'v>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<NoneType> {
        let state = running(eval, "store.put")?;
        let value = value.to_json_value().map_err(oops)?;
        state.runtime.keep().put(&key, &value).map_err(oops)?;
        Ok(NoneType)
    }

    /// Remove a key, and say whether it was there.
    ///
    /// `[R-STAR-028]`: removing a key that was never written is not an error,
    /// because a handler cleaning up should not have to check first.
    fn delete<'v>(
        #[starlark(require = pos)] key: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<bool> {
        let state = running(eval, "store.delete")?;
        state.runtime.keep().delete(&key).map_err(oops)
    }

    /// Every key with this prefix, sorted.
    fn keys<'v>(
        #[starlark(require = named, default = String::new())] prefix: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        let state = running(eval, "store.keys")?;
        let found = state.runtime.keep().keys(&prefix).map_err(oops)?;
        let heap = eval.heap();
        let keys: Vec<StarValue<'v>> = found.into_iter().map(|key| heap.alloc(key)).collect();
        Ok(heap.alloc(keys))
    }
}

/// `@std//text`: shaping text for a terminal or a prompt.
#[starlark_module]
fn text_module(builder: &mut GlobalsBuilder) {
    /// Wrap to a width, breaking between words.
    fn wrap<'v>(
        #[starlark(require = pos)] text: String,
        #[starlark(require = pos)] width: u32,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "text.wrap")?;
        if width == 0 {
            return Err(oops("`width` must be at least 1"));
        }
        Ok(wrap_to(&text, width as usize))
    }

    /// Remove the indentation every line shares.
    fn dedent<'v>(
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "text.dedent")?;
        Ok(dedent_all(&text))
    }

    /// Cut to a length, marking that something was cut.
    fn truncate<'v>(
        #[starlark(require = pos)] text: String,
        #[starlark(require = pos)] max: u32,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<String> {
        running(eval, "text.truncate")?;
        let max = max as usize;
        if text.chars().count() <= max {
            return Ok(text);
        }
        // The marker counts towards the limit, so the result is never longer
        // than what was asked for. A truncation that overruns its own bound is
        // how a prompt ends up one token over a context window.
        let keep = max.saturating_sub(3);
        let head: String = text.chars().take(keep).collect();
        Ok(format!("{head}..."))
    }

    /// About how many tokens a model will see.
    ///
    /// An estimate, and the same one the engine's compaction uses, so a
    /// handler that budgets a prompt against this number and an engine that
    /// decides to compact cannot disagree about how big the prompt is.
    fn tokens<'v>(
        #[starlark(require = pos)] text: String,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<u32> {
        running(eval, "text.tokens")?;
        Ok(u32::try_from(text.chars().count().div_ceil(4)).unwrap_or(u32::MAX))
    }
}

fn wrap_to(text: &str, width: usize) -> String {
    let mut out = String::with_capacity(text.len());
    for (i, line) in text.lines().enumerate() {
        if i > 0 {
            out.push('\n');
        }
        let mut column = 0;
        for (j, word) in line.split_whitespace().enumerate() {
            let len = word.chars().count();
            if j > 0 && column + 1 + len > width {
                out.push('\n');
                column = 0;
            } else if j > 0 {
                out.push(' ');
                column += 1;
            }
            out.push_str(word);
            column += len;
        }
    }
    out
}

fn dedent_all(text: &str) -> String {
    let indent = text
        .lines()
        .filter(|line| !line.trim().is_empty())
        .map(|line| line.len() - line.trim_start().len())
        .min()
        .unwrap_or(0);

    text.lines()
        .map(|line| {
            if line.len() >= indent {
                &line[indent..]
            } else {
                line.trim_start()
            }
        })
        .collect::<Vec<_>>()
        .join("\n")
}
