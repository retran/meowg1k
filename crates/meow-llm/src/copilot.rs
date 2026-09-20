// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! GitHub Copilot.
//!
//! The request shape is OpenAI's, so this is not a sixth copy of a streaming
//! parser. What differs is two things: the address, and that the credential a
//! person approved is not what is sent. A GitHub OAuth grant is exchanged for
//! a short-lived Copilot token, which expires and is renewed without asking
//! anybody again - `[R-LLM-004]` and `[R-AUTH-022]`.

use std::sync::Arc;

use crate::bearer::Exchanged;
use crate::error::Result;
use crate::openai::OpenAi;
use crate::transport::Transport;

/// Where the grant is exchanged.
const EXCHANGE: &str = "https://api.github.com/copilot_internal/v2/token";

/// Where the chat API lives.
const BASE: &str = "https://api.githubcopilot.com";

/// What Copilot wants to see on every request.
///
/// It refuses without them. The values name an editor because that is what the
/// API is for; sending them is a fact about the protocol rather than an
/// attempt to be one.
fn identity() -> Vec<(String, String)> {
    [
        ("editor-version", "Neovim/0.6.1"),
        ("editor-plugin-version", "copilot.vim/1.16.0"),
        ("copilot-integration-id", "vscode-chat"),
        ("openai-organization", "github-copilot"),
    ]
    .into_iter()
    .map(|(name, value)| (name.to_owned(), value.to_owned()))
    .collect()
}

/// A Copilot provider over this transport, using this GitHub grant.
///
/// # Errors
///
/// [`crate::LlmError::Transport`] when the exchange client cannot be built.
pub fn build<T: Transport>(transport: T, grant: impl Into<String>) -> Result<OpenAi<T>> {
    let bearer = Exchanged::new("copilot", EXCHANGE, grant, identity())?;

    Ok(OpenAi::with_bearer(transport, Arc::new(bearer))
        .with_base_url(BASE)
        .with_name("copilot")
        // Copilot takes the OpenAI body and does not honour
        // `response_format`, so a schema is asked for in the prompt and
        // checked here - `[R-LLM-050]`.
        .with_emulated_schema()
        .with_headers(identity()))
}
