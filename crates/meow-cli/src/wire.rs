// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Turning a parsed command line into a run.

use std::collections::HashMap;
use std::sync::Arc;

use clap::ArgMatches;
use meow_agent::Engine;
use meow_llm::{Anthropic, Gemini, Http, OpenAi, Provider, Voyage};
use meow_star::{Loaded, Ports, Registry, Runtime, Workspace};
use serde_json::{Map, Value};
use tokio_util::sync::CancellationToken;

use crate::ask;
use crate::exit::{self, Ending};
use crate::render::{self, Environment};

/// Run a command that needs no workspace, if this is one.
///
/// Returns `None` when the command does need one, so the caller can report a
/// missing or broken workspace once rather than in every arm.
pub fn without_workspace(matches: &ArgMatches) -> Option<Ending> {
    match matches.subcommand() {
        Some(("version", _)) => {
            println!("meow {}", env!("CARGO_PKG_VERSION"));
            Some(Ending::Passed)
        }
        Some(("completions", sub)) => Some(completions(sub)),
        Some(("init", _)) => Some(init()),
        _ => None,
    }
}

/// Run a command against a loaded workspace.
pub fn with_workspace(matches: &ArgMatches, workspace: Workspace, loaded: Loaded) -> Ending {
    let Some((name, sub)) = matches.subcommand() else {
        return Ending::Usage;
    };

    match name {
        "check" => check(&loaded.registry),
        "models" => models(&loaded.registry),
        "providers" => providers(&loaded.registry),
        "doctor" => doctor(&workspace, &loaded.registry),
        "policy" => policy(sub, &loaded.registry),
        "session" => session(sub, &workspace),
        "index" => index(sub, &workspace, &loaded.registry),
        "run" => match sub.get_one::<String>("name") {
            Some(name) => invoke(matches, name, &Map::new(), workspace, loaded),
            None => Ending::Usage,
        },
        other => {
            let args = arguments(&loaded.registry, other, sub);
            invoke(matches, other, &args, workspace, loaded)
        }
    }
}

/// `meow check`: load, and say so.
///
/// Loading already happened, and getting here means it worked: the whole
/// command is the difference between a workspace that loads and one that does
/// not, and the second never reaches this function.
fn check(registry: &Registry) -> Ending {
    // The one thing loading cannot catch: `meow-star` knows what a provider
    // declaration says and this binary knows which kinds it can talk to, and
    // only one of them can decide whether a kind exists.
    let unknown: Vec<&meow_star::Provider> = registry
        .providers()
        .filter(|p| !KINDS.contains(&p.kind.as_str()))
        .collect();

    if !unknown.is_empty() {
        for provider in &unknown {
            eprintln!(
                "`{}` names provider kind `{}`; the kinds are {}",
                provider.name,
                provider.kind,
                KINDS.join(", ")
            );
        }
        return Ending::Config;
    }

    println!(
        "ok: {} providers, {} models, {} commands",
        registry.providers().count(),
        registry.models().count(),
        registry.commands().count()
    );
    Ending::Passed
}

fn models(registry: &Registry) -> Ending {
    for model in registry.models() {
        println!(
            "{}\t{}\t{}\tcontext {}\tmax output {}",
            model.name, model.provider, model.id, model.context, model.max_output
        );
    }
    Ending::Passed
}

fn providers(registry: &Registry) -> Ending {
    for provider in registry.providers() {
        let key = if credential(provider).is_some() {
            "key present"
        } else {
            "no key"
        };
        println!("{}\t{}\t{key}", provider.name, provider.kind);
    }
    Ending::Passed
}

