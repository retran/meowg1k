// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The command line a user sees.
//!
//! A user's agents and tools sit at the top level with no prefix, because they
//! are what the binary is for: `meow review`, not `meow run review`. The
//! built-ins that would otherwise crowd them are grouped, and the handful that
//! a person reaches for before they have a workspace stay at the top.

use clap::{Arg, ArgAction, Command};
use meow_star::{Args, Registry};

/// Built-in commands that stay at the top level.
///
/// `[R-TUI-071]`. These are the ones somebody types before they have anything
/// declared, or while something is wrong: putting `doctor` under a group means
/// finding out what to type while the thing you are diagnosing is broken.
pub const TOP_LEVEL: &[&str] = &[
    "init",
    "run",
    "check",
    "models",
    "providers",
    "doctor",
    "trust",
    "completions",
    "version",
];

/// The groups every other built-in lives under.
pub const GROUPS: &[&str] = &["session", "auth", "index", "policy"];

/// Build the command line for a workspace.
///
/// Satisfies `[R-TUI-070]`: everything the workspace declares is a top-level
/// subcommand with no prefix. `registry` is `None` before a workspace has been
/// found, which is what `meow init` and `meow --help` outside a project get.
pub fn build(registry: Option<&Registry>) -> Command {
    let mut command = Command::new("meow")
        .about("A script-friendly AI companion")
        .version(env!("CARGO_PKG_VERSION"))
        .subcommand_required(true)
        .arg_required_else_help(true)
        .arg(
            Arg::new("format")
                .long("format")
                .value_name("FORMAT")
                .value_parser(["auto", "json"])
                .default_value("auto")
                .global(true)
                .help("How to render: auto picks by terminal, json emits one object per line"),
        )
        .arg(
            Arg::new("yes")
                .long("yes")
                .action(ArgAction::SetTrue)
                .global(true)
                .help("Never ask: every decision that needs a person is refused"),
        )
        .arg(
            Arg::new("continue")
                .long("continue")
                .action(ArgAction::SetTrue)
                .global(true)
                .help("Add to the most recent session of this command"),
        )
        .arg(
            Arg::new("dry-run")
                .long("dry-run")
                .action(ArgAction::SetTrue)
                .global(true)
                .help("Plan the tool calls and make none of them"),
        )
        .arg(
            Arg::new("color")
                .long("color")
                .value_name("WHEN")
                .value_parser(["auto", "never"])
                .default_value("auto")
                .global(true)
                .help("Whether to use colour"),
        );

    for builtin in builtins() {
        command = command.subcommand(builtin);
    }

    if let Some(registry) = registry {
        for name in registry.commands() {
            command = command.subcommand(declared(registry, name));
        }
    }

    command
}

