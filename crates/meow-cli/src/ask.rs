// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Asking a person something, and refusing to when there is nobody there.

use std::io::{BufRead, IsTerminal};
use std::sync::Arc;

use meow_agent::Approver;
use meow_policy::{Answer, Prompt};
use meow_star::port::{Ask, AskError};

use crate::render::Sink;

/// Whether this run may ask anything at all.
///
/// `[R-TUI-051]` and `[R-POLICY-020]` are the same rule seen from two sides:
/// with no terminal, or with `--yes`, a question has no one to answer it, so
/// asking fails and an `ask` decision becomes a denial. Neither may block, and
/// neither may become more permissive.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Interaction {
    /// Somebody is there.
    Possible,
    /// Standard input is not a terminal.
    NoTerminal,
    /// `--yes` was given.
    Unattended,
}

impl Interaction {
    /// Work out whether anybody can be asked.
    pub fn detect(yes: bool) -> Self {
        if yes {
            // Checked before the terminal, so `--yes` in an interactive shell
            // still means unattended. The flag is a statement about how the
            // run should behave, not about the terminal.
            return Self::Unattended;
        }
        if std::io::stdin().is_terminal() {
            Self::Possible
        } else {
            Self::NoTerminal
        }
    }

    fn refusal(self, what: &'static str) -> Option<AskError> {
        match self {
            Self::Possible => None,
            Self::NoTerminal => Some(AskError::NotATerminal { what }),
            // `--yes` is not consent. `[R-TUI-073]`: it makes every decision
            // that needs a person resolve the safe way, and never the other.
            Self::Unattended => Some(AskError::Declined),
        }
    }
}

/// Reads answers from the terminal, through the renderer.
pub struct Terminal {
    interaction: Interaction,
    sink: Arc<Sink>,
    /// Tools a person has approved for the rest of the process.
    ///
    /// `[R-POLICY-023]` and `[R-TUI-062]`: here and nowhere else. Nothing
    /// writes this back to a file, so a permission granted in a hurry does not
    /// outlive the terminal it was granted in.
    granted: std::sync::Mutex<std::collections::BTreeSet<String>>,
}

impl std::fmt::Debug for Terminal {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Terminal")
            .field("interaction", &self.interaction)
            .finish_non_exhaustive()
    }
}

impl Terminal {
    /// Ask through this renderer.
    pub fn new(interaction: Interaction, sink: Arc<Sink>) -> Self {
        Self {
            interaction,
            sink,
            granted: std::sync::Mutex::new(std::collections::BTreeSet::new()),
        }
    }

    /// Put a question up, wait for a line, and take the question down.
    fn question(&self, lines: &[String]) -> Result<String, AskError> {
        self.sink.prompt_open(lines);
        let answer = read_line();
        self.sink.prompt_close();
        answer.ok_or(AskError::Declined)
    }
}

/// One line from standard input, or nothing if it closed.
fn read_line() -> Option<String> {
    let mut line = String::new();
    match std::io::stdin().lock().read_line(&mut line) {
        Ok(0) | Err(_) => None,
        Ok(_) => Some(line.trim().to_owned()),
    }
}

impl Ask for Terminal {
    fn text(&self, prompt: &str, default: Option<&str>) -> Result<String, AskError> {
        if let Some(e) = self.interaction.refusal("ctx.ask.text") {
            return Err(e);
        }
        let shown = match default {
            Some(default) => format!("{prompt} [{default}]"),
            None => prompt.to_owned(),
        };
        let answer = self.question(&[shown])?;
        match (answer.is_empty(), default) {
            (true, Some(default)) => Ok(default.to_owned()),
            (true, None) => Err(AskError::Declined),
            _ => Ok(answer),
        }
    }

    fn confirm(&self, prompt: &str, default: bool) -> Result<bool, AskError> {
        if let Some(e) = self.interaction.refusal("ctx.ask.confirm") {
            return Err(e);
        }
        let hint = if default { "[Y/n]" } else { "[y/N]" };
        let answer = self.question(&[format!("{prompt} {hint}")])?;
        Ok(match answer.to_ascii_lowercase().as_str() {
            "y" | "yes" => true,
            "n" | "no" => false,
            _ => default,
        })
    }

    fn select(&self, prompt: &str, choices: &[String]) -> Result<String, AskError> {
        if let Some(e) = self.interaction.refusal("ctx.ask.select") {
            return Err(e);
        }
        if choices.is_empty() {
            return Err(AskError::Declined);
        }

        let mut lines = vec![prompt.to_owned()];
        for (i, choice) in choices.iter().enumerate() {
            lines.push(format!("  {}) {choice}", i + 1));
        }
        lines.push("number:".to_owned());

        let answer = self.question(&lines)?;
        answer
            .parse::<usize>()
            .ok()
            .and_then(|n| choices.get(n.wrapping_sub(1)))
            .cloned()
            .ok_or(AskError::Declined)
    }
}

impl Approver for Terminal {
    fn ask(&self, prompt: &Prompt) -> Answer {
        // An "always" already given answers without asking again, which is the
        // whole point of the answer.
        if self
            .granted
            .lock()
            .is_ok_and(|granted| granted.contains(&prompt.tool))
        {
            return Answer::Once;
        }

        if self.interaction.refusal("approval").is_some() {
            return Answer::Deny;
        }

        let lines = describe(prompt);
        let Ok(answer) = self.question(&lines) else {
            // Standard input closed mid-question. Nobody said yes.
            return Answer::Deny;
        };

        let chosen = answer
            .chars()
            .next()
            .and_then(Answer::from_key)
            // An unreadable answer to a security question is not a yes.
            .unwrap_or(Answer::Deny);

        if chosen == Answer::Always
            && let Ok(mut granted) = self.granted.lock()
        {
            granted.insert(prompt.tool.clone());
        }

        chosen
    }
}

