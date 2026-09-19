// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What a workspace declared.

use std::collections::BTreeMap;

pub use crate::agent::AgentDecl;
use crate::args::Args;
use crate::error::{Result, StarError, closest};

/// Commands the binary owns.
///
/// `[R-STAR-033]`: a user command with one of these names is refused at load
/// time rather than shadowed in either direction, because `meow session` being
/// sometimes one thing and sometimes another is worse than a clear refusal.
pub const RESERVED: &[&str] = &[
    "auth",
    "check",
    "completions",
    "doctor",
    "index",
    "init",
    "models",
    "policy",
    "providers",
    "run",
    "session",
    "trust",
    "version",
];

/// Where something was declared.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Origin(pub String);

impl std::fmt::Display for Origin {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.0)
    }
}

/// A place to reach a model.
#[derive(Debug, Clone)]
pub struct Provider {
    /// What other declarations call it.
    pub name: String,
    /// Which implementation.
    pub kind: String,
    /// The key, when one was given inline rather than resolved from the store.
    pub api_key: Option<String>,
    /// Where to reach it, when it is not where its kind usually lives.
    ///
    /// A proxy, a gateway, or a server somebody is running themselves. The
    /// kind decides the shape of the API and this decides the address, which
    /// is what lets one implementation serve every vendor that copied it.
    pub base_url: Option<String>,
    /// Where it was declared.
    pub origin: Origin,
}

/// What a model is for.
///
/// `[R-STAR-034]`. Two kinds rather than one, because an agent given an
/// embedding model and an index given a chat model both fail in ways that look
/// like a bad answer rather than a bad declaration.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum ModelKind {
    /// It answers.
    #[default]
    Chat,
    /// It turns text into vectors.
    Embedding,
}

impl ModelKind {
    /// How it is written in a declaration.
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Chat => "chat",
            Self::Embedding => "embedding",
        }
    }

    /// Read the spelling.
    pub fn parse(text: &str) -> Option<Self> {
        match text {
            "chat" => Some(Self::Chat),
            "embedding" => Some(Self::Embedding),
            _ => None,
        }
    }
}

/// A named model configuration.
///
/// Presets are gone. v0.2.x had provider, model, and preset, where the third
/// existed only to name a model-plus-temperature pair, which is what a named
/// model already is.
#[derive(Debug, Clone)]
pub struct Model {
    /// What agents call it.
    pub name: String,
    /// Which provider serves it.
    pub provider: String,
    /// What the provider calls it.
    pub id: String,
    /// How much it can hold.
    pub context: u32,
    /// How much it may produce.
    pub max_output: u32,
    /// How much to explore.
    pub temperature: Option<f32>,
    /// What it is for.
    pub kind: ModelKind,
    /// Where it was declared.
    pub origin: Origin,
}

/// How a workspace is indexed.
///
/// `[R-STAR-035]`: which model embeds it, and the parameters `[R-INDEX-003]`
/// and `[R-INDEX-012]` call configured. Declared at most once, because two
/// declarations would leave the index built by whichever the loader reached
/// first.
#[derive(Debug, Clone)]
pub struct IndexDecl {
    /// Which model embeds the workspace.
    pub model: String,
    /// How many lines a chunk holds.
    pub chunk_lines: Option<u32>,
    /// How many lines two neighbours share.
    pub overlap: Option<u32>,
    /// The size at which a file is skipped.
    pub max_bytes: Option<u64>,
    /// Where it was declared.
    pub origin: Origin,
}

/// A tool, as declared.
#[derive(Debug, Clone)]
pub struct ToolDecl {
    /// What the model calls it.
    pub name: String,
    /// What it does, in the model's terms.
    pub about: String,
    /// Where its implementation lives.
    ///
    /// A name rather than the function itself: the file it is defined in is
    /// frozen once loading ends, and a value from before the freeze cannot
    /// outlive the evaluator that made it.
    pub handler: crate::run::Handler,
    /// What it takes.
    ///
    /// Kept as the declaration rather than as a finished schema, because
    /// `[R-STAR-061]` asks the one declaration to produce the flag, the help
    /// line, and the model's schema. Storing only the schema would leave the
    /// command line to reconstruct what it needs, which is the drift the
    /// requirement exists to stop.
    pub args: Args,
    /// Where it was declared.
    pub origin: Origin,
}

