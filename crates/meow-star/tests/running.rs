// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The run phase: the handler context, the modules, and agents as values.
#![allow(clippy::unwrap_used)]

use std::path::Path;
use std::sync::{Arc, Mutex};

use meow_agent::Engine;
use meow_core::Usage;
use meow_core::view::{LiveKind, Output, ViewEvent};
use meow_llm::{Capabilities, LlmError, Provider, Request, Response, Structured, ToolCall};
use meow_star::port::{Ask, AskError, Events, Session, Stdin};
use meow_star::{Ports, Runtime, Workspace, load};
use serde_json::{Map, Value, json};
use tempfile::TempDir;
use tokio_util::sync::CancellationToken;

/// Keeps everything a handler wrote, so a test can read it back.
#[derive(Debug, Default)]
struct Recorder(Mutex<Vec<String>>);

impl Recorder {
    fn lines(&self) -> Vec<String> {
        self.0.lock().unwrap().clone()
    }
    fn push(&self, kind: &str, text: &str) {
        self.0.lock().unwrap().push(format!("{kind}: {text}"));
    }
}

impl Events for Recorder {
    fn event(&self, event: ViewEvent) {
        // Only what a handler said: the engine's own events have their own
        // tests, and mixing them in here would make every assertion depend on
        // how many progress ticks happened to fire.
        let ViewEvent::Live(LiveKind::Output(output)) = event else {
            return;
        };
        let text = match &output {
            Output::Write { text }
            | Output::Markdown { text }
            | Output::Note { text }
            | Output::Warn { text }
            | Output::Error { text }
            | Output::Step { text }
            | Output::Diff { patch: text } => text.clone(),
            Output::Table { columns, rows } => format!("{columns:?} {rows:?}"),
            Output::Finding {
                severity,
                location,
                summary,
            } => format!("{severity} {location} {summary}"),
            Output::Json { value } => value.to_string(),
        };
        self.push(output.call(), &text);
    }
}

/// Answers every question the same way.
#[derive(Debug)]
struct Willing(String);

impl Ask for Willing {
    fn text(&self, _prompt: &str, _default: Option<&str>) -> Result<String, AskError> {
        Ok(self.0.clone())
    }
    fn confirm(&self, _prompt: &str, _default: bool) -> Result<bool, AskError> {
        Ok(true)
    }
    fn select(&self, _prompt: &str, _choices: &[String]) -> Result<String, AskError> {
        Ok(self.0.clone())
    }
}

/// Something was piped in.
#[derive(Debug)]
struct Piped(String);

impl Stdin for Piped {
    fn is_piped(&self) -> bool {
        true
    }
    fn read(&self) -> std::io::Result<String> {
        Ok(self.0.clone())
    }
}

/// A model that says what the test told it to say.
#[derive(Debug, Default)]
struct Scripted {
    turns: Mutex<Vec<Response>>,
    seen: Mutex<Vec<Request>>,
}

impl Scripted {
    fn new(turns: Vec<Response>) -> Self {
        Self {
            turns: Mutex::new(turns),
            seen: Mutex::new(Vec::new()),
        }
    }

    fn tool_names_offered(&self) -> Vec<String> {
        self.seen
            .lock()
            .unwrap()
            .first()
            .map(|r| r.tools.iter().map(|t| t.name.clone()).collect())
            .unwrap_or_default()
    }
}

fn said(text: &str) -> Response {
    Response {
        text: text.to_owned(),
        thinking: None,
        tool_calls: Vec::new(),
        usage: Some(Usage {
            prompt: 10,
            completion: 5,
            cached: None,
            cost_micros: None,
        }),
        value: None,
    }
}

fn calls(name: &str, arguments: &str) -> Response {
    Response {
        text: String::new(),
        thinking: None,
        tool_calls: vec![ToolCall {
            id: "1".to_owned(),
            name: name.to_owned(),
            arguments: arguments.to_owned(),
        }],
        usage: None,
        value: None,
    }
}

#[async_trait::async_trait]
impl Provider for Scripted {
    fn name(&self) -> &str {
        "scripted"
    }

