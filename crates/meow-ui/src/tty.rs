// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The inline terminal renderer.

use meow_core::view::{LiveKind, Output, ViewEvent};
use meow_core::{EventKind, StopReason, Usage};
use ratatui::backend::Backend;
use ratatui::layout::Rect;
use ratatui::style::Style;
use ratatui::text::{Line, Span};
use ratatui::widgets::{Paragraph, Widget};
use ratatui::{Terminal, TerminalOptions, Viewport};

use crate::Renderer;
use crate::plain::{cost, duration, total};
use crate::theme::{Role, Theme};

/// How tall the live region is, for the life of the process.
///
/// `[R-TUI-016]`. Fixed, because a region that grows with content reflows the
/// terminal while you are reading it. It does not grow for a prompt either:
/// the prompt goes into the transcript, which costs no reflow and leaves the
/// question where somebody can find it after answering.
pub const RUN_ROWS: u16 = 3;

/// What the live region is currently showing.
#[derive(Debug, Clone, Default)]
struct Live {
    step: u32,
    elapsed_ms: u64,
    tokens: u32,
    cost_micros: Option<u64>,
    tool: Option<String>,
    frame: usize,
    agent: String,
    session: String,
}

/// An inline viewport over the bottom rows of the terminal.
///
/// Satisfies `[R-TUI-010]` by never switching to the alternate screen, and
/// `[R-TUI-011]` by committing each finalized line into scrollback with
/// `insert_before` and never touching it again. That is also what makes
/// `[R-TUI-013]`'s second half hold: an exit the process cannot observe still
/// leaves a valid transcript, because every line above the live region was
/// already written.
pub struct Tty<B: Backend>
where
    B::Error: std::error::Error + Send + Sync + 'static,
{
    terminal: Terminal<B>,
    theme: Theme,
    live: Live,
    /// Whether a question is waiting for an answer.
    ///
    /// `[R-TUI-016]`: the live region says so while one is, because the
    /// transcript above has scrolled and a reader needs to know the run is
    /// waiting for them rather than for a model.
    waiting: bool,
    /// The text of the current step, accumulated from deltas.
    ///
    /// Committed once when the step ends: a line per token would put partial
    /// words into scrollback, and scrollback cannot be rewritten.
    pending: String,
    decisions: std::collections::HashMap<String, String>,
    finished: bool,
}

impl<B: Backend> std::fmt::Debug for Tty<B>
where
    B::Error: std::error::Error + Send + Sync + 'static,
{
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Tty")
            .field("waiting", &self.waiting)
            .field("live", &self.live)
            .finish_non_exhaustive()
    }
}

impl<B: Backend> Tty<B>
where
    B::Error: std::error::Error + Send + Sync + 'static,
{
    /// Take over the bottom [`RUN_ROWS`] rows of a terminal.
    ///
    /// # Errors
    ///
    /// Whatever the backend failed with.
    pub fn new(backend: B, theme: Theme) -> std::io::Result<Self> {
        let terminal = Terminal::with_options(
            backend,
            TerminalOptions {
                viewport: Viewport::Inline(RUN_ROWS),
            },
        )
        .map_err(std::io::Error::other)?;
        Ok(Self {
            terminal,
            theme,
            live: Live::default(),
            waiting: false,
            pending: String::new(),
            decisions: std::collections::HashMap::new(),
            finished: false,
        })
    }

    /// The terminal underneath, for a test that wants to read the buffer.
    pub fn terminal(&self) -> &Terminal<B> {
        &self.terminal
    }

    /// Where the live region is on the screen.
    ///
    /// A renderer's whole job is what ends up on the screen, and the only
    /// honest way to check that the live region did not eat the transcript is
    /// to know which rows are which.
    pub fn live_area(&mut self) -> Rect {
        self.terminal.get_frame().area()
    }

    /// Put a finalized line into scrollback.
    ///
    /// `[R-TUI-011]` and `[R-TUI-015]`: a diagnostic from the logging layer
    /// comes through here too, which is what keeps it in order with the
    /// transcript instead of tearing through the live region.
    ///
    /// # Errors
    ///
    /// Whatever the backend failed with.
    pub fn commit(&mut self, line: Line<'static>) -> std::io::Result<()> {
        self.insert(line)?;
        // Inserting scrolls the live region off its rows, so it has to be
        // painted again or a diagnostic leaves a blank strip where the
        // progress was.
        self.redraw()
    }

    /// Put a line into scrollback without repainting the live region.
    ///
    /// Used when several lines go in at once: repainting between them costs a
    /// frame each and shows nothing a reader can see.
    fn insert(&mut self, line: Line<'static>) -> std::io::Result<()> {
        self.terminal
            .insert_before(1, |buf| {
                Paragraph::new(line).render(buf.area, buf);
            })
            .map_err(std::io::Error::other)?;
        Ok(())
    }

    fn commit_text(&mut self, text: &str, role: Role) -> std::io::Result<()> {
        let style = self.theme.style(role);
        for piece in text.lines() {
            let line = Line::from(vec![
                Span::raw(format!("{} ", self.theme.branch())),
                Span::styled(format!("{}{piece}", role.sigil()), style),
            ]);
            self.insert(line)?;
        }
        self.redraw()
    }

    /// Redraw the live region, and only the live region.
    ///
    /// `[R-TUI-012]`: the current tool, the elapsed time, the step count, and
    /// the budget consumed. `[R-TUI-014]`: a resize reflows this and nothing
    /// above it, because nothing above it is ours any more.
    fn redraw(&mut self) -> std::io::Result<()> {
        if self.finished {
            return Ok(());
        }
        let theme = self.theme;
        let live = self.live.clone();
        let waiting = self.waiting;
        self.terminal
            .draw(|frame| {
                let area = frame.area();
                render_live(frame.buffer_mut(), area, &live, theme, waiting);
            })
            .map_err(std::io::Error::other)?;
        Ok(())
    }
}

