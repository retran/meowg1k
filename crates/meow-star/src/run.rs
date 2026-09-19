// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Running what a workspace declared.
//!
//! The engine is async and `Send`; a Starlark evaluator is neither. The bridge
//! between them is one rule: every handler runs on its own blocking thread
//! with its own evaluator, and a builtin that needs the engine blocks that
//! thread on it. `[R-STAR-081]` asks for exactly this, and the alternative -
//! handing Starlark a future or a callback - would mean a value crossing a
//! thread boundary, which the type system correctly refuses.

use std::collections::HashMap;
use std::sync::Arc;

use meow_agent::{
    AgentSpec, Budget, Engine, Ledger, Outcome, Sink, SubAgent, Tool, ToolError, ToolSet,
};
use serde_json::{Map, Value};
use starlark::environment::{FrozenModule, Module};
use starlark::eval::Evaluator;
use tokio_util::sync::CancellationToken;

use crate::args::Args;
use crate::error::{Result, StarError, closest};
use crate::port::{Ask, Out, Session, Stdin};
use crate::registry::Registry;
use crate::state::{Phase, Running};
use crate::workspace::Workspace;

/// How deep a chain of sub-agents may go unless a declaration says otherwise.
const MAX_DEPTH: u32 = 3;

/// Where a tool's handler lives.
///
/// Recovered from the function's own display, which `starlark` renders as the
/// file it was defined in followed by its name. A lambda or a nested `def`
/// produces a name that is not a module-level symbol, and the lookup then
/// fails with a message saying so, which is the right answer: a handler has to
/// survive the module being frozen, and only a module-level function does.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Handler {
    /// The load key of the file that defines it.
    pub module: String,
    /// The name it is bound to there.
    pub symbol: String,
}

impl Handler {
    /// Read a handler out of a function's display form.
    ///
    /// # Errors
    ///
    /// [`StarError::Load`] when the value is not a named module-level
    /// function.
    pub fn parse(shown: &str) -> Result<Self> {
        match shown.rsplit_once('.') {
            Some((module, symbol)) if !module.is_empty() && !symbol.is_empty() => Ok(Self {
                module: module.to_owned(),
                symbol: symbol.to_owned(),
            }),
            _ => Err(StarError::Load {
                message: format!(
                    "`run` must be a function defined at the top level of a file, and `{shown}` is not"
                ),
            }),
        }
    }
}

/// Everything a run needs that outlives one handler.
pub struct Runtime {
    engine: Arc<Engine>,
    handle: tokio::runtime::Handle,
    workspace: Workspace,
    registry: Arc<Registry>,
    files: HashMap<String, FrozenModule>,
    std: Arc<crate::modules::Modules>,
    out: Arc<dyn Out>,
    ask: Arc<dyn Ask>,
    stdin: Arc<dyn Stdin>,
    session: Arc<dyn Session>,
    cancel: CancellationToken,
}

impl std::fmt::Debug for Runtime {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Runtime")
            .field("workspace", &self.workspace.root())
            .field("files", &self.files.keys().collect::<Vec<_>>())
            .finish_non_exhaustive()
    }
}

/// What a caller has to supply to run anything.
#[derive(Debug)]
pub struct Ports {
    /// Where output goes.
    pub out: Arc<dyn Out>,
    /// How to ask a person something.
    pub ask: Arc<dyn Ask>,
    /// What was piped in.
    pub stdin: Arc<dyn Stdin>,
    /// This invocation's session.
    pub session: Arc<dyn Session>,
}

impl Runtime {
    /// Build a runtime over a loaded workspace.
    pub fn new(
        loaded: crate::loader::Loaded,
        workspace: Workspace,
        engine: Arc<Engine>,
        handle: tokio::runtime::Handle,
        ports: Ports,
        cancel: CancellationToken,
    ) -> Self {
        Self {
            engine,
            handle,
            workspace,
            registry: Arc::new(loaded.registry),
            files: loaded.modules,
            std: loaded.std,
            out: ports.out,
            ask: ports.ask,
            stdin: ports.stdin,
            session: ports.session,
            cancel,
        }
    }

    /// What the workspace declared.
    pub fn registry(&self) -> &Registry {
        &self.registry
    }

