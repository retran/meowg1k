// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The command surface, and what the process exits with.
//!
//! The binary is driven as a binary: `cargo test` builds it and these run it
//! against a temporary workspace. An exit code that a shell can branch on is
//! only worth having if something checks the shell's view of it, and that view
//! is a process, not a function.
#![allow(clippy::unwrap_used)]

use std::path::Path;
use std::process::{Command, Output};

use tempfile::TempDir;

/// The binary this test was built alongside.
fn meow() -> Command {
    Command::new(env!("CARGO_BIN_EXE_meow"))
}

/// Enough of a workspace to declare a provider, a model, and two commands.
const WORKSPACE: &str = r#"
meow.provider(name = "anthropic", kind = "anthropic", api_key = "k")
meow.model(
    name = "fast",
    provider = "anthropic",
    id = "claude-haiku-4-5",
    context = 200000,
    max_output = 8192,
)

def greet(ctx):
    ctx.out.write("hello, %s" % ctx.args.name)
    return "true"

meow.command(meow.tool(
    name = "greet",
    about = "say hello",
    run = greet,
    args = {"name": meow.arg.string(about = "who to greet", positional = 0)},
))

def gate(ctx):
    return "false"

meow.command(meow.tool(name = "gate", about = "report a failure", run = gate))
"#;

fn workspace(source: &str) -> TempDir {
    let dir = tempfile::tempdir().unwrap();
    let config = dir.path().join(".meow");
    std::fs::create_dir_all(&config).unwrap();
    std::fs::write(config.join("meow.star"), source).unwrap();
    dir
}

fn run(dir: &Path, args: &[&str]) -> Output {
    meow()
        .current_dir(dir)
        // The key comes from the declaration in these tests, and a variable
        // left over from the developer's shell would hide a missing one.
        .env_remove("ANTHROPIC_API_KEY")
        .env_remove("NO_COLOR")
        .args(args)
        .output()
        .unwrap()
}

fn stdout(output: &Output) -> String {
    String::from_utf8_lossy(&output.stdout).into_owned()
}

fn stderr(output: &Output) -> String {
    String::from_utf8_lossy(&output.stderr).into_owned()
}

fn code(output: &Output) -> i32 {
    output.status.code().unwrap_or(-1)
}

/// [R-TUI-070] a declared command is reachable at the top level, with no prefix
#[test]
fn a_declared_command_needs_no_prefix() {
    let dir = workspace(WORKSPACE);

    let direct = run(dir.path(), &["greet", "world"]);
    assert_eq!(code(&direct), 0, "{}", stderr(&direct));
    assert_eq!(stdout(&direct).trim(), "hello, world");

    // And it shows up in the help, which is how anybody finds it.
    let help = run(dir.path(), &["--help"]);
    assert!(stdout(&help).contains("greet"), "{}", stdout(&help));
    assert!(stdout(&help).contains("say hello"), "{}", stdout(&help));
}

/// [R-TUI-070] a declared command's arguments become flags described by their
/// declaration
#[test]
fn a_declared_argument_becomes_a_described_flag() {
    let dir = workspace(WORKSPACE);
    let help = run(dir.path(), &["greet", "--help"]);

    assert!(stdout(&help).contains("who to greet"), "{}", stdout(&help));
    assert!(stdout(&help).contains("(required)"), "{}", stdout(&help));
}

