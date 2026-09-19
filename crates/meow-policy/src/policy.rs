// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Deciding.

use std::collections::BTreeSet;
use std::path::{Path, PathBuf};

use crate::error::{PolicyError, Result};
use crate::rule::{Rule, SelectorKind};

/// What a policy says about one call.
///
/// `[R-POLICY-010]` orders them: deny beats ask beats allow.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub enum Decision {
    /// Run it.
    Allow,
    /// Ask a person.
    Ask,
    /// Do not run it.
    Deny,
}

impl Decision {
    /// How it reads in a transcript.
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Allow => "allow",
            Self::Ask => "ask",
            Self::Deny => "deny",
        }
    }
}

/// Where a decision came from.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Source {
    /// A rule matched.
    Rule,
    /// No rule matched, so the default applied.
    ///
    /// `[R-POLICY-011]`: the default is deny. A tool an agent was never
    /// granted stays unreachable however convincingly it is asked for.
    NoMatch,
    /// A person answered a prompt.
    Interactive,
    /// A person granted it for this process earlier.
    SessionGrant,
}

impl Source {
    /// How it reads in a transcript.
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Rule => "rule",
            Self::NoMatch => "no rule matched",
            Self::Interactive => "answered",
            Self::SessionGrant => "session grant",
        }
    }
}

/// Whether a call reads or writes.
///
/// The distinction decides what a partially denied call does:
/// `[R-POLICY-008]` lets a read proceed and name what it skipped, and
/// `[R-POLICY-009]` denies a write whole, because a partial write leaves the
/// workspace in a state nobody chose.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Access {
    /// It only looks.
    Read,
    /// It changes something.
    Write,
}

/// A call, resolved and ready to be judged.
///
/// `[R-POLICY-003]` and `[R-POLICY-014]`: the paths are already absolute with
/// symlinks resolved, and the tool must act on exactly these. Re-resolving
/// later reopens the window in which a path allowed as a file becomes a
/// symlink to somewhere denied.
#[derive(Debug, Clone)]
pub struct Call {
    /// Which tool.
    pub tool: String,
    /// Whether it reads or writes.
    pub access: Access,
    /// The paths it would touch, resolved.
    pub paths: Vec<PathBuf>,
    /// The command line it would run.
    pub command: Option<String>,
    /// The host it would reach.
    pub host: Option<String>,
}

impl Call {
    /// A call that touches nothing in particular.
    pub fn new(tool: impl Into<String>, access: Access) -> Self {
        Self {
            tool: tool.into(),
            access,
            paths: Vec::new(),
            command: None,
            host: None,
        }
    }

    /// With resolved paths.
    #[must_use]
    pub fn with_paths(mut self, paths: Vec<PathBuf>) -> Self {
        self.paths = paths;
        self
    }

    /// With a command line.
    #[must_use]
    pub fn with_command(mut self, command: impl Into<String>) -> Self {
        self.command = Some(command.into());
        self
    }

    /// With a host.
    #[must_use]
    pub fn with_host(mut self, host: impl Into<String>) -> Self {
        self.host = Some(host.into());
        self
    }
}

/// What a policy decided, and why.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Verdict {
    /// What to do.
    pub decision: Decision,
    /// Which rule said so, when one did.
    ///
    /// `[R-POLICY-012]`: a decision that cannot name its rule cannot be acted
    /// on, because the user has nothing to change.
    pub rule: Option<String>,
    /// Where it was declared.
    pub origin: Option<String>,
    /// How it was reached.
    pub source: Source,
    /// Paths a read must skip, per `[R-POLICY-008]`.
    pub denied_paths: Vec<PathBuf>,
    /// How many higher-precedence rules were checked without matching.
    ///
    /// `[R-POLICY-051]`.
    pub checked: usize,
}

/// Grants a person made during this process.
///
/// `[R-POLICY-023]`: never written to a file. Persisting a grant made inside a
/// prompt is how a permission system rots: the next run inherits a decision
/// nobody remembers making.
#[derive(Debug, Clone, Default)]
pub struct Grants(BTreeSet<String>);

