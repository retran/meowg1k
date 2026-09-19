// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Policy at the tool boundary: who is asked, and what a dry run does.
#![allow(clippy::unwrap_used)]

use std::sync::{Arc, Mutex};

use meow_agent::{Approver, Engine};
use meow_core::Usage;
use meow_core::view::{LiveKind, Output, ViewEvent};
use meow_llm::{Capabilities, LlmError, Provider, Request, Response, Structured, ToolCall};
use meow_policy::{Answer, Prompt};
use meow_star::port::{Ask, AskError, Events, Session, Stdin};
use meow_star::{Ports, Runtime, Workspace, load};
use serde_json::{Map, json};
use tempfile::TempDir;
use tokio_util::sync::CancellationToken;

/// Keeps what a handler and the engine said.
#[derive(Debug, Default)]
struct Recorder(Mutex<Vec<String>>);

impl Recorder {
    fn lines(&self) -> Vec<String> {
        self.0.lock().unwrap().clone()
    }
}

impl Events for Recorder {
    fn event(&self, event: ViewEvent) {
        let text = match event {
            ViewEvent::Live(LiveKind::Output(Output::Note { text })) => format!("note: {text}"),
            ViewEvent::Live(LiveKind::Output(Output::Warn { text })) => format!("warn: {text}"),
            ViewEvent::Live(LiveKind::Output(Output::Write { text })) => format!("write: {text}"),
            ViewEvent::Logged(meow_core::EventKind::Policy { decision, .. }) => {
                format!("policy: {decision}")
            }
            _ => return,
        };
        self.0.lock().unwrap().push(text);
    }
}

/// Answers every approval the same way, and remembers what it was asked.
#[derive(Debug)]
struct Scripted {
    answer: Answer,
    asked: Mutex<Vec<Prompt>>,
}

impl Scripted {
    fn new(answer: Answer) -> Self {
        Self {
            answer,
            asked: Mutex::new(Vec::new()),
        }
    }
}

impl Approver for Scripted {
    fn ask(&self, prompt: &Prompt) -> Answer {
        self.asked.lock().unwrap().push(prompt.clone());
        self.answer
    }
}

#[derive(Debug)]
struct Nobody;

impl Ask for Nobody {
    fn text(&self, _prompt: &str, _default: Option<&str>) -> Result<String, AskError> {
        Err(AskError::Declined)
    }
    fn confirm(&self, _prompt: &str, _default: bool) -> Result<bool, AskError> {
        Err(AskError::Declined)
    }
    fn select(&self, _prompt: &str, _choices: &[String]) -> Result<String, AskError> {
        Err(AskError::Declined)
    }
}

#[derive(Debug)]
struct Closed;

impl Stdin for Closed {
    fn is_piped(&self) -> bool {
        false
    }
    fn read(&self) -> std::io::Result<String> {
        Ok(String::new())
    }
}

/// A model that asks for one tool and then answers.
#[derive(Debug)]
struct Caller(Mutex<Vec<Response>>);

fn said(text: &str) -> Response {
    Response {
        text: text.to_owned(),
        thinking: None,
        tool_calls: Vec::new(),
        usage: Some(Usage {
            prompt: 1,
            completion: 1,
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
            id: "c1".to_owned(),
            name: name.to_owned(),
            arguments: arguments.to_owned(),
        }],
        usage: None,
        value: None,
    }
}

#[async_trait::async_trait]
impl Provider for Caller {
    fn name(&self) -> &str {
        "caller"
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
        _request: &Request,
        _cancel: &CancellationToken,
    ) -> Result<Response, LlmError> {
        let mut turns = self.0.lock().unwrap();
        if turns.is_empty() {
            return Ok(said("done"));
        }
        Ok(turns.remove(0))
    }
    async fn embed(
        &self,
        _texts: &[String],
        _cancel: &CancellationToken,
    ) -> Result<Vec<Vec<f32>>, LlmError> {
        Err(LlmError::Unsupported {
            provider: "caller".to_owned(),
            capability: "embeddings",
        })
    }
}

