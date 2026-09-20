// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The transport against a server on localhost.
//!
//! A real socket rather than a mock: what this code does is speak HTTP, and a
//! test that stubs out HTTP tests the stub. The server here answers one
//! request and stops, which is enough to check a status, a header, and how a
//! response that arrives in pieces is split into lines.
#![allow(clippy::unwrap_used)]

use std::time::Duration;

use meow_llm::{Http, LlmError, Transport};
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::TcpListener;

/// Serve one response, in the chunks given, and return the port.
async fn serve(chunks: Vec<&'static str>) -> u16 {
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let port = listener.local_addr().unwrap().port();

    tokio::spawn(async move {
        let (mut socket, _) = listener.accept().await.unwrap();

        // Read until the end of the request head; the body follows and we do
        // not need it to answer.
        let mut seen = Vec::new();
        let mut buffer = [0_u8; 1024];
        while !seen.windows(4).any(|w| w == b"\r\n\r\n") {
            let read = socket.read(&mut buffer).await.unwrap();
            if read == 0 {
                break;
            }
            seen.extend_from_slice(&buffer[..read]);
        }

        for chunk in chunks {
            socket.write_all(chunk.as_bytes()).await.unwrap();
            socket.flush().await.unwrap();
            // A pause between writes is what makes the chunks arrive as
            // separate reads, which is the case a line splitter gets wrong.
            tokio::time::sleep(Duration::from_millis(10)).await;
        }
        socket.shutdown().await.unwrap();
    });

    port
}

fn url(port: u16) -> String {
    format!("http://127.0.0.1:{port}/v1/messages")
}

/// A status is reported, not turned into an error: the provider decides what
/// one of its codes means.
#[tokio::test(flavor = "multi_thread")]
async fn a_status_comes_back_rather_than_failing() {
    let port = serve(vec![
        "HTTP/1.1 429 Too Many Requests\r\nContent-Length: 12\r\nRetry-After: 7\r\n\r\n{\"error\":1}\n",
    ])
    .await;

    let http = Http::new("test").unwrap();
    let response = http
        .post(
            &url(port),
            &[("x-api-key".to_owned(), "k".to_owned())],
            "{}".to_owned(),
        )
        .await
        .unwrap();

    assert_eq!(response.status, 429);
    assert_eq!(response.retry_after, Some(Duration::from_secs(7)));
    assert!(response.body.contains("error"), "{}", response.body);
}

/// A stream is split into lines however the chunks fall.
#[tokio::test(flavor = "multi_thread")]
async fn a_stream_is_split_into_lines_across_chunks() {
    let port = serve(vec![
        "HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\n\r\n",
        "event: message_start\ndata: {\"a\"",
        ":1}\n\nevent: message_stop\n",
        "data: {\"b\":2}",
    ])
    .await;

    let http = Http::new("test").unwrap();
    let mut lines = http
        .post_lines(&url(port), &[], "{}".to_owned())
        .await
        .unwrap();

    let mut seen = Vec::new();
    while let Some(line) = lines.recv().await {
        seen.push(line.unwrap());
    }

    assert_eq!(
        seen,
        [
            "event: message_start",
            "data: {\"a\":1}",
            "",
            "event: message_stop",
            "data: {\"b\":2}",
        ],
        "a line that spanned two chunks was not put back together"
    );
}

/// A stream that fails before its first line fails, rather than delivering a
/// channel that never produces anything.
#[tokio::test(flavor = "multi_thread")]
async fn a_failed_stream_fails_instead_of_hanging() {
    let port = serve(vec![
        "HTTP/1.1 401 Unauthorized\r\nContent-Length: 24\r\n\r\n{\"error\":\"bad api key\"}\n",
    ])
    .await;

    let http = Http::new("test").unwrap();
    let error = http
        .post_lines(&url(port), &[], "{}".to_owned())
        .await
        .unwrap_err();

    match error {
        LlmError::Http {
            status, message, ..
        } => {
            assert_eq!(status, 401);
            assert!(message.contains("bad api key"), "{message}");
        }
        other => panic!("expected an HTTP error, got {other:?}"),
    }
}

/// A connection that goes nowhere names the provider, because "connection
/// refused" is unactionable when a workspace declares three.
#[tokio::test(flavor = "multi_thread")]
async fn a_refused_connection_names_the_provider() {
    // A host that cannot resolve, rather than a port nothing is listening on.
    // Binding a port and dropping it says only that the port was free a moment
    // ago: another test in this binary binds port 0 at the same time, the
    // kernel hands it the one just released, and this test connects to that
    // test's server. That is not a hypothesis - CI failed here with a 200
    // carrying the streaming test's body.
    //
    // `.invalid` is reserved by RFC 2606 and guaranteed never to resolve, so
    // the transport fails for a reason nothing else in this binary can
    // change. What the test is about - a transport failure naming its
    // provider - is unaffected by which transport failure it is.
    let http = Http::new("anthropic").unwrap();
    let error = http
        .post("http://meowg1k.invalid/v1/messages", &[], "{}".to_owned())
        .await
        .unwrap_err();

    match error {
        LlmError::Transport { provider, .. } => assert_eq!(provider, "anthropic"),
        other => panic!("expected a transport error, got {other:?}"),
    }
}