/// [R-TUI-071] built-ins are grouped, and only the named ones stay at the top
#[test]
fn only_the_named_builtins_stay_at_the_top_level() {
    let dir = workspace(WORKSPACE);
    let help = stdout(&run(dir.path(), &["--help"]));

    // Everything at the top level is either one of the nine, a group, one of
    // this workspace's own commands, or clap's own `help`.
    let commands: Vec<String> = help
        .lines()
        .skip_while(|line| !line.starts_with("Commands:"))
        .skip(1)
        .take_while(|line| !line.trim().is_empty())
        .filter_map(|line| line.split_whitespace().next().map(str::to_owned))
        .collect();

    assert!(!commands.is_empty(), "{help}");

    let declared = ["greet", "gate", "help"];
    for command in &commands {
        let known = meow_cli::surface::TOP_LEVEL.contains(&command.as_str())
            || meow_cli::surface::GROUPS.contains(&command.as_str())
            || declared.contains(&command.as_str());
        assert!(
            known,
            "`{command}` is at the top level and is neither a named built-in, a group, nor declared"
        );
    }

    // And at least one group is actually present, or the rule is vacuous.
    assert!(
        commands
            .iter()
            .any(|c| meow_cli::surface::GROUPS.contains(&c.as_str())),
        "no group is present: {help}"
    );
}

/// [R-TUI-071] a grouped built-in is not also at the top level
#[test]
fn a_grouped_builtin_is_reachable_only_through_its_group() {
    let dir = workspace(WORKSPACE);

    let grouped = run(dir.path(), &["policy", "show"]);
    assert_eq!(code(&grouped), 0, "{}", stderr(&grouped));

    let ungrouped = run(dir.path(), &["show"]);
    assert_eq!(code(&ungrouped), 2, "{}", stderr(&ungrouped));
}

/// [R-TUI-080] a finished run whose handler returned nothing exits zero, and
/// one that returned false exits one
#[test]
fn the_handlers_verdict_decides_between_zero_and_one() {
    let dir = workspace(WORKSPACE);

    assert_eq!(code(&run(dir.path(), &["greet", "world"])), 0);
    assert_eq!(code(&run(dir.path(), &["gate"])), 1);
}

/// [R-TUI-080] a bad command line exits two
#[test]
fn a_usage_mistake_exits_two() {
    let dir = workspace(WORKSPACE);

    assert_eq!(code(&run(dir.path(), &["nonesuch"])), 2);
    assert_eq!(
        code(&run(dir.path(), &["greet"])),
        2,
        "a missing argument is a usage mistake"
    );
    assert_eq!(code(&run(dir.path(), &["greet", "a", "--nope"])), 2);
}

/// [R-TUI-080] a workspace that will not load exits seven, and says why
#[test]
fn a_broken_workspace_exits_seven() {
    let dir = workspace("meow.provider(name = \"a\")\n");
    let output = run(dir.path(), &["check"]);

    assert_eq!(code(&output), 7, "{}", stderr(&output));
    assert!(stderr(&output).contains("meow.star"), "{}", stderr(&output));
}

/// [R-TUI-080] running outside a workspace exits seven and names what to do
#[test]
fn no_workspace_exits_seven() {
    let dir = tempfile::tempdir().unwrap();
    let output = run(dir.path(), &["check"]);

    assert_eq!(code(&output), 7, "{}", stderr(&output));
    assert!(stderr(&output).contains("meow init"), "{}", stderr(&output));
}

/// [R-TUI-080] a missing credential exits six
#[test]
fn a_missing_credential_exits_six() {
    let dir = workspace(
        r#"
meow.provider(name = "anthropic", kind = "anthropic")
meow.model(name = "fast", provider = "anthropic", id = "m", context = 1, max_output = 1)
"#,
    );

    let output = run(dir.path(), &["doctor"]);
    assert_eq!(code(&output), 6, "{}", stdout(&output));
    assert!(stdout(&output).contains("missing"), "{}", stdout(&output));
}

/// Help and version are not usage mistakes, whatever stream they go to.
#[test]
fn help_and_version_exit_zero() {
    let dir = workspace(WORKSPACE);

    assert_eq!(code(&run(dir.path(), &["--help"])), 0);
    assert_eq!(code(&run(dir.path(), &["version"])), 0);

    let version = run(dir.path(), &["version"]);
    assert!(
        stdout(&version).starts_with("meow "),
        "{}",
        stdout(&version)
    );
}