const SOURCE: &str = r#"
meow.provider(name = "p", kind = "anthropic")
meow.model(name = "fast", provider = "p", id = "m", context = 100000, max_output = 4096)

meow.policy(rules = [
    {"tools": ["touch"], "decision": "ask", "paths": ["**"]},
])

def touch(ctx):
    ctx.out.write("touched " + ctx.args.path)
    return "done"

touch_tool = meow.tool(
    name = "touch",
    about = "touch a file",
    run = touch,
    args = {"path": meow.arg.string()},
)

meow.command(touch_tool)
meow.command(meow.agent(
    name = "helper",
    model = "fast",
    system = "use the tool",
    tools = [touch_tool],
))
"#;

struct Harness {
    _dir: TempDir,
    runtime: Arc<Runtime>,
    out: Arc<Recorder>,
}

fn harness(approve: Option<Arc<dyn Approver>>, dry_run: bool, turns: Vec<Response>) -> Harness {
    let dir = tempfile::tempdir().unwrap();
    let config = dir.path().join(".meow");
    std::fs::create_dir_all(&config).unwrap();
    std::fs::write(config.join("meow.star"), SOURCE).unwrap();

    let workspace = Workspace::at(dir.path());
    let loaded = load(&workspace).unwrap();
    let out = Arc::new(Recorder::default());

    let engines = std::iter::once((
        "p".to_owned(),
        Arc::new(Engine::new(
            Arc::new(Caller(Mutex::new(turns))) as Arc<dyn Provider>
        )),
    ))
    .collect();

    let runtime = Arc::new(Runtime::new(
        loaded,
        workspace,
        engines,
        tokio::runtime::Handle::current(),
        Ports {
            events: Arc::clone(&out) as Arc<dyn Events>,
            approve,
            dry_run,
            search: Arc::new(meow_star::port::quiet::NoIndex),
            ask: Arc::new(Nobody),
            stdin: Arc::new(Closed),
            session: Arc::new(meow_star::port::quiet::Memory::new("s-1")) as Arc<dyn Session>,
        },
        CancellationToken::new(),
    ));

    Harness {
        _dir: dir,
        runtime,
        out,
    }
}

async fn run(h: &Harness, command: &str, args: Map<String, serde_json::Value>) -> String {
    let runtime = Arc::clone(&h.runtime);
    let command = command.to_owned();
    tokio::task::spawn_blocking(move || runtime.run_command(&command, &args))
        .await
        .unwrap()
        .unwrap()
}

