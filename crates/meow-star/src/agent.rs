// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! An agent declaration, and the one builder that every source of one uses.
//!
//! `[R-STAR-051]` asks a markdown agent and a Starlark agent to produce the
//! same value. The cheapest way to promise that and keep the promise is to
//! give both one intermediate and one builder, so there is no second place for
//! a default to be decided differently. [`Fields`] is that intermediate:
//! `meow.agent` fills it from its keyword arguments and a `.md` file
//! deserializes it from its frontmatter.

use std::time::Duration;

use meow_agent::{Budget, Compaction, ToolErrorPolicy};
use meow_policy::{Decision, Policy, Rule, SelectorKind};
use serde::Deserialize;
use serde_json::Value;

use crate::error::{Result, StarError};
use crate::registry::Origin;

/// What bounds a run, as a declaration writes it.
///
/// Minutes rather than a duration string, because YAML has no duration type
/// and `[R-STAR-050]` needs frontmatter to express everything a keyword
/// argument can.
#[derive(Debug, Clone, Default, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct BudgetFields {
    /// Prompt plus completion tokens across the run.
    pub tokens: Option<u32>,
    /// How many times the model may be asked.
    pub steps: Option<u32>,
    /// Wall clock, in minutes.
    pub minutes: Option<u64>,
    /// Estimated spend, in millionths.
    pub cost_micros: Option<u64>,
}

impl BudgetFields {
    /// Turn it into what the engine takes.
    ///
    /// An omitted field keeps the engine's default rather than becoming
    /// unbounded, so `budget: {steps: 5}` tightens one axis without quietly
    /// removing the other three.
    fn build(&self) -> Budget {
        let default = Budget::default();
        Budget {
            tokens: self.tokens.or(default.tokens),
            steps: self.steps.or(default.steps),
            duration: self
                .minutes
                .map(Duration::from_secs)
                .map(|m| m * 60)
                .or(default.duration),
            cost_micros: self.cost_micros.or(default.cost_micros),
        }
    }
}

/// When and how to summarise a long run, as a declaration writes it.
#[derive(Debug, Clone, Default, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct CompactionFields {
    /// The fraction of the context window at which to act.
    pub at: Option<f32>,
    /// How many of the most recent messages stay verbatim.
    pub keep_recent: Option<usize>,
    /// Which model summarises.
    pub model: Option<String>,
}

impl CompactionFields {
    fn build(&self) -> Compaction {
        let default = Compaction::default();
        Compaction {
            at: self.at.unwrap_or(default.at),
            keep_recent: self.keep_recent.unwrap_or(default.keep_recent),
            model: self.model.clone().or(default.model),
        }
    }
}

/// One permission rule, as a declaration writes it.
#[derive(Debug, Clone, Default, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RuleFields {
    /// Which tools it covers. `fs.*` and `*` are the shapes allowed.
    pub tools: Vec<String>,
    /// What to do: `allow`, `ask`, or `deny`.
    pub decision: String,
    /// Paths a file tool may touch.
    pub paths: Option<Vec<String>>,
    /// Command lines a shell tool may run.
    pub commands: Option<Vec<String>>,
    /// Hosts a network tool may reach.
    pub hosts: Option<Vec<String>>,
    /// Arguments whose values must never be shown or stored.
    pub sensitive: Option<Vec<String>>,
}

/// Everything a declaration can say about an agent.
///
/// The field set is `[R-STAR-040]`'s, minus `name`, which the caller supplies:
/// a Starlark agent is named by a keyword argument and a markdown agent by its
/// filename, and threading that difference through here would put the one
/// thing that is genuinely not shared into the shared type.
#[derive(Debug, Clone, Default, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Fields {
    /// Which model.
    pub model: Option<String>,
    /// How it is framed.
    ///
    /// A `.md` file supplies this from its body, so frontmatter carrying it is
    /// refused by [`Fields::build`] rather than by the deserializer, which
    /// lets the error say why instead of "unknown field".
    pub system: Option<String>,
    /// What it does.
    pub about: Option<String>,
    /// What it may call, by name.
    pub tools: Option<Vec<String>>,
    /// What bounds it.
    pub budget: Option<BudgetFields>,
    /// When and how to summarise.
    pub compaction: Option<CompactionFields>,
    /// A schema the answer must satisfy.
    pub output: Option<Value>,
    /// What to do when a tool fails: `report` or `abort`.
    pub on_tool_error: Option<String>,
    /// What it may do.
    pub policy: Option<Vec<RuleFields>>,
    /// `.meow/lib/*.md` files to prepend to the system prompt.
    ///
    /// Frontmatter only. A Starlark agent composes its prompt with ordinary
    /// string operations and needs no key for it.
    pub include: Option<Vec<String>>,
}

/// An agent, declared.
#[derive(Debug, Clone)]
pub struct AgentDecl {
    /// What to call it.
    pub name: String,
    /// What it does.
    pub about: String,
    /// Which model.
    pub model: String,
    /// How it is framed.
    pub system: String,
    /// What it may call, by name.
    pub tools: Vec<String>,
    /// What bounds it.
    pub budget: Budget,
    /// When and how to summarise.
    pub compaction: Compaction,
    /// A schema the answer must satisfy.
    pub output: Option<Value>,
    /// What to do when a tool fails.
    pub on_tool_error: ToolErrorPolicy,
    /// What it may do, or everything when it declares nothing.
    pub policy: Option<Policy>,
    /// Where it was declared.
    pub origin: Origin,
}