fn render_live(
    buffer: &mut ratatui::buffer::Buffer,
    area: Rect,
    live: &Live,
    theme: Theme,
    waiting: bool,
) {
    let spinner = if waiting {
        "?"
    } else {
        theme.spinner(live.frame)
    };
    let tool = if waiting {
        "waiting for your answer"
    } else {
        live.tool.as_deref().unwrap_or("thinking")
    };

    let lines = vec![
        Line::from(vec![
            Span::styled(format!("{spinner} "), theme.style(Role::Emphasis)),
            Span::styled(live.agent.clone(), theme.style(Role::Emphasis)),
            Span::styled(
                format!("  session {}", live.session),
                theme.style(Role::Muted),
            ),
        ]),
        Line::from(Span::styled(format!("   {tool}"), theme.style(Role::Plain))),
        Line::from(Span::styled(
            format!(
                "   step {} · {} tok · {} · {}",
                live.step,
                live.tokens,
                cost(&Usage {
                    prompt: 0,
                    completion: 0,
                    cached: None,
                    cost_micros: live.cost_micros,
                }),
                duration(live.elapsed_ms)
            ),
            theme.style(Role::Muted),
        )),
    ];

    Paragraph::new(lines)
        .style(Style::default())
        .render(area, buffer);
}

impl<B: Backend> Renderer for Tty<B>
where
    B::Error: std::error::Error + Send + Sync + 'static,
{
    fn event(&mut self, event: &ViewEvent) -> std::io::Result<()> {
        match event {
            ViewEvent::Live(LiveKind::Schema { .. }) => Ok(()),

            ViewEvent::Live(LiveKind::RunStart {
                agent,
                model,
                session,
            }) => {
                self.live.agent = agent.clone();
                self.live.session = session.clone();
                let style = self.theme.style(Role::Emphasis);
                let muted = self.theme.style(Role::Muted);
                self.commit(Line::from(vec![
                    Span::styled(agent.clone(), style),
                    Span::styled(format!("  model={model}  session={session}"), muted),
                ]))?;
                self.redraw()
            }

            ViewEvent::Live(LiveKind::StepStart { step }) => {
                // The previous step's text is finalized the moment a new step
                // begins, which is the last point at which it can still change.
                let pending = std::mem::take(&mut self.pending);
                if !pending.trim().is_empty() {
                    self.commit_text(pending.trim_end(), Role::Plain)?;
                }
                self.live.step = *step;
                self.live.tool = None;
                self.redraw()
            }

            ViewEvent::Live(LiveKind::TextDelta { delta }) => {
                self.pending.push_str(delta);
                Ok(())
            }

            // Reasoning is shown in the live region while it arrives and is
            // not committed: it is the model's working, not its answer, and
            // scrollback is for what a reader comes back to.
            ViewEvent::Live(LiveKind::ThinkingDelta { .. }) => Ok(()),

            ViewEvent::Live(LiveKind::ToolStart { id, name, args }) => {
                self.live.tool = Some(name.clone());
                self.live.frame = self.live.frame.wrapping_add(1);
                let decision = self
                    .decisions
                    .remove(id)
                    .map(|d| format!("  [{d}]"))
                    .unwrap_or_default();
                let plain = self.theme.style(Role::Plain);
                let muted = self.theme.style(Role::Muted);
                self.commit(Line::from(vec![
                    Span::raw(format!("{} ", self.theme.branch())),
                    Span::styled(name.clone(), plain),
                    Span::styled(format!(" {}", truncate(args, 60)), muted),
                    Span::styled(decision, muted),
                ]))?;
                self.redraw()
            }

            ViewEvent::Live(LiveKind::ToolEnd {
                name,
                duration_ms,
                error,
                ..
            }) => {
                self.live.tool = None;
                if let Some(error) = error {
                    self.commit_text(
                        &format!("{name} failed after {duration_ms}ms: {error}"),
                        Role::Failure,
                    )?;
                }
                self.redraw()
            }

            ViewEvent::Live(LiveKind::Output(output)) => self.output(output),

            ViewEvent::Live(LiveKind::Progress {
                step,
                elapsed_ms,
                tokens,
                cost_micros,
                tool,
            }) => {
                self.live.step = *step;
                self.live.elapsed_ms = *elapsed_ms;
                self.live.tokens = *tokens;
                self.live.cost_micros = *cost_micros;
                self.live.tool = tool.clone();
                self.live.frame = self.live.frame.wrapping_add(1);
                self.redraw()
            }

            ViewEvent::Live(LiveKind::RunEnd {
                stop,
                detail,
                steps,
                usage,
                elapsed_ms,
                session,
            }) => {
                let pending = std::mem::take(&mut self.pending);
                if !pending.trim().is_empty() {
                    self.commit_text(pending.trim_end(), Role::Plain)?;
                }

                // `[R-TUI-013]`: the live region becomes a final line naming
                // the stop reason, and that line is committed rather than
                // drawn, so it survives whatever happens next.
                let role = if *stop == StopReason::Finished {
                    Role::Success
                } else {
                    Role::Warning
                };
                let detail = detail
                    .as_ref()
                    .map(|d| format!(" ({d})"))
                    .unwrap_or_default();
                let style = self.theme.style(role);
                let footer = format!(
                    "{} {stop}{detail} · {steps} steps · {} tok · {} · {} · session {session}",
                    self.theme.last_branch(),
                    total(usage),
                    cost(usage),
                    duration(*elapsed_ms)
                );
                self.commit(Line::from(Span::styled(footer, style)))?;

                self.finished = true;
                self.terminal.clear().map_err(std::io::Error::other)?;
                self.terminal.flush().map_err(std::io::Error::other)
            }

            ViewEvent::Logged(EventKind::Policy {
                id, decision, rule, ..
            }) => {
                let described = match rule {
                    Some(rule) => format!("{decision}: {rule}"),
                    None => decision.clone(),
                };
                self.decisions.insert(id.clone(), described);
                Ok(())
            }

            ViewEvent::Logged(EventKind::Note { level, message }) => {
                let role = match level.as_str() {
                    "error" => Role::Failure,
                    "warn" => Role::Warning,
                    _ => Role::Muted,
                };
                self.commit_text(message, role)
            }

            ViewEvent::Logged(_) => Ok(()),
        }
    }

    fn prompt_open(&mut self, lines: &[String]) -> std::io::Result<()> {
        // `[R-TUI-060]`: committed, so it appears below everything already in
        // the transcript and stays there once it is answered. A permission
        // decision that vanishes when the prompt closes cannot be checked
        // afterwards.
        let theme = self.theme;
        for (i, line) in lines.iter().enumerate() {
            let role = if i == 0 { Role::Warning } else { Role::Plain };
            let rendered = Line::from(Span::styled(line.clone(), theme.style(role)));
            self.insert(rendered)?;
        }
        self.waiting = true;
        self.redraw()
    }

    fn prompt_close(&mut self) -> std::io::Result<()> {
        self.waiting = false;
        self.redraw()
    }

    fn finish(&mut self) -> std::io::Result<()> {
        if !self.finished {
            // An exit the process can observe but the engine did not report:
            // the live region still has to go, or the shell prompt lands on
            // top of a half-drawn frame.
            self.finished = true;
            self.terminal.clear().map_err(std::io::Error::other)?;
        }
        self.terminal.flush().map_err(std::io::Error::other)
    }
}