    fn capabilities(&self) -> Capabilities {
        Capabilities {
            streaming: false,
            tools: true,
            structured: Structured::Native,
            embeddings: false,
        }
    }

    async fn generate(
        &self,
        request: &Request,
        _cancel: &CancellationToken,
    ) -> Result<Response, LlmError> {
        self.seen.lock().unwrap().push(request.clone());
        let mut turns = self.turns.lock().unwrap();
        if turns.is_empty() {
            return Ok(said(""));
        }
        Ok(turns.remove(0))
    }

    async fn embed(
        &self,
        _texts: &[String],
        _cancel: &CancellationToken,
    ) -> Result<Vec<Vec<f32>>, LlmError> {
        Err(LlmError::Unsupported {
            provider: "scripted".to_owned(),
            capability: "embeddings",
        })
    }
}

/// A workspace, a runtime over it, and the recorder its output went to.
struct Harness {
    _dir: TempDir,
    runtime: Arc<Runtime>,
    out: Arc<Recorder>,
    provider: Arc<Scripted>,
}

fn harness(files: &[(&str, &str)], turns: Vec<Response>) -> Harness {
    let dir = tempfile::tempdir().unwrap();
    for (name, body) in files {
        let path = dir.path().join(".meow").join(name);
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(path, body).unwrap();
    }

    let workspace = Workspace::at(dir.path());
    let loaded = load(&workspace).unwrap();

    let out = Arc::new(Recorder::default());
    let provider = Arc::new(Scripted::new(turns));

    let runtime = Arc::new(Runtime::new(
        loaded,
        workspace,
        // One engine per declared provider, keyed by the name a model gives it.
        std::iter::once((
            "p".to_owned(),
            Arc::new(Engine::new(Arc::clone(&provider) as Arc<dyn Provider>)),
        ))
        .collect(),
        tokio::runtime::Handle::current(),
        Ports {
            events: Arc::clone(&out) as Arc<dyn Events>,
            ask: Arc::new(Willing("yes".to_owned())) as Arc<dyn Ask>,
            stdin: Arc::new(Piped("piped text".to_owned())) as Arc<dyn Stdin>,
            session: Arc::new(meow_star::port::quiet::Memory::new("s-1")) as Arc<dyn Session>,
        },
        CancellationToken::new(),
    ));

    Harness {
        _dir: dir,
        runtime,
        out,
        provider,
    }
}

fn args(pairs: &[(&str, Value)]) -> Map<String, Value> {
    pairs
        .iter()
        .map(|(k, v)| ((*k).to_owned(), v.clone()))
        .collect()
}

const MODELS: &str = r#"
meow.provider(name = "p", kind = "anthropic")
meow.model(name = "fast", provider = "p", id = "m", context = 100000, max_output = 4096)
"#;

/// Run a handler off the reactor, the way the binary does.
async fn call(runtime: &Arc<Runtime>, name: &str, given: Map<String, Value>) -> String {
    let runtime = Arc::clone(runtime);
    let name = name.to_owned();
    tokio::task::spawn_blocking(move || runtime.run_command(&name, &given))
        .await
        .unwrap()
        .unwrap()
}

/// Run a handler and keep the failure.
async fn fails(runtime: &Arc<Runtime>, name: &str) -> String {
    let runtime = Arc::clone(runtime);
    let name = name.to_owned();
    tokio::task::spawn_blocking(move || runtime.run_command(&name, &Map::new()))
        .await
        .unwrap()
        .unwrap_err()
        .to_string()
}

/// [R-STAR-020] the context has exactly six members and `cancelled()`
#[tokio::test(flavor = "multi_thread")]
async fn the_context_has_six_members_and_cancelled() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
def handler(ctx):
    ctx.out.write(",".join(sorted(dir(ctx))))
    return ""

meow.command(meow.tool(name = "probe", about = "look at ctx", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        ["write: args,ask,cancelled,out,session,stdin,workspace"],
        "the context is not the six members plus cancelled()"
    );
}