impl Grants {
    /// No grants.
    pub fn new() -> Self {
        Self::default()
    }

    /// Remember one, for this process only.
    pub fn grant(&mut self, tool: &str) {
        self.0.insert(tool.to_owned());
    }

    /// Whether this tool was granted.
    pub fn has(&self, tool: &str) -> bool {
        self.0.contains(tool)
    }
}

/// What a workspace or an agent permits.
#[derive(Debug, Clone, Default)]
pub struct Policy {
    deny: Vec<Rule>,
    ask: Vec<Rule>,
    allow: Vec<Rule>,
}

impl Policy {
    /// A policy that permits nothing, which is where every policy starts.
    pub fn new() -> Self {
        Self::default()
    }

    /// Add a rule at one precedence.
    #[must_use]
    pub fn with(mut self, decision: Decision, rule: Rule) -> Self {
        match decision {
            Decision::Deny => self.deny.push(rule),
            Decision::Ask => self.ask.push(rule),
            Decision::Allow => self.allow.push(rule),
        }
        self
    }

    /// Check that every selector applies to something.
    ///
    /// Satisfies `[R-POLICY-005]`. `supports` answers which selectors a tool
    /// matching a pattern has; the caller knows its own tools, and the policy
    /// layer does not.
    ///
    /// # Errors
    ///
    /// [`PolicyError::UnsupportedSelector`] naming the rule and the selector.
    pub fn validate(
        &self,
        supports: &dyn Fn(&crate::rule::NamePattern, SelectorKind) -> bool,
    ) -> Result<()> {
        for rule in self.deny.iter().chain(&self.ask).chain(&self.allow) {
            if let Some(selector) = &rule.selector
                && !supports(&rule.name, selector.kind())
            {
                return Err(PolicyError::UnsupportedSelector {
                    pattern: rule.describe(),
                    selector: selector.kind().as_str(),
                });
            }
        }
        Ok(())
    }

    /// Decide about one call.
    ///
    /// Satisfies `[R-POLICY-010]` by checking deny, then ask, then allow, and
    /// returning the first match; `[R-POLICY-011]` by denying when nothing
    /// matches; `[R-POLICY-013]` by touching no filesystem, network, or clock,
    /// and by taking the grants as an argument rather than reading them from
    /// somewhere, so the same call and the same policy and the same grants
    /// always give the same answer.
    pub fn evaluate(&self, call: &Call, grants: &Grants) -> Verdict {
        let mut checked = 0;

        for (decision, rules) in [
            (Decision::Deny, &self.deny),
            (Decision::Ask, &self.ask),
            (Decision::Allow, &self.allow),
        ] {
            for rule in rules {
                if !rule.name.matches(&call.tool) {
                    checked += 1;
                    continue;
                }
                let Some(hit) = self.selector_hit(rule, call) else {
                    checked += 1;
                    continue;
                };
                // A session grant turns an ask into an allow, and never
                // touches a deny: [R-POLICY-031] forbids widening, and a
                // person answering a prompt is not a reason to stop denying
                // what the workspace denied.
                let (decision, source) = if decision == Decision::Ask && grants.has(&call.tool) {
                    (Decision::Allow, Source::SessionGrant)
                } else {
                    (decision, Source::Rule)
                };
                return Verdict {
                    decision,
                    rule: Some(rule.describe()),
                    origin: Some(rule.origin.clone()),
                    source,
                    denied_paths: hit.denied,
                    checked,
                };
            }
        }

        Verdict {
            decision: Decision::Deny,
            rule: None,
            origin: None,
            source: Source::NoMatch,
            denied_paths: call.paths.clone(),
            checked,
        }
    }