impl<B: Backend> Tty<B>
where
    B::Error: std::error::Error + Send + Sync + 'static,
{
    fn output(&mut self, output: &Output) -> std::io::Result<()> {
        match output {
            Output::Write { text } | Output::Markdown { text } => {
                self.commit_text(text, Role::Plain)
            }
            Output::Note { text } => self.commit_text(text, Role::Muted),
            Output::Warn { text } => self.commit_text(text, Role::Warning),
            Output::Error { text } => self.commit_text(text, Role::Failure),
            Output::Step { text } => self.commit_text(text, Role::Emphasis),
            Output::Table { columns, rows } => {
                let mut buffer = Vec::new();
                crate::plain::write_table(&mut buffer, columns, rows)?;
                let text = String::from_utf8_lossy(&buffer).into_owned();
                self.commit_text(&text, Role::Plain)
            }
            Output::Diff { patch } => {
                for line in patch.lines() {
                    let role = match line.as_bytes().first() {
                        Some(b'+') => Role::Success,
                        Some(b'-') => Role::Failure,
                        Some(b'@') => Role::Emphasis,
                        _ => Role::Muted,
                    };
                    self.commit_text(line, role)?;
                }
                Ok(())
            }
            Output::Finding {
                severity,
                location,
                summary,
            } => {
                let role = match severity.as_str() {
                    "high" => Role::Failure,
                    "medium" => Role::Warning,
                    _ => Role::Muted,
                };
                self.commit_text(&format!("{severity} {location}  {summary}"), role)
            }
            Output::Json { value } => self.commit_text(&value.to_string(), Role::Plain),
        }
    }
}

/// Cut a long argument list down to something that fits a line.
fn truncate(text: &str, max: usize) -> String {
    if text.chars().count() <= max {
        return text.to_owned();
    }
    let head: String = text.chars().take(max.saturating_sub(3)).collect();
    format!("{head}...")
}