/// [R-TUI-002] --format json selects the JSON renderer even from a terminal
#[test]
fn format_json_produces_one_object_per_line() {
    let dir = workspace(WORKSPACE);
    let output = run(dir.path(), &["--format", "json", "greet", "world"]);

    assert_eq!(code(&output), 0, "{}", stderr(&output));

    let lines: Vec<serde_json::Value> = stdout(&output)
        .lines()
        .map(|line| serde_json::from_str(line).unwrap())
        .collect();

    assert_eq!(lines[0]["type"], "Schema");
    assert!(
        lines
            .iter()
            .any(|l| l["call"] == "write" && l["text"] == "hello, world"),
        "{lines:?}"
    );
}

/// `meow init` writes a workspace that loads.
#[test]
fn init_writes_something_that_loads() {
    let dir = tempfile::tempdir().unwrap();

    let created = run(dir.path(), &["init"]);
    assert_eq!(code(&created), 0, "{}", stderr(&created));

    let checked = meow()
        .current_dir(dir.path())
        .env("ANTHROPIC_API_KEY", "k")
        .arg("check")
        .output()
        .unwrap();
    assert_eq!(code(&checked), 0, "{}", stderr(&checked));

    // A second init does not overwrite what is already there.
    let again = run(dir.path(), &["init"]);
    assert_eq!(code(&again), 7, "{}", stderr(&again));
}

/// `meow models` and `meow providers` report what was declared.
#[test]
fn models_and_providers_list_what_was_declared() {
    let dir = workspace(WORKSPACE);

    let models = run(dir.path(), &["models"]);
    assert!(
        stdout(&models).contains("claude-haiku-4-5"),
        "{}",
        stdout(&models)
    );

    let providers = run(dir.path(), &["providers"]);
    assert!(
        stdout(&providers).contains("key present"),
        "{}",
        stdout(&providers)
    );
}

/// [R-TUI-051] asking with no terminal fails, and does not block
#[test]
fn asking_with_no_terminal_fails_rather_than_waiting() {
    let dir = workspace(
        r#"
meow.provider(name = "anthropic", kind = "anthropic", api_key = "k")
meow.model(name = "fast", provider = "anthropic", id = "m", context = 1, max_output = 1)

def who(ctx):
    return ctx.ask.text("who are you?", default = "nobody")

meow.command(meow.tool(name = "who", about = "ask", run = who))
"#,
    );

    // `output()` gives the child a closed stdin, which is the case a pipeline
    // produces. A test that hung here would hang the suite, which is the
    // failure this requirement exists to prevent.
    let output = run(dir.path(), &["who"]);

    assert_ne!(code(&output), 0);
    assert!(
        stderr(&output).contains("needs a terminal"),
        "{}",
        stderr(&output)
    );
}

/// [R-TUI-073] --yes refuses rather than approving
#[test]
fn yes_refuses_rather_than_approving() {
    let dir = workspace(
        r#"
meow.provider(name = "anthropic", kind = "anthropic", api_key = "k")
meow.model(name = "fast", provider = "anthropic", id = "m", context = 1, max_output = 1)

def who(ctx):
    return ctx.ask.confirm("shall I?", default = True)

meow.command(meow.tool(name = "who", about = "ask", run = who))
"#,
    );

    let output = run(dir.path(), &["--yes", "who"]);

    assert_ne!(code(&output), 0, "{}", stdout(&output));
    assert!(
        stderr(&output).contains("not answered"),
        "--yes answered a question instead of refusing it: {}",
        stderr(&output)
    );
}

