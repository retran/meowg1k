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
use meow_star::port::{Ask, AskError, Events, Found, Indexed, Search, Session, Stats, Stdin};
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

/// An index that answers, so the `search` and `index` modules can be driven
/// without a store, a provider, or a built graph.
#[derive(Debug, Default)]
struct FakeIndex {
    /// Every call, in order, so a test can check what reached the port.
    seen: Mutex<Vec<String>>,
}

impl FakeIndex {
    fn note(&self, what: String) {
        self.seen.lock().unwrap().push(what);
    }
    fn calls(&self) -> Vec<String> {
        self.seen.lock().unwrap().clone()
    }
}

impl Search for FakeIndex {
    fn code(&self, query: &str, limit: usize, paths: &[String]) -> Result<Vec<Found>, String> {
        self.query(query, limit, paths, 0.0)
    }

    fn query(
        &self,
        query: &str,
        limit: usize,
        paths: &[String],
        min_score: f32,
    ) -> Result<Vec<Found>, String> {
        self.note(format!("query {query} {limit} {paths:?} {min_score}"));
        Ok(vec![Found {
            path: "src/lib.rs".to_owned(),
            first_line: 10,
            last_line: 12,
            text: "fn main() {}".to_owned(),
            score: 0.75,
        }])
    }

    fn update(&self) -> Result<Indexed, String> {
        self.note("update".to_owned());
        Ok(Indexed {
            files: 7,
            added: 2,
            changed: 1,
            removed: 0,
            embedded: 0,
        })
    }

    fn build(&self) -> Result<Indexed, String> {
        self.note("build".to_owned());
        Ok(Indexed {
            files: 7,
            added: 2,
            changed: 1,
            removed: 0,
            embedded: 3,
        })
    }

    fn stats(&self) -> Result<Stats, String> {
        self.note("stats".to_owned());
        Ok(Stats {
            chunks: 40,
            embedded: 39,
            model: Some("embed".to_owned()),
        })
    }

    fn text(
        &self,
        pattern: &str,
        regex: bool,
        limit: usize,
        paths: &[String],
    ) -> Result<Vec<Found>, String> {
        self.note(format!("text {pattern} regex={regex} {limit} {paths:?}"));
        Ok(vec![Found {
            path: "README.md".to_owned(),
            first_line: 3,
            last_line: 3,
            text: "a line that matched".to_owned(),
            score: 1.0,
        }])
    }

    fn files(&self, pattern: &str, limit: usize) -> Result<Vec<String>, String> {
        self.note(format!("files {pattern} {limit}"));
        Ok(vec!["src/lib.rs".to_owned(), "src/main.rs".to_owned()])
    }
}

/// A workspace, a runtime over it, and the recorder its output went to.
struct Harness {
    _dir: TempDir,
    runtime: Arc<Runtime>,
    out: Arc<Recorder>,
    provider: Arc<Scripted>,
    index: Arc<FakeIndex>,
}

fn harness(files: &[(&str, &str)], turns: Vec<Response>) -> Harness {
    harness_with(files, turns, true)
}