/// [R-STAR-020] each member does what it says
#[tokio::test(flavor = "multi_thread")]
async fn every_context_member_works() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
def handler(ctx):
    ctx.out.note("target=%s" % ctx.args.target)
    ctx.out.warn("stdin piped: %s" % ctx.stdin.is_piped())
    ctx.out.write(ctx.stdin.read())
    ctx.out.step(ctx.ask.text("who?"))
    ctx.out.markdown("id " + ctx.session.id())
    ctx.session.set("plan", ["a", "b"])
    ctx.out.finding("high", "x.star:1", str(ctx.session.get("plan")))
    return ctx.workspace

meow.command(meow.tool(
    name = "probe",
    about = "use every member",
    run = handler,
    args = {{"target": meow.arg.string()}},
))
"#
            ),
        )],
        Vec::new(),
    );

    let returned = call(&h.runtime, "probe", args(&[("target", json!("src"))])).await;

    assert_eq!(
        h.out.lines(),
        [
            "note: target=src",
            "warn: stdin piped: True",
            "write: piped text",
            "step: yes",
            "markdown: id s-1",
            "finding: high x.star:1 [\"a\", \"b\"]",
        ]
    );
    assert!(Path::new(&returned).is_absolute(), "{returned}");
}

/// [R-STAR-021] a capability is reached with `load`, not off the context
#[tokio::test(flavor = "multi_thread")]
async fn a_capability_is_loaded_rather_than_a_context_member() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"load("@std//path", "join", "base")
{MODELS}

def handler(ctx):
    ctx.out.write(base(join("a", "b", "c.star")))
    return ""

meow.command(meow.tool(name = "probe", about = "use a module", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;
    assert_eq!(h.out.lines(), ["write: c.star"]);
}

/// [R-STAR-030] a declaration inside a handler fails
#[tokio::test(flavor = "multi_thread")]
async fn a_handler_may_not_declare_anything() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
def handler(ctx):
    meow.provider(name = "sneaky", kind = "anthropic")
    return ""

meow.command(meow.tool(name = "probe", about = "declare late", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    let error = fails(&h.runtime, "probe").await;

    assert!(
        error.contains("only be called while .meow/ is being loaded"),
        "{error}"
    );
    assert!(h.runtime.registry().provider("sneaky").is_none());
}

/// [R-STAR-011] a tool inside an agent loop sees the same modules, behaving the
/// same way, as a handler called from the command line
#[tokio::test(flavor = "multi_thread")]
async fn a_tool_in_an_agent_loop_sees_the_same_modules() {
    let source = format!(
        r#"load("@std//path", "base")
{MODELS}

def shorten(ctx):
    return base(ctx.args.path)

shorten_tool = meow.tool(
    name = "shorten",
    about = "the last segment of a path",
    run = shorten,
    args = {{"path": meow.arg.string()}},
)

meow.command(shorten_tool)
meow.command(meow.agent(
    name = "helper",
    model = "fast",
    system = "use the tool",
    tools = [shorten_tool],
))
"#
    );

    let direct = harness(&[("meow.star", &source)], Vec::new());
    let from_cli = call(
        &direct.runtime,
        "shorten",
        args(&[("path", json!("a/b/c.star"))]),
    )
    .await;
    assert_eq!(from_cli, "c.star");

    let looped = harness(
        &[("meow.star", &source)],
        vec![
            calls("shorten", r#"{"path": "a/b/c.star"}"#),
            said("the file is c.star"),
        ],
    );
    let from_loop = call(
        &looped.runtime,
        "helper",
        args(&[("task", json!("shorten it"))]),
    )
    .await;

    assert_eq!(from_loop, "the file is c.star");
    assert_eq!(looped.provider.tool_names_offered(), ["shorten"]);
}

/// [R-STAR-041] an agent goes in another agent's tools list, and is offered to
/// the model the same way a tool is
#[tokio::test(flavor = "multi_thread")]
async fn an_agent_is_usable_where_a_tool_is() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
inner = meow.agent(name = "inner", model = "fast", system = "answer")

meow.command(meow.agent(
    name = "outer",
    model = "fast",
    system = "delegate",
    tools = [inner],
))
"#
            ),
        )],
        vec![said("done")],
    );

    call(&h.runtime, "outer", args(&[("task", json!("go"))])).await;

    assert_eq!(
        h.provider.tool_names_offered(),
        ["inner"],
        "an agent was not offered as a tool"
    );
}

