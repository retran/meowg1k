// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! Deleting old sessions, whole and in the right order.

use meow_core::SessionId;

use crate::error::Result;
use crate::log::Sessions;

/// What to keep.
///
/// `[R-SESSION-082]`: age, count, and total size, and the strictest of the
/// configured limits wins. Strictest rather than first-matching, because three
/// limits that each delete a different set would otherwise depend on the order
/// they were checked in, which nobody could predict from the configuration.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub struct Retention {
    /// Delete anything older than this many seconds.
    pub max_age_secs: Option<i64>,
    /// Keep at most this many sessions.
    pub max_count: Option<usize>,
    /// Delete oldest-first until the database is no larger than this.
    pub max_bytes: Option<u64>,
    /// Whether a named session may be deleted.
    ///
    /// `[R-SESSION-081]`: off unless the caller asks, because a name is how
    /// somebody said this one matters.
    pub include_named: bool,
}

/// What a sweep did.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Swept {
    /// The sessions that went, oldest first.
    pub deleted: Vec<SessionId>,
    /// Sessions a limit chose but a rule protected.
    pub kept: Vec<SessionId>,
}

impl Sessions {
    /// Delete what the retention policy no longer keeps.
    ///
    /// Satisfies `[R-SESSION-080]` by deleting whole sessions and taking a
    /// parent only together with its descendants, and `[R-SESSION-081]` by
    /// protecting named sessions unless asked. `now` is an argument for the
    /// same reason an identifier is: a clock is input, and a sweep that reads
    /// one cannot be tested.
    ///
    /// # Errors
    ///
    /// Whatever the store said.
    pub fn sweep(&mut self, retention: Retention, now: i64) -> Result<Swept> {
        let mut rows = self.store().sessions_by_age()?;
        // Oldest first: every limit here deletes from the old end, and a sweep
        // that deleted the newest run would be a surprise nobody recovers
        // from.
        rows.sort_by_key(|row| row.created_at);

        let total = rows.len();
        let mut swept = Swept::default();
        let mut candidates: Vec<String> = Vec::new();

        for (index, row) in rows.iter().enumerate() {
            let too_old = retention
                .max_age_secs
                .is_some_and(|max| now - row.created_at > max);
            let over_count = retention
                .max_count
                .is_some_and(|max| total.saturating_sub(index) > max);

            if too_old || over_count {
                candidates.push(row.id.clone());
            }
        }

        // Size is different in kind: it is not a property of one session, so
        // it keeps deleting from the old end until the file fits rather than
        // selecting a set.
        if let Some(max_bytes) = retention.max_bytes {
            let mut index = 0;
            while self.store().size_bytes()? > max_bytes && index < rows.len() {
                let id = rows[index].id.clone();
                if !candidates.contains(&id) {
                    candidates.push(id);
                }
                index += 1;
                // Deleting is what changes the size, so the check has to
                // happen after each one rather than against a plan made up
                // front.
                self.delete_tree(&candidates, &mut swept, retention.include_named)?;
                candidates.clear();
            }
        }

        self.delete_tree(&candidates, &mut swept, retention.include_named)?;
        Ok(swept)
    }

    /// Delete each session together with everything below it.
    fn delete_tree(
        &mut self,
        ids: &[String],
        swept: &mut Swept,
        include_named: bool,
    ) -> Result<()> {
        for id in ids {
            let id = SessionId::from_text(id.clone());
            if swept.deleted.contains(&id) || swept.kept.contains(&id) {
                continue;
            }

            let mut tree = vec![id.clone()];
            let mut at = 0;
            while at < tree.len() {
                let children = self.children(&tree[at])?;
                tree.extend(children.into_iter().map(|c| c.id));
                at += 1;
            }

            // A named session anywhere in the tree protects the whole tree:
            // `[R-SESSION-080]` forbids deleting a parent without its
            // descendants, so keeping one descendant means keeping all of it.
            if !include_named {
                let mut protected = None;
                for member in &tree {
                    if self.session(member)?.name.is_some() {
                        protected = Some(member.clone());
                        break;
                    }
                }
                if let Some(protected) = protected {
                    swept.kept.push(protected);
                    continue;
                }
            }

            // Leaves first: a parent with children cannot be deleted, which is
            // the foreign key refusing to orphan them.
            for member in tree.iter().rev() {
                if swept.deleted.contains(member) {
                    continue;
                }
                self.store_mut().delete_session(member.as_str())?;
                swept.deleted.push(member.clone());
            }
        }
        Ok(())
    }
}