/// `meow doctor`: what is here and what is missing.
fn doctor(workspace: &Workspace, registry: &Registry) -> Ending {
    println!("workspace\t{}", workspace.root().display());
    println!("config\t{}", workspace.config_dir().display());

    let mut wrong = Vec::new();
    for provider in registry.providers() {
        if !KINDS.contains(&provider.kind.as_str()) {
            wrong.push(format!("{} ({})", provider.name, provider.kind));
        }
    }
    if wrong.is_empty() {
        println!("kinds\tall known");
    } else {
        println!("kinds\tunknown for {}", wrong.join(", "));
    }

    let mut missing = Vec::new();
    for provider in registry.providers() {
        if credential(provider).is_none() {
            missing.push(provider.name.clone());
        }
    }

    if missing.is_empty() {
        println!(
            "credentials\tall {} providers have one",
            registry.providers().count()
        );
    } else {
        println!("credentials\tmissing for {}", missing.join(", "));
    }

    // A wrong kind is a mistake in the declaration and a missing credential is
    // a mistake in the environment, so they exit differently. Both are what
    // `doctor` exists to find, and reporting either and exiting zero would
    // make the command useless in a script.
    match (wrong.is_empty(), missing.is_empty()) {
        (true, true) => Ending::Passed,
        (false, _) => Ending::Config,
        (true, false) => Ending::Provider,
    }
}

fn policy(matches: &ArgMatches, registry: &Registry) -> Ending {
    match matches.subcommand() {
        Some(("show", _)) => {
            match registry.policy() {
                Some(policy) => {
                    for (decision, rule) in policy.rules() {
                        println!("{}\t{}", decision.as_str(), rule.describe());
                    }
                }
                None => println!("no policy is declared, so every tool needs a grant"),
            }
            Ending::Passed
        }
        _ => Ending::Usage,
    }
}

/// `meow session ...`
fn session(matches: &ArgMatches, workspace: &Workspace) -> Ending {
    let mut sessions = match crate::session::open(workspace) {
        Ok(sessions) => sessions,
        Err(e) => {
            eprintln!("{e}");
            return Ending::Stopped(meow_core::StopReason::Failed);
        }
    };

    let found = |sessions: &meow_session::Sessions, matches: &ArgMatches| {
        matches
            .get_one::<String>("id")
            .ok_or_else(|| "no session was named".to_owned())
            .and_then(|needle| sessions.resolve(needle).map_err(|e| e.to_string()))
    };

    match matches.subcommand() {
        Some(("list", sub)) => {
            let agent = sub.get_one::<String>("agent").map(String::as_str);
            let limit = sub.get_one::<i64>("limit").copied().unwrap_or(20);
            match sessions.list(agent, limit) {
                Ok(rows) => {
                    for row in &rows {
                        println!("{}", crate::session::describe(&sessions, row));
                    }
                    Ending::Passed
                }
                Err(e) => failed(&e),
            }
        }

        Some(("show", sub)) => match found(&sessions, sub) {
            Ok(session) => {
                match sessions.export_markdown(&session.id, &meow_session::Redaction::default()) {
                    Ok(export) => {
                        print!("{}", export.text);
                        Ending::Passed
                    }
                    Err(e) => failed(&e),
                }
            }
            Err(e) => {
                eprintln!("{e}");
                Ending::Usage
            }
        },

        Some(("fork", sub)) => {
            let at = sub.get_one::<u64>("at").copied().unwrap_or(1);
            let origin = match found(&sessions, sub) {
                Ok(session) => session.id,
                Err(e) => {
                    eprintln!("{e}");
                    return Ending::Usage;
                }
            };
            let id = meow_core::SessionId::new(now_millis(), entropy());
            match sessions.fork(&origin, at, &id) {
                Ok(id) => {
                    println!("{id}");
                    Ending::Passed
                }
                // A bad fork point is the caller naming a sequence that is not
                // there, which is a usage mistake and not a storage failure.
                Err(e) => {
                    eprintln!("{e}");
                    Ending::Usage
                }
            }
        }

        Some(("export", sub)) => {
            let session = match found(&sessions, sub) {
                Ok(session) => session,
                Err(e) => {
                    eprintln!("{e}");
                    return Ending::Usage;
                }
            };
            let redaction = meow_session::Redaction {
                arguments: Vec::new(),
                include_thinking: sub.get_flag("thinking"),
            };
            let export = if sub.get_one::<String>("as").is_some_and(|f| f == "json") {
                sessions.export_json(&session.id, &redaction)
            } else {
                sessions.export_markdown(&session.id, &redaction)
            };
            match export {
                Ok(export) => {
                    print!("{}", export.text);
                    Ending::Passed
                }
                Err(e) => failed(&e),
            }
        }

        Some(("gc", sub)) => {
            let retention = meow_session::Retention {
                max_age_secs: sub
                    .get_one::<i64>("older-than-days")
                    .map(|days| days * 24 * 60 * 60),
                max_count: sub.get_one::<usize>("keep").copied(),
                max_bytes: None,
                include_named: sub.get_flag("named"),
            };
            match sessions.sweep(retention, now_secs()) {
                Ok(swept) => {
                    for id in &swept.deleted {
                        println!("deleted\t{}", id.short());
                    }
                    for id in &swept.kept {
                        println!("kept\t{}\tnamed", id.short());
                    }
                    Ending::Passed
                }
                Err(e) => failed(&e),
            }
        }

        _ => Ending::Usage,
    }
}

