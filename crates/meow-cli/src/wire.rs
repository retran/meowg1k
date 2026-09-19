// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Turning a parsed command line into a run.

use std::collections::HashMap;
use std::sync::Arc;

use clap::ArgMatches;
use meow_agent::Engine;
use meow_llm::{Anthropic, Http, Provider};
use meow_star::port::quiet::{Closed, Memory, NoTerminal};
use meow_star::{Loaded, Ports, Registry, Runtime, Workspace};
use serde_json::{Map, Value};
use tokio_util::sync::CancellationToken;

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
        return Ending::Passed;
    }

    println!("credentials\tmissing for {}", missing.join(", "));
    // A missing credential is what `doctor` exists to find, so reporting it
    // and exiting zero would make the command useless in a script.
    Ending::Provider
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
            let Some(text) = matches.get_one::<String>(&arg.name) else {
                if matches.get_flag(&arg.name) {
                    out.insert(arg.name.clone(), Value::Bool(true));
                }
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

    let sink = Arc::new(render::build(Environment {
        json: matches
            .get_one::<String>("format")
            .is_some_and(|f| f == "json"),
        color_never: matches
            .get_one::<String>("color")
            .is_some_and(|c| c == "never"),
    }));

    let cancel = CancellationToken::new();
    watch(&runtime, cancel.clone());

    let star = Arc::new(Runtime::new(
        loaded,
        workspace,
        engines,
        runtime.handle().clone(),
        Ports {
            events: Arc::clone(&sink) as Arc<dyn meow_star::port::Events>,
            ask: Arc::new(NoTerminal),
            stdin: Arc::new(Closed),
            session: Arc::new(Memory::new("local")),
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
        Ok(Ok(returned)) => exit::finished(verdict(&returned)),
        Ok(Err(e)) => {
            eprintln!("{e}");
            Ending::Stopped(meow_core::StopReason::Failed)
        }
        Err(e) => {
            eprintln!("the run did not finish: {e}");
            Ending::Stopped(meow_core::StopReason::Failed)
        }
    }
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

        let built: Arc<dyn Provider> = match provider.kind.as_str() {
            "anthropic" => Arc::new(Anthropic::new(
                Http::new(provider.name.clone()).map_err(|e| e.to_string())?,
                key,
            )),
            other => {
                return Err(format!(
                    "`{}` names provider kind `{other}`, and the only kind implemented is `anthropic`",
                    provider.name
                ));
            }
        };

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
