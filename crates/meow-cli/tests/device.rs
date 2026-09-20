// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The device-code flow, against a server that speaks the protocol.
//!
//! Not against GitHub. A test that reaches a real authorisation server needs
//! a person to approve it, which is the one thing a test cannot do, and it
//! would fail for everybody offline.
#![allow(clippy::unwrap_used)]

use std::io::{Read, Write};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use meow_cli::device::{DeviceError, Endpoints, run};
use tokio_util::sync::CancellationToken;

/// A server that answers the token endpoint differently each time.
struct Authorisation {
    port: u16,
    /// What the token endpoint says, in order; the last is repeated.
    answers: Arc<Mutex<Vec<String>>>,
    /// How many times it was asked.
    asked: Arc<Mutex<usize>>,
    stop: Arc<std::sync::atomic::AtomicBool>,
}

impl Authorisation {
    /// Answer the code endpoint with this, then the token endpoint with each
    /// of these in turn.
    fn new(interval: u64, expires: u64, answers: Vec<&str>) -> Self {
        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let port = listener.local_addr().unwrap().port();
        let answers: Arc<Mutex<Vec<String>>> =
            Arc::new(Mutex::new(answers.into_iter().map(str::to_owned).collect()));
        let asked = Arc::new(Mutex::new(0));
        let stop = Arc::new(std::sync::atomic::AtomicBool::new(false));

        let code = format!(
            r#"{{"device_code":"dc","user_code":"ABCD-1234","verification_uri":"https://example.invalid/device","interval":{interval},"expires_in":{expires}}}"#
        );

        let queued = Arc::clone(&answers);
        let counted = Arc::clone(&asked);
        let halt = Arc::clone(&stop);
        std::thread::spawn(move || {
            for stream in listener.incoming() {
                if halt.load(std::sync::atomic::Ordering::Relaxed) {
                    return;
                }
                let Ok(mut stream) = stream else { continue };
                let mut buffer = [0_u8; 4096];
                let read = stream.read(&mut buffer).unwrap_or(0);
                let request = String::from_utf8_lossy(&buffer[..read]).into_owned();

                let body = if request.contains("POST /code") {
                    code.clone()
                } else {
                    *counted.lock().unwrap() += 1;
                    let mut queue = queued.lock().unwrap();
                    if queue.len() > 1 {
                        queue.remove(0)
                    } else {
                        queue.first().cloned().unwrap_or_default()
                    }
                };

                let response = format!(
                    "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                );
                let _ = stream.write_all(response.as_bytes());
                let _ = stream.flush();
            }
        });

        Self {
            port,
            answers,
            asked,
            stop,
        }
    }

    fn endpoints(&self) -> Endpoints {
        Endpoints {
            code: format!("http://127.0.0.1:{}/code", self.port),
            token: format!("http://127.0.0.1:{}/token", self.port),
            client_id: "test".to_owned(),
            scope: "read:user".to_owned(),
        }
    }

    fn asked(&self) -> usize {
        *self.asked.lock().unwrap()
    }
}

impl Drop for Authorisation {
    fn drop(&mut self) {
        self.stop.store(true, std::sync::atomic::Ordering::Relaxed);
        let _ = std::net::TcpStream::connect(("127.0.0.1", self.port));
        drop(self.answers.lock());
    }
}

/// [R-AUTH-020] the person is shown a code and a URL, and the token is stored
#[tokio::test(flavor = "multi_thread")]
async fn an_approved_flow_returns_the_token() {
    let server = Authorisation::new(
        1,
        600,
        vec![
            r#"{"error":"authorization_pending"}"#,
            r#"{"access_token":"at","refresh_token":"rt","expires_in":3600}"#,
        ],
    );

    let shown = Arc::new(Mutex::new(Vec::new()));
    let recorded = Arc::clone(&shown);

    let granted = run(
        &server.endpoints(),
        &CancellationToken::new(),
        move |code, uri| {
            recorded.lock().unwrap().push(format!("{code} {uri}"));
        },
    )
    .await
    .unwrap();

    assert_eq!(granted.access, "at");
    assert_eq!(granted.refresh, "rt");
    assert!(
        granted.expires > 0,
        "an expiry must be turned into an instant"
    );

    assert_eq!(
        shown.lock().unwrap().as_slice(),
        ["ABCD-1234 https://example.invalid/device"],
        "the code and the URL must both be shown, and once"
    );
    assert_eq!(
        server.asked(),
        2,
        "pending must be polled through, not given up on"
    );
}

/// [R-AUTH-021] `slow_down` is honoured rather than ignored
#[tokio::test(flavor = "multi_thread")]
async fn slow_down_lengthens_the_wait() {
    let server = Authorisation::new(
        1,
        600,
        vec![
            r#"{"error":"slow_down","interval":3}"#,
            r#"{"access_token":"at","refresh_token":"rt","expires_in":3600}"#,
        ],
    );

    let began = std::time::Instant::now();
    let granted = run(&server.endpoints(), &CancellationToken::new(), |_, _| {})
        .await
        .unwrap();
    let took = began.elapsed();

    assert_eq!(granted.access, "at");
    // One second for the first poll, then the three the server asked for.
    assert!(
        took >= Duration::from_secs(4),
        "the server asked for three seconds and got {took:?}"
    );
}

/// [R-AUTH-021] a code the server says has expired stops the flow
#[tokio::test(flavor = "multi_thread")]
async fn an_expired_code_stops() {
    let server = Authorisation::new(1, 600, vec![r#"{"error":"expired_token"}"#]);

    let error = run(&server.endpoints(), &CancellationToken::new(), |_, _| {})
        .await
        .unwrap_err();

    assert!(
        matches!(error, DeviceError::Expired),
        "expected an expiry, got {error:?}"
    );
    assert!(
        error.to_string().contains("run the command again"),
        "the message must say what to do: {error}"
    );
}

/// [R-AUTH-021] the flow stops running when the run is cancelled
#[tokio::test(flavor = "multi_thread")]
async fn cancelling_stops_the_flow() {
    let server = Authorisation::new(1, 600, vec![r#"{"error":"authorization_pending"}"#]);

    let cancel = CancellationToken::new();
    let stop = cancel.clone();
    tokio::spawn(async move {
        tokio::time::sleep(Duration::from_millis(200)).await;
        stop.cancel();
    });

    let error = run(&server.endpoints(), &cancel, |_, _| {})
        .await
        .unwrap_err();

    assert!(
        matches!(error, DeviceError::Cancelled),
        "expected a cancellation, got {error:?}"
    );
}

/// [R-AUTH-021] a refusal is reported with what the server said
#[tokio::test(flavor = "multi_thread")]
async fn a_refusal_carries_the_servers_reason() {
    let server = Authorisation::new(
        1,
        600,
        vec![r#"{"error":"access_denied","error_description":"the user cancelled"}"#],
    );

    let error = run(&server.endpoints(), &CancellationToken::new(), |_, _| {})
        .await
        .unwrap_err();

    assert!(
        error.to_string().contains("the user cancelled"),
        "the server's own words must reach the person: {error}"
    );
}

/// [R-AUTH-021] a server that sends neither a token nor an error is not
/// polled forever
#[tokio::test(flavor = "multi_thread")]
async fn a_silent_server_fails_rather_than_hanging() {
    let server = Authorisation::new(1, 600, vec!["{}"]);

    let error = run(&server.endpoints(), &CancellationToken::new(), |_, _| {})
        .await
        .unwrap_err();

    assert!(
        error.to_string().contains("neither a token nor an error"),
        "a hang must become a failure that says why: {error}"
    );
}
