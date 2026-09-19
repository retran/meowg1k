// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Turning what a person typed into one session.

use meow_core::SessionId;

use crate::Session;
use crate::error::{Result, SessionError};
use crate::log::Sessions;

/// What a person typed, once it has been recognised.
///
/// `[R-SESSION-032]` fixes the three selectors. They resolve at the moment of
/// use, never when they were written down: `@last` in a script means the most
/// recent run each time the script runs, which is the only reading that is
/// useful in a shell.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Selector {
    /// The Nth most recent session in the workspace, counting from zero.
    Last(usize),
    /// The most recent session of one agent.
    Agent(String),
    /// A full identifier, a short one, or a name.
    Literal(String),
}

impl Selector {
    /// Recognise a selector.
    ///
    /// Anything starting with `@` is a selector; `[R-SESSION-033]` forbids a
    /// name from starting with one, so the two namespaces cannot collide.
    pub fn parse(text: &str) -> Self {
        let Some(rest) = text.strip_prefix('@') else {
            return Self::Literal(text.to_owned());
        };
        if rest == "last" {
            return Self::Last(0);
        }
        if let Some(Ok(n)) = rest.strip_prefix("last-").map(str::parse::<usize>) {
            return Self::Last(n);
        }
        Self::Agent(rest.to_owned())
    }
}

impl Sessions {
    /// Give a session a name.
    ///
    /// Satisfies `[R-SESSION-033]`: unique within the workspace, and never
    /// starting with `@`, so a name can never shadow a selector.
    pub fn name_session(&self, id: &SessionId, name: &str) -> Result<()> {
        if name.starts_with('@') {
            return Err(SessionError::InvalidName {
                name: name.to_owned(),
                reason: "a name starting with @ would shadow a selector".to_owned(),
            });
        }
        if name.is_empty() {
            return Err(SessionError::InvalidName {
                name: name.to_owned(),
                reason: "a name cannot be empty".to_owned(),
            });
        }
        self.store().set_session_name(id.as_str(), name)?;
        Ok(())
    }

    /// One session, by whatever a person typed.
    ///
    /// Satisfies `[R-SESSION-031]`: a short identifier matching several
    /// sessions fails with the candidates listed. Picking the newest would be
    /// convenient right up until it deleted the wrong run.
    pub fn resolve(&self, text: &str) -> Result<Session> {
        match Selector::parse(text) {
            Selector::Last(n) => self
                .store()
                .list_sessions(None, (n + 1) as i64)?
                .into_iter()
                .nth(n)
                .map(into_session)
                .ok_or_else(|| SessionError::NotFound {
                    needle: text.to_owned(),
                }),
            Selector::Agent(agent) => self
                .store()
                .list_sessions(Some(&agent), 1)?
                .into_iter()
                .next()
                .map(into_session)
                .ok_or_else(|| SessionError::NotFound {
                    needle: text.to_owned(),
                }),
            Selector::Literal(needle) => {
                let mut found = self.store().resolve_session(&needle)?;
                match found.len() {
                    0 => Err(SessionError::NotFound { needle }),
                    // `remove` rather than `next().expect(...)`: the length
                    // check and the take cannot drift apart here.
                    1 => Ok(into_session(found.remove(0))),
                    _ => Err(SessionError::Ambiguous {
                        needle,
                        candidates: found.into_iter().map(|r| r.id).collect(),
                    }),
                }
            }
        }
    }

    /// One session, by identifier.
    pub fn session(&self, id: &SessionId) -> Result<Session> {
        self.store()
            .resolve_session(id.as_str())?
            .into_iter()
            .next()
            .map(into_session)
            .ok_or_else(|| SessionError::NotFound {
                needle: id.to_string(),
            })
    }

    /// Recent sessions, newest first.
    pub fn list(&self, agent: Option<&str>, limit: i64) -> Result<Vec<Session>> {
        Ok(self
            .store()
            .list_sessions(agent, limit)?
            .into_iter()
            .map(into_session)
            .collect())
    }
}

fn into_session(r: meow_store::SessionRow) -> Session {
    Session {
        id: SessionId::from_text(r.id),
        name: r.name,
        agent: r.agent,
        parent: r.parent_id.map(SessionId::from_text),
    }
}