/// The built-in commands, top level and grouped.
fn builtins() -> Vec<Command> {
    vec![
        Command::new("init").about("Create a .meow/ in this directory"),
        Command::new("run")
            .about("Run a declared command by name")
            .arg(Arg::new("name").required(true).help("Which command"))
            .arg(
                Arg::new("rest")
                    .num_args(0..)
                    .trailing_var_arg(true)
                    .allow_hyphen_values(true)
                    .help("Its arguments"),
            ),
        Command::new("check").about("Load .meow/ and report what is wrong with it"),
        Command::new("models").about("List the declared models"),
        Command::new("providers").about("List the declared providers"),
        Command::new("doctor").about("Check the toolchain, the workspace, and the credentials"),
        Command::new("completions")
            .about("Print a shell completion script")
            .arg(Arg::new("shell").required(true).value_parser([
                "bash",
                "zsh",
                "fish",
                "elvish",
                "powershell",
            ])),
        Command::new("version").about("Print the version"),
        Command::new("session")
            .about("Work with session logs")
            .subcommand_required(true)
            .subcommand(
                Command::new("list")
                    .about("List sessions, newest first")
                    .arg(
                        Arg::new("agent")
                            .long("agent")
                            .value_name("NAME")
                            .help("Only this agent's sessions"),
                    )
                    .arg(
                        Arg::new("limit")
                            .long("limit")
                            .value_name("N")
                            .default_value("20")
                            .value_parser(clap::value_parser!(i64).range(1..)),
                    ),
            )
            .subcommand(
                Command::new("show")
                    .about("Print one session's transcript")
                    .arg(
                        Arg::new("id")
                            .required(true)
                            .help("An id, a name, or @last"),
                    ),
            )
            .subcommand(
                Command::new("fork")
                    .about("Branch a session at one of its events")
                    .arg(
                        Arg::new("id")
                            .required(true)
                            .help("An id, a name, or @last"),
                    )
                    .arg(
                        Arg::new("at")
                            .long("at")
                            .value_name("SEQ")
                            .required(true)
                            .value_parser(clap::value_parser!(u64).range(1..))
                            .help("The last event to copy"),
                    ),
            )
            .subcommand(
                Command::new("export")
                    .about("Write a session out")
                    .arg(
                        Arg::new("id")
                            .required(true)
                            .help("An id, a name, or @last"),
                    )
                    .arg(
                        Arg::new("as")
                            .long("as")
                            .value_name("FORMAT")
                            .value_parser(["md", "json"])
                            .default_value("md"),
                    )
                    .arg(
                        Arg::new("thinking")
                            .long("thinking")
                            .action(ArgAction::SetTrue)
                            .help("Include the model's reasoning"),
                    ),
            )
            .subcommand(
                Command::new("gc")
                    .about("Delete sessions the retention limits no longer keep")
                    .arg(
                        Arg::new("older-than-days")
                            .long("older-than-days")
                            .value_name("DAYS")
                            .value_parser(clap::value_parser!(i64).range(0..)),
                    )
                    .arg(
                        Arg::new("keep")
                            .long("keep")
                            .value_name("N")
                            .value_parser(clap::value_parser!(usize)),
                    )
                    .arg(
                        Arg::new("named")
                            .long("named")
                            .action(ArgAction::SetTrue)
                            .help("Delete named sessions too"),
                    ),
            ),
        Command::new("index")
            .about("Work with the semantic index")
            .subcommand_required(true)
            .subcommand(
                Command::new("build").about("Walk the workspace, chunk it, and embed what changed"),
            )
            .subcommand(
                Command::new("update")
                    .about("Chunk what changed without embedding it")
                    .long_about(
                        "Walk the workspace and re-chunk the files whose content or chunking \
                         changed, leaving the embedding to `meow index build`.",
                    ),
            )
            .subcommand(Command::new("stats").about("Say how much is indexed"))
            .subcommand(
                Command::new("query")
                    .about("Ask the index a question")
                    .arg(Arg::new("text").required(true).help("What to look for"))
                    .arg(
                        Arg::new("limit")
                            .long("limit")
                            .value_name("N")
                            .default_value("10")
                            .value_parser(clap::value_parser!(u32).range(1..)),
                    )
                    .arg(
                        Arg::new("path")
                            .long("path")
                            .value_name("GLOB")
                            .action(ArgAction::Append)
                            .help("Only files matching this glob; may be repeated"),
                    ),
            )
            .subcommand(Command::new("clear").about("Forget the index, keeping everything else")),
        Command::new("policy")
            .about("Work with permission rules")
            .subcommand_required(true)
            .subcommand(Command::new("show").about("Print the rules this workspace declares")),
    ]
}

/// One declared command, with the flags its arguments describe.
///
/// `[R-STAR-061]`: the flag, the help text, and the model's schema come out of
/// one declaration, so a flag here cannot describe something the tool does not
/// accept.
fn declared(registry: &Registry, name: &str) -> Command {
    let (about, args) = match (registry.tool(name), registry.agent(name)) {
        (Some(tool), _) => (tool.about.clone(), Some(tool.args.clone())),
        (None, Some(agent)) => (agent.about.clone(), None),
        (None, None) => (String::new(), None),
    };

    let mut command = Command::new(name.to_owned()).about(about);

    match args {
        Some(args) => {
            for arg in flags(&args) {
                command = command.arg(arg);
            }
        }
        // An agent takes a task rather than declared arguments, because what
        // it is given is prose.
        None => {
            command = command.arg(
                Arg::new("task")
                    .num_args(0..)
                    .trailing_var_arg(true)
                    .allow_hyphen_values(true)
                    .help("What to do"),
            );
        }
    }

    command
}

/// Turn declared arguments into clap arguments.
pub fn flags(args: &Args) -> Vec<Arg> {
    let mut out = Vec::new();

    for (index, arg) in args.positional().into_iter().enumerate() {
        out.push(
            Arg::new(arg.name.clone())
                .help(arg.help())
                .required(arg.required())
                .index(index + 1),
        );
    }

    for arg in args.flags() {
        let boolean = arg.node.get("type").and_then(serde_json::Value::as_str) == Some("boolean");
        let mut flag = Arg::new(arg.name.clone())
            .long(arg.flag().trim_start_matches("--").to_owned())
            .help(arg.help())
            .required(arg.required());

        flag = if boolean {
            flag.action(ArgAction::SetTrue)
        } else {
            flag.value_name(arg.name.to_uppercase())
        };

        if let Some(serde_json::Value::Array(values)) = arg.node.get("enum") {
            let allowed: Vec<String> = values
                .iter()
                .filter_map(|v| v.as_str().map(str::to_owned))
                .collect();
            flag = flag.value_parser(allowed);
        }

        out.push(flag);
    }

    out
}