/// `meow index ...`
fn index(matches: &ArgMatches, workspace: &Workspace, registry: &Registry) -> Ending {
    let (model_id, chunking, walk) = match crate::index::configure(registry) {
        Ok(configured) => configured,
        Err(message) => {
            // [R-STAR-035]: a workspace that declares no index is told to
            // declare one rather than having a model chosen for it.
            eprintln!("{message}");
            return Ending::Config;
        }
    };

    let mut index = match crate::index::open(workspace, chunking, walk) {
        Ok(index) => index,
        Err(e) => return failed(&e),
    };

    match matches.subcommand() {
        Some(("stats", _)) => {
            let (total, embedded) = match index.counts() {
                Ok(counts) => counts,
                Err(e) => return failed(&e),
            };
            let built_by = index.model().ok().flatten();
            println!("chunks\t{total}");
            println!("embedded\t{embedded}");
            println!("model\t{}", built_by.as_deref().unwrap_or("-"));
            Ending::Passed
        }

        Some(("clear", _)) => match index.clear() {
            Ok(()) => {
                println!("cleared");
                Ending::Passed
            }
            Err(e) => failed(&e),
        },

        Some(("update", _)) => match index.update() {
            Ok(built) => {
                report(&built);
                Ending::Passed
            }
            Err(e) => failed(&e),
        },

        Some(("build", _)) => {
            let built = match index.update() {
                Ok(built) => built,
                Err(e) => return failed(&e),
            };
            report(&built);

            let Some((runtime, embedder)) = embedder(registry, &model_id) else {
                eprintln!("no credential for the provider `{model_id}` is served by");
                return Ending::Provider;
            };
            let _guard = runtime;

            match index.embed(&embedder, meow_index::embed::DEFAULT_BATCH) {
                Ok(count) => {
                    println!("embedded\t{count}");
                    Ending::Passed
                }
                Err(e) => failed(&e),
            }
        }

        Some(("query", sub)) => {
            let Some(text) = sub.get_one::<String>("text") else {
                return Ending::Usage;
            };
            let paths: Vec<String> = sub
                .get_many::<String>("path")
                .map(|values| values.cloned().collect())
                .unwrap_or_default();

            let Some((runtime, embedder)) = embedder(registry, &model_id) else {
                eprintln!("no credential for the provider `{model_id}` is served by");
                return Ending::Provider;
            };
            let _guard = runtime;

            let query = meow_index::Query {
                limit: sub.get_one::<u32>("limit").copied().unwrap_or(10) as usize,
                min_score: 0.0,
                paths,
            };

            match index.query(&embedder, text, &query) {
                Ok(hits) => {
                    for hit in hits {
                        println!(
                            "{}:{}-{}\t{:.3}",
                            hit.path, hit.first_line, hit.last_line, hit.score
                        );
                    }
                    Ending::Passed
                }
                // An empty index is somebody forgetting to build one, which is
                // a mistake in what was typed rather than a broken workspace.
                Err(e @ meow_index::IndexError::Empty) => {
                    eprintln!("{e}");
                    Ending::Usage
                }
                Err(e) => failed(&e),
            }
        }

        _ => Ending::Usage,
    }
}

/// What an update did, one line per outcome that happened.
fn report(built: &meow_index::Built) {
    println!("added\t{}", built.added);
    println!("changed\t{}", built.changed);
    println!("removed\t{}", built.removed);
    println!("unchanged\t{}", built.unchanged);
    if built.skipped > 0 {
        println!("skipped\t{}", built.skipped);
    }
}