/// Everything one workspace declared.
///
/// One table, per `[R-STAR-010]`. v0.2.x assembled its context in two places,
/// `ctx_run.go` and `module_llm.go`, which had already diverged on UI nesting
/// depth: a module added to one was silently missing from tools running inside
/// an agent loop. There is nowhere here for a second copy to live.
#[derive(Debug, Default)]
pub struct Registry {
    providers: BTreeMap<String, Provider>,
    models: BTreeMap<String, Model>,
    tools: BTreeMap<String, ToolDecl>,
    agents: BTreeMap<String, AgentDecl>,
    commands: BTreeMap<String, Origin>,
    policy: Option<(meow_policy::Policy, Origin)>,
    index: Option<IndexDecl>,
}

impl Registry {
    /// An empty registry.
    pub fn new() -> Self {
        Self::default()
    }

    /// Declare a provider.
    ///
    /// # Errors
    ///
    /// [`StarError::Duplicate`] naming both declaration sites.
    pub fn add_provider(&mut self, p: Provider) -> Result<()> {
        if let Some(first) = self.providers.get(&p.name) {
            return Err(duplicate("provider", &p.name, &first.origin, &p.origin));
        }
        self.providers.insert(p.name.clone(), p);
        Ok(())
    }

    /// Declare a model.
    ///
    /// # Errors
    ///
    /// [`StarError::Duplicate`] naming both declaration sites.
    pub fn add_model(&mut self, m: Model) -> Result<()> {
        if let Some(first) = self.models.get(&m.name) {
            return Err(duplicate("model", &m.name, &first.origin, &m.origin));
        }
        self.models.insert(m.name.clone(), m);
        Ok(())
    }

    /// Declare a tool.
    ///
    /// # Errors
    ///
    /// [`StarError::Duplicate`] naming both declaration sites.
    pub fn add_tool(&mut self, t: ToolDecl) -> Result<()> {
        if let Some(first) = self.tools.get(&t.name) {
            return Err(duplicate("tool", &t.name, &first.origin, &t.origin));
        }
        self.tools.insert(t.name.clone(), t);
        Ok(())
    }

    /// Declare an agent.
    ///
    /// # Errors
    ///
    /// [`StarError::Duplicate`] naming both declaration sites.
    pub fn add_agent(&mut self, a: AgentDecl) -> Result<()> {
        if let Some(first) = self.agents.get(&a.name) {
            return Err(duplicate("agent", &a.name, &first.origin, &a.origin));
        }
        self.agents.insert(a.name.clone(), a);
        Ok(())
    }

    /// Say how this workspace is indexed.
    ///
    /// # Errors
    ///
    /// [`StarError::Duplicate`] naming both declaration sites.
    pub fn set_index(&mut self, index: IndexDecl) -> Result<()> {
        if let Some(first) = &self.index {
            return Err(duplicate(
                "index",
                "workspace",
                &first.origin,
                &index.origin,
            ));
        }
        self.index = Some(index);
        Ok(())
    }

    /// How this workspace is indexed, when it says.
    pub fn index(&self) -> Option<&IndexDecl> {
        self.index.as_ref()
    }

    /// Set what every agent in this workspace may do.
    ///
    /// One policy, declared once. Two `meow.policy` calls would have to be
    /// combined, and there is no combination that is obviously right: taking
    /// the stricter surprises whoever wrote the second, and taking the later
    /// makes the result depend on load order.
    ///
    /// # Errors
    ///
    /// [`StarError::Duplicate`] naming both declaration sites.
    pub fn set_policy(&mut self, policy: meow_policy::Policy, origin: Origin) -> Result<()> {
        if let Some((_, first)) = &self.policy {
            return Err(duplicate("policy", "workspace", first, &origin));
        }
        self.policy = Some((policy, origin));
        Ok(())
    }

    /// What every agent in this workspace may do, when one was declared.
    pub fn policy(&self) -> Option<&meow_policy::Policy> {
        self.policy.as_ref().map(|(policy, _)| policy)
    }

    /// Put something on the command line.
    ///
    /// # Errors
    ///
    /// [`StarError::Reserved`] for a built-in's name, or
    /// [`StarError::Duplicate`] when it is exposed twice.
    pub fn add_command(&mut self, name: &str, origin: Origin) -> Result<()> {
        if RESERVED.contains(&name) {
            return Err(StarError::Reserved {
                name: name.to_owned(),
            });
        }
        if let Some(first) = self.commands.get(name) {
            return Err(duplicate("command", name, first, &origin));
        }
        self.commands.insert(name.to_owned(), origin);
        Ok(())
    }