    /// The workspace root, as a handler sees it.
    pub fn workspace(&self) -> &Workspace {
        &self.workspace
    }

    /// Where output goes.
    pub fn out(&self) -> &dyn Out {
        self.out.as_ref()
    }

    /// How to ask a person something.
    pub fn ask(&self) -> &dyn Ask {
        self.ask.as_ref()
    }

    /// What was piped in.
    pub fn stdin(&self) -> &dyn Stdin {
        self.stdin.as_ref()
    }

    /// This invocation's session.
    pub fn session(&self) -> &dyn Session {
        self.session.as_ref()
    }

    /// Whether the user has interrupted.
    pub fn cancelled(&self) -> bool {
        self.cancel.is_cancelled()
    }

    /// The token every engine call is threaded with.
    pub fn cancel_token(&self) -> &CancellationToken {
        &self.cancel
    }

    /// Wait for an asynchronous call from a script thread.
    ///
    /// `[R-STAR-081]`: the thread blocks and Starlark never sees a future.
    /// Only ever called from a thread that is not driving the reactor - the
    /// thread a handler runs on is always a `spawn_blocking` one.
    pub fn block_on<F: std::future::Future>(&self, future: F) -> F::Output {
        self.handle.block_on(future)
    }

    /// Run a command by name, with arguments already parsed.
    ///
    /// # Errors
    ///
    /// [`StarError`] when the name is not a command, the arguments do not
    /// satisfy their declaration, or the handler fails.
    pub fn run_command(self: &Arc<Self>, name: &str, args: &Map<String, Value>) -> Result<String> {
        if let Some(tool) = self.registry.tool(name) {
            let bound = tool.args.bind(args).map_err(violations)?;
            let ledger = Ledger::new(Budget::default());
            return self.call_handler(&tool.handler.clone(), &bound, &ledger, 0);
        }

        if self.registry.agent(name).is_some() {
            let task = args
                .get("task")
                .and_then(Value::as_str)
                .unwrap_or_default()
                .to_owned();
            let outcome = self.run_agent(name, &task, None, 0)?;
            return Ok(outcome.text);
        }

        Err(StarError::Unknown {
            kind: "command",
            name: name.to_owned(),
            closest: closest(name, self.registry.commands().map(String::as_str)),
        })
    }

    /// Run a declared agent to its end.
    ///
    /// # Errors
    ///
    /// [`StarError`] when the agent, its model, or one of its tools is not
    /// declared. A run that starts always returns an outcome, per
    /// `[R-AGENT-001]`; only building the run can fail.
    pub fn run_agent(
        self: &Arc<Self>,
        name: &str,
        task: &str,
        caller: Option<&Ledger>,
        depth: u32,
    ) -> Result<Outcome> {
        let spec = self.agent_spec(name, depth)?;
        let ledger = match caller {
            Some(caller) => caller.child(spec.budget),
            None => Ledger::new(spec.budget),
        };
        let mut sink = crate::run::sink(self);
        Ok(self.block_on(
            self.engine
                .run(&spec, task, &ledger, sink.as_mut(), &self.cancel),
        ))
    }

    /// Assemble an agent's specification from what was declared.
    ///
    /// # Errors
    ///
    /// [`StarError::Unknown`] naming the agent, its model, or a tool it names.
    pub fn agent_spec(self: &Arc<Self>, name: &str, depth: u32) -> Result<AgentSpec> {
        let declared = self
            .registry
            .agent(name)
            .ok_or_else(|| StarError::Unknown {
                kind: "agent",
                name: name.to_owned(),
                closest: closest(name, self.registry.agents().map(|a| a.name.as_str())),
            })?;

        let model = self
            .registry
            .model(&declared.model)
            .ok_or_else(|| StarError::Unknown {
                kind: "model",
                name: declared.model.clone(),
                closest: None,
            })?;

        let ledger = Ledger::new(declared.budget);
        let tools = self.tools_for(&declared.tools, &ledger, depth)?;

        Ok(AgentSpec {
            name: declared.name.clone(),
            model: model.id.clone(),
            system: Some(declared.system.clone()),
            tools,
            budget: declared.budget,
            output: declared.output.clone(),
            on_tool_error: declared.on_tool_error,
            max_output_tokens: model.max_output,
            compaction: declared.compaction.clone(),
            context_window: model.context,
            max_depth: MAX_DEPTH,
            policy: declared
                .policy
                .clone()
                .or_else(|| self.registry.policy().cloned()),
            grants: meow_policy::Grants::new(),
            describe_call: None,
        })
    }

