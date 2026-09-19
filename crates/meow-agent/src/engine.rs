// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

//! The loop.

use std::sync::Arc;

use meow_core::{StopReason, Usage};
use meow_llm::{LlmError, Message, Provider, Request, Role};
use serde_json::Value;
use tokio_util::sync::CancellationToken;

use crate::budget::Ledger;
use crate::event::{AgentEvent, Sink};
use crate::outcome::{Outcome, Step};
use crate::spec::{AgentSpec, ToolErrorPolicy};
use crate::tool::{Checked, check};

/// Runs agents.
pub struct Engine {
    provider: Arc<dyn Provider>,
}

impl std::fmt::Debug for Engine {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Engine")
            .field("provider", &self.provider.name())
            .finish()
    }
}

/// How a step ended, inside the loop.
enum Turn {
    /// The model is done.
    Done,
    /// It asked for tools; the loop continues.
    Continue,
    /// Stop, for this reason.
    Stop(StopReason, Option<String>),
}

impl Engine {
    /// An engine over one provider.
    pub fn new(provider: Arc<dyn Provider>) -> Self {
        Self { provider }
    }

    /// Run an agent to its end.
    ///
    /// Satisfies `[R-AGENT-001]`: every run that starts returns an outcome.
    /// No stop condition is reported as an error, because an error discards
    /// the transcript the run already paid for - which is what v0.2.x did
    /// when it hit `max_iterations`.
    pub async fn run(
        &self,
        spec: &AgentSpec,
        task: &str,
        ledger: &Ledger,
        sink: &mut dyn Sink,
        cancel: &CancellationToken,
    ) -> Outcome {
        let mut messages = Vec::new();
        if let Some(system) = &spec.system {
            messages.push(Message::new(Role::System, system.clone()));
        }
        messages.push(Message::new(Role::User, task));

        let mut run = Run {
            spec,
            ledger,
            cancel,
            deliver: Delivery::new(sink),
            messages,
            state: RunState::default(),
        };
        run.deliver.send(AgentEvent::RunStart {
            agent: spec.name.clone(),
        });

        let (stop, detail) = loop {
            if cancel.is_cancelled() {
                break (StopReason::Cancelled, None);
            }
            // [R-AGENT-015] and [R-AGENT-017]: reserved before the call, so a
            // run cannot overshoot by a whole step and concurrent runs sharing
            // a ledger cannot both see the same remaining amount.
            if let Err(axis) = ledger.reserve_step() {
                break (StopReason::Budget, Some(axis.as_str().to_owned()));
            }

            let step = ledger.steps_taken();
            run.deliver.send(AgentEvent::StepStart { step });

            if let Err(why) = self.compact(&mut run, cancel).await {
                run.deliver.send(AgentEvent::StepEnd { step });
                break (StopReason::Failed, Some(why));
            }

            match self.turn(&mut run, step).await {
                Turn::Done => {
                    run.deliver.send(AgentEvent::StepEnd { step });
                    break (StopReason::Finished, run.state.finish_detail());
                }
                Turn::Stop(reason, detail) => {
                    run.deliver.send(AgentEvent::StepEnd { step });
                    break (reason, detail);
                }
                Turn::Continue => run.deliver.send(AgentEvent::StepEnd { step }),
            }

            if let Some(axis) = ledger.exceeded() {
                break (StopReason::Budget, Some(axis.as_str().to_owned()));
            }
        };

        let value = self.final_value(spec, &run.state, stop);
        run.deliver.send(AgentEvent::RunEnd {
            stop,
            detail: detail.clone(),
        });

        Outcome {
            stop,
            detail,
            // [R-AGENT-003]: the last text, whatever the stop reason. A budget
            // stop is a result to inspect, not work to throw away.
            text: run.state.text,
            value,
            usage: run.state.usage,
            steps: run.state.steps,
        }
    }

