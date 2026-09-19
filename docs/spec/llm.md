# LLM

Status: draft
Elaborates: docs/design/0.3.0-architecture.md sections 4, 7, 9

## Scope

`meow-llm` defines what a model provider is: the request and response types,
the streaming event stream, the error classification, and the retry policy.
Each `meow-provider-*` crate implements the trait over one vendor's HTTP API.

It does not run an agent loop, decide what tools exist, or persist anything.
It turns a request into a response or a typed error.

## Boundary

The `Provider` trait, the request and response types, the stream event enum,
and the error enum. No vendor type crosses this boundary, which is what lets
the engine be written once.

## Requirements

### The trait

**[R-LLM-001]** A provider MUST implement generation, and MUST declare
whether it supports streaming, tool calling, structured output, and
embeddings.

**[R-LLM-002]** Calling a capability a provider does not declare MUST fail
with an error naming the provider and the capability, before any request is
sent.

**[R-LLM-003]** A trait method signature MUST NOT name a type from a vendor
SDK or wire format.

### Messages and tools

**[R-LLM-010]** A message MUST carry exactly one role: system, user,
assistant, or tool.

**[R-LLM-011]** An assistant message MUST be able to carry both text and a
list of tool calls in the same message.

**[R-LLM-012]** A tool result message MUST carry the identifier of the tool
call it answers.

**[R-LLM-013]** A provider MUST assign an identifier to every tool call it
returns that lacks one, and those identifiers MUST be unique within a
response.

**[R-LLM-014]** A provider MUST deduplicate tool calls that arrive more than
once with the same identifier, before returning the response, regardless of
any session or history setting.

### Streaming

**[R-LLM-020]** The stream event kinds MUST be exactly: `Text`, `Thinking`,
`ToolCallStart`, `ToolCallDelta`, `ToolCallEnd`, `Usage`, `Error`, and `Done`.

**[R-LLM-021]** A streaming call MUST return the same aggregated response as
the equivalent non-streaming call for the same request and model output.

**[R-LLM-022]** A provider that has no native streaming endpoint MUST declare
streaming as unsupported rather than synthesising events from a completed
response.

**[R-LLM-023]** An error raised by the stream consumer MUST abort the request
and MUST propagate to the caller unchanged.

### Errors and retry

**[R-LLM-030]** Every provider error MUST be classified as exactly one of
`Transient`, `Fatal`, or `QuotaExhausted`.

**[R-LLM-031]** HTTP 408, 429, 500, 502, 503, 504, and connection or read
timeouts MUST classify as `Transient`.

**[R-LLM-032]** HTTP 400, 401, 403, 404, and 422, and any schema rejection,
MUST classify as `Fatal`.

**[R-LLM-033]** Only `Transient` errors MUST be retried. A `Fatal` error MUST
surface to the caller on the first occurrence, without delay.

**[R-LLM-034]** Retry MUST use exponential backoff with random jitter, MUST
honour a `Retry-After` header when the response carries one, and MUST stop
after a configured maximum attempt count.

**[R-LLM-035]** A `QuotaExhausted` error MUST surface immediately with the
provider named and MUST NOT be retried.

**[R-LLM-036]** Every retry MUST check the cancellation token before sleeping
and before the next attempt.

### Usage

**[R-LLM-040]** A response MUST report prompt tokens, completion tokens, and
cached prompt tokens. A provider that does not report cached tokens MUST
report zero rather than omitting the field.

**[R-LLM-041]** When a provider reports no usage at all, the response MUST say
so explicitly, so that a missing count is distinguishable from a zero count.

### Structured output

**[R-LLM-050]** A request MAY carry a JSON Schema. A provider that declares
native structured output MUST use it; one that does not MUST request JSON and
validate the result against the schema.

**[R-LLM-051]** A response that fails schema validation MUST be retried with
the validation error included in the follow-up request, up to a configured
attempt count, and MUST then fail with the last validation error.

**[R-LLM-052]** A response that satisfies the schema MUST be returned parsed,
not as a string.

### Cancellation

**[R-LLM-060]** Every request MUST take a cancellation token and MUST abort
the in-flight HTTP request when it fires.

**[R-LLM-061]** A cancelled request MUST return a distinct cancellation error,
never a timeout or a transport error.

## Changes from v0.2.x

`RetryWithBackoff` in `internal/adapters/gateway/retry.go` retries every error
that is not a hard quota error, so an invalid API key costs the full backoff
schedule before it surfaces. [R-LLM-030] through [R-LLM-033] replace that with
a classification.

Tool call deduplication in `module_llm.go` runs only inside
`if useSession && ...`, so with `use_session=False` a duplicated streamed call
executes twice. [R-LLM-014] moves it to the provider boundary, where the
duplication actually happens.

`llama.go` synthesises stream events from a completed response, which reports
progress that never occurred. [R-LLM-022] requires it to declare streaming as
unsupported instead.

Cached prompt tokens are not reported anywhere in v0.2.x, which makes the main
cost lever of a well-built agent invisible.

## Open questions

- **Prompt caching control.** Anthropic needs explicit cache breakpoints;
  OpenAI caches automatically. Exposing breakpoints leaks one vendor's model
  into the trait, and not exposing them leaves the largest saving on the
  table. Recommendation: an optional `cache_hint` on a message that providers
  without the concept ignore.
- **Thinking and reasoning content.** Whether it is persisted to the session
  or only streamed. Recommendation: stream it, do not persist it, and revisit
  if a provider requires replaying it.
