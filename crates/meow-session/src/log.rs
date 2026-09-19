// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The append-only log itself.

use meow_core::{EventKind, SessionId, StopReason, Usage};
use meow_store::Store;

use crate::error::{Result, SessionError};
use crate::{Session, State};

/// How often a running writer says it is alive, in seconds.
///
/// `[R-SESSION-043]`. A session whose heartbeat is older than three of these
/// is treated as dead, so the window in which a crashed run still looks alive
/// is bounded and tunable, which is what a process identifier plus a start
/// time could not give without per-platform code.
pub const HEARTBEAT_SECS: i64 = 10;

/// One event, read back.
#[derive(Debug, Clone)]
pub struct Event {
    /// Its position in the session, from one.
    pub seq: u64,
    /// When it was recorded, in seconds since the Unix epoch.
    pub at: i64,
    /// What it records.
    pub kind: EventKind,
}

/// The session layer, over one workspace's store.
#[derive(Debug)]
pub struct Sessions {
    store: Store,
}

impl Sessions {
    /// Open the session layer over a store.
    pub fn new(store: Store) -> Self {
        Self { store }
    }

    /// Borrow the underlying store.
    pub fn store(&self) -> &Store {
        &self.store
    }

    /// Borrow the underlying store mutably.
    pub fn store_mut(&mut self) -> &mut Store {
        &mut self.store
    }

    /// Create a session and open its first run.
    ///
    /// Satisfies `[R-SESSION-005]`: a session holds one or more runs, and each
    /// begins with a `Started` event. Satisfies `[R-SESSION-060]` by recording
    /// the parent, and `[R-SESSION-061]` by leaving the foreign key to refuse
    /// a parent that does not exist, which is what makes a cycle impossible
    /// rather than merely unlikely.
    pub fn start(
        &self,
        id: &SessionId,
        agent: &str,
        task: &str,
        parent: Option<&SessionId>,
    ) -> Result<Session> {
        let row = self
            .store
            .create_session(id.as_str(), agent, parent.map(SessionId::as_str))?;
        self.append(
            id,
            EventKind::Started {
                task: task.to_owned(),
                agent: agent.to_owned(),
            },
        )?;
        self.store
            .set_state_cache(id.as_str(), State::Running.as_str())?;
        self.store.heartbeat(id.as_str())?;
        Ok(Session {
            id: id.clone(),
            name: row.name,
            agent: row.agent,
            parent: row.parent_id.map(SessionId::from_text),
        })
    }

    /// Open another run on an existing session.
    ///
    /// Satisfies `[R-SESSION-006]`: resuming appends a new `Started` event, so
    /// a session that has already ended continues without rewriting the event
    /// that ended it. Satisfies `[R-SESSION-050]` by growing the same session
    /// and the same sequence rather than creating a new one.
    pub fn resume(&self, id: &SessionId, task: &str) -> Result<()> {
        if self.state(id)? == State::Running {
            return Err(SessionError::Lifecycle(format!(
                "session {} has a run still open; finish it before resuming",
                id.short()
            )));
        }
        let agent = self.session(id)?.agent;
        self.append(
            id,
            EventKind::Started {
                task: task.to_owned(),
                agent,
            },
        )?;
        self.store
            .set_state_cache(id.as_str(), State::Running.as_str())?;
        Ok(())
    }

    /// Close the open run.
    ///
    /// Satisfies `[R-SESSION-005]`: exactly one `Finished` closes a run, and
    /// it must come before the next `Started`.
    pub fn finish(&self, id: &SessionId, stop: StopReason, detail: Option<&str>) -> Result<()> {
        if self.state(id)? != State::Running {
            return Err(SessionError::Lifecycle(format!(
                "session {} has no open run to finish",
                id.short()
            )));
        }
        self.append(
            id,
            EventKind::Finished {
                stop,
                detail: detail.map(str::to_owned),
            },
        )?;
        self.store.set_state_cache(id.as_str(), stop.as_str())?;
        Ok(())
    }

    /// Append one event.
    ///
    /// Satisfies `[R-SESSION-001]` and `[R-SESSION-002]`: the log only grows,
    /// and the sequence is monotonic and gapless because the store allocates
    /// it from the current maximum under the write lock.
    pub fn append(&self, id: &SessionId, kind: EventKind) -> Result<u64> {
        let seq = self.store.next_seq(id.as_str())?;
        let body = serde_json::to_string(&kind).map_err(|source| SessionError::Decode {
            seq: seq as u64,
            source,
        })?;
        self.store.append(id.as_str(), seq, kind.name(), &body)?;

        // The typed side tables, for the two things read on every rebuild.
        match &kind {
            EventKind::Usage(u) => self.store.put_usage(
                id.as_str(),
                seq,
                i64::from(u.prompt),
                i64::from(u.completion),
                u.cached.map(i64::from),
                u.cost_micros.map(|c| c as i64),
            )?,
            EventKind::Compaction {
                supersedes,
                tokens_saved,
                ..
            } => self.store.put_compaction(
                id.as_str(),
                seq,
                *supersedes.start() as i64,
                *supersedes.end() as i64,
                i64::from(*tokens_saved),
            )?,
            _ => {}
        }
        Ok(seq as u64)
    }