    /// Summarise the older part of the conversation, when it has grown too
    /// long for the model to hold.
    ///
    /// Satisfies `[R-AGENT-040]` by acting before the next call rather than
    /// after a provider rejects one; `[R-AGENT-044]` by summarising rather
    /// than dropping, because a silent drop leaves the model confidently wrong
    /// about what it already knows; `[R-AGENT-042]` by emitting the event that
    /// records the range, without deleting anything; and `[R-AGENT-043]` by
    /// failing the run when the summary cannot be produced, rather than
    /// carrying on with a context the provider will reject anyway.
    async fn compact(
        &self,
        run: &mut Run<'_, '_>,
        cancel: &CancellationToken,
    ) -> Result<(), String> {
        let policy = &run.spec.compaction;
        let Some(range) =
            crate::compaction::range_to_compact(&run.messages, policy, run.spec.context_window)
        else {
            return Ok(());
        };

        let before = crate::compaction::estimate_tokens(&run.messages[range.clone()]);
        let transcript: String = run.messages[range.clone()]
            .iter()
            .map(|m| format!("{:?}: {}\n", m.role, m.content))
            .collect();

        let mut request = Request::new(
            policy
                .model
                .clone()
                .unwrap_or_else(|| run.spec.model.clone()),
            vec![
                Message::new(
                    Role::System,
                    "Summarise this conversation. Keep decisions, file names, \
                     and anything the next turn needs. Drop pleasantries.",
                ),
                Message::new(Role::User, transcript),
            ],
            1024,
        );
        request.tools.clear();

        let summary = match self.provider.generate(&request, cancel).await {
            Ok(r) => r.text,
            Err(e) => return Err(format!("compaction failed: {e}")),
        };

        let replacement = Message::new(
            Role::User,
            format!("Summary of the earlier conversation:\n{summary}"),
        );
        let saved = before.saturating_sub(crate::compaction::estimate_tokens(
            std::slice::from_ref(&replacement),
        ));

        run.deliver.send(AgentEvent::Compacted {
            supersedes: range.clone(),
            summary,
            tokens_saved: saved,
        });
        run.messages.splice(range, std::iter::once(replacement));
        Ok(())
    }

    /// One pass: ask the model, then run whatever it asked for.
    async fn turn(&self, run: &mut Run<'_, '_>, step: u32) -> Turn {
        let (spec, ledger, cancel) = (run.spec, run.ledger, run.cancel);
        let messages = &mut run.messages;
        let state = &mut run.state;
        let deliver = &mut run.deliver;
        let mut request = Request::new(&spec.model, messages.clone(), spec.max_output_tokens);
        request.tools = spec.tools.definitions();
        request.output_schema = spec.output.clone();

        let streaming = self.provider.capabilities().streaming && deliver.wants_deltas();
        let response = if streaming {
            let mut relay = Relay { deliver };
            self.provider
                .generate_stream(&request, &mut relay, cancel)
                .await
        } else {
            self.provider.generate(&request, cancel).await
        };

        let response = match response {
            Ok(r) => r,
            Err(LlmError::Cancelled) => return Turn::Stop(StopReason::Cancelled, None),
            Err(e) => return Turn::Stop(StopReason::Failed, Some(e.to_string())),
        };

        if let Some(usage) = response.usage {
            ledger.charge(&usage);
            state.usage = state.usage.add(&usage);
            deliver.send(AgentEvent::Usage(usage));
        }
        if !deliver.wants_deltas() && !response.text.is_empty() {
            deliver.send(AgentEvent::StepText {
                step,
                text: response.text.clone(),
            });
        }

        state.text = response.text.clone();
        state.value = response.value.clone();
        let mut record = Step {
            index: step,
            text: response.text.clone(),
            tool_calls: Vec::new(),
        };

        let mut assistant = Message::new(Role::Assistant, response.text.clone());
        assistant.tool_calls = response.tool_calls.clone();
        assistant.thinking = response.thinking.clone();
        messages.push(assistant);

        // [R-AGENT-005]: no tool calls means finished, even when the text is
        // empty. Empty text is not a failure; v0.2.x reported it as one.
        if response.tool_calls.is_empty() {
            state.empty_finish = response.text.trim().is_empty();
            state.steps.push(record);
            return Turn::Done;
        }

        // [R-AGENT-025]: in the order the model returned them.
        for call in &response.tool_calls {
            record.tool_calls.push(call.name.clone());
            deliver.send(AgentEvent::ToolStart {
                id: call.id.clone(),
                name: call.name.clone(),
                args: call.arguments.clone(),
            });

            let started = std::time::Instant::now();
            let result = self.invoke(spec, call, record.index, deliver, cancel).await;
            let (content, error) = (result.content, result.error);
            let duration_ms = started.elapsed().as_millis() as u64;
            deliver.send(AgentEvent::ToolEnd {
                id: call.id.clone(),
                duration_ms,
                error: error.clone(),
            });

            messages.push(Message::tool_result(&call.id, &content));

            // [R-AGENT-024]: `report` returns the error to the model and the
            // run continues; `abort` stops. An argument correction is not a
            // tool error and never aborts, which [R-AGENT-021] requires, so
            // only a real failure reaches here with `error` set.
            // [R-AGENT-006]: `denied` when policy refused, `tool_aborted`
            // when a tool failed. A user needs to know the boundary held
            // rather than that something broke.
            if result.stop {
                state.steps.push(record);
                return Turn::Stop(StopReason::Denied, Some(call.name.clone()));
            }

            if spec.on_tool_error == ToolErrorPolicy::Abort {
                if result.denied {
                    state.steps.push(record);
                    return Turn::Stop(StopReason::Denied, Some(call.name.clone()));
                }
                if error.is_some() {
                    state.steps.push(record);
                    return Turn::Stop(StopReason::ToolAborted, Some(call.name.clone()));
                }
            }
        }

        state.steps.push(record);
        Turn::Continue
    }

