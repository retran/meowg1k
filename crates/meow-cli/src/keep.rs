// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! `@std//store`, over the workspace database.

use meow_star::Workspace;
use meow_star::port::Keep;
use serde_json::Value;

/// The workspace's durable key-value table.
///
/// `[R-STAR-026]`: the table is the workspace's rather than a session's, so
/// `meow session gc` cannot take it with them. `meow-store` already keeps it
/// in a separate table for exactly that reason, and this is the thin layer
/// that turns bytes into the values a handler put in.
pub struct Durable {
    /// The lock is not for contention. `rusqlite::Connection` is not `Sync`,
    /// and a handler or a tool calls this from whichever thread it is on.
    store: std::sync::Mutex<meow_store::Store>,
}

impl std::fmt::Debug for Durable {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Durable").finish_non_exhaustive()
    }
}

impl Durable {
    /// Open the workspace database.
    ///
    /// # Errors
    ///
    /// Whatever opening it said.
    pub fn open(workspace: &Workspace) -> Result<Self, meow_store::StoreError> {
        Ok(Self {
            store: std::sync::Mutex::new(meow_store::Store::open(workspace.root())?),
        })
    }

    fn held(&self) -> Result<std::sync::MutexGuard<'_, meow_store::Store>, String> {
        self.store
            .lock()
            .map_err(|_| "the store is not usable in this run".to_owned())
    }
}

impl Keep for Durable {
    fn get(&self, key: &str) -> Result<Option<Value>, String> {
        let store = self.held()?;
        let Some(bytes) = store.kv_get(key).map_err(|e| e.to_string())? else {
            return Ok(None);
        };
        // A row this module did not write, or one from a version that encoded
        // differently. Saying so beats handing back a value nobody stored.
        serde_json::from_slice(&bytes)
            .map_err(|e| format!("`{key}` does not hold a value this run can read: {e}"))
    }

    fn put(&self, key: &str, value: &Value) -> Result<(), String> {
        let bytes = serde_json::to_vec(value).map_err(|e| e.to_string())?;
        self.held()?.kv_put(key, &bytes).map_err(|e| e.to_string())
    }

    fn delete(&self, key: &str) -> Result<bool, String> {
        let store = self.held()?;
        // `[R-STAR-028]`: the caller wants to know whether it was there, and
        // `kv_delete` does not say, so ask first. Both statements run on one
        // connection, so nothing can slip between them.
        let was = store.kv_get(key).map_err(|e| e.to_string())?.is_some();
        store.kv_delete(key).map_err(|e| e.to_string())?;
        Ok(was)
    }

    fn keys(&self, prefix: &str) -> Result<Vec<String>, String> {
        let keys = self.held()?.kv_keys().map_err(|e| e.to_string())?;
        Ok(keys
            .into_iter()
            .filter(|key| key.starts_with(prefix))
            .collect())
    }
}