/// [R-TUI-074] --continue with nothing to continue fails rather than starting
/// a fresh run
#[test]
fn continue_with_nothing_to_continue_fails() {
    let dir = workspace(WORKSPACE);
    let output = run(dir.path(), &["--continue", "greet", "world"]);

    assert_eq!(code(&output), 2, "{}", stderr(&output));
    assert!(
        stderr(&output).contains("no earlier session"),
        "{}",
        stderr(&output)
    );
    assert!(stderr(&output).contains("greet"), "{}", stderr(&output));

    // And nothing ran, so nothing was written.
    assert!(stdout(&output).trim().is_empty(), "{}", stdout(&output));
}

/// [R-SESSION-054] [R-TUI-074] a run starts a session, and --continue adds to
/// the most recent one of that command
#[test]
fn continue_adds_to_the_most_recent_session_of_that_command() {
    let dir = workspace(WORKSPACE);

    assert_eq!(code(&run(dir.path(), &["greet", "one"])), 0);
    assert_eq!(code(&run(dir.path(), &["greet", "two"])), 0);

    let listed = run(dir.path(), &["session", "list"]);
    assert_eq!(
        stdout(&listed).lines().count(),
        2,
        "a second run should be a second session: {}",
        stdout(&listed)
    );

    // Continuing adds to one of them rather than making a third.
    assert_eq!(code(&run(dir.path(), &["--continue", "greet", "three"])), 0);
    let after = run(dir.path(), &["session", "list"]);
    assert_eq!(
        stdout(&after).lines().count(),
        2,
        "--continue started a new session: {}",
        stdout(&after)
    );
}

/// [R-TUI-074] --continue picks the command being invoked, not the newest run
#[test]
fn continue_does_not_resume_another_commands_session() {
    let dir = workspace(WORKSPACE);

    assert_eq!(code(&run(dir.path(), &["greet", "one"])), 0);
    // `gate` has never run, so there is nothing of its own to continue even
    // though `greet` has a session sitting right there.
    let output = run(dir.path(), &["--continue", "gate"]);

    assert_eq!(code(&output), 2, "{}", stderr(&output));
    assert!(stderr(&output).contains("gate"), "{}", stderr(&output));
}

/// `meow session export` writes a transcript in both formats.
#[test]
fn a_session_exports_as_markdown_and_as_json() {
    let dir = workspace(WORKSPACE);
    assert_eq!(code(&run(dir.path(), &["greet", "world"])), 0);

    let markdown = run(dir.path(), &["session", "export", "@last"]);
    assert_eq!(code(&markdown), 0, "{}", stderr(&markdown));
    assert!(
        stdout(&markdown).contains("# greet"),
        "{}",
        stdout(&markdown)
    );

    let json = run(dir.path(), &["session", "export", "@last", "--as", "json"]);
    assert_eq!(code(&json), 0, "{}", stderr(&json));
    let first: serde_json::Value =
        serde_json::from_str(stdout(&json).lines().next().unwrap()).unwrap();
    assert_eq!(first["type"], "Schema");
}

/// `meow session fork` branches, and refuses a sequence that is not there.
#[test]
fn a_session_forks_and_refuses_a_bad_point() {
    let dir = workspace(WORKSPACE);
    assert_eq!(code(&run(dir.path(), &["greet", "world"])), 0);

    let forked = run(dir.path(), &["session", "fork", "@last", "--at", "1"]);
    assert_eq!(code(&forked), 0, "{}", stderr(&forked));
    assert_eq!(stdout(&forked).trim().len(), 26, "{}", stdout(&forked));

    let refused = run(dir.path(), &["session", "fork", "@last", "--at", "999"]);
    assert_eq!(code(&refused), 2, "{}", stderr(&refused));
    assert!(
        stderr(&refused).contains("cannot fork at 999"),
        "{}",
        stderr(&refused)
    );
}