/// An embedder, and the runtime it blocks on.
///
/// The runtime is returned so the caller keeps it alive: an embedder that
/// outlived its reactor would block for ever on the first request.
fn embedder(
    registry: &Registry,
    model_id: &str,
) -> Option<(tokio::runtime::Runtime, crate::index::Embedder)> {
    let declared = registry.index().and_then(|i| registry.model(&i.model))?;
    let provider = built_providers(registry).remove(&declared.provider)?;
    let runtime = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .ok()?;
    let handle = runtime.handle().clone();
    Some((
        runtime,
        crate::index::Embedder::new(provider, handle, model_id.to_owned()),
    ))
}

fn failed(e: &impl std::fmt::Display) -> Ending {
    eprintln!("{e}");
    Ending::Stopped(meow_core::StopReason::Failed)
}

/// Now, in milliseconds, for minting an identifier.
fn now_millis() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map_or(0, |d| u64::try_from(d.as_millis()).unwrap_or(u64::MAX))
}

/// Now, in seconds, for retention.
fn now_secs() -> i64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map_or(0, |d| i64::try_from(d.as_secs()).unwrap_or(i64::MAX))
}

/// Eighty bits of entropy for an identifier.
///
/// From the clock's sub-millisecond digits and the process id rather than a
/// random number generator: an identifier needs to be unique within a
/// workspace, not unguessable, and one fewer dependency is worth more than
/// cryptographic randomness nothing depends on.
fn entropy() -> [u8; 10] {
    let nanos = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map_or(0, |d| d.subsec_nanos());
    let pid = std::process::id();
    let mut out = [0_u8; 10];
    out[..4].copy_from_slice(&nanos.to_be_bytes());
    out[4..8].copy_from_slice(&pid.to_be_bytes());
    out[8..].copy_from_slice(&(nanos as u16).to_be_bytes());
    out
}

fn completions(matches: &ArgMatches) -> Ending {
    let Some(shell) = matches.get_one::<String>("shell") else {
        return Ending::Usage;
    };
    let Ok(shell) = shell.parse::<clap_complete::Shell>() else {
        return Ending::Usage;
    };

    // Completions are generated against the built-ins only. A user's commands
    // depend on the workspace the shell happens to be in, and a completion
    // script that hard-codes one project's commands is wrong everywhere else.
    let mut command = crate::surface::build(None);
    clap_complete::generate(shell, &mut command, "meow", &mut std::io::stdout());
    Ending::Passed
}

/// `meow init`: the smallest workspace that loads.
fn init() -> Ending {
    let Ok(here) = std::env::current_dir() else {
        eprintln!("could not read the current directory");
        return Ending::Config;
    };
    let config = here.join(".meow");
    let entry = config.join("meow.star");

    if entry.exists() {
        eprintln!("{} already exists", entry.display());
        return Ending::Config;
    }

    if let Err(e) = std::fs::create_dir_all(&config) {
        eprintln!("could not create {}: {e}", config.display());
        return Ending::Config;
    }
    if let Err(e) = std::fs::write(&entry, TEMPLATE) {
        eprintln!("could not write {}: {e}", entry.display());
        return Ending::Config;
    }

    println!("created {}", entry.display());
    Ending::Passed
}

const TEMPLATE: &str = r#"load("@std//env", "get")

meow.provider(
    name = "anthropic",
    kind = "anthropic",
    api_key = get("ANTHROPIC_API_KEY"),
)

meow.model(
    name = "fast",
    provider = "anthropic",
    id = "claude-haiku-4-5",
    context = 200000,
    max_output = 8192,
)
"#;

/// Collect the arguments a declared command was given.
fn arguments(registry: &Registry, name: &str, matches: &ArgMatches) -> Map<String, Value> {
    let mut out = Map::new();

    if let Some(tool) = registry.tool(name) {
        for arg in tool.args.iter() {
            // A boolean is the only kind clap stores as a flag, and asking it
            // for one of any other kind is a panic rather than a `false`.
            if arg.node.get("type").and_then(Value::as_str) == Some("boolean") {
                if matches.get_flag(&arg.name) {
                    out.insert(arg.name.clone(), Value::Bool(true));
                }
                continue;
            }

            let Some(text) = matches.get_one::<String>(&arg.name) else {
                continue;
            };
            // A flag arrives as text and the declaration says what it means,
            // which is the one place the two have to agree.
            out.insert(arg.name.clone(), parse(&arg.node, text));
        }
        return out;
    }

    if registry.agent(name).is_some() {
        let task: Vec<String> = matches
            .get_many::<String>("task")
            .map(|values| values.cloned().collect())
            .unwrap_or_default();
        out.insert("task".to_owned(), Value::String(task.join(" ")));
    }

    out
}