/// [R-STAR-042] a run returns the whole outcome, not a bare string
#[tokio::test(flavor = "multi_thread")]
async fn a_run_returns_text_stop_and_usage() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
helper = meow.agent(name = "helper", model = "fast", system = "answer")

def handler(ctx):
    r = helper.run("what is it")
    ctx.out.write(r.text)
    ctx.out.note(r.stop)
    ctx.out.warn(str(r.ok))
    ctx.out.step("%d/%d" % (r.usage.prompt, r.usage.completion))
    ctx.out.markdown(r.session)
    ctx.out.finding("low", "steps", str(len(r.steps)))
    return ""

meow.command(meow.tool(name = "probe", about = "run an agent", run = handler))
"#
            ),
        )],
        vec![said("it is a cat")],
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        [
            "write: it is a cat",
            "note: finished",
            "warn: True",
            "step: 10/5",
            "markdown: s-1",
            "finding: low steps 1",
        ]
    );
}

/// [R-STAR-043] `call` builds an invocation, and `meow.parallel` takes a list
/// of them, in order
#[tokio::test(flavor = "multi_thread")]
async fn parallel_takes_invocations_and_keeps_their_order() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
helper = meow.agent(name = "helper", model = "fast", system = "answer")

def handler(ctx):
    results = meow.parallel([helper.call("one"), helper.call("two")])
    for r in results:
        ctx.out.write(r.text)
    return ""

meow.command(meow.tool(name = "probe", about = "fan out", run = handler))
"#
            ),
        )],
        vec![said("first"), said("second")],
    );

    call(&h.runtime, "probe", args(&[])).await;
    assert_eq!(h.out.lines(), ["write: first", "write: second"]);
}

/// [R-STAR-044] a Starlark function is refused, with the reason
#[tokio::test(flavor = "multi_thread")]
async fn parallel_refuses_a_starlark_function() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
def work():
    return 1

def handler(ctx):
    return str(meow.parallel([work]))

meow.command(meow.tool(name = "probe", about = "fan out wrongly", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    let error = fails(&h.runtime, "probe").await;

    assert!(error.contains("agent.call"), "{error}");
    assert!(
        error.contains("cannot be one"),
        "the reason is missing: {error}"
    );
}

/// [R-STAR-081] a builtin that runs an agent blocks and returns a value, with
/// no future or callback in sight
#[tokio::test(flavor = "multi_thread")]
async fn a_blocking_builtin_returns_a_value_not_a_future() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
helper = meow.agent(name = "helper", model = "fast", system = "answer")

def handler(ctx):
    # The run has already finished by the time this line returns, which is
    # what makes reading `.text` on the next line work at all.
    r = helper.run("go")
    ctx.out.write(type(r.text))
    ctx.out.note(r.text)
    return ""

meow.command(meow.tool(name = "probe", about = "block on a run", run = handler))
"#
            ),
        )],
        vec![said("answered")],
    );

    call(&h.runtime, "probe", args(&[])).await;
    assert_eq!(h.out.lines(), ["write: string", "note: answered"]);
}

/// A tool value runs directly, which is what replaced `ctx.run("name", ...)`.
#[tokio::test(flavor = "multi_thread")]
async fn a_tool_value_runs_by_itself() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
def double(ctx):
    return str(ctx.args.n * 2)

doubler = meow.tool(
    name = "double",
    about = "twice",
    run = double,
    args = {{"n": meow.arg.int()}},
)

def handler(ctx):
    ctx.out.write(doubler.run(n = 21))
    return ""

meow.command(meow.tool(name = "probe", about = "call a tool", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;
    assert_eq!(h.out.lines(), ["write: 42"]);
}
