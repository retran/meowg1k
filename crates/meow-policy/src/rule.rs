// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What a rule matches.

use std::path::Path;

use globset::{Glob, GlobSet, GlobSetBuilder};

use crate::error::{PolicyError, Result};

/// Which tools a rule is about.
///
/// `[R-POLICY-002]`: an exact name, or a trailing wildcard. A leading wildcard
/// is refused, because `*.write` reads as "every write" and would silently not
/// be, once two tool families spell the operation differently.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum NamePattern {
    /// One tool.
    Exact(String),
    /// Every tool under a prefix, as in `fs.*`.
    Prefix(String),
    /// Every tool.
    Any,
}

impl NamePattern {
    /// Read a pattern, refusing the shapes `[R-POLICY-002]` forbids.
    ///
    /// # Errors
    ///
    /// [`PolicyError::BadPattern`] for a leading or interior wildcard.
    pub fn parse(text: &str) -> Result<Self> {
        if text == "*" {
            return Ok(Self::Any);
        }
        if let Some(prefix) = text.strip_suffix(".*") {
            if prefix.contains('*') {
                return Err(PolicyError::BadPattern {
                    pattern: text.to_owned(),
                    reason: "only one wildcard, at the end".to_owned(),
                });
            }
            return Ok(Self::Prefix(prefix.to_owned()));
        }
        if text.contains('*') {
            return Err(PolicyError::BadPattern {
                pattern: text.to_owned(),
                reason: "a wildcard is only allowed as a trailing `.*`".to_owned(),
            });
        }
        Ok(Self::Exact(text.to_owned()))
    }

    /// Whether it covers this tool.
    pub fn matches(&self, tool: &str) -> bool {
        match self {
            Self::Any => true,
            Self::Exact(name) => name == tool,
            Self::Prefix(prefix) => tool
                .strip_prefix(prefix.as_str())
                .is_some_and(|rest| rest.starts_with('.')),
        }
    }
}

/// What kind of thing a rule narrows on.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SelectorKind {
    /// Paths a file tool would touch.
    Paths,
    /// The command line a shell tool would run.
    Commands,
    /// The host a network tool would reach.
    Hosts,
}

impl SelectorKind {
    /// How it is spelled in a declaration.
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Paths => "paths",
            Self::Commands => "commands",
            Self::Hosts => "hosts",
        }
    }
}

/// A rule's narrowing, compiled.
#[derive(Debug, Clone)]
pub struct Selector {
    kind: SelectorKind,
    patterns: Vec<String>,
    set: GlobSet,
}

impl Selector {
    /// Compile a selector.
    ///
    /// # Errors
    ///
    /// [`PolicyError::BadGlob`] when a pattern will not compile.
    pub fn new(kind: SelectorKind, patterns: &[&str]) -> Result<Self> {
        let mut builder = GlobSetBuilder::new();
        for p in patterns {
            let glob = Glob::new(p).map_err(|e| PolicyError::BadGlob {
                pattern: (*p).to_owned(),
                reason: e.to_string(),
            })?;
            builder.add(glob);
        }
        Ok(Self {
            kind,
            patterns: patterns.iter().map(|p| (*p).to_owned()).collect(),
            set: builder.build().map_err(|e| PolicyError::BadGlob {
                pattern: patterns.join(", "),
                reason: e.to_string(),
            })?,
        })
    }

    /// What it narrows on.
    pub fn kind(&self) -> SelectorKind {
        self.kind
    }

    /// What it was written as.
    pub fn patterns(&self) -> &[String] {
        &self.patterns
    }

    /// Whether it covers this path.
    ///
    /// `[R-POLICY-003]` expects a path already made absolute with symlinks
    /// resolved. Resolving here would make evaluation touch the filesystem,
    /// which `[R-POLICY-013]` forbids, and would open a window between the
    /// check and the use.
    pub fn matches_path(&self, path: &Path) -> bool {
        self.set.is_match(path)
    }

    /// Whether it covers this text.
    ///
    /// `[R-POLICY-004]`: the whole command line as one string, so a rule says
    /// what may run rather than which binary may be named.
    pub fn matches_text(&self, text: &str) -> bool {
        self.set.is_match(text)
    }
}

/// One rule.
#[derive(Debug, Clone)]
pub struct Rule {
    /// Where it was written, for an explanation that can be acted on.
    pub origin: String,
    /// Which tools.
    pub name: NamePattern,
    /// What it narrows on, when it narrows.
    pub selector: Option<Selector>,
    /// Arguments whose values must never be shown or stored.
    ///
    /// `[R-POLICY-060]`.
    pub sensitive: Vec<String>,
}

impl Rule {
    /// A rule covering a whole tool or family.
    pub fn new(origin: impl Into<String>, pattern: &str) -> Result<Self> {
        Ok(Self {
            origin: origin.into(),
            name: NamePattern::parse(pattern)?,
            selector: None,
            sensitive: Vec::new(),
        })
    }

    /// Narrow it.
    ///
    /// # Errors
    ///
    /// [`PolicyError::BadGlob`] when a pattern will not compile.
    pub fn narrowed(mut self, kind: SelectorKind, patterns: &[&str]) -> Result<Self> {
        self.selector = Some(Selector::new(kind, patterns)?);
        Ok(self)
    }

    /// Mark arguments whose values must be redacted.
    #[must_use]
    pub fn marking_sensitive(mut self, arguments: &[&str]) -> Self {
        self.sensitive = arguments.iter().map(|a| (*a).to_owned()).collect();
        self
    }

    /// How it reads in an explanation.
    pub fn describe(&self) -> String {
        let name = match &self.name {
            NamePattern::Any => "*".to_owned(),
            NamePattern::Exact(n) => n.clone(),
            NamePattern::Prefix(p) => format!("{p}.*"),
        };
        match &self.selector {
            None => name,
            Some(s) => format!("{name} {}: {}", s.kind().as_str(), s.patterns().join(", ")),
        }
    }
}