/// What the prompt shows.
///
/// `[R-TUI-061]` and `[R-POLICY-021]`: the tool, the arguments verbatim, the
/// rule that caused the question, and which agent asked at which step. The
/// arguments are not summarised, because a prompt that paraphrases what it is
/// approving asks for consent to something the reader did not see.
pub fn describe(prompt: &Prompt) -> Vec<String> {
    let offered = Answer::ALL
        .iter()
        .map(|a| format!("[{}] {}", a.key(), a.label()))
        .collect::<Vec<_>>()
        .join("  ");

    let mut lines = vec![
        format!("allow {}?", prompt.tool),
        format!("  arguments  {}", prompt.arguments),
        format!("  rule       {}", prompt.rule),
    ];
    if let Some(origin) = &prompt.origin {
        lines.push(format!("  declared   {origin}"));
    }
    lines.push(format!(
        "  asked by   {} at step {}",
        prompt.agent, prompt.step
    ));
    lines.push(offered);
    lines
}

/// What was piped in.
#[derive(Debug, Clone, Copy)]
pub struct Stdin;

impl meow_star::port::Stdin for Stdin {
    fn is_piped(&self) -> bool {
        !std::io::stdin().is_terminal()
    }

    fn read(&self) -> std::io::Result<String> {
        use std::io::Read;
        let mut text = String::new();
        std::io::stdin().lock().read_to_string(&mut text)?;
        Ok(text)
    }
}

/// Read a secret from the terminal.
///
/// `[R-AUTH-013]`: `meow auth login` takes a key from a person, and the
/// alternative - a `--key` flag - puts it in shell history and in the process
/// list. The flag exists for scripts that already have the key somewhere
/// safer; this is the path a person should take.
///
/// There is no echo suppression here. Doing it portably means a terminal
/// crate and raw mode, and a half-done version that echoes on one platform is
/// worse than not promising it at all, so the prompt says what will happen.
///
/// # Errors
///
/// [`AskError::NotATerminal`] when there is nobody to type it, because a
/// pipeline silently reading a blank key would store one.
pub fn secret(prompt: &str) -> Result<String, AskError> {
    use std::io::Write;

    if !std::io::stdin().is_terminal() {
        return Err(AskError::NotATerminal {
            what: "meow auth login",
        });
    }

    eprint!("{prompt} (it will be visible as you type): ");
    let _ = std::io::stderr().flush();

    let mut line = String::new();
    std::io::stdin()
        .lock()
        .read_line(&mut line)
        .map_err(|_| AskError::Declined)?;

    Ok(line.trim_end_matches(['\r', '\n']).to_owned())
}

#[cfg(test)]
mod tests {
    use super::{Interaction, describe};
    use meow_policy::{Answer, Prompt};
    use meow_star::port::AskError;

    fn prompt() -> Prompt {
        Prompt {
            tool: "shell".to_owned(),
            arguments: r#"{"command":"rm -rf build"}"#.to_owned(),
            rule: "shell (ask)".to_owned(),
            origin: Some("meow.star:14".to_owned()),
            agent: "reviewer".to_owned(),
            step: 3,
        }
    }

    /// [R-TUI-061] the prompt shows the tool, the arguments verbatim, the
    /// rule, and who asked at which step
    #[test]
    fn the_prompt_shows_what_is_being_approved() {
        let shown = describe(&prompt()).join("\n");

        assert!(shown.contains("shell"), "{shown}");
        assert!(
            shown.contains(r#"{"command":"rm -rf build"}"#),
            "the arguments are summarised rather than shown: {shown}"
        );
        assert!(shown.contains("shell (ask)"), "{shown}");
        assert!(shown.contains("meow.star:14"), "{shown}");
        assert!(shown.contains("reviewer at step 3"), "{shown}");
    }

    /// [R-TUI-061] all four answers are offered, and each has a key
    #[test]
    fn all_four_answers_are_offered() {
        let shown = describe(&prompt()).join("\n");

        for answer in Answer::ALL {
            assert!(
                shown.contains(answer.label()),
                "`{}` is not offered: {shown}",
                answer.label()
            );
        }
        assert_eq!(Answer::ALL.len(), 4);

        // And each key reads back as the answer it belongs to, so the prompt
        // and the parser cannot disagree.
        for answer in Answer::ALL {
            assert_eq!(Answer::from_key(answer.key()), Some(answer));
        }
    }

    /// [R-TUI-051] [R-TUI-073] `--yes` means unattended even in a terminal,
    /// and never means yes
    #[test]
    fn yes_is_unattended_and_is_not_consent() {
        assert_eq!(Interaction::detect(true), Interaction::Unattended);

        let refusal = Interaction::Unattended.refusal("ctx.ask.confirm");
        assert!(matches!(refusal, Some(AskError::Declined)));

        // Nothing resolves to an approval.
        assert!(Interaction::Unattended.refusal("approval").is_some());
    }

    /// [R-TUI-051] with no terminal, asking fails and says why
    #[test]
    fn no_terminal_refuses_and_names_the_call() {
        let refusal = Interaction::NoTerminal.refusal("ctx.ask.text");
        match refusal {
            Some(AskError::NotATerminal { what }) => assert_eq!(what, "ctx.ask.text"),
            other => panic!("expected a terminal refusal, got {other:?}"),
        }
    }

    /// An unreadable answer to a security question is not a yes.
    #[test]
    fn an_unrecognised_key_is_not_an_approval() {
        for key in ['x', '1', ' ', 'Z'] {
            assert_eq!(Answer::from_key(key), None, "`{key}` was read as an answer");
        }
    }
}
