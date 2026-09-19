// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! What a workspace declared.

use std::collections::BTreeMap;

use serde_json::Value;

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
    /// Where it was declared.
    pub origin: Origin,
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

/// An agent, as declared.
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
    /// A schema the answer must satisfy.
    pub output: Option<Value>,
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
            if !self.models.contains_key(&agent.model) {
                return Err(StarError::Unknown {
                    kind: "model",
                    name: agent.model.clone(),
                    closest: closest(&agent.model, self.models.keys().map(String::as_str)),
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

    /// One tool.
    pub fn tool(&self, name: &str) -> Option<&ToolDecl> {
        self.tools.get(name)
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
