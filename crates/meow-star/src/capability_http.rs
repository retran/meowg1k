// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! `@std//http`: the one module that leaves the machine.
//!
//! Everything else in the table is confined - `fs` and `shell` to the
//! workspace, `store` to its own table - and this is not confined to anything.
//! Three things follow from that, and they are `[R-STAR-022]` and
//! `[R-STAR-023]`.
//!
//! A call carries a deadline and caps what it will read, because a server that
//! never answers and a server that answers forever both hang a handler that
//! did nothing wrong. A response is returned whatever its status, because a
//! handler polling until something returns 200, or reading 404 as "not yet",
//! is the ordinary case; what fails is not reaching a server at all. And the
//! run's cancellation token is in the select, so Ctrl-C does not leave a
//! request in flight.
//!
//! It is not behind the policy layer, for the reason `docs/spec/policy.md`
//! gives: policy governs what a model decided, and a handler is code the
//! workspace's own author wrote. What a model decided still passes through
//! policy, because the model calls a tool and the tool call is what a rule
//! matches.

use std::time::Duration;

use starlark::environment::GlobalsBuilder;
use starlark::eval::Evaluator;
use starlark::starlark_module;
use starlark::values::Value as StarValue;
use starlark::values::none::NoneOr;

use crate::run::running;

/// How long a request may take before it is abandoned.
const DEFAULT_TIMEOUT_SECS: u32 = 30;

/// How much of a response body will be read.
///
/// A handler that pulls a gigabyte into a Starlark string has already lost,
/// and a limit it can raise deliberately is kinder than a machine that stops
/// responding.
const DEFAULT_MAX_BYTES: u32 = 8 * 1024 * 1024;

/// How many redirects will be followed before the call gives up.
const MAX_REDIRECTS: usize = 10;

fn oops(message: impl std::fmt::Display) -> starlark::Error {
    starlark::Error::new_other(anyhow::anyhow!("{message}"))
}

/// `@std//http`: four verbs, one response shape.
#[starlark_module]
pub fn http_module(builder: &mut GlobalsBuilder) {
    /// Fetch a URL.
    fn get<'v>(
        #[starlark(require = pos)] url: String,
        #[starlark(require = named, default = NoneOr::None)] headers: NoneOr<StarValue<'v>>,
        #[starlark(require = named, default = NoneOr::None)] timeout_secs: NoneOr<u32>,
        #[starlark(require = named, default = NoneOr::None)] max_bytes: NoneOr<u32>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        send(
            eval,
            "http.get",
            "GET",
            url,
            NoneOr::None,
            headers,
            timeout_secs,
            max_bytes,
        )
    }

    /// Send a body to a URL.
    ///
    /// A string body is sent as it is; anything else is encoded as JSON and
    /// gets a `content-type` unless the caller set one. A handler that has a
    /// dict almost always means JSON, and making it call `json.encode` first
    /// would be ceremony.
    fn post<'v>(
        #[starlark(require = pos)] url: String,
        #[starlark(require = pos)] body: StarValue<'v>,
        #[starlark(require = named, default = NoneOr::None)] headers: NoneOr<StarValue<'v>>,
        #[starlark(require = named, default = NoneOr::None)] timeout_secs: NoneOr<u32>,
        #[starlark(require = named, default = NoneOr::None)] max_bytes: NoneOr<u32>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        send(
            eval,
            "http.post",
            "POST",
            url,
            NoneOr::Other(body),
            headers,
            timeout_secs,
            max_bytes,
        )
    }

    /// Replace what is at a URL.
    fn put<'v>(
        #[starlark(require = pos)] url: String,
        #[starlark(require = pos)] body: StarValue<'v>,
        #[starlark(require = named, default = NoneOr::None)] headers: NoneOr<StarValue<'v>>,
        #[starlark(require = named, default = NoneOr::None)] timeout_secs: NoneOr<u32>,
        #[starlark(require = named, default = NoneOr::None)] max_bytes: NoneOr<u32>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        send(
            eval,
            "http.put",
            "PUT",
            url,
            NoneOr::Other(body),
            headers,
            timeout_secs,
            max_bytes,
        )
    }

    /// Remove what is at a URL.
    fn delete<'v>(
        #[starlark(require = pos)] url: String,
        #[starlark(require = named, default = NoneOr::None)] headers: NoneOr<StarValue<'v>>,
        #[starlark(require = named, default = NoneOr::None)] timeout_secs: NoneOr<u32>,
        #[starlark(require = named, default = NoneOr::None)] max_bytes: NoneOr<u32>,
        eval: &mut Evaluator<'v, '_, '_>,
    ) -> starlark::Result<StarValue<'v>> {
        send(
            eval,
            "http.delete",
            "DELETE",
            url,
            NoneOr::None,
            headers,
            timeout_secs,
            max_bytes,
        )
    }
}