    /// Turn declared tool names into tools the engine can call.
    ///
    /// A name may be a tool or another agent, which is what `[R-STAR-041]`
    /// means by an agent being usable where a tool is. Construction stops
    /// adding sub-agents at the depth limit rather than recursing forever on
    /// a pair of agents that name each other.
    fn tools_for(
        self: &Arc<Self>,
        names: &[String],
        ledger: &Ledger,
        depth: u32,
    ) -> Result<ToolSet> {
        let mut set = ToolSet::new();
        for name in names {
            if let Some(tool) = self.registry.tool(name) {
                set = set.with(StarlarkTool {
                    runtime: Arc::clone(self),
                    name: tool.name.clone(),
                    about: tool.about.clone(),
                    schema: tool.args.json_schema(),
                    args: tool.args.clone(),
                    handler: tool.handler.clone(),
                    depth,
                });
                continue;
            }
            if self.registry.agent(name).is_some() {
                if depth + 1 >= MAX_DEPTH {
                    continue;
                }
                let spec = Arc::new(self.agent_spec(name, depth + 1)?);
                set = set.with(SubAgent::new(
                    spec,
                    Arc::clone(&self.engine),
                    ledger,
                    depth + 1,
                ));
                continue;
            }
            return Err(StarError::Unknown {
                kind: "tool",
                name: name.clone(),
                closest: closest(
                    name,
                    self.registry
                        .tools()
                        .map(|t| t.name.as_str())
                        .chain(self.registry.agents().map(|a| a.name.as_str())),
                ),
            });
        }
        Ok(set)
    }

    /// Call one Starlark handler on this thread.
    ///
    /// `[R-STAR-080]`: a fresh evaluator on a scoped heap, so nothing the
    /// handler allocated survives the call and nothing crosses a thread.
    ///
    /// # Errors
    ///
    /// [`StarError::Starlark`] carrying the diagnostic, which is what makes
    /// `[R-STAR-090]` hold for a run-time failure as well as a load-time one.
    pub fn call_handler(
        self: &Arc<Self>,
        handler: &Handler,
        args: &Map<String, Value>,
        ledger: &Ledger,
        depth: u32,
    ) -> Result<String> {
        let module = self
            .files
            .get(&handler.module)
            .ok_or_else(|| StarError::Load {
                message: format!("`{}` was not loaded", handler.module),
            })?;

        let function = module.get(&handler.symbol).map_err(|_| StarError::Load {
            message: format!(
                "`{}` has no top-level function `{}`. A tool handler has to be one, because it must survive the file being frozen.",
                handler.module, handler.symbol
            ),
        })?;

        let state = Running {
            runtime: Arc::clone(self),
            args: args.clone(),
            ledger: ledger.clone(),
            depth,
        };

        let loader = crate::loader::RuntimeLoader::new(self);

        Module::with_temp_heap(|scope| {
            // The handler was frozen into another heap when its file was
            // loaded. This registers that heap as a dependency of the scoped
            // one, which is what keeps it alive for the length of the call.
            let function = scope.heap().access_owned_frozen_value(&function);
            let ctx = crate::context::build(&scope, &state)?;

            let mut eval = Evaluator::new(&scope);
            eval.extra = Some(&state);
            eval.set_loader(&loader);
            eval.set_print_handler(&crate::loader::NoPrint);

            let returned = eval
                .eval_function(function, &[ctx], &[])
                .map_err(|e| StarError::Starlark(format!("{e}")))?;

            Ok(match returned.unpack_str() {
                Some(text) => text.to_owned(),
                None if returned.is_none() => String::new(),
                None => returned
                    .to_json()
                    .map_err(|e| StarError::Starlark(format!("{e}")))?,
            })
        })
    }

    /// The modules the workspace loaded, by load key.
    pub(crate) fn files(&self) -> &HashMap<String, FrozenModule> {
        &self.files
    }