    /// Record a summarised range.
    ///
    /// Satisfies `[R-SESSION-010]`: the events in the range stay where they
    /// are. Satisfies `[R-SESSION-013]` by refusing a range that an earlier
    /// compaction already covers, which would otherwise make the two rebuilds
    /// disagree about what a model saw.
    pub fn compact(
        &self,
        id: &SessionId,
        supersedes: std::ops::RangeInclusive<u64>,
        summary: &str,
        tokens_saved: u32,
    ) -> Result<u64> {
        for existing in self.store.compactions(id.as_str())? {
            let overlaps = *supersedes.start() as i64 <= existing.to_seq
                && *supersedes.end() as i64 >= existing.from_seq;
            if overlaps {
                return Err(SessionError::AlreadySuperseded {
                    from: *supersedes.start(),
                    to: *supersedes.end(),
                    existing: existing.seq as u64,
                });
            }
        }
        self.append(
            id,
            EventKind::Compaction {
                supersedes,
                summary: summary.to_owned(),
                tokens_saved,
            },
        )
    }

    /// Every event, as it was written.
    ///
    /// Satisfies `[R-SESSION-012]`: this is the rebuild for a person, and it
    /// returns the originals. A superseded event is still here, which is what
    /// makes compaction reversible and the transcript honest.
    pub fn events(&self, id: &SessionId) -> Result<Vec<Event>> {
        let mut out = Vec::new();
        for row in self.store.events(id.as_str())? {
            let kind: EventKind =
                serde_json::from_str(&row.body).map_err(|source| SessionError::Decode {
                    seq: row.seq as u64,
                    source,
                })?;
            out.push(Event {
                seq: row.seq as u64,
                at: row.at,
                kind,
            });
        }
        Ok(out)
    }

    /// The events a model should see next.
    ///
    /// Satisfies `[R-SESSION-011]`: every superseded range is skipped and its
    /// summary substituted. Written as its own walk rather than as a flag on
    /// [`Sessions::events`], because the two readings diverge and a shared
    /// function with a boolean is how they would drift back together.
    pub fn events_for_model(&self, id: &SessionId) -> Result<Vec<Event>> {
        let ranges = self.store.compactions(id.as_str())?;
        let mut out = Vec::new();
        for event in self.events(id)? {
            let superseded = ranges
                .iter()
                .any(|r| event.seq as i64 >= r.from_seq && event.seq as i64 <= r.to_seq);
            if !superseded {
                out.push(event);
            }
        }
        Ok(out)
    }

    /// What state a session is in.
    ///
    /// Satisfies `[R-SESSION-040]` and `[R-SESSION-041]`: the answer comes
    /// from the log, never from the cached copy, so no stored field can
    /// disagree with its own history.
    pub fn state(&self, id: &SessionId) -> Result<State> {
        match self.store.last_event(id.as_str())? {
            None => Ok(State::Running),
            Some(row) if row.kind == "Finished" => {
                let kind: EventKind =
                    serde_json::from_str(&row.body).map_err(|source| SessionError::Decode {
                        seq: row.seq as u64,
                        source,
                    })?;
                match kind {
                    EventKind::Finished { stop, .. } => Ok(State::Stopped(stop)),
                    _ => Ok(State::Running),
                }
            }
            Some(_) => Ok(State::Running),
        }
    }

    /// Say the writer of a session is still alive.
    ///
    /// `[R-SESSION-043]`.
    pub fn heartbeat(&self, id: &SessionId) -> Result<()> {
        self.store.heartbeat(id.as_str())?;
        Ok(())
    }

    /// Close a session whose writer stopped saying it was alive.
    ///
    /// Satisfies `[R-SESSION-042]`: a run whose process died leaves no
    /// `Finished` event, and a session that claims to be running forever is
    /// worse than one that admits it failed. Returns whether it acted.
    pub fn reap_if_dead(&self, id: &SessionId, now: i64) -> Result<bool> {
        if self.state(id)? != State::Running {
            return Ok(false);
        }
        let Some(beat) = self.store.heartbeat_at(id.as_str())? else {
            return Ok(false);
        };
        if now - beat <= HEARTBEAT_SECS * 3 {
            return Ok(false);
        }
        self.append(
            id,
            EventKind::Finished {
                stop: StopReason::Failed,
                detail: Some("process exited".to_owned()),
            },
        )?;
        self.store
            .set_state_cache(id.as_str(), StopReason::Failed.as_str())?;
        Ok(true)
    }

    /// What a session spent, including everything its children spent.
    ///
    /// Satisfies `[R-SESSION-022]`.
    pub fn usage(&self, id: &SessionId) -> Result<Usage> {
        let t = self.store.usage_totals(id.as_str())?;
        let mut total = Usage {
            prompt: t.prompt as u32,
            completion: t.completion as u32,
            cached: t.cached.map(|c| c as u32),
            cost_micros: t.cost_micros.map(|c| c as u64),
        };
        for child in self.children(id)? {
            total = total.add(&self.usage(&child.id)?);
        }
        Ok(total)
    }

    /// The children of a session, oldest first.
    ///
    /// `[R-SESSION-060]`.
    pub fn children(&self, id: &SessionId) -> Result<Vec<Session>> {
        Ok(self
            .store
            .children(id.as_str())?
            .into_iter()
            .map(|r| Session {
                id: SessionId::from_text(r.id),
                name: r.name,
                agent: r.agent,
                parent: r.parent_id.map(SessionId::from_text),
            })
            .collect())
    }
}