/// Read a flag's text as the type its declaration gives it.
fn parse(node: &Value, text: &str) -> Value {
    match node.get("type").and_then(Value::as_str) {
        Some("integer") => text
            .parse::<i64>()
            .map_or_else(|_| Value::String(text.to_owned()), Value::from),
        Some("number") => text
            .parse::<f64>()
            .ok()
            .and_then(serde_json::Number::from_f64)
            .map_or_else(|| Value::String(text.to_owned()), Value::Number),
        Some("boolean") => match text {
            "true" => Value::Bool(true),
            "false" => Value::Bool(false),
            other => Value::String(other.to_owned()),
        },
        // A wrong value stays a string and the declaration's own check reports
        // it, so the message names the constraint rather than the parse.
        _ => Value::String(text.to_owned()),
    }
}

/// Run a declared command.
fn invoke(
    matches: &ArgMatches,
    name: &str,
    args: &Map<String, Value>,
    workspace: Workspace,
    loaded: Loaded,
) -> Ending {
    let Ok(runtime) = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
    else {
        eprintln!("could not start the runtime");
        return Ending::Provider;
    };

    let engines = match engines(&loaded.registry) {
        Ok(engines) => engines,
        Err(message) => {
            eprintln!("{message}");
            return Ending::Provider;
        }
    };
    let built = built_providers(&loaded.registry);

    let sink = Arc::new(render::build(Environment {
        json: matches
            .get_one::<String>("format")
            .is_some_and(|f| f == "json"),
        color_never: matches
            .get_one::<String>("color")
            .is_some_and(|c| c == "never"),
    }));

    // The session is opened before the run, because `--continue` decides what
    // the run is given and not what it does afterwards.
    let session = match open_session(&workspace, name, args, matches.get_flag("continue")) {
        Ok(session) => session,
        Err(Opening::Missing(message)) => {
            // Asking to continue something that is not there is a mistake in
            // what was typed, not a broken workspace.
            eprintln!("{message}");
            return Ending::Usage;
        }
        Err(Opening::Broken(message)) => {
            eprintln!("{message}");
            return Ending::Config;
        }
    };

    // One object answers questions and approvals alike, so an `always` and a
    // `ctx.ask` cannot disagree about whether anybody is there.
    let terminal = Arc::new(ask::Terminal::new(
        ask::Interaction::detect(matches.get_flag("yes")),
        Arc::clone(&sink),
    ));

    let cancel = CancellationToken::new();
    watch(&runtime, cancel.clone());

    // Built before the runtime takes the workspace and the registry, because
    // it reads both. A workspace that declares no index, or one whose
    // embedding provider has no credential, still runs: `search.code` is what
    // fails, and it says why.
    let search = searcher(&workspace, &loaded.registry, &built, runtime.handle());

    // `[R-STAR-026]`: the store is the workspace's database, which every
    // command already opens. When it will not open, every `store` call says
    // so - an in-memory fallback would let a handler write a value, read it
    // back inside the run, and find it gone next time with nothing explaining
    // why.
    let keep: Arc<dyn meow_star::port::Keep> = match crate::keep::Durable::open(&workspace) {
        Ok(durable) => Arc::new(durable),
        Err(error) => Arc::new(meow_star::port::quiet::Unopened(format!(
            "the workspace store will not open: {error}"
        ))),
    };

    let star = Arc::new(Runtime::new(
        loaded,
        workspace,
        engines,
        runtime.handle().clone(),
        Ports {
            events: Arc::clone(&sink) as Arc<dyn meow_star::port::Events>,
            approve: Some(Arc::clone(&terminal) as Arc<dyn meow_agent::Approver>),
            dry_run: matches.get_flag("dry-run"),
            ask: Arc::clone(&terminal) as Arc<dyn meow_star::port::Ask>,
            stdin: Arc::new(crate::ask::Stdin),
            session: Arc::clone(&session) as Arc<dyn meow_star::port::Session>,
            search,
            keep,
        },
        cancel.clone(),
    ));

    let name = name.to_owned();
    let args = args.clone();
    let result = runtime.block_on(async move {
        tokio::task::spawn_blocking(move || star.run_command(&name, &args)).await
    });

    sink.finish();

    match result {
        Ok(Ok(returned)) => {
            // The run finished either way: a handler that returned false
            // reported a verdict about what it looked at, not about the run.
            session.finish(meow_core::StopReason::Finished, None);
            exit::finished(verdict(&returned))
        }
        Ok(Err(e)) => {
            eprintln!("{e}");
            session.finish(meow_core::StopReason::Failed, Some(&e.to_string()));
            Ending::Stopped(meow_core::StopReason::Failed)
        }
        Err(e) => {
            eprintln!("the run did not finish: {e}");
            session.finish(meow_core::StopReason::Failed, None);
            Ending::Stopped(meow_core::StopReason::Failed)
        }
    }
}