/// Where a declaration came from, and what that lets it say.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Source {
    /// A `meow.agent` call.
    Starlark,
    /// A `.md` file under `.meow/agents/`.
    Markdown,
}

impl Fields {
    /// Turn a declaration into an agent.
    ///
    /// `prompts` resolves an `include` entry to the text of a `.meow/lib/*.md`
    /// file. It is a closure because reading those files is the loader's job
    /// and this module has no business touching the disk.
    ///
    /// # Errors
    ///
    /// [`StarError::Load`] naming the field and what is wrong with it.
    pub fn build(
        &self,
        name: &str,
        source: Source,
        origin: &Origin,
        prompts: impl Fn(&str) -> Result<String>,
    ) -> Result<AgentDecl> {
        let model = self.model.clone().ok_or_else(|| missing(name, "model"))?;

        let system = match (source, &self.system) {
            // [R-STAR-050]: the body is the prompt, so frontmatter carrying
            // `system` has two prompts and no rule for which wins. Refused,
            // rather than picking one and surprising half of its readers.
            (Source::Markdown, Some(_)) => {
                return Err(StarError::Load {
                    message: format!(
                        "`{name}`: the body of a markdown agent is its system prompt, so frontmatter may not carry `system`"
                    ),
                });
            }
            (Source::Markdown, None) => String::new(),
            (Source::Starlark, Some(system)) => system.clone(),
            (Source::Starlark, None) => return Err(missing(name, "system")),
        };

        if source == Source::Starlark && self.include.is_some() {
            return Err(StarError::Load {
                message: format!(
                    "`{name}`: `include` belongs to a markdown agent. A Starlark agent builds its prompt with string operations."
                ),
            });
        }

        // [R-STAR-054]: prepended in the order given, separated by a blank
        // line, and that is the whole composition. Substitution and
        // conditionals are how a configuration format turns into a bad
        // programming language; an agent that needs logic is a Starlark agent.
        let mut parts = Vec::new();
        for path in self.include.iter().flatten() {
            parts.push(prompts(path)?);
        }
        parts.push(system);
        let system = parts
            .into_iter()
            .filter(|part| !part.trim().is_empty())
            .collect::<Vec<_>>()
            .join("\n\n");

        let on_tool_error = match self.on_tool_error.as_deref() {
            None | Some("report") => ToolErrorPolicy::Report,
            Some("abort") => ToolErrorPolicy::Abort,
            Some(other) => {
                return Err(StarError::Load {
                    message: format!(
                        "`{name}`: `on_tool_error` is `report` or `abort`, and is `{other}`"
                    ),
                });
            }
        };

        let policy = self
            .policy
            .as_ref()
            .map(|rules| build_policy(rules, origin))
            .transpose()?;

        Ok(AgentDecl {
            name: name.to_owned(),
            about: self.about.clone().unwrap_or_default(),
            model,
            system,
            tools: self.tools.clone().unwrap_or_default(),
            budget: self.budget.clone().unwrap_or_default().build(),
            compaction: self.compaction.clone().unwrap_or_default().build(),
            output: self.output.clone(),
            on_tool_error,
            policy,
            origin: origin.clone(),
        })
    }
}

/// Compile declared rules into a policy.
///
/// # Errors
///
/// [`StarError::Load`] carrying what `meow-policy` said about the pattern or
/// the glob, so a bad rule fails where it is written rather than on the call
/// it was meant to stop.
pub fn build_policy(rules: &[RuleFields], origin: &Origin) -> Result<Policy> {
    let mut policy = Policy::new();
    for declared in rules {
        let decision = match declared.decision.as_str() {
            "allow" => Decision::Allow,
            "ask" => Decision::Ask,
            "deny" => Decision::Deny,
            other => {
                return Err(StarError::Load {
                    message: format!("`decision` is `allow`, `ask`, or `deny`, and is `{other}`"),
                });
            }
        };

        for pattern in &declared.tools {
            // One rule carries one selector, so a declaration naming both
            // paths and commands becomes two rules with the same decision.
            // That is what the declaration means, and flattening it here keeps
            // `meow policy explain` printing one line per thing it matched.
            let selectors = [
                (SelectorKind::Paths, declared.paths.as_ref()),
                (SelectorKind::Commands, declared.commands.as_ref()),
                (SelectorKind::Hosts, declared.hosts.as_ref()),
            ];

            let mut narrowed = false;
            for (kind, patterns) in selectors {
                let Some(patterns) = patterns else { continue };
                narrowed = true;
                let refs: Vec<&str> = patterns.iter().map(String::as_str).collect();
                policy = policy.with(
                    decision,
                    rule(origin, pattern, declared)?
                        .narrowed(kind, &refs)
                        .map_err(policy_error)?,
                );
            }
            if !narrowed {
                policy = policy.with(decision, rule(origin, pattern, declared)?);
            }
        }
    }
    Ok(policy)
}

fn rule(origin: &Origin, pattern: &str, declared: &RuleFields) -> Result<Rule> {
    let built = Rule::new(origin.to_string(), pattern).map_err(policy_error)?;
    Ok(match &declared.sensitive {
        Some(arguments) => {
            let refs: Vec<&str> = arguments.iter().map(String::as_str).collect();
            built.marking_sensitive(&refs)
        }
        None => built,
    })
}

fn policy_error(e: meow_policy::PolicyError) -> StarError {
    StarError::Load {
        message: e.to_string(),
    }
}

fn missing(name: &str, field: &str) -> StarError {
    StarError::Load {
        message: format!("`{name}` needs a `{field}`"),
    }
}