/// The same, with an index that answers or one that is not there.
fn harness_with(files: &[(&str, &str)], turns: Vec<Response>, indexed: bool) -> Harness {
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
    let index = Arc::new(FakeIndex::default());

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
            approve: None,
            dry_run: false,
            search: if indexed {
                Arc::clone(&index) as Arc<dyn Search>
            } else {
                Arc::new(meow_star::port::quiet::NoIndex) as Arc<dyn Search>
            },
            keep: Arc::new(meow_star::port::quiet::Ephemeral::default()),
        },
        CancellationToken::new(),
    ));

    Harness {
        _dir: dir,
        runtime,
        out,
        provider,
        index,
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

/// [R-TUI-050] ctx.ask has exactly text, confirm, and select
#[tokio::test(flavor = "multi_thread")]
async fn ctx_ask_has_exactly_three_calls() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
def handler(ctx):
    ctx.out.write(",".join(sorted(dir(ctx.ask))))
    return ""

meow.command(meow.tool(name = "probe", about = "look at ask", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;
    assert_eq!(h.out.lines(), ["write: confirm,select,text"]);
}

/// [R-TUI-041] ctx.out has exactly the ten calls the spec fixes it at
#[tokio::test(flavor = "multi_thread")]
async fn ctx_out_has_exactly_ten_calls() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
def handler(ctx):
    ctx.out.write(",".join(sorted(dir(ctx.out))))
    return ""

meow.command(meow.tool(name = "probe", about = "look at out", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;
    assert_eq!(
        h.out.lines(),
        ["write: diff,error,finding,json,markdown,note,step,table,warn,write"],
        "ctx.out is not the ten semantic calls"
    );
}

/// `@std//fs` reads and writes inside the workspace.
#[tokio::test(flavor = "multi_thread")]
async fn fs_reads_and_writes_inside_the_workspace() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"load("@std//fs", "read", "write", "append", "exists", "glob", "mkdir", "remove")
{MODELS}

def handler(ctx):
    write("notes/a.txt", "first\n")
    append("notes/a.txt", "second\n")
    ctx.out.write(read("notes/a.txt").strip())
    ctx.out.note(str(exists("notes/a.txt")))
    mkdir("notes/deep")
    write("notes/deep/b.txt", "x")
    ctx.out.step(",".join(glob("notes/**/*.txt")))
    remove("notes/a.txt")
    ctx.out.warn(str(exists("notes/a.txt")))
    return ""

meow.command(meow.tool(name = "probe", about = "use fs", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        [
            "write: first\nsecond",
            "note: True",
            "step: notes/a.txt,notes/deep/b.txt",
            "warn: False",
        ]
    );
}

/// A path that leaves the workspace is refused, whichever way it is written.
#[tokio::test(flavor = "multi_thread")]
async fn fs_refuses_a_path_outside_the_workspace() {
    for (call_text, expected) in [
        (r#"read("../secret.txt")"#, "climbs out"),
        (r#"write("../secret.txt", "x")"#, "climbs out"),
        // Rooted with no drive letter: absolute on Unix, and on Windows
        // neither absolute nor workspace-relative, which is the case a check
        // written only against `is_absolute` lets through.
        (r#"read("/etc/hosts")"#, "outside the workspace"),
    ] {
        let h = harness(
            &[(
                "meow.star",
                &format!(
                    r#"load("@std//fs", "read", "write")
{MODELS}

def handler(ctx):
    return str({call_text})

meow.command(meow.tool(name = "probe", about = "escape", run = handler))
"#
                ),
            )],
            Vec::new(),
        );

        let runtime = Arc::clone(&h.runtime);
        let error = tokio::task::spawn_blocking(move || runtime.run_command("probe", &Map::new()))
            .await
            .unwrap()
            .unwrap_err()
            .to_string();

        assert!(error.contains(expected), "`{call_text}` gave: {error}");
    }
}

/// `fs.remove` will not delete the workspace or the store.
#[tokio::test(flavor = "multi_thread")]
async fn fs_remove_will_not_take_the_workspace_or_the_store() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"load("@std//fs", "remove")
{MODELS}

def handler(ctx):
    remove(".meow/.data")
    return ""

meow.command(meow.tool(name = "probe", about = "delete the store", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    let runtime = Arc::clone(&h.runtime);
    let error = tokio::task::spawn_blocking(move || runtime.run_command("probe", &Map::new()))
        .await
        .unwrap()
        .unwrap_err()
        .to_string();

    assert!(
        error.contains("not something `fs.remove` will delete"),
        "{error}"
    );
}

/// `@std//shell` runs a command and reports what it did.
#[tokio::test(flavor = "multi_thread")]
async fn shell_runs_a_command_and_reports_it() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"load("@std//shell", "run", "capture", "which")
{MODELS}

def handler(ctx):
    ctx.out.write(run(["echo", "hello"]).strip())
    r = capture(["sh", "-c", "exit 3"])
    ctx.out.note("%d %s" % (r.code, r.ok))
    ctx.out.step(str(which("nonesuch-program-xyz") == None))
    return ""

meow.command(meow.tool(name = "probe", about = "use shell", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        ["write: hello", "note: 3 False", "step: True"]
    );
}

/// A command is a list of words, never a string for a shell to split.
#[tokio::test(flavor = "multi_thread")]
async fn shell_refuses_a_command_as_one_string() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"load("@std//shell", "run")
{MODELS}

def handler(ctx):
    return run("echo hello && rm -rf /")

meow.command(meow.tool(name = "probe", about = "one string", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    let runtime = Arc::clone(&h.runtime);
    let error = tokio::task::spawn_blocking(move || runtime.run_command("probe", &Map::new()))
        .await
        .unwrap()
        .unwrap_err()
        .to_string();

    assert!(error.contains("takes a list of words"), "{error}");
    assert!(
        error.contains("never saw the shape of"),
        "the reason is missing: {error}"
    );
}

/// A command that will not finish is killed rather than waited on.
#[tokio::test(flavor = "multi_thread")]
async fn shell_kills_a_command_that_overruns() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"load("@std//shell", "run")
{MODELS}

def handler(ctx):
    return run(["sleep", "30"], timeout_secs = 1)

meow.command(meow.tool(name = "probe", about = "hang", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    let runtime = Arc::clone(&h.runtime);
    let error = tokio::task::spawn_blocking(move || runtime.run_command("probe", &Map::new()))
        .await
        .unwrap()
        .unwrap_err()
        .to_string();

    assert!(error.contains("did not finish in 1 seconds"), "{error}");
}

/// [R-STAR-084] a capability module is loadable during declaration and
/// refuses to be called there
#[tokio::test(flavor = "multi_thread")]
async fn fs_and_shell_refuse_to_run_during_declaration() {
    for (module, symbol, call_text) in [
        ("fs", "read", r#"read("a.txt")"#),
        ("shell", "run", r#"run(["echo", "x"])"#),
    ] {
        let dir = tempfile::tempdir().unwrap();
        let config = dir.path().join(".meow");
        std::fs::create_dir_all(&config).unwrap();
        std::fs::write(
            config.join("meow.star"),
            format!("load(\"@std//{module}\", \"{symbol}\")\n{call_text}\n"),
        )
        .unwrap();

        let error = meow_star::load(&Workspace::at(dir.path()))
            .unwrap_err()
            .to_string();

        assert!(
            error.contains("not available while .meow/ is being loaded"),
            "@std//{module} ran during declaration: {error}"
        );
    }
}

/// Turn a harness's workspace into a git repository with one commit.
///
/// A real repository rather than a fake one: `@std//git` shells out to the
/// user's own `git`, and a test against a stub would check the stub's idea of
/// what `--porcelain` prints.
fn make_repo(dir: &std::path::Path) {
    let run = |args: &[&str]| {
        let status = std::process::Command::new("git")
            .args(args)
            .current_dir(dir)
            .env("GIT_AUTHOR_NAME", "Test")
            .env("GIT_AUTHOR_EMAIL", "test@example.com")
            .env("GIT_COMMITTER_NAME", "Test")
            .env("GIT_COMMITTER_EMAIL", "test@example.com")
            .stdout(std::process::Stdio::null())
            .stderr(std::process::Stdio::null())
            .status()
            .unwrap();
        assert!(status.success(), "git {args:?} failed");
    };

    run(&["init", "--initial-branch=main"]);
    run(&["config", "user.name", "Test"]);
    run(&["config", "user.email", "test@example.com"]);
    std::fs::write(dir.join("README.md"), "# a project\n").unwrap();
    // The workspace's own configuration is under version control and its
    // store is not, which is what a real one looks like. Leaving `.meow/`
    // untracked would put it in every `status` these tests read.
    std::fs::write(dir.join(".gitignore"), ".meow/.data/\n").unwrap();
    run(&["add", "README.md", ".gitignore", ".meow"]);
    run(&["commit", "-m", "first"]);
}

const GIT_WORKSPACE: &str = r#"
load("@std//git", "diff", "status", "log", "show", "branch", "stage", "commit")
load("@std//fs", "write")

meow.provider(name = "p", kind = "anthropic")
meow.model(name = "fast", provider = "p", id = "m", context = 100000, max_output = 4096)
"#;

/// `@std//git` reads a repository through the program that owns it.
#[tokio::test(flavor = "multi_thread")]
async fn git_reads_the_repository() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{GIT_WORKSPACE}
def handler(ctx):
    ctx.out.write(branch())
    ctx.out.note(log(limit = 1).strip().split(" ", 1)[1])

    write("new.txt", "hello\n")
    ctx.out.step(status().strip())

    stage(["new.txt"])
    ctx.out.markdown("staged" if "new.txt" in diff() else "missing")
    return ""

meow.command(meow.tool(name = "probe", about = "read git", run = handler))
"#
            ),
        )],
        Vec::new(),
    );
    make_repo(h.runtime.workspace().root());

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        [
            "write: main",
            "note: first",
            "step: ?? new.txt",
            "markdown: staged",
        ]
    );
}

/// `git.commit` takes only what is staged, and hands back the commit.
#[tokio::test(flavor = "multi_thread")]
async fn git_commits_only_what_is_staged() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{GIT_WORKSPACE}
def handler(ctx):
    write("staged.txt", "in\n")
    write("loose.txt", "out\n")
    stage(["staged.txt"])

    sha = commit("add staged.txt")
    ctx.out.write(str(len(sha) > 0))
    ctx.out.note(show(sha).strip().split("\n")[0][:6])
    # The loose file is still untracked, which is the point.
    ctx.out.step(status().strip())
    return ""

meow.command(meow.tool(name = "probe", about = "commit", run = handler))
"#
            ),
        )],
        Vec::new(),
    );
    make_repo(h.runtime.workspace().root());

    call(&h.runtime, "probe", args(&[])).await;

    let lines = h.out.lines();
    assert_eq!(lines[0], "write: True");
    assert_eq!(lines[1], "note: commit");
    assert_eq!(lines[2], "step: ?? loose.txt");
}