/// Open the session this run writes to.
///
/// `[R-TUI-074]`: `--continue` resumes the most recent session of the command
/// being invoked and fails when there is none, rather than quietly starting a
/// fresh run somebody thought they were adding to.
fn open_session(
    workspace: &Workspace,
    agent: &str,
    args: &Map<String, Value>,
    resuming: bool,
) -> std::result::Result<Arc<crate::session::Log>, Opening> {
    let sessions = crate::session::open(workspace).map_err(|e| Opening::Broken(e.to_string()))?;

    let previous = if resuming {
        let found = crate::session::most_recent(&sessions, agent)
            .map_err(|e| Opening::Broken(e.to_string()))?;
        Some(found.ok_or_else(|| {
            Opening::Missing(format!(
                "`--continue` found no earlier session of `{agent}` in this workspace"
            ))
        })?)
    } else {
        None
    };

    let task = args
        .get("task")
        .and_then(Value::as_str)
        .unwrap_or_default()
        .to_owned();

    crate::session::Log::open(sessions, agent, &task, previous, now_millis(), entropy())
        .map(Arc::new)
        .map_err(|e| Opening::Broken(e.to_string()))
}

/// Why a session could not be opened.
#[derive(Debug)]
enum Opening {
    /// `--continue` found nothing to continue.
    Missing(String),
    /// The store would not open, or the log would not be written.
    Broken(String),
}

/// What a handler's return value says about the run.
///
/// `[R-TUI-080]` distinguishes `false` from everything else: a handler
/// returning a string is reporting a result, not a verdict.
fn verdict(returned: &str) -> Option<bool> {
    match returned.trim() {
        "false" | "False" => Some(false),
        "true" | "True" => Some(true),
        _ => None,
    }
}

/// Stop the run on the first interrupt.
fn watch(runtime: &tokio::runtime::Runtime, cancel: CancellationToken) {
    runtime.spawn(async move {
        if tokio::signal::ctrl_c().await.is_ok() {
            cancel.cancel();
        }
    });
}

/// Search over this workspace's index, or something that says why not.
fn searcher(
    workspace: &Workspace,
    registry: &Registry,
    providers: &HashMap<String, Arc<dyn Provider>>,
    handle: &tokio::runtime::Handle,
) -> Arc<dyn meow_star::port::Search> {
    let refused = || Arc::new(meow_star::port::quiet::NoIndex) as Arc<dyn meow_star::port::Search>;

    let Ok((model_id, chunking, walk)) = crate::index::configure(registry) else {
        return refused();
    };
    let Some(declared) = registry.index().and_then(|i| registry.model(&i.model)) else {
        return refused();
    };
    let Some(provider) = providers.get(&declared.provider) else {
        return refused();
    };
    let Ok(index) = crate::index::open(workspace, chunking, walk) else {
        return refused();
    };

    Arc::new(crate::index::Searcher::new(
        index,
        crate::index::Embedder::new(Arc::clone(provider), handle.clone(), model_id),
    ))
}