    /// Check the arguments, then run the tool.
    ///
    /// Returns what to tell the model, what went wrong if anything did, and
    /// whether policy refused. A correction is not a failure: the error stays
    /// `None`, so an abort policy does not fire on the model's first typo.
    async fn invoke(
        &self,
        spec: &AgentSpec,
        call: &meow_llm::ToolCall,
        step: u32,
        deliver: &mut Delivery<'_>,
        cancel: &CancellationToken,
    ) -> Invoked {
        let Some(tool) = spec.tools.get(&call.name) else {
            // [R-AGENT-023]: not found here means not found. No wider
            // registry is consulted.
            return Invoked::told(format!(
                "no tool named `{}` is available to this agent",
                call.name
            ));
        };

        let args: Value = serde_json::from_str(&call.arguments).unwrap_or(Value::Null);

        // [R-POLICY-040]: the decision comes before the tool runs, never after.
        if let (Some(policy), Some(describe)) = (&spec.policy, &spec.describe_call) {
            let judged = describe(&call.name, &args);
            let verdict = policy.evaluate(&judged, &spec.grants);
            deliver.send(AgentEvent::Policy {
                id: call.id.clone(),
                decision: verdict.decision.as_str().to_owned(),
            });
            // [R-POLICY-020]: `ask` becomes `deny` when nobody can be asked,
            // so an unattended run cannot approve itself. When somebody can
            // be, the answer decides, and "stop" ends the run rather than
            // letting the model work around a refusal it was meant to respect.
            let refused = match (verdict.decision, &spec.approve) {
                (meow_policy::Decision::Allow, _) => None,
                (meow_policy::Decision::Ask, Some(approver)) => {
                    let prompt = meow_policy::Prompt::new(
                        &judged,
                        &args,
                        &verdict,
                        &policy.sensitive_for(&call.name),
                        &spec.name,
                        step,
                    );
                    match approver.ask(&prompt) {
                        meow_policy::Answer::Once | meow_policy::Answer::Always => None,
                        meow_policy::Answer::Deny => Some(false),
                        meow_policy::Answer::Stop => Some(true),
                    }
                }
                _ => Some(false),
            };

            if let Some(stop) = refused {
                // [R-POLICY-041]: the model is told which tool and that policy
                // refused, so it can choose another approach rather than
                // repeating itself against a wall.
                let rule = verdict.rule.unwrap_or_else(|| "no rule matched".to_owned());
                return Invoked {
                    content: format!(
                        "policy denied `{}` ({rule}). Try something else.",
                        call.name
                    ),
                    error: None,
                    denied: true,
                    stop,
                };
            }
        }

        match check(&args, &tool.schema()) {
            Checked::Correction(message) => Invoked::told(message),
            Checked::Ready(args) => match tool.call(&args, cancel).await {
                Ok(output) => Invoked::told(output),
                Err(e) => Invoked {
                    content: format!("the tool failed: {e}"),
                    error: Some(e.to_string()),
                    denied: false,
                    stop: false,
                },
            },
        }
    }