    /// Check that every reference resolves.
    ///
    /// Satisfies `[R-STAR-032]`: at load time, not at first use, and after
    /// every declaration file has been evaluated, so that declaration order
    /// inside and between files does not matter. Resolving as each declaration
    /// is made would mean an agent could not name a model declared below it,
    /// which is a rule nobody would guess from reading a file top to bottom.
    ///
    /// # Errors
    ///
    /// [`StarError::Unknown`] naming what is missing and the closest declared
    /// name when one is near.
    pub fn resolve(&self) -> Result<()> {
        for model in self.models.values() {
            if !self.providers.contains_key(&model.provider) {
                return Err(StarError::Unknown {
                    kind: "provider",
                    name: model.provider.clone(),
                    closest: closest(&model.provider, self.providers.keys().map(String::as_str)),
                });
            }
        }
        for agent in self.agents.values() {
            let Some(model) = self.models.get(&agent.model) else {
                return Err(StarError::Unknown {
                    kind: "model",
                    name: agent.model.clone(),
                    closest: closest(&agent.model, self.models.keys().map(String::as_str)),
                });
            };
            // [R-STAR-034]: an agent given an embedding model produces
            // nonsense that looks like an answer, so it is refused here.
            if model.kind != ModelKind::Chat {
                return Err(StarError::WrongKind {
                    name: model.name.clone(),
                    wanted: ModelKind::Chat.as_str(),
                    is: model.kind.as_str(),
                    used_by: format!("agent `{}`", agent.name),
                });
            }
            for tool in &agent.tools {
                if !self.tools.contains_key(tool) && !self.agents.contains_key(tool) {
                    return Err(StarError::Unknown {
                        kind: "tool",
                        name: tool.clone(),
                        closest: closest(
                            tool,
                            self.tools
                                .keys()
                                .chain(self.agents.keys())
                                .map(String::as_str),
                        ),
                    });
                }
            }
        }
        if let Some(index) = &self.index {
            let Some(model) = self.models.get(&index.model) else {
                return Err(StarError::Unknown {
                    kind: "model",
                    name: index.model.clone(),
                    closest: closest(&index.model, self.models.keys().map(String::as_str)),
                });
            };
            if model.kind != ModelKind::Embedding {
                return Err(StarError::WrongKind {
                    name: model.name.clone(),
                    wanted: ModelKind::Embedding.as_str(),
                    is: model.kind.as_str(),
                    used_by: "meow.index".to_owned(),
                });
            }
        }

        for name in self.commands.keys() {
            if !self.tools.contains_key(name) && !self.agents.contains_key(name) {
                return Err(StarError::Unknown {
                    kind: "tool or agent",
                    name: name.clone(),
                    closest: closest(
                        name,
                        self.tools
                            .keys()
                            .chain(self.agents.keys())
                            .map(String::as_str),
                    ),
                });
            }
        }
        Ok(())
    }

    /// One provider.
    pub fn provider(&self, name: &str) -> Option<&Provider> {
        self.providers.get(name)
    }

    /// One model.
    pub fn model(&self, name: &str) -> Option<&Model> {
        self.models.get(name)
    }

    /// Every model, by name.
    pub fn models(&self) -> impl Iterator<Item = &Model> {
        self.models.values()
    }

    /// Every provider, by name.
    pub fn providers(&self) -> impl Iterator<Item = &Provider> {
        self.providers.values()
    }

    /// One tool.
    pub fn tool(&self, name: &str) -> Option<&ToolDecl> {
        self.tools.get(name)
    }

    /// Every tool, by name.
    pub fn tools(&self) -> impl Iterator<Item = &ToolDecl> {
        self.tools.values()
    }

    /// One agent.
    pub fn agent(&self, name: &str) -> Option<&AgentDecl> {
        self.agents.get(name)
    }

    /// Every agent, by name.
    pub fn agents(&self) -> impl Iterator<Item = &AgentDecl> {
        self.agents.values()
    }

    /// Every name on the command line.
    pub fn commands(&self) -> impl Iterator<Item = &String> {
        self.commands.keys()
    }
}

fn duplicate(kind: &'static str, name: &str, first: &Origin, second: &Origin) -> StarError {
    StarError::Duplicate {
        kind,
        name: name.to_owned(),
        first: first.to_string(),
        second: second.to_string(),
    }
}