/// `meow session gc` deletes by age and keeps a named session.
#[test]
fn gc_deletes_what_retention_no_longer_keeps() {
    let dir = workspace(WORKSPACE);
    assert_eq!(code(&run(dir.path(), &["greet", "one"])), 0);
    assert_eq!(code(&run(dir.path(), &["greet", "two"])), 0);

    let swept = run(dir.path(), &["session", "gc", "--keep", "1"]);
    assert_eq!(code(&swept), 0, "{}", stderr(&swept));
    assert!(
        stdout(&swept).starts_with("deleted\t"),
        "{}",
        stdout(&swept)
    );

    let left = run(dir.path(), &["session", "list"]);
    assert_eq!(stdout(&left).lines().count(), 1, "{}", stdout(&left));
}

/// A workspace with an index and a command that searches it.
const INDEXED: &str = r#"
meow.provider(name = "anthropic", kind = "anthropic", api_key = "k")
meow.model(name = "fast", provider = "anthropic", id = "m", context = 1, max_output = 1)
meow.model(
    name = "embed",
    provider = "anthropic",
    id = "voyage-3",
    context = 32000,
    max_output = 0,
    kind = "embedding",
)

meow.index(model = "embed", chunk_lines = 40, overlap = 8)

load("@std//search", "code")

def find(ctx):
    for hit in code(ctx.args.text, limit = 3):
        ctx.out.write("%s:%d" % (hit.path, hit.first_line))
    return "true"

meow.command(meow.tool(
    name = "find",
    about = "search the workspace",
    run = find,
    args = {"text": meow.arg.string(positional = 0)},
))
"#;

/// [R-TUI-071] `index` is a group, and its commands live under it
#[test]
fn the_index_commands_are_grouped() {
    let dir = workspace(INDEXED);

    let grouped = run(dir.path(), &["index", "stats"]);
    assert_eq!(code(&grouped), 0, "{}", stderr(&grouped));
    assert!(stdout(&grouped).contains("chunks"), "{}", stdout(&grouped));

    // And not at the top level.
    assert_eq!(code(&run(dir.path(), &["stats"])), 2);
}

/// [R-STAR-035] a workspace that declares no index is told to declare one
#[test]
fn an_index_command_without_a_declaration_says_so() {
    let dir = workspace(WORKSPACE);
    let output = run(dir.path(), &["index", "stats"]);

    assert_eq!(code(&output), 7, "{}", stderr(&output));
    assert!(
        stderr(&output).contains("declares no index"),
        "{}",
        stderr(&output)
    );
    assert!(
        stderr(&output).contains("meow.index"),
        "{}",
        stderr(&output)
    );
}

/// `meow index update` chunks the workspace and reports what it did.
#[test]
fn index_update_chunks_and_reports() {
    let dir = workspace(INDEXED);
    std::fs::write(dir.path().join("retry.rs"), "// the retry budget\n").unwrap();

    let first = run(dir.path(), &["index", "update"]);
    assert_eq!(code(&first), 0, "{}", stderr(&first));
    assert!(stdout(&first).contains("added\t2"), "{}", stdout(&first));

    let again = run(dir.path(), &["index", "update"]);
    assert!(
        stdout(&again).contains("unchanged\t2"),
        "{}",
        stdout(&again)
    );

    let stats = run(dir.path(), &["index", "stats"]);
    assert!(stdout(&stats).contains("embedded\t0"), "{}", stdout(&stats));
    assert!(stdout(&stats).contains("model\t-"), "{}", stdout(&stats));
}

/// [R-INDEX-041] a query before a build says the index is empty
#[test]
fn a_query_before_a_build_says_the_index_is_empty() {
    let dir = workspace(INDEXED);
    let output = run(dir.path(), &["index", "query", "retry"]);

    assert_eq!(code(&output), 2, "{}", stderr(&output));
    assert!(
        stderr(&output).contains("meow index build"),
        "{}",
        stderr(&output)
    );
}