    /// Whether a rule's selector covers this call, and which paths it misses.
    fn selector_hit(&self, rule: &Rule, call: &Call) -> Option<Hit> {
        let Some(selector) = &rule.selector else {
            return Some(Hit { denied: Vec::new() });
        };
        match selector.kind() {
            SelectorKind::Paths => {
                if call.paths.is_empty() {
                    return None;
                }
                // [R-POLICY-006]: once per resolved path.
                let (matched, missed): (Vec<&PathBuf>, Vec<&PathBuf>) = call
                    .paths
                    .iter()
                    .partition(|p| selector.matches_path(p.as_path()));
                if matched.is_empty() {
                    return None;
                }
                Some(Hit {
                    denied: missed.into_iter().cloned().collect(),
                })
            }
            SelectorKind::Commands => {
                let command = call.command.as_deref()?;
                selector
                    .matches_text(command)
                    .then_some(Hit { denied: Vec::new() })
            }
            // [R-POLICY-007]: a policy can allow one host without allowing the
            // network. Egress is where a prompt-injected agent does the most
            // damage, so an all-or-nothing network rule is not enough.
            SelectorKind::Hosts => {
                let host = call.host.as_deref()?;
                selector
                    .matches_text(host)
                    .then_some(Hit { denied: Vec::new() })
            }
        }
    }

    /// The effective policy when an agent narrows the workspace's.
    ///
    /// Satisfies `[R-POLICY-030]` and `[R-POLICY-031]`: for every call, the
    /// more restrictive of what the two say, ordering deny above ask above
    /// allow. An agent can therefore tighten and never loosen, which is what
    /// makes the workspace policy a boundary rather than a suggestion.
    pub fn narrowed_by(&self, agent: &Policy) -> Narrowed<'_> {
        Narrowed {
            workspace: self,
            agent: agent.clone(),
        }
    }

    /// Which arguments of a call must never be shown or stored.
    ///
    /// `[R-POLICY-060]`.
    pub fn sensitive_for(&self, tool: &str) -> Vec<String> {
        let mut out = Vec::new();
        for rule in self.deny.iter().chain(&self.ask).chain(&self.allow) {
            if rule.name.matches(tool) {
                out.extend(rule.sensitive.iter().cloned());
            }
        }
        out.sort_unstable();
        out.dedup();
        out
    }
}

struct Hit {
    denied: Vec<PathBuf>,
}

/// A workspace policy an agent has narrowed.
#[derive(Debug)]
pub struct Narrowed<'a> {
    workspace: &'a Policy,
    agent: Policy,
}

impl Narrowed<'_> {
    /// Decide, taking the stricter of the two.
    pub fn evaluate(&self, call: &Call, grants: &Grants) -> Verdict {
        let w = self.workspace.evaluate(call, grants);
        let a = self.agent.evaluate(call, grants);
        if a.decision >= w.decision { a } else { w }
    }
}

/// Replace a sensitive value.
///
/// `[R-POLICY-061]`: a fixed placeholder, revealing nothing, not even how long
/// the value was.
pub const REDACTED: &str = "[redacted]";

/// Redact the marked arguments of a call.
///
/// `[R-POLICY-060]`: the same text is used in the approval prompt, the
/// transcript, and every export, so a secret cannot be shown in one and hidden
/// in another.
pub fn redact(args: &serde_json::Value, sensitive: &[String]) -> serde_json::Value {
    let Some(obj) = args.as_object() else {
        return args.clone();
    };
    let mut out = obj.clone();
    for name in sensitive {
        if out.contains_key(name) {
            out.insert(name.clone(), serde_json::Value::String(REDACTED.to_owned()));
        }
    }
    serde_json::Value::Object(out)
}

/// Resolve a path the way `[R-POLICY-003]` expects, before evaluation.
///
/// Here rather than inside `evaluate`, because evaluation must touch no
/// filesystem and because the tool has to act on exactly what was judged.
///
/// # Errors
///
/// Whatever the filesystem said.
pub fn resolve(path: &Path) -> std::io::Result<PathBuf> {
    std::fs::canonicalize(path)
}