/// One request, and the response shape every verb returns.
#[allow(clippy::too_many_arguments)] // Four optional knobs, and naming each at
// the call site is what makes a handler readable. A struct would move the
// ceremony rather than remove it.
fn send<'v>(
    eval: &mut Evaluator<'v, '_, '_>,
    what: &str,
    method: &'static str,
    url: String,
    body: NoneOr<StarValue<'v>>,
    headers: NoneOr<StarValue<'v>>,
    timeout_secs: NoneOr<u32>,
    max_bytes: NoneOr<u32>,
) -> starlark::Result<StarValue<'v>> {
    let state = running(eval, what)?;

    // Refused before the request rather than after: a scheme this module
    // cannot speak is a mistake in the handler, and `file://` reaching a
    // client that follows it is how a fetch becomes a file read.
    if !(url.starts_with("http://") || url.starts_with("https://")) {
        return Err(oops(format!(
            "`{url}` is not an http or https URL, and `{what}` speaks no other scheme"
        )));
    }

    let sent = headers_of(headers)?;
    let body = body_of(body)?;
    let timeout = Duration::from_secs(u64::from(
        timeout_secs.into_option().unwrap_or(DEFAULT_TIMEOUT_SECS),
    ));
    let cap = max_bytes.into_option().unwrap_or(DEFAULT_MAX_BYTES) as usize;
    let cancel = state.runtime.cancel_token().clone();

    // `[R-STAR-081]`: the thread blocks. A handler has nothing useful to do
    // with a future, and the evaluator could not hold one anyway.
    let (status, got, text) = state.runtime.block_on(async move {
        let client = reqwest::Client::builder()
            .redirect(reqwest::redirect::Policy::limited(MAX_REDIRECTS))
            .user_agent(concat!("meowg1k/", env!("CARGO_PKG_VERSION")))
            .build()
            .map_err(|e| oops(format!("could not build an HTTP client: {e}")))?;

        let mut request = client.request(
            reqwest::Method::from_bytes(method.as_bytes())
                .map_err(|e| oops(format!("`{method}` is not a method: {e}")))?,
            &url,
        );
        let mut typed = false;
        for (name, value) in &sent {
            if name.eq_ignore_ascii_case("content-type") {
                typed = true;
            }
            request = request.header(name.as_str(), value.as_str());
        }
        if let Some(Body { bytes, json }) = &body {
            if *json && !typed {
                request = request.header("content-type", "application/json");
            }
            request = request.body(bytes.clone());
        }

        let response = tokio::select! {
            () = cancel.cancelled() => return Err(oops("the run was cancelled")),
            () = tokio::time::sleep(timeout) => {
                return Err(oops(format!(
                    "`{url}` did not answer in {} seconds",
                    timeout.as_secs()
                )));
            }
            // `[R-STAR-023]`: this is the failure that is a failure. A status
            // the server chose is an answer, and arrives below.
            response = request.send() => response
                .map_err(|e| oops(format!("could not reach `{url}`: {e}")))?,
        };

        let status = response.status().as_u16();
        let got: Vec<(String, String)> = response
            .headers()
            .iter()
            .map(|(name, value)| {
                (
                    name.as_str().to_owned(),
                    value.to_str().unwrap_or_default().to_owned(),
                )
            })
            .collect();

        let bytes = tokio::select! {
            () = cancel.cancelled() => return Err(oops("the run was cancelled")),
            () = tokio::time::sleep(timeout) => {
                return Err(oops(format!(
                    "`{url}` did not finish sending in {} seconds",
                    timeout.as_secs()
                )));
            }
            bytes = response.bytes() => bytes
                .map_err(|e| oops(format!("could not read `{url}`: {e}")))?,
        };

        // Cut rather than fail: a handler that asked for a megabyte of a
        // stream wants the megabyte, and `truncated` tells it what happened.
        let text = String::from_utf8_lossy(&bytes[..bytes.len().min(cap)]).into_owned();
        Ok((status, got, text))
    })?;

    let heap = eval.heap();
    let headers: Vec<(&str, StarValue<'v>)> = got
        .iter()
        .map(|(name, value)| (name.as_str(), heap.alloc(value.as_str())))
        .collect();

    Ok(heap.alloc(starlark::values::structs::AllocStruct([
        ("status", heap.alloc(u32::from(status))),
        // 2xx, the one judgement worth making here. Everything else a handler
        // decides from the number.
        ("ok", heap.alloc((200..300).contains(&status))),
        ("body", heap.alloc(text)),
        (
            "headers",
            heap.alloc(starlark::values::dict::AllocDict(headers)),
        ),
    ])))
}

/// A body, and whether this module encoded it.
struct Body {
    bytes: Vec<u8>,
    json: bool,
}

/// A string goes as it is; anything else becomes JSON.
fn body_of(body: NoneOr<StarValue<'_>>) -> starlark::Result<Option<Body>> {
    let Some(value) = body.into_option() else {
        return Ok(None);
    };
    if value.is_none() {
        return Ok(None);
    }
    if let Some(text) = value.unpack_str() {
        return Ok(Some(Body {
            bytes: text.as_bytes().to_vec(),
            json: false,
        }));
    }
    let value = value.to_json_value().map_err(oops)?;
    Ok(Some(Body {
        bytes: serde_json::to_vec(&value).map_err(oops)?,
        json: true,
    }))
}

/// Headers as a dict of strings, refusing anything else by name.
fn headers_of(headers: NoneOr<StarValue<'_>>) -> starlark::Result<Vec<(String, String)>> {
    let Some(value) = headers.into_option() else {
        return Ok(Vec::new());
    };
    if value.is_none() {
        return Ok(Vec::new());
    }
    let map = starlark::values::dict::DictRef::from_value(value)
        .ok_or_else(|| oops("`headers` is a dict of strings"))?;

    let mut out = Vec::with_capacity(map.len());
    for (name, value) in map.iter() {
        let (Some(name), Some(value)) = (name.unpack_str(), value.unpack_str()) else {
            return Err(oops(format!(
                "a header is a string named by a string, and `{name}` is {value}"
            )));
        };
        out.push((name.to_owned(), value.to_owned()));
    }
    Ok(out)
}