/// A value that would be read as an option is refused rather than passed on.
#[tokio::test(flavor = "multi_thread")]
async fn git_refuses_a_value_that_looks_like_an_option() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{GIT_WORKSPACE}
def handler(ctx):
    return show("--upload-pack=touch owned")

meow.command(meow.tool(name = "probe", about = "smuggle a flag", run = handler))
"#
            ),
        )],
        Vec::new(),
    );
    make_repo(h.runtime.workspace().root());

    let runtime = Arc::clone(&h.runtime);
    let error = tokio::task::spawn_blocking(move || runtime.run_command("probe", &Map::new()))
        .await
        .unwrap()
        .unwrap_err()
        .to_string();

    assert!(error.contains("may not begin with `-`"), "{error}");
    assert!(error.contains("would read"), "{error}");
}

/// A `git` that fails says what it said, with the command that failed.
#[tokio::test(flavor = "multi_thread")]
async fn git_reports_what_the_program_said() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{GIT_WORKSPACE}
def handler(ctx):
    return show("nonesuch-revision")

meow.command(meow.tool(name = "probe", about = "bad revision", run = handler))
"#
            ),
        )],
        Vec::new(),
    );
    make_repo(h.runtime.workspace().root());

    let runtime = Arc::clone(&h.runtime);
    let error = tokio::task::spawn_blocking(move || runtime.run_command("probe", &Map::new()))
        .await
        .unwrap()
        .unwrap_err()
        .to_string();

    assert!(
        error.contains("git --no-pager show nonesuch-revision"),
        "{error}"
    );
    assert!(error.contains("exited"), "{error}");
}

/// An empty commit message is refused before `git` is reached.
#[tokio::test(flavor = "multi_thread")]
async fn git_refuses_an_empty_commit_message() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{GIT_WORKSPACE}
def handler(ctx):
    return commit("   ")

meow.command(meow.tool(name = "probe", about = "empty message", run = handler))
"#
            ),
        )],
        Vec::new(),
    );
    make_repo(h.runtime.workspace().root());

    let runtime = Arc::clone(&h.runtime);
    let error = tokio::task::spawn_blocking(move || runtime.run_command("probe", &Map::new()))
        .await
        .unwrap()
        .unwrap_err()
        .to_string();

    assert!(error.contains("needs a message"), "{error}");
}

/// [R-STAR-012] `re` and `time` are in the table and resolve like any module
#[tokio::test(flavor = "multi_thread")]
async fn re_and_time_resolve_from_the_table() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//re", "match")
load("@std//time", "now")

def handler(ctx):
    ctx.out.write("%s %s" % (type(match), type(now)))
    return ""

meow.command(meow.tool(name = "probe", about = "load both", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        ["write: function function"],
        "`re` and `time` did not resolve to callables"
    );
}

/// [R-STAR-013] a match is a list of groups, and a group that did not
/// participate is `None`
#[tokio::test(flavor = "multi_thread")]
async fn match_returns_groups_and_none_for_the_ones_that_did_not_take_part() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//re", "match")

def handler(ctx):
    ctx.out.write(str(match(r"(a)(b)?(c)", "ac")))
    ctx.out.write(str(match(r"(a)(b)?(c)", "abc")))
    return ""

meow.command(meow.tool(name = "probe", about = "match", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        [
            r#"write: ["ac", "a", None, "c"]"#,
            r#"write: ["abc", "a", "b", "c"]"#,
        ],
        "group 0 must be the whole match and an absent group must be None"
    );
}

/// [R-STAR-013] no match is `None`, which is how it is told from matching empty
#[tokio::test(flavor = "multi_thread")]
async fn no_match_is_none_and_an_empty_match_is_a_list() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//re", "match")

def handler(ctx):
    ctx.out.write(str(match(r"z+", "aaa")))
    ctx.out.write(str(match(r"z*", "aaa")))
    return ""

