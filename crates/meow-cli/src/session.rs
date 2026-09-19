// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The session behind a run, and the commands that read it.

use std::sync::Mutex;

use meow_core::{EventKind, SessionId, StopReason};
use meow_llm::{Message, Role};
use meow_session::{Sessions, State};
use serde_json::Value;

/// A session that is actually written to disk.
///
/// The lock is not for contention: a run has one writer by construction, and
/// `[R-SESSION-043]` says so. It is because `[R-SESSION-050]` numbers events
/// in sequence and the sequence is read-then-write, so two threads asking at
/// once would produce the same number.
pub struct Log {
    sessions: Mutex<Sessions>,
    id: SessionId,
    /// What the run has already seen, rebuilt once when it starts.
    ///
    /// Read once rather than on demand: the history is what the previous run
    /// saw, and an event this run writes must not appear in it.
    history: Vec<Message>,
    /// Values `ctx.session.set` stored, kept beside the log for reading back.
    values: Mutex<std::collections::BTreeMap<String, Value>>,
}

impl std::fmt::Debug for Log {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Log")
            .field("id", &self.id)
            .finish_non_exhaustive()
    }
}

impl Log {
    /// Open a session for this run: a fresh one, or the one being continued.
    ///
    /// `[R-SESSION-054]`: a run that names no session starts one. Continuation
    /// is never inferred from the workspace, which is why `resuming` is an
    /// argument and not something this works out.
    ///
    /// # Errors
    ///
    /// Whatever the session layer said.
    pub fn open(
        sessions: Sessions,
        agent: &str,
        task: &str,
        resuming: Option<SessionId>,
        now_millis: u64,
        entropy: [u8; 10],
    ) -> meow_session::Result<Self> {
        let (id, history) = match resuming {
            Some(id) => {
                // `[R-SESSION-050]`: the same session and the same sequence.
                let history = rebuild(&sessions, &id)?;
                sessions.resume(&id, task)?;
                (id, history)
            }
            None => {
                let id = SessionId::new(now_millis, entropy);
                sessions.start(&id, agent, task, None)?;
                (id, Vec::new())
            }
        };

        Ok(Self {
            sessions: Mutex::new(sessions),
            id,
            history,
            values: Mutex::new(std::collections::BTreeMap::new()),
        })
    }

    /// Which session this run is writing to.
    pub fn id(&self) -> &SessionId {
        &self.id
    }

    /// Close the run.
    pub fn finish(&self, stop: StopReason, detail: Option<&str>) {
        if let Ok(sessions) = self.sessions.lock() {
            let _ = sessions.finish(&self.id, stop, detail);
        }
    }
}

/// Turn a session's own view of itself into messages.
///
/// `[R-SESSION-051]`: through `events_for_model`, so a resumed run sees
/// compaction exactly as the original did rather than the full history the
/// original had already decided was too long.
fn rebuild(sessions: &Sessions, id: &SessionId) -> meow_session::Result<Vec<Message>> {
    let mut out = Vec::new();
    for event in sessions.events_for_model(id)? {
        match event.kind {
            EventKind::Started { task, .. } | EventKind::UserMessage { content: task } => {
                out.push(Message::new(Role::User, task));
            }
            EventKind::Assistant { content, .. } if !content.trim().is_empty() => {
                out.push(Message::new(Role::Assistant, content));
            }
            EventKind::Compaction { summary, .. } => {
                // The summary stands where the range did, which is what makes
                // the rebuild the same size the original run saw.
                out.push(Message::new(Role::Assistant, summary));
            }
            _ => {}
        }
    }
    Ok(out)
}

impl meow_star::port::Session for Log {
    fn id(&self) -> String {
        self.id.to_string()
    }

    fn get(&self, key: &str) -> Option<Value> {
        self.values.lock().ok()?.get(key).cloned()
    }

    fn set(&self, key: &str, value: Value) {
        if let Ok(mut values) = self.values.lock() {
            values.insert(key.to_owned(), value.clone());
        }
        // Into the log as well, so a value a handler stored is replayable with
        // the run that stored it rather than living only in this process.
        self.record(EventKind::Note {
            level: "state".to_owned(),
            message: format!("{key}={value}"),
        });
    }

    fn record(&self, kind: EventKind) {
        if let Ok(sessions) = self.sessions.lock() {
            let _ = sessions.append(&self.id, kind);
        }
    }

    fn history(&self) -> Vec<Message> {
        self.history.clone()
    }
}

/// Open the store for a workspace.
///
/// # Errors
///
/// Whatever opening the database said.
pub fn open(workspace: &meow_star::Workspace) -> meow_store::Result<Sessions> {
    Ok(Sessions::new(meow_store::Store::open(workspace.root())?))
}

/// The most recent session of one agent, for `--continue`.
///
/// Satisfies `[R-TUI-074]`: the most recent session of the command being
/// invoked, in this workspace, and nothing when there is none. Returning the
/// newest session of any agent would resume somebody else's run.
pub fn most_recent(sessions: &Sessions, agent: &str) -> meow_session::Result<Option<SessionId>> {
    Ok(sessions
        .list(Some(agent), 1)?
        .into_iter()
        .next()
        .map(|s| s.id))
}

/// One line per session, for `meow session list`.
pub fn describe(sessions: &Sessions, session: &meow_session::Session) -> String {
    let state = sessions.state(&session.id).unwrap_or(State::Running);
    let usage = sessions.usage(&session.id).unwrap_or_default();
    let name = session.name.as_deref().unwrap_or("-");

    format!(
        "{}\t{}\t{}\t{}\t{} tok",
        session.id.short(),
        session.agent,
        state,
        name,
        usage.prompt.saturating_add(usage.completion)
    )
}
