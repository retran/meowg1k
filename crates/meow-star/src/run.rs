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
use std::path::{Path, PathBuf};
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
use crate::port::{Ask, Events, Session, Stdin};
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
    engines: HashMap<String, Arc<Engine>>,
    handle: tokio::runtime::Handle,
    workspace: Workspace,
    registry: Arc<Registry>,
    files: HashMap<String, FrozenModule>,
    std: Arc<crate::modules::Modules>,
    events: Arc<dyn Events>,
    approve: Option<Arc<dyn meow_agent::Approver>>,
    dry_run: bool,
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
    /// Where events go.
    pub events: Arc<dyn Events>,
    /// Who answers when the policy says to ask.
    ///
    /// `None` resolves every `ask` to `deny`, per `[R-POLICY-020]`: an
    /// unattended run must not be able to approve itself.
    pub approve: Option<Arc<dyn meow_agent::Approver>>,
    /// Whether to plan tool calls instead of making them.
    pub dry_run: bool,
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
        engines: HashMap<String, Arc<Engine>>,
        handle: tokio::runtime::Handle,
        ports: Ports,
        cancel: CancellationToken,
    ) -> Self {
        Self {
            engines,
            handle,
            workspace,
            registry: Arc::new(loaded.registry),
            files: loaded.modules,
            std: loaded.std,
            events: ports.events,
            approve: ports.approve,
            dry_run: ports.dry_run,
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

    /// Where events go.
    pub fn events(&self) -> &dyn Events {
        self.events.as_ref()
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

    /// The engine that serves an agent's model.
    ///
    /// A workspace declares several providers and each model names one, so
    /// which engine runs an agent is a property of the declarations rather
    /// than of the run. The engine itself takes one provider and has no way to
    /// know that `claude-haiku-4-5` belongs to Anthropic, which is why the
    /// choice is made here.
    ///
    /// # Errors
    ///
    /// [`StarError::Unknown`] when no engine was built for the provider the
    /// model names, which means a credential was missing at startup.
    fn engine_for(&self, model: &str) -> Result<&Arc<Engine>> {
        let declared = self
            .registry
            .model(model)
            .ok_or_else(|| StarError::Unknown {
                kind: "model",
                name: model.to_owned(),
                closest: closest(model, self.registry.models().map(|m| m.name.as_str())),
            })?;
        self.engines
            .get(&declared.provider)
            .ok_or_else(|| StarError::Unknown {
                kind: "provider",
                name: declared.provider.clone(),
                closest: closest(&declared.provider, self.engines.keys().map(String::as_str)),
            })
    }

    /// Which model an agent names.
    fn agent_model(&self, name: &str) -> Result<String> {
        self.registry
            .agent(name)
            .map(|a| a.model.clone())
            .ok_or_else(|| StarError::Unknown {
                kind: "agent",
                name: name.to_owned(),
                closest: closest(name, self.registry.agents().map(|a| a.name.as_str())),
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
        let engine = Arc::clone(self.engine_for(&self.agent_model(name)?)?);
        let mut sink = crate::run::sink(self);

        // `[R-SESSION-051]`: the history is whatever the session says it saw,
        // superseded ranges and all. An empty one is a fresh run, which is
        // `[R-SESSION-054]`: continuation is never inferred.
        let history = self.session.history();

        Ok(
            self.block_on(engine.resume(
                &spec,
                history,
                task,
                &ledger,
                sink.as_mut(),
                &self.cancel,
            )),
        )
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
            approve: self.approve.clone(),
            describe_call: Some(describe_call(self.workspace.root().to_path_buf())),
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
                if self.dry_run {
                    set = set.with(Planned {
                        events: Arc::clone(&self.events),
                        name: tool.name.clone(),
                        about: tool.about.clone(),
                        schema: tool.args.json_schema(),
                        warned: std::sync::atomic::AtomicBool::new(false),
                    });
                    continue;
                }
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
                let engine = Arc::clone(self.engine_for(&self.agent_model(name)?)?);
                set = set.with(SubAgent::new(spec, engine, ledger, depth + 1));
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

/// Turn a tool call into something the policy can judge.
///
/// `[R-POLICY-003]`: the paths are resolved here, before the decision, and the
/// tool acts on exactly these. Re-resolving afterwards reopens the window in
/// which a path allowed as a file becomes a symbolic link to somewhere denied.
///
/// The convention is the argument's name. The engine does not know which
/// argument is a path and which is a command line, and a declaration does not
/// say; naming one `path`, `paths`, `file`, `command`, or `url` is what marks
/// it, and a tool that wants to be governed uses those names.
fn describe_call(root: PathBuf) -> meow_agent::DescribeCall {
    Arc::new(move |name: &str, args: &Value| {
        let write = name.contains("write")
            || name.contains("remove")
            || name.contains("append")
            || name.contains("mkdir");
        let access = if write {
            meow_policy::Access::Write
        } else {
            meow_policy::Access::Read
        };

        let mut call = meow_policy::Call::new(name, access);

        let mut paths = Vec::new();
        for key in ["path", "file", "paths", "files"] {
            match args.get(key) {
                Some(Value::String(one)) => paths.push(resolve(&root, one)),
                Some(Value::Array(many)) => {
                    paths.extend(
                        many.iter()
                            .filter_map(Value::as_str)
                            .map(|p| resolve(&root, p)),
                    );
                }
                _ => {}
            }
        }
        if !paths.is_empty() {
            call = call.with_paths(paths);
        }

        if let Some(command) = args.get("command").and_then(Value::as_str) {
            call = call.with_command(command);
        }
        if let Some(url) = args.get("url").and_then(Value::as_str)
            && let Some(host) = host_of(url)
        {
            call = call.with_host(host);
        }

        call
    })
}

/// A path as the policy will judge it: absolute, with symbolic links resolved.
fn resolve(root: &Path, path: &str) -> PathBuf {
    let given = Path::new(path);
    let joined = if given.is_absolute() {
        given.to_path_buf()
    } else {
        root.join(given)
    };
    // A path that does not exist yet cannot be resolved, and a write to a new
    // file is exactly that case. The lexical form is what the policy judges
    // then, which is the same thing the tool will create.
    meow_policy::resolve(&joined).unwrap_or(joined)
}

/// The host part of a URL, without parsing the whole thing.
fn host_of(url: &str) -> Option<&str> {
    let rest = url.split_once("://")?.1;
    let host = rest.split(['/', '?', '#']).next()?;
    let host = host.rsplit_once('@').map_or(host, |(_, h)| h);
    Some(host.split(':').next().unwrap_or(host)).filter(|h| !h.is_empty())
}

/// Where an agent's events go while a handler is waiting on it.
fn sink(runtime: &Arc<Runtime>) -> Box<dyn Sink + Send> {
    Box::new(Relay {
        events: Arc::clone(&runtime.events),
        log: Arc::clone(&runtime.session),
        session: runtime.session.id(),
        started: std::time::Instant::now(),
        names: HashMap::new(),
        said: String::new(),
        steps: 0,
        usage: meow_core::Usage::default(),
    })
}

/// Turns what the engine reports into what a renderer reads.
///
/// The mapping lives here rather than in `meow-ui`, because the engine's event
/// type belongs to `meow-agent` and `meow-ui` is not allowed to know the
/// engine exists. That boundary is what lets a renderer be driven from a
/// recorded log with no engine behind it.
///
/// It also keeps the little state the view needs and the engine does not: the
/// name a tool call was made under, how many steps have passed, and what has
/// been spent. The engine has no reason to repeat those on every event, and a
/// renderer has no way to derive them.
struct Relay {
    events: Arc<dyn Events>,
    log: Arc<dyn Session>,
    session: String,
    started: std::time::Instant,
    names: HashMap<String, String>,
    /// What the model has said in the step that is running.
    said: String,
    steps: u32,
    usage: meow_core::Usage,
}

impl Relay {
    fn elapsed_ms(&self) -> u64 {
        u64::try_from(self.started.elapsed().as_millis()).unwrap_or(u64::MAX)
    }

    fn send(&self, event: meow_core::view::ViewEvent) {
        self.events.event(event);
    }

    /// Write down what the model said in a step, if it said anything.
    fn wrote(&self, text: String) {
        if text.trim().is_empty() {
            return;
        }
        self.log.record(meow_core::EventKind::Assistant {
            content: text,
            tool_calls: Vec::new(),
        });
    }

    /// Show it and write it down.
    ///
    /// One call, because an event that reached the transcript and not the log
    /// is a run whose record disagrees with what somebody watched happen.
    fn keep(&self, kind: meow_core::EventKind) {
        self.log.record(kind.clone());
        self.send(meow_core::view::ViewEvent::Logged(kind));
    }

    /// Tell the live region where the run has got to.
    ///
    /// `[R-TUI-012]` wants the current tool, the elapsed time, the step count,
    /// and the budget consumed, and this is the only place that knows all four
    /// at once.
    fn progress(&self, tool: Option<String>) {
        self.send(meow_core::view::ViewEvent::Live(
            meow_core::view::LiveKind::Progress {
                step: self.steps,
                elapsed_ms: self.elapsed_ms(),
                tokens: self.usage.prompt.saturating_add(self.usage.completion),
                cost_micros: self.usage.cost_micros,
                tool,
            },
        ));
    }
}

impl Sink for Relay {
    fn event(&mut self, event: meow_agent::AgentEvent) -> std::result::Result<(), String> {
        use meow_agent::AgentEvent as E;
        use meow_core::view::{LiveKind, ViewEvent};

        match event {
            E::RunStart { agent } => {
                self.send(ViewEvent::Live(LiveKind::RunStart {
                    agent,
                    model: String::new(),
                    session: self.session.clone(),
                }));
            }
            E::StepStart { step } => {
                self.steps = step;
                self.send(ViewEvent::Live(LiveKind::StepStart { step }));
                self.progress(None);
            }
            E::Text(delta) => {
                self.said.push_str(&delta);
                self.send(ViewEvent::Live(LiveKind::TextDelta { delta }));
            }
            E::Thinking(delta) => self.send(ViewEvent::Live(LiveKind::ThinkingDelta { delta })),
            // Whichever of the two a sink asked for: with deltas the text
            // arrives in pieces and is written down when the step ends, and
            // without them it arrives whole. Recording both would log every
            // step twice.
            E::StepText { text, .. } => self.wrote(text),
            E::StepEnd { .. } => {
                let text = std::mem::take(&mut self.said);
                self.wrote(text);
            }
            E::ToolStart { id, name, args } => {
                self.names.insert(id.clone(), name.clone());
                self.log.record(meow_core::EventKind::ToolCall {
                    id: id.clone(),
                    name: name.clone(),
                    args: args.clone(),
                });
                self.send(ViewEvent::Live(LiveKind::ToolStart {
                    id,
                    name: name.clone(),
                    args,
                }));
                self.progress(Some(name));
            }
            E::ToolEnd {
                id,
                duration_ms,
                error,
            } => {
                let name = self.names.remove(&id).unwrap_or_default();
                self.log.record(meow_core::EventKind::ToolResult {
                    id: id.clone(),
                    output: String::new(),
                    duration_ms,
                    error: error.clone(),
                });
                self.send(ViewEvent::Live(LiveKind::ToolEnd {
                    id,
                    name,
                    duration_ms,
                    error,
                }));
                self.progress(None);
            }
            E::Policy { id, decision } => {
                // `[R-SESSION-070]` and `[R-SESSION-071]`: before the tool
                // runs, and for the denials as well as the approvals.
                self.keep(meow_core::EventKind::Policy {
                    id,
                    decision,
                    rule: None,
                    source: "rule".to_owned(),
                });
            }
            E::Usage(usage) => {
                self.usage = self.usage.add(&usage);
                self.keep(meow_core::EventKind::Usage(usage));
                self.progress(None);
            }
            E::Compacted {
                supersedes,
                summary,
                tokens_saved,
            } => {
                let start = u64::try_from(supersedes.start).unwrap_or(0);
                let end = u64::try_from(supersedes.end.saturating_sub(1)).unwrap_or(0);
                self.keep(meow_core::EventKind::Compaction {
                    supersedes: start..=end,
                    summary,
                    tokens_saved,
                });
            }
            E::RunEnd { stop, detail } => {
                self.send(ViewEvent::Live(LiveKind::RunEnd {
                    stop,
                    detail,
                    steps: self.steps,
                    usage: self.usage,
                    elapsed_ms: self.elapsed_ms(),
                    session: self.session.clone(),
                }));
            }
        }
        Ok(())
    }
}

/// A tool that is planned rather than run.
///
/// Satisfies `[R-TUI-072]`: the policy decision still happens, because the
/// engine takes it before the tool is reached, and the transcript records what
/// would have run. The model is fed a placeholder.
///
/// The warning is the important half. A dry run diverges from a real one the
/// moment the first placeholder goes back to the model, because what the model
/// does next depends on a result it never received. Everything after that
/// point is a plausible run, not the run.
struct Planned {
    events: Arc<dyn Events>,
    name: String,
    about: String,
    schema: Value,
    warned: std::sync::atomic::AtomicBool,
}

impl std::fmt::Debug for Planned {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Planned").field("name", &self.name).finish()
    }
}

#[async_trait::async_trait]
impl Tool for Planned {
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
        use std::sync::atomic::Ordering;

        if !self.warned.swap(true, Ordering::Relaxed) {
            crate::port::say(
                self.events.as_ref(),
                meow_core::view::Output::Warn {
                    text: "this is a dry run: from here on the model is answering a result it never received, so what follows is a plausible run rather than the run".to_owned(),
                },
            );
        }

        crate::port::say(
            self.events.as_ref(),
            meow_core::view::Output::Note {
                text: format!("would run {}({args})", self.name),
            },
        );

        Ok(format!(
            "this was a dry run, so `{}` did not run and there is no result. Continue as if it had succeeded.",
            self.name
        ))
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