meow.command(meow.tool(name = "probe", about = "match", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        ["write: None", r#"write: [""]"#],
        "a subject that did not match must be told from one that matched empty"
    );
}

/// [R-STAR-013] `find_all`, `replace`, and `split` over the same pattern
#[tokio::test(flavor = "multi_thread")]
async fn find_all_replace_and_split_agree_about_what_matched() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//re", "find_all", "replace", "split")

def handler(ctx):
    ctx.out.write(str([m[1] for m in find_all(r"(\w+)@example.com", "a@example.com b@example.com")]))
    ctx.out.write(replace(r"(\w+)@example.com", "a@example.com b@example.com", "$1"))
    ctx.out.write(replace(r"\s+", "a  b   c", "-", first_only = True))
    ctx.out.write(str(split(r",\s*", "one, two,three")))
    return ""

meow.command(meow.tool(name = "probe", about = "the rest", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        [
            r#"write: ["a", "b"]"#,
            "write: a b",
            "write: a-b   c",
            r#"write: ["one", "two", "three"]"#,
        ],
        "find_all, replace, and split did not agree about the same pattern"
    );
}

/// [R-STAR-013] `limit` bounds `find_all`, which a pattern matching empty needs
#[tokio::test(flavor = "multi_thread")]
async fn find_all_stops_at_the_limit_it_was_given() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//re", "find_all")

def handler(ctx):
    ctx.out.write(str(len(find_all(r"a", "aaaaa"))))
    ctx.out.write(str(len(find_all(r"a", "aaaaa", limit = 2))))
    return ""

meow.command(meow.tool(name = "probe", about = "limit", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        ["write: 5", "write: 2"],
        "limit did not bound the number of matches"
    );
}

/// [R-STAR-013] a pattern that will not compile fails at the call, and says why
#[tokio::test(flavor = "multi_thread")]
async fn a_pattern_that_does_not_compile_names_itself_and_the_reason() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//re", "match")

def handler(ctx):
    return str(match(r"(unclosed", "anything"))

meow.command(meow.tool(name = "probe", about = "bad pattern", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    let error = fails(&h.runtime, "probe").await;

    assert!(
        error.contains("(unclosed") && error.contains("not a regular expression"),
        "the error must name the pattern and say what was wrong with it: {error}"
    );
}

/// [R-STAR-014] `time` round-trips an instant through text without moving it
#[tokio::test(flavor = "multi_thread")]
async fn parse_and_format_round_trip_in_utc() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//time", "parse", "format")

def handler(ctx):
    at = parse("2026-09-20T09:02:46Z")
    ctx.out.write(str(at))
    ctx.out.write(format(at))
    ctx.out.write(format(at, layout = "%Y-%m-%d"))
    return ""

meow.command(meow.tool(name = "probe", about = "time", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        [
            "write: 1789894966",
            "write: 2026-09-20T09:02:46Z",
            "write: 2026-09-20",
        ],
        "an instant must survive the trip through text unmoved, in UTC"
    );
}

/// [R-STAR-014] `since` measures forwards and backwards from an instant
#[tokio::test(flavor = "multi_thread")]
async fn since_is_negative_for_an_instant_that_has_not_happened() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//time", "now", "since")

def handler(ctx):
    at = now()
    ctx.out.write(str(since(at - 60) >= 60))
    ctx.out.write(str(since(at + 3600) <= -3599))
    return ""

meow.command(meow.tool(name = "probe", about = "since", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        ["write: True", "write: True"],
        "an instant in the future must give a negative duration, not an error"
    );
}

/// [R-STAR-014] text that is not a timestamp fails at the call, and says why
#[tokio::test(flavor = "multi_thread")]
async fn text_that_is_not_a_timestamp_names_itself() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//time", "parse")

def handler(ctx):
    return str(parse("last tuesday"))

meow.command(meow.tool(name = "probe", about = "bad time", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    let error = fails(&h.runtime, "probe").await;

    assert!(
        error.contains("last tuesday") && error.contains("RFC 3339"),
        "the error must name the text and the format it wanted: {error}"
    );
}

/// [R-STAR-084] neither module may be called while `.meow/` is being evaluated
#[tokio::test(flavor = "multi_thread")]
async fn re_and_time_are_refused_during_declaration() {
    for (module, call) in [("re", r#"match("a", "a")"#), ("time", "now()")] {
        let name = if module == "re" { "match" } else { "now" };
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join(".meow").join("meow.star");
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(
            path,
            format!(
                r#"load("@std//{module}", "{name}")
{call}
"#
            ),
        )
        .unwrap();

        let error = load(&Workspace::at(dir.path())).unwrap_err().to_string();
        assert!(
            error.contains(module),
            "calling `{module}` during declaration must be refused by name: {error}"
        );
    }
}

/// [R-STAR-015] the four encoders are in the table and resolve
#[tokio::test(flavor = "multi_thread")]
async fn the_encoders_resolve_from_the_table() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//yaml", yaml_parse = "parse")
load("@std//toml", toml_parse = "parse")
load("@std//csv", csv_parse = "parse")
load("@std//xml", xml_parse = "parse")

def handler(ctx):
    names = [type(f) for f in [yaml_parse, toml_parse, csv_parse, xml_parse]]
    ctx.out.write(" ".join(names))
    return ""

meow.command(meow.tool(name = "probe", about = "load them", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        ["write: function function function function"],
        "the four encoders did not all resolve"
    );
}

/// [R-STAR-016] the same data through three formats is the same value
#[tokio::test(flavor = "multi_thread")]
async fn yaml_toml_and_json_agree_about_the_same_data() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//yaml", yaml_parse = "parse")
load("@std//toml", toml_parse = "parse")
load("@std//json", json_parse = "parse", json_encode = "encode")

def handler(ctx):
    a = yaml_parse("name: meow\ncount: 3\ntags:\n  - one\n  - two\n")
    b = toml_parse('name = "meow"\ncount = 3\ntags = ["one", "two"]\n')
    c = json_parse('{{"name": "meow", "count": 3, "tags": ["one", "two"]}}')
    ctx.out.write(str(a == b))
    ctx.out.write(str(b == c))
    ctx.out.write(json_encode(a))
    return ""

meow.command(meow.tool(name = "probe", about = "three formats", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        [
            "write: True",
            "write: True",
            r#"write: {"count":3,"name":"meow","tags":["one","two"]}"#,
        ],
        "three formats describing the same data must produce one value"
    );
}

/// [R-STAR-016] a value read as one format encodes as another
#[tokio::test(flavor = "multi_thread")]
async fn a_document_read_as_yaml_writes_as_toml() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//yaml", yaml_parse = "parse")
load("@std//toml", toml_encode = "encode")

def handler(ctx):
    ctx.out.write(toml_encode(yaml_parse("name: meow\ncount: 3\n")))
    return ""

meow.command(meow.tool(name = "probe", about = "cross", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        ["write: count = 3\nname = \"meow\"\n"],
        "a value read as YAML must be encodable as TOML"
    );
}

/// [R-STAR-015] TOML's top level is a table, and anything else is refused
#[tokio::test(flavor = "multi_thread")]
async fn toml_refuses_a_top_level_that_is_not_a_table() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//toml", "encode")

def handler(ctx):
    return encode([1, 2, 3])

meow.command(meow.tool(name = "probe", about = "bad toml", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    let error = fails(&h.runtime, "probe").await;

    assert!(
        error.contains("table") && error.contains("dict"),
        "the error must say TOML's top level is a table: {error}"
    );
}

/// [R-STAR-015] text the format rejects fails at the call, with the reason
#[tokio::test(flavor = "multi_thread")]
async fn text_the_format_rejects_fails_rather_than_returning_a_partial_value() {
    for (module, bad) in [
        ("yaml", "a:\n  - b\n c: broken\n"),
        ("toml", "this is not = = toml"),
        ("xml", "<open>text"),
    ] {
        let h = harness(
            &[(
                "meow.star",
                &format!(
                    r#"{MODELS}
load("@std//{module}", "parse")

def handler(ctx):
    return str(parse({bad:?}))

meow.command(meow.tool(name = "probe", about = "bad input", run = handler))
"#
                ),
            )],
            Vec::new(),
        );

        let error = fails(&h.runtime, "probe").await;
        assert!(
            error.to_lowercase().contains(module),
            "`{module}` must say what format it wanted: {error}"
        );
    }
}

/// [R-STAR-017] a header makes rows dicts, and no header makes them lists
#[tokio::test(flavor = "multi_thread")]
async fn csv_shape_follows_whether_the_first_record_names_the_columns() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//csv", "parse")

def handler(ctx):
    ctx.out.write(str(parse("name,count\nmeow,3\n")))
    ctx.out.write(str(parse("meow,3\n", header = False)))
    return ""

meow.command(meow.tool(name = "probe", about = "csv", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        [
            r#"write: [{"name": "meow", "count": "3"}]"#,
            r#"write: [["meow", "3"]]"#,
        ],
        "the shape must follow the header"
    );
}

/// [R-STAR-017] a record that disagrees with the header names its own number
#[tokio::test(flavor = "multi_thread")]
async fn a_short_csv_record_names_which_record_it_was() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//csv", "parse")

def handler(ctx):
    return str(parse("a,b,c\n1,2,3\n4,5\n"))

meow.command(meow.tool(name = "probe", about = "ragged", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    let error = fails(&h.runtime, "probe").await;

    assert!(
        error.contains("record 2") && error.contains('3'),
        "the error must name the record and both widths: {error}"
    );
}

/// [R-STAR-017] `encode` takes either shape back
#[tokio::test(flavor = "multi_thread")]
async fn csv_encode_round_trips_both_shapes() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//csv", "parse", "encode")

def handler(ctx):
    ctx.out.write(encode(parse("name,count\nmeow,3\n")))
    ctx.out.write(encode([["meow", "3"]]))
    return ""

meow.command(meow.tool(name = "probe", about = "round trip", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        ["write: name,count\nmeow,3\n", "write: meow,3\n"],
        "both shapes must survive the round trip"
    );
}

/// [R-STAR-018] an element is a tag, attributes, ordered children, and text
#[tokio::test(flavor = "multi_thread")]
async fn xml_parse_keeps_what_a_dictionary_would_lose() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//xml", "parse")

def handler(ctx):
    root = parse('<feed kind="atom"><item>one</item><item>two</item></feed>')
    ctx.out.write(root.tag)
    ctx.out.write(str(root.attrs))
    ctx.out.write(",".join([c.text for c in root.children]))
    ctx.out.write(str(len(root.children)))
    return ""

meow.command(meow.tool(name = "probe", about = "xml", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        [
            "write: feed",
            r#"write: {"kind": "atom"}"#,
            "write: one,two",
            "write: 2",
        ],
        "two children sharing a tag must both survive, in order"
    );
}

/// [R-STAR-018] `encode` escapes, so text cannot close a tag nobody opened
#[tokio::test(flavor = "multi_thread")]
async fn xml_encode_escapes_text_and_attributes() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//xml", "encode")

def handler(ctx):
    el = {{
        "tag": "note",
        "attrs": {{"by": 'a "quoted" name'}},
        "children": [],
        "text": "</note><script>",
    }}
    ctx.out.write(encode(el))
    return ""

meow.command(meow.tool(name = "probe", about = "escape", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        [concat!(
            r#"write: <note by="a &quot;quoted&quot; name">"#,
            "&lt;/note&gt;&lt;script&gt;</note>"
        )],
        "text and attribute values must be escaped"
    );
}

/// [R-STAR-018] a tree survives the round trip
#[tokio::test(flavor = "multi_thread")]
async fn xml_round_trips_a_tree() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//xml", "parse", "encode")

def handler(ctx):
    doc = '<feed kind="atom"><item id="1">one</item><empty/></feed>'
    ctx.out.write(encode(parse(doc)))
    return ""

meow.command(meow.tool(name = "probe", about = "round trip", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        [r#"write: <feed kind="atom"><item id="1">one</item><empty/></feed>"#],
        "a tree must come back as the document it was read from"
    );
}

/// [R-STAR-084] no encoder may be called while `.meow/` is being evaluated
#[tokio::test(flavor = "multi_thread")]
async fn the_encoders_are_refused_during_declaration() {
    for module in ["yaml", "toml", "csv", "xml"] {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join(".meow").join("meow.star");
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(
            path,
            format!(
                r#"load("@std//{module}", "parse")
parse("")
"#
            ),
        )
        .unwrap();

        let error = load(&Workspace::at(dir.path())).unwrap_err().to_string();
        assert!(
            error.contains(module),
            "calling `{module}` during declaration must be refused by name: {error}"
        );
    }
}

/// [R-STAR-027] what a handler put in is what it gets back, of the same type
#[tokio::test(flavor = "multi_thread")]
async fn a_value_survives_the_store_unchanged() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//store", "get", "put")

def handler(ctx):
    for key, value in [
        ("n", 42),
        ("s", "meow"),
        ("b", True),
        ("l", [1, "two", None]),
        ("d", {{"nested": {{"deep": [1, 2]}}}}),
        ("none", None),
    ]:
        put(key, value)
        back = get(key)
        ctx.out.write("%s %s %s" % (key, back == value, type(back)))
    return ""

meow.command(meow.tool(name = "probe", about = "store", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        [
            "write: n True int",
            "write: s True string",
            "write: b True bool",
            "write: l True list",
            "write: d True dict",
            "write: none True NoneType",
        ],
        "every value a handler can build must survive the round trip as itself"
    );
}

/// [R-STAR-027] an absent key gives the caller's default, and `None` for none
#[tokio::test(flavor = "multi_thread")]
async fn an_absent_key_gives_the_default_the_caller_named() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//store", "get", "put")

def handler(ctx):
    ctx.out.write(str(get("never-written")))
    ctx.out.write(str(get("never-written", default = [])))
    put("written-as-none", None)
    # Stored `None` and absent are the same answer only when the caller asked
    # for `None`; with a default they are different.
    ctx.out.write(str(get("written-as-none", default = "fallback")))
    return ""

meow.command(meow.tool(name = "probe", about = "defaults", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        ["write: None", "write: []", "write: None"],
        "a default must be returned for an absent key and not for a stored None"
    );
}

/// [R-STAR-028] `delete` says whether the key was there, and never fails
#[tokio::test(flavor = "multi_thread")]
async fn delete_says_whether_the_key_was_there() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//store", "put", "delete", "get")

def handler(ctx):
    put("here", 1)
    ctx.out.write(str(delete("here")))
    ctx.out.write(str(delete("here")))
    ctx.out.write(str(delete("never-was")))
    ctx.out.write(str(get("here")))
    return ""

meow.command(meow.tool(name = "probe", about = "delete", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        ["write: True", "write: False", "write: False", "write: None"],
        "deleting a key that was never written must be false rather than an error"
    );
}

/// [R-STAR-028] `keys` is sorted, and takes a prefix
#[tokio::test(flavor = "multi_thread")]
async fn keys_are_sorted_and_a_prefix_narrows_them() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//store", "put", "keys")

def handler(ctx):
    for key in ["review:b", "review:a", "plan:1"]:
        put(key, True)
    ctx.out.write(",".join(keys()))
    ctx.out.write(",".join(keys(prefix = "review:")))
    ctx.out.write(str(keys(prefix = "nothing:")))
    return ""

meow.command(meow.tool(name = "probe", about = "keys", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        [
            "write: plan:1,review:a,review:b",
            "write: review:a,review:b",
            "write: []",
        ],
        "keys must be sorted, and a prefix must narrow them"
    );
}

/// [R-STAR-084] the store may not be reached while `.meow/` is being evaluated
#[tokio::test(flavor = "multi_thread")]
async fn the_store_is_refused_during_declaration() {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join(".meow").join("meow.star");
    std::fs::create_dir_all(path.parent().unwrap()).unwrap();
    std::fs::write(
        path,
        r#"load("@std//store", "get")
get("anything")
"#,
    )
    .unwrap();

    let error = load(&Workspace::at(dir.path())).unwrap_err().to_string();
    assert!(
        error.contains("store"),
        "reading the store during declaration must be refused by name: {error}"
    );
}

/// A server on a loopback port, scripted with what to answer.
///
/// A real socket rather than a mocked client: `[R-STAR-023]` is a claim about
/// what comes back from a server, and a fake that returns a struct would be
/// asserting that this test builds the struct correctly.
struct Server {
    port: u16,
    seen: Arc<Mutex<Vec<String>>>,
    stop: Arc<std::sync::atomic::AtomicBool>,
}

impl Server {
    /// Answer every request with this status and body, recording each one.
    fn answering(status: u16, body: &'static str) -> Self {
        Self::scripted(move |_| (status, body.to_owned()))
    }

    /// Answer with whatever the request deserves.
    fn scripted(reply: impl Fn(&str) -> (u16, String) + Send + Sync + 'static) -> Self {
        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();
        let seen = Arc::new(Mutex::new(Vec::new()));
        let stop = Arc::new(std::sync::atomic::AtomicBool::new(false));

        let kept = Arc::clone(&seen);
        let halt = Arc::clone(&stop);
        std::thread::spawn(move || {
            use std::io::{Read, Write};
            for stream in listener.incoming() {
                if halt.load(std::sync::atomic::Ordering::Relaxed) {
                    return;
                }
                let Ok(mut stream) = stream else { continue };
                let mut buffer = [0_u8; 8192];
                let read = stream.read(&mut buffer).unwrap_or(0);
                let request = String::from_utf8_lossy(&buffer[..read]).into_owned();
                kept.lock().unwrap().push(request.clone());

                let (status, body) = reply(&request);
                let response = format!(
                    "HTTP/1.1 {status} X\r\nContent-Length: {}\r\nX-Answered-By: \
                     test\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                );
                let _ = stream.write_all(response.as_bytes());
                let _ = stream.flush();
            }
        });

        Self { port, seen, stop }
    }

    fn url(&self, path: &str) -> String {
        format!("http://127.0.0.1:{}{path}", self.port)
    }

    fn requests(&self) -> Vec<String> {
        self.seen.lock().unwrap().clone()
    }
}

impl Drop for Server {
    fn drop(&mut self) {
        self.stop.store(true, std::sync::atomic::Ordering::Relaxed);
        // Unblock the accept loop so the thread can see the flag and leave.
        let _ = std::net::TcpStream::connect(("127.0.0.1", self.port));
    }
}

/// [R-STAR-022] a response carries the status, the headers, and the body
#[tokio::test(flavor = "multi_thread")]
async fn a_response_carries_what_the_server_sent() {
    let server = Server::answering(200, r#"{"ok": true}"#);
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//http", "get")

def handler(ctx):
    r = get("{}")
    ctx.out.write("%d %s %s" % (r.status, r.ok, r.body))
    ctx.out.write(r.headers["x-answered-by"])
    return ""

meow.command(meow.tool(name = "probe", about = "get", run = handler))
"#,
                server.url("/thing")
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        [r#"write: 200 True {"ok": true}"#, "write: test"],
        "the status, the flag, the body, and the headers must all arrive"
    );
}

/// [R-STAR-023] a status the server chose is an answer, not a failure
#[tokio::test(flavor = "multi_thread")]
async fn a_404_is_a_response_rather_than_an_error() {
    let server = Server::answering(404, "nothing here");
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//http", "get")

def handler(ctx):
    r = get("{}")
    ctx.out.write("%d %s %s" % (r.status, r.ok, r.body))
    return ""

meow.command(meow.tool(name = "probe", about = "404", run = handler))
"#,
                server.url("/missing")
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        ["write: 404 False nothing here"],
        "a 404 must be a response a handler can branch on"
    );
}

/// [R-STAR-023] a request that never reached a response does fail
#[tokio::test(flavor = "multi_thread")]
async fn a_connection_that_is_refused_fails() {
    // Bound and dropped, so nothing is listening and the port is not in use
    // by something else that would answer.
    let port = {
        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        listener.local_addr().unwrap().port()
    };

    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//http", "get")

def handler(ctx):
    return get("http://127.0.0.1:{port}/").body

meow.command(meow.tool(name = "probe", about = "refused", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    let error = fails(&h.runtime, "probe").await;
    assert!(
        error.contains("could not reach"),
        "a refused connection must fail saying it could not reach the server: {error}"
    );
}

/// [R-STAR-022] a dict body goes as JSON, and a string goes as it is
#[tokio::test(flavor = "multi_thread")]
async fn a_dict_body_is_json_and_a_string_body_is_itself() {
    let server = Server::answering(201, "made");
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//http", "post", "put")

def handler(ctx):
    post("{url}", {{"name": "meow"}})
    put("{url}", "plain text", headers = {{"content-type": "text/plain"}})
    return ""

meow.command(meow.tool(name = "probe", about = "bodies", run = handler))
"#,
                url = server.url("/thing")
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    let seen = server.requests();
    assert!(
        seen[0].contains("POST /thing")
            && seen[0]
                .to_lowercase()
                .contains("content-type: application/json")
            && seen[0].contains(r#"{"name":"meow"}"#),
        "a dict must be sent as JSON with a content type: {}",
        seen[0]
    );
    assert!(
        seen[1].contains("PUT /thing")
            && seen[1].to_lowercase().contains("content-type: text/plain")
            && seen[1].ends_with("plain text"),
        "a string must be sent as it is, and the caller's content type kept: {}",
        seen[1]
    );
}

/// [R-STAR-022] `max_bytes` cuts the body rather than failing
#[tokio::test(flavor = "multi_thread")]
async fn max_bytes_cuts_a_body_that_is_too_long() {
    let server = Server::answering(200, "0123456789");
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//http", "get")

def handler(ctx):
    ctx.out.write(get("{}", max_bytes = 4).body)
    return ""

meow.command(meow.tool(name = "probe", about = "cap", run = handler))
"#,
                server.url("/long")
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        ["write: 0123"],
        "a handler that asked for four bytes must get four bytes"
    );
}

/// [R-STAR-022] a scheme this module cannot speak is refused before the call
#[tokio::test(flavor = "multi_thread")]
async fn a_scheme_that_is_not_http_is_refused() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//http", "get")

def handler(ctx):
    return get("file:///etc/hosts").body

meow.command(meow.tool(name = "probe", about = "scheme", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    let error = fails(&h.runtime, "probe").await;
    assert!(
        error.contains("file:///etc/hosts") && error.contains("no other scheme"),
        "a `file://` URL must be refused by name: {error}"
    );
}

/// [R-STAR-084] the network may not be reached while `.meow/` is evaluated
#[tokio::test(flavor = "multi_thread")]
async fn http_is_refused_during_declaration() {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join(".meow").join("meow.star");
    std::fs::create_dir_all(path.parent().unwrap()).unwrap();
    std::fs::write(
        path,
        r#"load("@std//http", "get")
get("http://example.com/")
"#,
    )
    .unwrap();

    let error = load(&Workspace::at(dir.path())).unwrap_err().to_string();
    assert!(
        error.contains("http"),
        "reaching the network during declaration must be refused by name: {error}"
    );
}

/// [R-STAR-019] `search.text` reports the path and the line of every hit
#[tokio::test(flavor = "multi_thread")]
async fn search_text_reports_where_each_hit_was() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//search", "text")

def handler(ctx):
    for hit in text("TODO"):
        ctx.out.write("%s:%d %s" % (hit.path, hit.first_line, hit.text))
    return ""

meow.command(meow.tool(name = "probe", about = "text", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        ["write: README.md:3 a line that matched"],
        "a hit must carry its path and its line number"
    );
    assert_eq!(
        h.index.calls(),
        ["text TODO regex=false 50 []"],
        "the default must be a literal, not a regular expression"
    );
}

/// [R-STAR-019] the same call takes a regular expression when asked
#[tokio::test(flavor = "multi_thread")]
async fn search_text_takes_a_regular_expression_when_asked() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//search", "text")

def handler(ctx):
    text(r"TODO\(\w+\)", regex = True, limit = 5, paths = ["src"])
    return ""

meow.command(meow.tool(name = "probe", about = "regex", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.index.calls(),
        [r#"text TODO\(\w+\) regex=true 5 ["src"]"#],
        "every argument must reach the port as written"
    );
}

/// [R-STAR-019] `search.files` answers with paths and nothing else
#[tokio::test(flavor = "multi_thread")]
async fn search_files_answers_with_paths() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//search", "files")

def handler(ctx):
    ctx.out.write(",".join(files("src/**/*.rs")))
    return ""

meow.command(meow.tool(name = "probe", about = "files", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        ["write: src/lib.rs,src/main.rs"],
        "`files` must return paths"
    );
    assert_eq!(h.index.calls(), ["files src/**/*.rs 500"]);
}

/// [R-STAR-024] `build` and `update` report what changed as counts
#[tokio::test(flavor = "multi_thread")]
async fn build_and_update_report_counts_rather_than_a_sentence() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//index", "build", "update")

def handler(ctx):
    a = update()
    ctx.out.write("%d %d %d %d" % (a.files, a.added, a.changed, a.embedded))
    b = build()
    ctx.out.write("%d %d" % (b.added, b.embedded))
    return ""

meow.command(meow.tool(name = "probe", about = "index", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        ["write: 7 2 1 0", "write: 2 3"],
        "the counts must arrive as numbers a handler can compare"
    );
    assert_eq!(
        h.index.calls(),
        ["update", "build"],
        "`update` must not embed, and `build` must"
    );
}

/// [R-STAR-024] `stats` says how much is indexed and by which model
#[tokio::test(flavor = "multi_thread")]
async fn stats_says_how_much_is_indexed_and_by_what() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//index", "stats")

def handler(ctx):
    s = stats()
    ctx.out.write("%d/%d by %s" % (s.embedded, s.chunks, s.model))
    return ""

meow.command(meow.tool(name = "probe", about = "stats", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(h.out.lines(), ["write: 39/40 by embed"]);
}

/// [R-STAR-024] `query` carries the floor, and `search.code` is it at zero
#[tokio::test(flavor = "multi_thread")]
async fn query_carries_the_floor_and_code_is_the_same_call_without_one() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//index", "query")
load("@std//search", "code")

def handler(ctx):
    hit = query("what does the loader do", limit = 3, min_score = 0.4)[0]
    ctx.out.write("%s %s" % (hit.path, hit.score))
    query("an integer floor is fine too", min_score = 0)
    code("the same question", limit = 3)
    return ""

meow.command(meow.tool(name = "probe", about = "query", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        ["write: src/lib.rs 0.75"],
        "a hit must come back in the one shape every search returns"
    );
    assert_eq!(
        h.index.calls(),
        [
            "query what does the loader do 3 [] 0.4",
            "query an integer floor is fine too 10 [] 0",
            "query the same question 3 [] 0",
        ],
        "`code` must be `query` with a floor of zero, and an integer floor must work"
    );
}

/// [R-STAR-025] with no index, every call says so rather than answering
#[tokio::test(flavor = "multi_thread")]
async fn no_index_is_told_apart_from_no_results() {
    for call in ["build()", "update()", "stats()", r#"query("anything")"#] {
        let name = call.split('(').next().unwrap();
        let h = harness_with(
            &[(
                "meow.star",
                &format!(
                    r#"{MODELS}
load("@std//index", "{name}")

def handler(ctx):
    return str({call})

meow.command(meow.tool(name = "probe", about = "no index", run = handler))
"#
                ),
            )],
            Vec::new(),
            false,
        );

        let error = fails(&h.runtime, "probe").await;
        assert!(
            error.contains("meow index build"),
            "`{name}` must say there is no index rather than answering: {error}"
        );
    }
}

/// [R-STAR-084] neither module may be called while `.meow/` is being evaluated
#[tokio::test(flavor = "multi_thread")]
async fn search_and_index_are_refused_during_declaration() {
    for (module, name, call) in [
        ("index", "stats", "stats()"),
        ("search", "files", r#"files("*")"#),
    ] {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join(".meow").join("meow.star");
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(
            path,
            format!(
                r#"load("@std//{module}", "{name}")
{call}
"#
            ),
        )
        .unwrap();

        let error = load(&Workspace::at(dir.path())).unwrap_err().to_string();
        assert!(
            error.contains(module),
            "calling `{module}` during declaration must be refused by name: {error}"
        );
    }
}

/// [R-STAR-029] `path` speaks one separator, and it is `/`
///
/// The native separator is the obvious choice and the wrong one:
/// `search.files` and `fs.glob` report `/`, so a handler that built a path
/// natively and compared it against a reported one matched here and failed on
/// Windows, with nothing saying why.
#[tokio::test(flavor = "multi_thread")]
async fn paths_are_written_with_one_separator() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//path", "join", "dir", "base", "ext", "rel", "abs")

def handler(ctx):
    ctx.out.write(join("a", "b", "c.rs"))
    ctx.out.write(dir("a/b/c.rs"))
    ctx.out.write(base("a/b/c.rs"))
    ctx.out.write(ext("a/b/c.rs"))
    ctx.out.write(rel("a/b/c.rs", "a"))
    ctx.out.write("abs %s" % ("/" in abs("a/b") and "\\" not in abs("a/b")))
    return ""

meow.command(meow.tool(name = "probe", about = "paths", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    assert_eq!(
        h.out.lines(),
        [
            "write: a/b/c.rs",
            "write: a/b",
            "write: c.rs",
            "write: rs",
            "write: b/c.rs",
            "write: abs True",
        ],
        "a path must come back with forward slashes on every platform"
    );
}

/// [R-STAR-029] a path given with the native separator is understood
#[tokio::test(flavor = "multi_thread")]
async fn a_path_given_with_a_backslash_is_understood() {
    let h = harness(
        &[(
            "meow.star",
            &format!(
                r#"{MODELS}
load("@std//path", "dir", "base", "join")

def handler(ctx):
    # What a handler gets back from a program that speaks the platform.
    ctx.out.write(base("a\\b\\c.rs"))
    ctx.out.write(join("a\\b", "c.rs"))
    return ""

meow.command(meow.tool(name = "probe", about = "backslash", run = handler))
"#
            ),
        )],
        Vec::new(),
    );

    call(&h.runtime, "probe", args(&[])).await;

    // On Unix a backslash is an ordinary character in a file name, so nothing
    // is rewritten and a file really called `a\b\c.rs` keeps its name. On
    // Windows it is a separator and the path is read as three components.
    let expected: Vec<String> = if std::path::MAIN_SEPARATOR == '/' {
        vec![r"write: a\b\c.rs".to_owned(), r"write: a\b/c.rs".to_owned()]
    } else {
        vec!["write: c.rs".to_owned(), "write: a/b/c.rs".to_owned()]
    };

    assert_eq!(
        h.out.lines(),
        expected,
        "a native separator must be understood where it is one"
    );
}