/// One provider per declared name, for whatever needs the provider itself.
fn built_providers(registry: &Registry) -> HashMap<String, Arc<dyn Provider>> {
    let mut out = HashMap::new();
    for declared in registry.providers() {
        let Some(key) = credential(declared) else {
            continue;
        };
        if let Ok(provider) = build_provider(declared, &key) {
            out.insert(declared.name.clone(), provider);
        }
    }
    out
}

/// Every provider kind this binary knows how to talk to.
///
/// `openai` covers anything that speaks OpenAI's shape, which is most of
/// them: OpenRouter, Together, `llama.cpp`'s server, LM Studio. What they
/// differ in is the address and whether `response_format` is real, and both
/// are declarations rather than code.
pub const KINDS: &[&str] = &[
    "anthropic",
    "openai",
    "openrouter",
    "gemini",
    "voyage",
    "llama",
];

/// Build one provider from its declaration.
///
/// # Errors
///
/// A message naming the kind and what there is, per `[R-STAR-091]`'s spirit:
/// a typo in a kind should say what the kinds are.
fn build_provider(declared: &meow_star::Provider, key: &str) -> Result<Arc<dyn Provider>, String> {
    let transport = Http::new(declared.name.clone()).map_err(|e| e.to_string())?;
    let base = declared.base_url.clone();

    let built: Arc<dyn Provider> = match declared.kind.as_str() {
        "anthropic" => {
            let mut provider = Anthropic::new(transport, key);
            if let Some(url) = base {
                provider = provider.with_base_url(url);
            }
            Arc::new(provider)
        }
        "openai" => {
            let mut provider = OpenAi::new(transport, key).with_name(declared.name.clone());
            if let Some(url) = base {
                provider = provider.with_base_url(url);
            }
            Arc::new(provider)
        }
        "openrouter" => {
            let provider = OpenAi::new(transport, key)
                .with_name(declared.name.clone())
                .with_base_url(base.unwrap_or_else(|| "https://openrouter.ai/api".to_owned()));
            Arc::new(provider)
        }
        // A local server takes the body and ignores `response_format`, so a
        // schema is asked for in the prompt and checked here instead. There is
        // no address to default to: whoever runs one knows where it is.
        "llama" => {
            let provider = OpenAi::new(transport, key)
                .with_name(declared.name.clone())
                .with_base_url(base.unwrap_or_else(|| "http://localhost:8080".to_owned()))
                .with_emulated_schema();
            Arc::new(provider)
        }
        "gemini" => {
            let mut provider = Gemini::new(transport, key);
            if let Some(url) = base {
                provider = provider.with_base_url(url);
            }
            Arc::new(provider)
        }
        "voyage" => {
            let mut provider = Voyage::new(transport, key, "voyage-3");
            if let Some(url) = base {
                provider = provider.with_base_url(url);
            }
            Arc::new(provider)
        }
        other => {
            return Err(format!(
                "`{}` names provider kind `{other}`; the kinds are {}",
                declared.name,
                KINDS.join(", ")
            ));
        }
    };

    Ok(built)
}

/// Build one engine per declared provider.
fn engines(registry: &Registry) -> Result<HashMap<String, Arc<Engine>>, String> {
    let mut out = HashMap::new();

    for provider in registry.providers() {
        let Some(key) = credential(provider) else {
            // Skipped rather than refused: a workspace may declare three
            // providers and a run may need one of them, and failing at startup
            // would make a missing key for an unused provider fatal.
            continue;
        };

        let built = build_provider(provider, &key).map_err(|e| e.to_string())?;
        out.insert(provider.name.clone(), Arc::new(Engine::new(built)));
    }

    Ok(out)
}

/// The key for a provider, from the declaration or the environment.
///
/// The declaration wins, because that is where `@std//env` put whatever the
/// author decided the key should be. The conventional variable is the fallback
/// so a workspace that says nothing still works.
fn credential(provider: &meow_star::Provider) -> Option<String> {
    if let Some(key) = &provider.api_key
        && !key.is_empty()
    {
        return Some(key.clone());
    }
    let variable = format!("{}_API_KEY", provider.kind.to_uppercase());
    std::env::var(variable).ok().filter(|key| !key.is_empty())
}
