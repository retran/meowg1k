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

**[R-AGENT-001]** Every run that starts MUST return an `AgentOutcome`. No stop
condition, cancellation included, may be reported as an error, because an error
discards the transcript the run already paid for.

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
as a failure, and the outcome's detail MUST record that the model returned no
text, so a caller is not handed a silent success.

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

**[R-AGENT-017]** Budget MUST be reserved before a call, not checked before and
charged after. Concurrent invocations sharing one budget MUST NOT be able to
overspend it by each observing the same remaining amount.

**[R-AGENT-014]** A sub-agent MUST NOT be given a budget larger than its
caller's remaining budget.

**[R-AGENT-015]** The budget MUST be checked before each model call and after
each tool result, so a run cannot overshoot by a whole step.

**[R-AGENT-016]** The default budget MUST be 200,000 tokens, 40 steps, and 30
minutes, with cost unbounded.

### Tool calls

**[R-AGENT-020]** Before invoking a tool the engine MUST validate the
arguments against the tool's schema.

**[R-AGENT-021]** A required argument the model omitted MUST NOT be replaced
with a default or a zero value. The engine MUST return a message to the model
naming the argument and its type, and MUST continue the run. An argument
correction is not a tool error and MUST NOT abort the run under any error
policy.

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

**[R-AGENT-044]** Compaction MUST summarise the superseded range with a model
call and MUST record the number of tokens the summary saved. It MUST NOT drop
messages without summarising them, because a silent drop leaves the model
confidently wrong about what it already knows.

**[R-AGENT-045]** The compaction policy MUST accept a model of its own, and
MUST fall back to the agent's model when none is given.

### Sub-agents

**[R-AGENT-050]** An agent used as a tool MUST run in its own session, with
the calling session recorded as its parent.

**[R-AGENT-051]** A sub-agent's outcome MUST be returned to the calling model
as a tool result containing its text, its stop reason, and its parsed value
when it declared an output schema.

**[R-AGENT-052]** The engine MUST refuse to start a sub-agent that would
exceed the configured maximum nesting depth, and MUST report that refusal as a
tool error rather than a panic.

**[R-AGENT-053]** A sub-agent's message list MUST NOT contain its caller's
messages. The only information that crosses MUST be the task and the arguments
the caller passed, so that an agent behaves the same wherever it is called
from.

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

**[R-AGENT-073]** A sink MUST be able to decline delta events. When it does,
the engine MUST deliver the completed text once per step instead, because a
sink backed by a script callback would otherwise pay one call per token.

**[R-AGENT-071]** An error returned by the sink MUST be handled identically
for every event kind: the engine MUST stop delivering to that sink, MUST record
a `Note` event naming the error, and MUST continue the run. Rendering is not
the work, so a broken sink MUST NOT destroy a run in progress, and it MUST NOT
fail silently either.

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

## Decisions

**Sub-agents are isolated**, by [R-AGENT-053]. Sharing the caller's history
would give a sub-agent more context, and it would also make the same agent
behave differently depending on who called it, which makes it untestable and
its budget unpredictable.

**Compaction summarises**, by [R-AGENT-044]. It costs a model call at the
moment the run is already long, and the alternative is losing information
without saying so. Recording the tokens saved makes that cost visible instead
of mysterious.

That cost is why [R-AGENT-045] lets compaction name its own model. Summarising
is a cheap task that the first draft would have run on the agent's expensive
model, at the worst possible moment, on every long run.

**The default budget is 200,000 tokens, 40 steps, and 30 minutes**, by
[R-AGENT-016]. Tokens and steps are what actually bound cost, and they are set
so a runaway loop costs cents rather than dollars.

Wall clock is the axis that misfires. It bounds patience, not spend, and a
slow provider or a long tool makes it fire on a run that is working perfectly,
turning a good answer into a `budget` stop with partial results. Five minutes,
the first number chosen, is less than 40 steps of a slow model. Thirty minutes
still stops a wedged run without punishing a slow one.

Cost stays unbounded because capping it means estimating the price of a call
before making it, and the token cap is the same guard with fewer moving
parts.