    /// The parsed answer, when there is one to have.
    ///
    /// `[R-AGENT-080]`: present when the run finished, absent otherwise. A
    /// value from a run that stopped early would be a partial answer wearing
    /// the shape of a complete one.
    fn final_value(&self, spec: &AgentSpec, state: &RunState, stop: StopReason) -> Option<Value> {
        if spec.output.is_none() || stop != StopReason::Finished {
            return None;
        }
        state.value.clone()
    }
}

/// Everything one run carries while it is in flight.
///
/// A struct rather than eight arguments: the parts change together, and
/// passing them separately made it easy to forget one.
struct Run<'a, 'b> {
    spec: &'a AgentSpec,
    ledger: &'a Ledger,
    cancel: &'a CancellationToken,
    deliver: Delivery<'b>,
    messages: Vec<Message>,
    state: RunState,
}

/// What invoking a tool produced.
struct Invoked {
    content: String,
    error: Option<String>,
    denied: bool,
    /// Whether the run should end here, whatever the tool-error policy says.
    ///
    /// A person answering "stop" is not reporting a tool failure; they are
    /// ending the run, and a `report` policy must not talk them out of it.
    stop: bool,
}

impl Invoked {
    /// Something to tell the model that is not a failure.
    fn told(content: String) -> Self {
        Self {
            content,
            error: None,
            denied: false,
            stop: false,
        }
    }
}

#[derive(Default)]
struct RunState {
    text: String,
    value: Option<Value>,
    usage: Usage,
    steps: Vec<Step>,
    empty_finish: bool,
}

impl RunState {
    /// `[R-AGENT-005]`: a finish with no text is recorded, so a caller is not
    /// handed a silent success.
    fn finish_detail(&self) -> Option<String> {
        self.empty_finish
            .then(|| "the model returned no text".to_owned())
    }
}

/// Delivers events, and stops trying once a sink has failed.
///
/// `[R-AGENT-071]`: identical handling for every kind. Rendering is not the
/// work, so a broken sink does not destroy a run in progress; and it does not
/// fail silently either, because the failure is recorded and the sink stops
/// receiving.
struct Delivery<'a> {
    sink: &'a mut dyn Sink,
    broken: Option<String>,
}

impl<'a> Delivery<'a> {
    fn new(sink: &'a mut dyn Sink) -> Self {
        Self { sink, broken: None }
    }

    fn send(&mut self, event: AgentEvent) {
        if self.broken.is_some() {
            return;
        }
        if let Err(e) = self.sink.event(event) {
            self.broken = Some(e);
        }
    }

    fn wants_deltas(&self) -> bool {
        self.broken.is_none() && self.sink.wants_deltas()
    }
}

/// Turns the provider's stream into the engine's events.
struct Relay<'a, 'b> {
    deliver: &'a mut Delivery<'b>,
}

impl meow_llm::Sink for Relay<'_, '_> {
    fn event(&mut self, event: meow_llm::StreamEvent) -> meow_llm::Result<()> {
        match event {
            meow_llm::StreamEvent::Text(t) => self.deliver.send(AgentEvent::Text(t)),
            meow_llm::StreamEvent::Thinking(t) => self.deliver.send(AgentEvent::Thinking(t)),
            _ => {}
        }
        Ok(())
    }
}