    /// The `@std//` table, shared with the load phase.
    pub(crate) fn std(&self) -> &crate::modules::Modules {
        &self.std
    }
}

/// Where an agent's events go while a handler is waiting on it.
fn sink(runtime: &Arc<Runtime>) -> Box<dyn Sink + Send> {
    Box::new(Relay {
        out: Arc::clone(&runtime.out),
    })
}

/// Turns engine events into lines on the output port.
struct Relay {
    out: Arc<dyn Out>,
}

impl Sink for Relay {
    fn event(&mut self, event: meow_agent::AgentEvent) -> std::result::Result<(), String> {
        match event {
            meow_agent::AgentEvent::Text(delta) => self.out.write(&delta),
            meow_agent::AgentEvent::ToolStart { name, .. } => {
                self.out.step(&format!("{name}(...)"));
            }
            _ => {}
        }
        Ok(())
    }
}

/// A declared tool, as the engine calls it.
struct StarlarkTool {
    runtime: Arc<Runtime>,
    name: String,
    about: String,
    schema: Value,
    args: Args,
    handler: Handler,
    depth: u32,
}

impl std::fmt::Debug for StarlarkTool {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("StarlarkTool")
            .field("name", &self.name)
            .finish()
    }
}

#[async_trait::async_trait]
impl Tool for StarlarkTool {
    fn name(&self) -> &str {
        &self.name
    }

    fn description(&self) -> &str {
        &self.about
    }

    fn schema(&self) -> Value {
        self.schema.clone()
    }

    async fn call(
        &self,
        args: &Value,
        _cancel: &CancellationToken,
    ) -> std::result::Result<String, ToolError> {
        let given = match args {
            Value::Object(map) => map.clone(),
            Value::Null => Map::new(),
            other => {
                return Err(ToolError(format!(
                    "arguments must be an object, and are {other}"
                )));
            }
        };

        // `[R-STAR-063]`: the model's arguments meet the same declaration the
        // command line does, so a constraint cannot be enforced on one path
        // and not the other.
        let bound = self
            .args
            .bind(&given)
            .map_err(|problems| ToolError(describe(&problems)))?;

        let runtime = Arc::clone(&self.runtime);
        let handler = self.handler.clone();
        let ledger = Ledger::new(Budget::default());
        let depth = self.depth;

        // `[R-STAR-080]`: its own thread and its own evaluator. The handler
        // cannot run on the thread that is already blocked waiting for this
        // call, and a Starlark value cannot move between the two.
        tokio::task::spawn_blocking(move || runtime.call_handler(&handler, &bound, &ledger, depth))
            .await
            .map_err(|e| ToolError(format!("the handler did not finish: {e}")))?
            .map_err(|e| ToolError(e.to_string()))
    }
}

fn describe(problems: &[crate::schema::Violation]) -> String {
    problems
        .iter()
        .map(ToString::to_string)
        .collect::<Vec<_>>()
        .join("; ")
}

fn violations(problems: Vec<crate::schema::Violation>) -> StarError {
    StarError::Load {
        message: describe(&problems),
    }
}

/// Which phase an evaluator is in, from inside a builtin.
pub(crate) fn phase_of(eval: &Evaluator<'_, '_, '_>) -> Option<Phase> {
    let extra = eval.extra?;
    if extra.downcast_ref::<crate::state::Declaring>().is_some() {
        return Some(Phase::Declaring);
    }
    if extra.downcast_ref::<Running>().is_some() {
        return Some(Phase::Running);
    }
    None
}

/// Reach the run state from inside a builtin.
///
/// `[R-STAR-084]`: a runtime module called while `.meow/` is being evaluated
/// says so, rather than failing with something about a missing value.
pub(crate) fn running<'a>(
    eval: &Evaluator<'_, 'a, '_>,
    what: &str,
) -> starlark::Result<&'a Running> {
    if let Some(state) = eval.extra.and_then(|e| e.downcast_ref::<Running>()) {
        return Ok(state);
    }
    let message = match phase_of(eval) {
        Some(Phase::Declaring) => StarError::ModuleUnavailable {
            module: what.to_owned(),
        }
        .to_string(),
        _ => format!("`{what}` is not available here"),
    };
    Err(starlark::Error::new_other(anyhow::anyhow!("{message}")))
}