/// [R-POLICY-020] [R-TUI-073] with nobody to ask, an `ask` is a denial
#[tokio::test(flavor = "multi_thread")]
async fn an_ask_with_nobody_to_ask_is_a_denial() {
    let h = harness(
        None,
        false,
        vec![calls("touch", r#"{"path":"a.txt"}"#), said("could not")],
    );

    let answer = run(&h, "helper", args(&[("task", json!("touch a.txt"))])).await;

    assert_eq!(answer, "could not");
    assert!(
        h.out.lines().contains(&"policy: ask".to_owned()),
        "{:?}",
        h.out.lines()
    );
    assert!(
        !h.out.lines().iter().any(|l| l.contains("touched")),
        "the tool ran without an answer: {:?}",
        h.out.lines()
    );
}

/// [R-TUI-061] the prompt carries the tool, the arguments, the rule, and who
/// asked
#[tokio::test(flavor = "multi_thread")]
async fn the_prompt_carries_everything_the_reader_needs() {
    let approver = Arc::new(Scripted::new(Answer::Once));
    let h = harness(
        Some(Arc::clone(&approver) as Arc<dyn Approver>),
        false,
        vec![calls("touch", r#"{"path":"a.txt"}"#), said("done")],
    );

    run(&h, "helper", args(&[("task", json!("touch a.txt"))])).await;

    let asked = approver.asked.lock().unwrap();
    assert_eq!(asked.len(), 1, "nobody was asked");

    let prompt = &asked[0];
    assert_eq!(prompt.tool, "touch");
    assert!(
        prompt.arguments.contains("a.txt"),
        "the arguments are not verbatim: {}",
        prompt.arguments
    );
    assert!(prompt.rule.contains("touch"), "{}", prompt.rule);
    assert_eq!(prompt.agent, "helper");
    assert_eq!(prompt.step, 1);
}

/// [R-TUI-061] "once" lets the call through
#[tokio::test(flavor = "multi_thread")]
async fn once_lets_the_call_through() {
    let approver = Arc::new(Scripted::new(Answer::Once));
    let h = harness(
        Some(approver as Arc<dyn Approver>),
        false,
        vec![calls("touch", r#"{"path":"a.txt"}"#), said("done")],
    );

    run(&h, "helper", args(&[("task", json!("touch a.txt"))])).await;

    assert!(
        h.out.lines().iter().any(|l| l == "write: touched a.txt"),
        "{:?}",
        h.out.lines()
    );
}

/// [R-TUI-061] "stop" ends the run rather than letting the model work around it
#[tokio::test(flavor = "multi_thread")]
async fn stop_ends_the_run() {
    let approver = Arc::new(Scripted::new(Answer::Stop));
    let h = harness(
        Some(approver as Arc<dyn Approver>),
        false,
        vec![
            calls("touch", r#"{"path":"a.txt"}"#),
            said("I will try something else"),
        ],
    );

    let runtime = Arc::clone(&h.runtime);
    let outcome =
        tokio::task::spawn_blocking(move || runtime.run_agent("helper", "touch a.txt", None, 0))
            .await
            .unwrap()
            .unwrap();

    assert_eq!(outcome.stop, meow_core::StopReason::Denied);
    assert!(
        !h.out.lines().iter().any(|l| l.contains("touched")),
        "{:?}",
        h.out.lines()
    );
}

/// [R-TUI-072] a dry run decides policy, plans the call, and makes none
#[tokio::test(flavor = "multi_thread")]
async fn a_dry_run_plans_without_running() {
    let approver = Arc::new(Scripted::new(Answer::Once));
    let h = harness(
        Some(approver as Arc<dyn Approver>),
        true,
        vec![calls("touch", r#"{"path":"a.txt"}"#), said("done")],
    );

    run(&h, "helper", args(&[("task", json!("touch a.txt"))])).await;
    let lines = h.out.lines();

    assert!(
        lines.contains(&"policy: ask".to_owned()),
        "policy did not decide: {lines:?}"
    );
    assert!(
        lines.iter().any(|l| l.starts_with("note: would run touch")),
        "the call was not recorded: {lines:?}"
    );
    assert!(
        !lines.iter().any(|l| l.contains("touched")),
        "the tool ran: {lines:?}"
    );
    assert!(
        lines
            .iter()
            .any(|l| l.starts_with("warn: this is a dry run") && l.contains("plausible run")),
        "the divergence was not stated: {lines:?}"
    );
}

/// [R-TUI-072] the divergence is stated once, not on every call
#[tokio::test(flavor = "multi_thread")]
async fn the_divergence_is_stated_once() {
    let approver = Arc::new(Scripted::new(Answer::Once));
    let h = harness(
        Some(approver as Arc<dyn Approver>),
        true,
        vec![
            calls("touch", r#"{"path":"a.txt"}"#),
            calls("touch", r#"{"path":"b.txt"}"#),
            said("done"),
        ],
    );

    run(&h, "helper", args(&[("task", json!("touch two files"))])).await;

    let warnings = h
        .out
        .lines()
        .into_iter()
        .filter(|l| l.starts_with("warn: this is a dry run"))
        .count();
    assert_eq!(warnings, 1);
}

fn args(pairs: &[(&str, serde_json::Value)]) -> Map<String, serde_json::Value> {
    pairs
        .iter()
        .map(|(k, v)| ((*k).to_owned(), v.clone()))
        .collect()
}
