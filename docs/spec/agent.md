# Agent

Status: draft
Elaborates: docs/design/0.3.0-architecture.md sections 3, 4.1, 5, 7; docs/design/0.3.0-starlark-api.md section 5

## Scope

`meow-agent` runs the loop: send a prompt and the tool schemas to a model,
execute the tool calls it asks for, feed the results back, and stop when the
model is done or a bound is reached. It owns budgets, cancellation,
compaction, and the outcome it returns.

It does not know Starlark exists, does not render anything, and does not talk
to a vendor API. `meow-star` declares agents to it, `meow-ui` observes it, and
`meow-llm` carries its requests.

## Boundary

`AgentSpec`, `AgentRun`, `AgentOutcome`, the `EventSink` trait, and the errors
the engine returns.

## Requirements

### Outcome

**[R-AGENT-001]** Every run that starts MUST return an `AgentOutcome`, or a
cancellation error. The engine MUST NOT report a stop condition by returning
an error that discards the transcript.

**[R-AGENT-002]** The stop reasons MUST be exactly: `finished`, `budget`,
`cancelled`, `denied`, `tool_aborted`, and `failed`.

**[R-AGENT-003]** `AgentOutcome.text` MUST carry the model's last text output,
whatever the stop reason, including when that text is empty.

**[R-AGENT-004]** An outcome MUST carry the step transcript, the usage
totals, the session identifier, and a detail string that explains the stop
reason: which budget axis bound the run, which tool aborted it, which rule
denied it.

**[R-AGENT-005]** A model response with no tool calls MUST stop the run with
`finished`, including when its text is empty. Empty text MUST NOT be treated
as a failure.

**[R-AGENT-006]** `denied` MUST be the stop reason when the run ends because
policy denied a call and the error policy is `abort`; `tool_aborted` MUST be
the stop reason when it ends because a tool returned an error and the error
policy is `abort`.

### Budget

**[R-AGENT-010]** A budget MUST bound a run on four axes: total tokens,
steps, wall-clock duration, and estimated cost.

**[R-AGENT-011]** A run MUST stop with `budget` as soon as any configured axis
is reached, and the outcome MUST name which axis bound it.

**[R-AGENT-012]** A budget axis left unset MUST be unbounded, and a spec with
no budget at all MUST take the configured default rather than running
unbounded.

**[R-AGENT-013]** A sub-agent's spend MUST count against its caller's
remaining budget, transitively.

**[R-AGENT-014]** A sub-agent MUST NOT be given a budget larger than its
caller's remaining budget.

**[R-AGENT-015]** The budget MUST be checked before each model call and after
each tool result, so a run cannot overshoot by a whole step.

### Tool calls

**[R-AGENT-020]** Before invoking a tool the engine MUST validate the
arguments against the tool's schema.

**[R-AGENT-021]** A required argument the model omitted MUST NOT be replaced
with a default or a zero value. The engine MUST return a message to the model
naming the argument and its type, and MUST continue the run.

**[R-AGENT-022]** An optional argument the model omitted MUST take its
declared default, and an argument with no default MUST be absent rather than
zeroed, so that absent is distinguishable from zero, empty, and false.

**[R-AGENT-023]** A tool call naming a tool the agent was not given MUST
return an error to the model and MUST NOT search any wider registry.

**[R-AGENT-024]** With error policy `report`, a tool error MUST be returned to
the model as a tool result and the run MUST continue. With `abort`, the run
MUST stop.

**[R-AGENT-025]** Tool calls within one model response MUST execute in the
order the model returned them.

### Cancellation

**[R-AGENT-030]** Every run MUST accept a cancellation token, and MUST check
it before each model call, before each tool call, and while awaiting either.

**[R-AGENT-031]** A cancelled run MUST stop with `cancelled`, MUST write its
`Finished` event, and MUST return the transcript accumulated so far.

**[R-AGENT-032]** Cancelling a parent MUST cancel its running sub-agents.

### Compaction

**[R-AGENT-040]** When the rebuilt message list exceeds the configured
fraction of the model's context window, the engine MUST compact before the
next model call.

**[R-AGENT-041]** Compaction MUST keep the configured number of most recent
messages verbatim.

**[R-AGENT-042]** Compaction MUST record a `Compaction` session event per
[R-SESSION-010] and MUST NOT delete events.

**[R-AGENT-043]** A compaction that fails MUST fail the run rather than
continuing with an over-long context that the provider will reject.

### Sub-agents

**[R-AGENT-050]** An agent used as a tool MUST run in its own session, with
the calling session recorded as its parent.

**[R-AGENT-051]** A sub-agent's outcome MUST be returned to the calling model
as a tool result containing its text and its stop reason.

**[R-AGENT-052]** The engine MUST refuse to start a sub-agent that would
exceed the configured maximum nesting depth, and MUST report that refusal as a
tool error rather than a panic.

### Concurrency

**[R-AGENT-060]** The engine MUST accept a list of agent invocations and run
them concurrently, returning results in the order given.

**[R-AGENT-061]** One failing invocation MUST NOT abort the others. Its result
MUST be an outcome with a non-`finished` stop reason.

**[R-AGENT-062]** Concurrent invocations MUST share the caller's budget, and
the engine MUST stop starting new ones once the budget is exhausted.

### Events

**[R-AGENT-070]** The engine MUST emit every observable transition to its
`EventSink`: run start and end, step start and end, text and thinking deltas,
tool call start and end, policy decisions, and usage.

**[R-AGENT-071]** An error returned by the sink MUST be handled identically
for every event kind. The engine MUST NOT propagate a sink error for one kind
and swallow it for another.

**[R-AGENT-072]** The engine MUST function with no sink attached.

### Structured output

**[R-AGENT-080]** When a spec declares an output schema, the outcome's parsed
value MUST be present when the stop reason is `finished`, and MUST be absent
otherwise.

**[R-AGENT-081]** A final response that fails schema validation MUST be
retried per [R-LLM-051] before the run stops with `failed`.

## Changes from v0.2.x

`ctx.llm.agent_turn` returns a bare string, so a caller cannot tell why it
stopped. Worse, reaching `max_iterations` raises an error and discards the
whole transcript, and a model that legitimately returns empty text produces the
same error. [R-AGENT-001] through [R-AGENT-005] replace that with an outcome.

`executeToolForAgentic` fills a missing argument with the zero value for its
type, so a model that omits a required integer receives `0` and returns a
confident wrong answer. [R-AGENT-021] and [R-AGENT-022] replace it.

Budgets, cancellation, and compaction do not exist in the engine.
`max_iterations` is the only bound, and compaction lives in a userland
`.star` library that each command has to remember to call.

`emitEvent` discards callback errors while `makeCallback` propagates them, so a
bug in an event handler is fatal for text deltas and silent for tool events.
[R-AGENT-071] makes the two consistent.

## Open questions

- **Whether a sub-agent shares its parent's message history.** Sharing gives
  context; isolating gives a clean budget and a reusable agent.
  Recommendation: isolate, and pass what the sub-agent needs in its task,
  because a shared history makes an agent's behaviour depend on its caller.
- **Whether compaction should summarise or drop.** Summarising costs a model
  call at the worst moment; dropping loses information silently.
  Recommendation: summarise, and record the token count saved so the cost is
  visible.
- **The default budget.** A bounded default is required by [R-AGENT-012], but
  the numbers are a product decision that wants real runs behind it.
