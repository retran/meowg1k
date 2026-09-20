// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! A credential that is asked for rather than held.
#![allow(clippy::unwrap_used)]

use std::io::{Read, Write};
use std::sync::{Arc, Mutex};

use meow_llm::{Bearer, Exchanged, Fixed, LlmError};
use tokio_util::sync::CancellationToken;

/// A server that exchanges a grant for a token, counting how often it is asked.
struct Exchange {
    port: u16,
    asked: Arc<Mutex<usize>>,
    seen: Arc<Mutex<Vec<String>>>,
    stop: Arc<std::sync::atomic::AtomicBool>,
}

impl Exchange {
    /// Answer with a token that expires this many seconds from now, or with a
    /// status when `status` is not 200.
    fn new(status: u16, expires_in: i64) -> Self {
        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();
        let asked = Arc::new(Mutex::new(0));
        let seen = Arc::new(Mutex::new(Vec::new()));
        let stop = Arc::new(std::sync::atomic::AtomicBool::new(false));

        let counted = Arc::clone(&asked);
        let recorded = Arc::clone(&seen);
        let halt = Arc::clone(&stop);
        std::thread::spawn(move || {
            for stream in listener.incoming() {
                if halt.load(std::sync::atomic::Ordering::Relaxed) {
                    return;
                }
                let Ok(mut stream) = stream else { continue };
                let mut buffer = [0_u8; 4096];
                let read = stream.read(&mut buffer).unwrap_or(0);
                recorded
                    .lock()
                    .unwrap()
                    .push(String::from_utf8_lossy(&buffer[..read]).into_owned());

                let mut count = counted.lock().unwrap();
                *count += 1;
                let nth = *count;
                drop(count);

                let now = std::time::SystemTime::now()
                    .duration_since(std::time::UNIX_EPOCH)
                    .unwrap()
                    .as_secs() as i64;
                // A different token each time, so a test can tell a renewal
                // from a reuse.
                let body = format!(r#"{{"token":"t{nth}","expires_at":{}}}"#, now + expires_in);
                let response = format!(
                    "HTTP/1.1 {status} X\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                );
                let _ = stream.write_all(response.as_bytes());
                let _ = stream.flush();
            }
        });

        Self {
            port,
            asked,
            seen,
            stop,
        }
    }

    fn url(&self) -> String {
        format!("http://127.0.0.1:{}/token", self.port)
    }

    fn asked(&self) -> usize {
        *self.asked.lock().unwrap()
    }
}

impl Drop for Exchange {
    fn drop(&mut self) {
        self.stop.store(true, std::sync::atomic::Ordering::Relaxed);
        let _ = std::net::TcpStream::connect(("127.0.0.1", self.port));
    }
}

/// [R-LLM-004] a fixed key is handed over unchanged, and asks nobody
#[tokio::test(flavor = "multi_thread")]
async fn a_fixed_key_is_what_it_was_given() {
    let bearer = Fixed::new("sk-a-key");
    let cancel = CancellationToken::new();

    assert_eq!(bearer.token(&cancel).await.unwrap(), "sk-a-key");
    assert_eq!(bearer.token(&cancel).await.unwrap(), "sk-a-key");
}

/// [R-LLM-004] a token that is still good is reused rather than re-fetched
#[tokio::test(flavor = "multi_thread")]
async fn a_token_that_has_not_expired_is_not_fetched_again() {
    let server = Exchange::new(200, 3600);
    let bearer = Exchanged::new("copilot", server.url(), "grant", Vec::new()).unwrap();
    let cancel = CancellationToken::new();

    let first = bearer.token(&cancel).await.unwrap();
    let second = bearer.token(&cancel).await.unwrap();

    assert_eq!(first, second, "a good token must be reused");
    assert_eq!(server.asked(), 1, "it was exchanged twice");
}

/// [R-AUTH-022] an expired token is renewed without asking anybody
#[tokio::test(flavor = "multi_thread")]
async fn an_expired_token_is_renewed() {
    // Expiring in ten seconds, which is inside the window that treats a token
    // as already gone, so every call renews.
    let server = Exchange::new(200, 10);
    let bearer = Exchanged::new("copilot", server.url(), "grant", Vec::new()).unwrap();
    let cancel = CancellationToken::new();

    let first = bearer.token(&cancel).await.unwrap();
    let second = bearer.token(&cancel).await.unwrap();

    assert_ne!(first, second, "an expiring token must be renewed");
    assert_eq!(server.asked(), 2);
}

/// [R-LLM-004] the grant is what is sent to the exchange
#[tokio::test(flavor = "multi_thread")]
async fn the_grant_authenticates_the_exchange() {
    let server = Exchange::new(200, 3600);
    let bearer = Exchanged::new(
        "copilot",
        server.url(),
        "the-grant",
        vec![("editor-version".to_owned(), "Neovim/0.6.1".to_owned())],
    )
    .unwrap();

    bearer.token(&CancellationToken::new()).await.unwrap();

    let seen = server.seen.lock().unwrap();
    assert!(
        seen[0].contains("token the-grant"),
        "the grant must authenticate the exchange: {}",
        seen[0]
    );
    assert!(
        seen[0]
            .to_lowercase()
            .contains("editor-version: neovim/0.6.1"),
        "the identifying headers must be sent: {}",
        seen[0]
    );
}

/// [R-AUTH-022] a refusal names the provider and the command that fixes it
#[tokio::test(flavor = "multi_thread")]
async fn a_refused_renewal_says_what_to_run() {
    let server = Exchange::new(401, 3600);
    let bearer = Exchanged::new("copilot", server.url(), "stale", Vec::new()).unwrap();

    let error = bearer.token(&CancellationToken::new()).await.unwrap_err();

    assert!(
        matches!(error, LlmError::Auth { .. }),
        "expected an auth failure, got {error:?}"
    );
    let said = error.to_string();
    assert!(
        said.contains("copilot") && said.contains("meow auth login copilot"),
        "the message must name the provider and the command: {said}"
    );
}

/// [R-LLM-004] a renewal that will not work is fatal, not retried
#[tokio::test(flavor = "multi_thread")]
async fn a_refused_renewal_is_not_worth_retrying() {
    let server = Exchange::new(401, 3600);
    let bearer = Exchanged::new("copilot", server.url(), "stale", Vec::new()).unwrap();

    let error = bearer.token(&CancellationToken::new()).await.unwrap_err();

    assert_eq!(
        error.class(),
        meow_llm::Class::Fatal,
        "retrying a grant that will not renew spends the backoff to reach the same message"
    );
}

/// [R-LLM-004] a renewal stops when the run is cancelled
#[tokio::test(flavor = "multi_thread")]
async fn cancelling_stops_a_renewal() {
    let server = Exchange::new(200, 3600);
    let bearer = Exchanged::new("copilot", server.url(), "grant", Vec::new()).unwrap();

    let cancel = CancellationToken::new();
    cancel.cancel();

    let error = bearer.token(&cancel).await.unwrap_err();
    assert!(
        matches!(error, LlmError::Cancelled),
        "expected a cancellation, got {error:?}"
    );
}