/// [R-INDEX-052] `meow index clear` forgets the index
#[test]
fn index_clear_forgets_what_was_chunked() {
    let dir = workspace(INDEXED);
    assert_eq!(code(&run(dir.path(), &["index", "update"])), 0);

    let cleared = run(dir.path(), &["index", "clear"]);
    assert_eq!(code(&cleared), 0, "{}", stderr(&cleared));

    let stats = run(dir.path(), &["index", "stats"]);
    assert!(stdout(&stats).contains("chunks\t0"), "{}", stdout(&stats));
}

/// [R-STAR-021] `search.code` is reached with `load`, and says why when there
/// is nothing to search
#[test]
fn search_code_is_loaded_and_reports_an_empty_index() {
    let dir = workspace(INDEXED);
    let output = run(dir.path(), &["find", "retry budget"]);

    // The workspace loads and the handler runs; what fails is the search, and
    // it names the thing to do about it.
    assert_ne!(code(&output), 0);
    assert!(
        stderr(&output).contains("meow index build"),
        "{}",
        stderr(&output)
    );
}

/// A workspace naming several provider kinds.
const KINDS: &str = r#"
meow.provider(name = "anthropic", kind = "anthropic", api_key = "k")
meow.provider(name = "openai", kind = "openai", api_key = "k")
meow.provider(name = "router", kind = "openrouter", api_key = "k")
meow.provider(name = "gemini", kind = "gemini", api_key = "k")
meow.provider(name = "voyage", kind = "voyage", api_key = "k")
meow.provider(name = "local", kind = "llama", base_url = "http://127.0.0.1:1234", api_key = "k")

meow.model(name = "fast", provider = "anthropic", id = "m", context = 1, max_output = 1)
meow.model(name = "gpt", provider = "openai", id = "g", context = 1, max_output = 1)
meow.model(name = "flash", provider = "gemini", id = "f", context = 1, max_output = 1)
"#;

/// Every kind this binary knows is accepted, and its models resolve.
#[test]
fn every_provider_kind_loads() {
    let dir = workspace(KINDS);

    let checked = run(dir.path(), &["check"]);
    assert_eq!(code(&checked), 0, "{}", stderr(&checked));
    assert!(
        stdout(&checked).contains("6 providers"),
        "{}",
        stdout(&checked)
    );

    let doctor = run(dir.path(), &["doctor"]);
    assert_eq!(code(&doctor), 0, "{}", stderr(&doctor));
    assert!(
        stdout(&doctor).contains("kinds\tall known"),
        "{}",
        stdout(&doctor)
    );
}

/// A kind nothing implements is reported by name, with what there is.
#[test]
fn an_unknown_provider_kind_says_what_the_kinds_are() {
    let dir = workspace(
        r#"
meow.provider(name = "oops", kind = "nonesuch", api_key = "k")
"#,
    );

    let checked = run(dir.path(), &["check"]);
    assert_eq!(code(&checked), 7, "{}", stdout(&checked));
    assert!(stderr(&checked).contains("`oops`"), "{}", stderr(&checked));
    assert!(
        stderr(&checked).contains("nonesuch"),
        "{}",
        stderr(&checked)
    );
    assert!(
        stderr(&checked).contains("anthropic") && stderr(&checked).contains("gemini"),
        "the kinds that exist are not listed: {}",
        stderr(&checked)
    );

    // `doctor` finds the same thing and says so with the rest of the report.
    let doctor = run(dir.path(), &["doctor"]);
    assert_eq!(code(&doctor), 7, "{}", stdout(&doctor));
    assert!(
        stdout(&doctor).contains("kinds\tunknown for oops (nonesuch)"),
        "{}",
        stdout(&doctor)
    );
}

/// A declared address reaches the provider rather than its default.
#[test]
fn a_declared_base_url_is_kept() {
    let dir = workspace(KINDS);
    let checked = run(dir.path(), &["check"]);

    // Nothing here reaches the network; what matters is that a provider with
    // an address and no default still loads, which `llama` is: it has no
    // vendor endpoint to fall back to.
    assert_eq!(code(&checked), 0, "{}", stderr(&checked));
}
