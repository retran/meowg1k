// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	gatewayimpl "github.com/retran/meowg1k/internal/adapters/gateway"
	"github.com/retran/meowg1k/internal/core/model"
	"github.com/retran/meowg1k/internal/domain/gateway"
	domainpreset "github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/internal/domain/session"
	"github.com/retran/meowg1k/internal/ports"
)

// LLMServices holds references to LLM-related services.
type LLMServices struct {
	PresetService     ports.PresetResolver
	ModelService      *model.Service
	GatewayFactory    ports.GenerationGatewayFactory
	EmbeddingsFactory *gatewayimpl.Factory
}

// LLMModule wraps LLM functionality with session tracking.
type LLMModule struct {
	runtime        *Runtime
	currentSession *session.Session
}

// SetLLMServices configures the LLM module with required services.
func (r *Runtime) SetLLMServices(services *LLMServices) {
	r.llmServices = services
}

// createLLMModule creates the llm built-in module.
// currentSession is the session this context belongs to (can be nil if no session).
func (r *Runtime) createLLMModule(currentSession *session.Session) starlark.Value {
	module := &LLMModule{
		runtime:        r,
		currentSession: currentSession,
	}
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"chat":       starlark.NewBuiltin("chat", module.llmChat),
		"agent_turn": starlark.NewBuiltin("agent_turn", module.llmAgentTurn),
		"embed":      starlark.NewBuiltin("embed", module.llmEmbed),
	})
}

// loadSessionHistory loads all non-obsolete events for the current session and converts
// them to gateway.Message for use as conversation history.
func (m *LLMModule) loadSessionHistory(ctx context.Context) ([]gateway.Message, error) {
	if m.runtime.sessionService == nil || m.currentSession == nil {
		return nil, nil
	}

	events, err := m.runtime.sessionService.GetAllEvents(ctx, m.currentSession.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session history: %w", err)
	}

	messages := make([]gateway.Message, 0, len(events))
	for _, e := range events {
		switch e.Type {
		case session.EventTypeUserMessage:
			messages = append(messages, gateway.Message{
				Role:    gateway.MessageRoleUser,
				Content: e.Content,
			})
		case session.EventTypeAssistantMessage:
			// Convert session tool calls to gateway tool calls
			var gatewayToolCalls []gateway.ToolCall
			for _, tc := range e.ToolCalls {
				gatewayToolCalls = append(gatewayToolCalls, gateway.ToolCall{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Params,
				})
			}
			messages = append(messages, gateway.Message{
				Role:      gateway.MessageRoleAssistant,
				Content:   e.Content,
				ToolCalls: gatewayToolCalls,
			})
		case session.EventTypeToolResult:
			toolCallID := ""
			if e.ToolCallID != nil {
				toolCallID = *e.ToolCallID
			}
			messages = append(messages, gateway.Message{
				Role:       gateway.MessageRoleTool,
				Content:    e.Content,
				ToolCallID: toolCallID,
			})
		case session.EventTypeSystem:
			// System events are not replayed as messages
		}
	}

	return messages, nil
}

// streamEventToStarlark converts a gateway.StreamEvent to a Starlark dict.
func streamEventToStarlark(event gateway.StreamEvent) *starlark.Dict {
	d := starlark.NewDict(4)

	var kindStr string
	switch event.Kind {
	case gateway.StreamEventText:
		kindStr = "text"
	case gateway.StreamEventThinking:
		kindStr = "thinking"
	case gateway.StreamEventUsage:
		kindStr = "usage"
	case gateway.StreamEventDone:
		kindStr = "done"
	case gateway.StreamEventError:
		kindStr = "error"
	case gateway.StreamEventToolCallStart:
		kindStr = "tool_call_start"
	case gateway.StreamEventToolCallEnd:
		kindStr = "tool_call_end"
	case gateway.StreamEventToolCallError:
		kindStr = "tool_call_error"
	default:
		kindStr = "unknown"
	}

	d.SetKey(starlark.String("kind"), starlark.String(kindStr)) //nolint:errcheck

	switch event.Kind {
	case gateway.StreamEventText, gateway.StreamEventThinking:
		d.SetKey(starlark.String("delta"), starlark.String(event.Delta)) //nolint:errcheck
	case gateway.StreamEventUsage, gateway.StreamEventDone:
		if event.Usage != nil {
			usageDict := starlark.NewDict(3)
			usageDict.SetKey(starlark.String("prompt"), starlark.MakeInt(event.Usage.PromptTokens))         //nolint:errcheck
			usageDict.SetKey(starlark.String("completion"), starlark.MakeInt(event.Usage.CompletionTokens)) //nolint:errcheck
			usageDict.SetKey(starlark.String("total"), starlark.MakeInt(event.Usage.TotalTokens))           //nolint:errcheck
			d.SetKey(starlark.String("usage"), usageDict)                                                   //nolint:errcheck
		}
	case gateway.StreamEventError:
		d.SetKey(starlark.String("error"), starlark.String(event.Error))           //nolint:errcheck
		d.SetKey(starlark.String("recoverable"), starlark.Bool(event.Recoverable)) //nolint:errcheck
	case gateway.StreamEventToolCallStart, gateway.StreamEventToolCallEnd, gateway.StreamEventToolCallError:
		d.SetKey(starlark.String("tool_name"), starlark.String(event.ToolName)) //nolint:errcheck
		d.SetKey(starlark.String("tool_id"), starlark.String(event.ToolID))     //nolint:errcheck
		if event.Arguments != nil {
			argsDict := starlark.NewDict(len(event.Arguments))
			for k, v := range event.Arguments {
				argsDict.SetKey(starlark.String(k), goToStarlark(v)) //nolint:errcheck
			}
			d.SetKey(starlark.String("arguments"), argsDict) //nolint:errcheck
		}
		if event.Kind != gateway.StreamEventToolCallStart {
			d.SetKey(starlark.String("duration_ms"), starlark.MakeInt64(event.DurationMS)) //nolint:errcheck
		}
		if event.Kind == gateway.StreamEventToolCallError {
			d.SetKey(starlark.String("error"), starlark.String(event.Error)) //nolint:errcheck
		}
	}

	return d
}

// responseToStarlark converts the final response text to a Starlark value.
// If responseFormat is "json_object", parses the text as JSON and returns a dict.
func responseToStarlark(text, responseFormat string) (starlark.Value, error) {
	if responseFormat == "json_object" {
		var raw interface{}
		if err := json.Unmarshal([]byte(text), &raw); err != nil {
			return nil, fmt.Errorf("failed to parse JSON response: %w", err)
		}
		return goToStarlark(raw), nil
	}
	return starlark.String(text), nil
}

// llmChat implements ctx.llm.chat().
//
//	chat(prompt, preset, system=None, use_session=True, stream=False,
//	     on_event=None, response_format=None, response_schema=None) → str | dict
func (m *LLMModule) llmChat(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		prompt         string
		presetName     string
		system         string
		useSession     bool = true
		stream         bool = false
		onEventFn      starlark.Callable
		responseFormat string
		responseSchema *starlark.Dict
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"prompt", &prompt,
		"preset", &presetName,
		"system?", &system,
		"use_session?", &useSession,
		"stream?", &stream,
		"on_event?", &onEventFn,
		"response_format?", &responseFormat,
		"response_schema?", &responseSchema,
	); err != nil {
		return nil, err
	}

	if presetName == "" {
		return nil, fmt.Errorf("chat: preset must be explicitly provided")
	}

	if m.runtime.llmServices == nil {
		return nil, fmt.Errorf("llm services not configured")
	}

	ctx := context.Background()

	// Resolve preset
	presetObj, err := m.runtime.llmServices.PresetService.Get(domainpreset.Preset(presetName))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve preset %s: %w", presetName, err)
	}

	// Create gateway
	llmGateway, err := m.runtime.llmServices.GatewayFactory.NewGenerationGateway(ctx, presetObj)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM gateway for preset '%s': %w", presetName, err)
	}

	// Build conversation history if use_session=True
	var history []gateway.Message
	if useSession {
		history, err = m.loadSessionHistory(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Build request
	request := gateway.NewGenerateContentRequest(
		presetObj.Model,
		system,
		prompt,
		presetObj.MaxOutputTokens,
	)

	// Attach history (minus the current user prompt, which is in request already)
	if len(history) > 0 {
		request.WithMessages(history)
	}

	// Apply response format/schema
	if responseFormat != "" {
		request.WithResponseFormat(&responseFormat)
	}
	if responseSchema != nil {
		schema := starlarkToGo(responseSchema)
		if schemaMap, ok := schema.(map[string]interface{}); ok {
			request.WithResponseSchema(schemaMap)
		} else {
			return nil, fmt.Errorf("response_schema must be a dict")
		}
	}

	applyPresetParameters(request, presetObj)

	// Execute: streaming or non-streaming
	var responseText string
	var usage *gateway.UsageMetadata

	startTime := time.Now()

	if stream && onEventFn != nil {
		resp, streamErr := llmGateway.GenerateContentStream(ctx, request, func(event gateway.StreamEvent) error {
			eventDict := streamEventToStarlark(event)
			_, callErr := starlark.Call(thread, onEventFn, starlark.Tuple{eventDict}, nil)
			return callErr
		})
		if streamErr != nil {
			return nil, fmt.Errorf("LLM streaming failed with preset '%s': %w", presetName, streamErr)
		}
		responseText = resp.Text()
		usage = resp.Usage
	} else if stream {
		// stream=True but no on_event: still stream under the hood, discard events
		resp, streamErr := llmGateway.GenerateContentStream(ctx, request, func(_ gateway.StreamEvent) error {
			return nil
		})
		if streamErr != nil {
			return nil, fmt.Errorf("LLM streaming failed with preset '%s': %w", presetName, streamErr)
		}
		responseText = resp.Text()
		usage = resp.Usage
	} else {
		resp, genErr := llmGateway.GenerateContent(ctx, request)
		if genErr != nil {
			return nil, fmt.Errorf("LLM generation failed with preset '%s': %w", presetName, genErr)
		}
		responseText = resp.Text()
		usage = resp.Usage
	}

	duration := time.Since(startTime)

	// Persist user message and assistant response in session (only after successful response)
	if useSession && m.runtime.sessionService != nil && m.currentSession != nil {
		if err := m.runtime.sessionService.AddUserMessage(ctx, m.currentSession.ID, prompt); err != nil {
			log.Printf("session: failed to write user message: %v", err)
		}
		if err := m.runtime.sessionService.AddAssistantMessage(ctx, m.currentSession.ID, responseText, nil); err != nil {
			log.Printf("session: failed to write assistant message: %v", err)
		}

		ts := time.Now().UnixNano()
		if err := m.runtime.sessionService.SetMetadata(ctx, m.currentSession.ID,
			fmt.Sprintf("llm_duration_ms_%d", ts),
			fmt.Sprintf("%d", duration.Milliseconds())); err != nil {
			log.Printf("session: failed to write duration metadata: %v", err)
		}

		if usage != nil {
			if err := m.runtime.sessionService.SetMetadata(ctx, m.currentSession.ID,
				fmt.Sprintf("llm_tokens_%d", ts),
				fmt.Sprintf("prompt=%d,completion=%d,total=%d",
					usage.PromptTokens,
					usage.CompletionTokens,
					usage.TotalTokens)); err != nil {
				log.Printf("session: failed to write token metadata: %v", err)
			}
		}
	}

	return responseToStarlark(responseText, responseFormat)
}

// llmAgentTurn implements ctx.llm.agent_turn().
//
//	agent_turn(prompt, preset, tools, system=None, use_session=True, stream=False,
//	           on_event=None, max_iterations=50, on_tool_error="return",
//	           response_format=None, response_schema=None) → str | dict
func (m *LLMModule) llmAgentTurn(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		prompt         string
		presetName     string
		toolsList      *starlark.List
		system         string
		useSession     bool = true
		stream         bool = false
		onEventFn      starlark.Callable
		maxIterations  int    = 50
		onToolError    string = "return"
		responseFormat string
		responseSchema *starlark.Dict
	)

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"prompt", &prompt,
		"preset", &presetName,
		"tools", &toolsList,
		"system?", &system,
		"use_session?", &useSession,
		"stream?", &stream,
		"on_event?", &onEventFn,
		"max_iterations?", &maxIterations,
		"on_tool_error?", &onToolError,
		"response_format?", &responseFormat,
		"response_schema?", &responseSchema,
	); err != nil {
		return nil, err
	}

	if presetName == "" {
		return nil, fmt.Errorf("agent_turn: preset must be explicitly provided")
	}

	// Validate on_tool_error
	if onToolError != "return" && onToolError != "abort" {
		return nil, fmt.Errorf("on_tool_error must be 'return' or 'abort', got '%s'", onToolError)
	}

	if m.runtime.llmServices == nil {
		return nil, fmt.Errorf("llm services not configured")
	}

	ctx := context.Background()

	// Build tool definitions
	toolDefinitions := make([]gateway.ToolDefinition, 0, toolsList.Len())
	toolsByName := make(map[string]*Tool)
	for i := 0; i < toolsList.Len(); i++ {
		toolValue, ok := toolsList.Index(i).(*ToolValue)
		if !ok {
			return nil, fmt.Errorf("tools[%d] is not a tool object", i)
		}
		tool := toolValue.Tool
		toolsByName[tool.Name] = tool
		schema := tool.GenerateToolSchema()
		toolDefinitions = append(toolDefinitions, gateway.ToolDefinition{
			Name:        schema.Name,
			Description: schema.Description,
			Parameters:  schema.Parameters,
		})
	}

	// Resolve preset
	presetObj, err := m.runtime.llmServices.PresetService.Get(domainpreset.Preset(presetName))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve preset %s: %w", presetName, err)
	}

	// Create gateway
	llmGateway, err := m.runtime.llmServices.GatewayFactory.NewGenerationGateway(ctx, presetObj)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM gateway for preset '%s': %w", presetName, err)
	}

	// Load session history if use_session=True
	var history []gateway.Message
	if useSession {
		history, err = m.loadSessionHistory(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Build the working messages slice: history + current user prompt
	messages := make([]gateway.Message, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, gateway.Message{
		Role:    gateway.MessageRoleUser,
		Content: prompt,
	})

	// makeCallback builds a StreamCallback that forwards events to on_event if provided.
	makeCallback := func() gateway.StreamCallback {
		if !stream || onEventFn == nil {
			return func(_ gateway.StreamEvent) error { return nil }
		}
		return func(event gateway.StreamEvent) error {
			eventDict := streamEventToStarlark(event)
			_, callErr := starlark.Call(thread, onEventFn, starlark.Tuple{eventDict}, nil)
			return callErr
		}
	}

	// Agentic loop
	var finalResponse string
	userMessageWritten := false
	for iteration := 0; iteration < maxIterations; iteration++ {
		// Build request
		request := gateway.NewGenerateContentRequest(
			presetObj.Model,
			system,
			"", // Prompt is in messages
			presetObj.MaxOutputTokens,
		)
		request.WithMessages(messages).WithTools(toolDefinitions)

		if responseFormat != "" {
			request.WithResponseFormat(&responseFormat)
		}
		if responseSchema != nil {
			schema := starlarkToGo(responseSchema)
			if schemaMap, ok := schema.(map[string]interface{}); ok {
				request.WithResponseSchema(schemaMap)
			} else {
				return nil, fmt.Errorf("response_schema must be a dict")
			}
		}
		applyPresetParameters(request, presetObj)

		// Call LLM
		startTime := time.Now()
		var resp *gateway.GenerateContentResponse
		var callErr error

		if stream {
			resp, callErr = llmGateway.GenerateContentStream(ctx, request, makeCallback())
		} else {
			resp, callErr = llmGateway.GenerateContent(ctx, request)
		}

		if callErr != nil {
			if errors.Is(callErr, gateway.ErrToolCallingNotSupported) {
				return nil, fmt.Errorf("agent_turn requires tool calling support, but gateway does not support it")
			}
			return nil, fmt.Errorf("LLM generation failed: %w", callErr)
		}

		duration := time.Since(startTime)
		toolCalls := resp.ToolCalls()
		responseText := resp.Text()

		// Persist messages in session (user message written once, after first successful LLM call)
		if useSession && m.runtime.sessionService != nil && m.currentSession != nil {
			if !userMessageWritten {
				if wErr := m.runtime.sessionService.AddUserMessage(ctx, m.currentSession.ID, prompt); wErr != nil {
					log.Printf("session: failed to write user message: %v", wErr)
				}
				userMessageWritten = true
			}

			sessionToolCalls := make([]session.ToolCall, 0, len(toolCalls))
			for _, tc := range toolCalls {
				sessionToolCalls = append(sessionToolCalls, session.ToolCall{
					ID:     tc.ID,
					Name:   tc.Name,
					Params: tc.Arguments,
				})
			}
			if wErr := m.runtime.sessionService.AddAssistantMessage(ctx, m.currentSession.ID, responseText, sessionToolCalls); wErr != nil {
				log.Printf("session: failed to write assistant message: %v", wErr)
			}

			ts := fmt.Sprintf("%d_%d", iteration, time.Now().UnixNano())
			if wErr := m.runtime.sessionService.SetMetadata(ctx, m.currentSession.ID,
				fmt.Sprintf("llm_duration_ms_%s", ts),
				fmt.Sprintf("%d", duration.Milliseconds())); wErr != nil {
				log.Printf("session: failed to write duration metadata: %v", wErr)
			}

			if resp.Usage != nil {
				if wErr := m.runtime.sessionService.SetMetadata(ctx, m.currentSession.ID,
					fmt.Sprintf("llm_tokens_%s", ts),
					fmt.Sprintf("prompt=%d,completion=%d,total=%d",
						resp.Usage.PromptTokens,
						resp.Usage.CompletionTokens,
						resp.Usage.TotalTokens)); wErr != nil {
					log.Printf("session: failed to write token metadata: %v", wErr)
				}
			}
		}

		// No tool calls means we're done
		if len(toolCalls) == 0 {
			finalResponse = responseText
			break
		}

		// Add assistant turn with tool calls to working messages
		messages = append(messages, gateway.Message{
			Role:      gateway.MessageRoleAssistant,
			Content:   responseText,
			ToolCalls: toolCalls,
		})

		// Execute each tool call
		for _, toolCall := range toolCalls {
			tool, exists := toolsByName[toolCall.Name]
			if !exists {
				errorMsg := fmt.Sprintf("Tool '%s' not found", toolCall.Name)
				if onToolError == "abort" {
					return nil, fmt.Errorf("tool '%s' not found", toolCall.Name)
				}
				if useSession && m.runtime.sessionService != nil && m.currentSession != nil {
					if wErr := m.runtime.sessionService.AddToolResult(ctx, m.currentSession.ID, toolCall.ID, errorMsg); wErr != nil {
						log.Printf("session: failed to write tool result: %v", wErr)
					}
				}
				messages = append(messages, gateway.Message{
					Role:       gateway.MessageRoleTool,
					Content:    errorMsg,
					ToolCallID: toolCall.ID,
					ToolName:   toolCall.Name,
				})
				continue
			}

			result, toolErr := m.executeToolForAgentic(thread, tool, toolCall.Arguments)

			var resultContent string
			if toolErr != nil {
				resultContent = fmt.Sprintf("Error: %s", toolErr.Error())
				if onToolError == "abort" {
					return nil, fmt.Errorf("tool '%s' failed: %w", toolCall.Name, toolErr)
				}
			} else {
				resultContent = result.String()
			}

			if useSession && m.runtime.sessionService != nil && m.currentSession != nil {
				if wErr := m.runtime.sessionService.AddToolResult(ctx, m.currentSession.ID, toolCall.ID, resultContent); wErr != nil {
					log.Printf("session: failed to write tool result: %v", wErr)
				}
			}

			messages = append(messages, gateway.Message{
				Role:       gateway.MessageRoleTool,
				Content:    resultContent,
				ToolCallID: toolCall.ID,
				ToolName:   toolCall.Name,
			})
		}
	}

	if finalResponse == "" {
		return nil, fmt.Errorf("agent_turn: maximum iterations (%d) reached without final response", maxIterations)
	}

	return responseToStarlark(finalResponse, responseFormat)
}

// llmEmbed implements llm.embed().
func (m *LLMModule) llmEmbed(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var texts *starlark.List
	var presetName string

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "texts", &texts, "preset", &presetName); err != nil {
		return nil, err
	}

	if presetName == "" {
		return nil, fmt.Errorf("embed: preset must be explicitly provided")
	}

	if m.runtime.llmServices == nil {
		return nil, fmt.Errorf("llm services not configured")
	}

	// Resolve preset
	presetObj, err := m.runtime.llmServices.PresetService.Get(domainpreset.Preset(presetName))
	if err != nil {
		return nil, fmt.Errorf("failed to get preset %s: %w", presetName, err)
	}

	// Create embeddings gateway
	ctx := context.Background()
	embGateway, err := m.runtime.llmServices.EmbeddingsFactory.NewEmbeddingsGateway(ctx, presetObj)
	if err != nil {
		return nil, fmt.Errorf("failed to create embeddings gateway: %w", err)
	}

	// Convert texts to string slice
	textSlice := make([]string, 0, texts.Len())
	for i := 0; i < texts.Len(); i++ {
		if str, ok := texts.Index(i).(starlark.String); ok {
			textSlice = append(textSlice, string(str))
		}
	}

	// Compute embeddings
	request := gateway.NewComputeEmbeddingsRequest(
		presetObj.Model,
		textSlice,
		gateway.RetrievalDocument,
	)

	embeddings, err := embGateway.ComputeEmbeddings(ctx, request)

	// If single batch exceeds rate limit capacity, recursively split it
	if err != nil && len(textSlice) > 1 {
		errMsg := err.Error()
		if strings.Contains(errMsg, "exceeds rate limit capacity") {
			return m.splitAndComputeEmbeddings(ctx, embGateway, presetObj.Model, textSlice)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to compute embeddings for %d texts with preset '%s': %w", texts.Len(), presetName, err)
	}

	// Convert embeddings to Starlark list of lists
	result := make([]starlark.Value, len(embeddings))
	for i, emb := range embeddings {
		embList := make([]starlark.Value, len(emb))
		for j, val := range emb {
			embList[j] = starlark.Float(val)
		}
		result[i] = starlark.NewList(embList)
	}

	return starlark.NewList(result), nil
}

// applyPresetParameters applies all preset parameters to a gateway request.
func applyPresetParameters(request *gateway.GenerateContentRequest, preset *domainpreset.ResolvedPreset) {
	if preset == nil {
		return
	}

	// Sampling parameters
	if preset.Temperature != nil {
		request.WithTemperature(preset.Temperature)
	}
	if preset.TopP != nil {
		request.WithTopP(preset.TopP)
	}
	if preset.TopK != nil {
		request.WithTopK(preset.TopK)
	}
	if preset.FrequencyPenalty != nil {
		request.WithFrequencyPenalty(preset.FrequencyPenalty)
	}
	if preset.PresencePenalty != nil {
		request.WithPresencePenalty(preset.PresencePenalty)
	}
	if preset.Seed != nil {
		request.WithSeed(preset.Seed)
	}

	// Stop sequences
	if len(preset.Stop) > 0 {
		request.WithStop(preset.Stop)
	}

	// Candidate configuration
	if preset.CandidateCount != nil {
		request.WithCandidateCount(preset.CandidateCount)
	}

	// Log probabilities
	if preset.LogProbs != nil {
		request.WithLogProbs(preset.LogProbs)
	}
	if preset.TopLogProbs != nil {
		request.WithTopLogProbs(preset.TopLogProbs)
	}

	// Logit bias
	if len(preset.LogitBias) > 0 {
		request.WithLogitBias(preset.LogitBias)
	}

	// System parameters
	if preset.ServiceTier != nil {
		request.WithServiceTier(preset.ServiceTier)
	}
	if preset.User != nil {
		request.WithUser(preset.User)
	}

	// Advanced sampling parameters
	if preset.RepetitionPenalty != nil {
		request.WithRepetitionPenalty(preset.RepetitionPenalty)
	}
	if preset.MinP != nil {
		request.WithMinP(preset.MinP)
	}
	if preset.TopA != nil {
		request.WithTopA(preset.TopA)
	}
	if preset.TypicalP != nil {
		request.WithTypicalP(preset.TypicalP)
	}
	if preset.Mirostat != nil {
		request.WithMirostat(preset.Mirostat)
	}
	if preset.MirostatTau != nil {
		request.WithMirostatTau(preset.MirostatTau)
	}
	if preset.MirostatEta != nil {
		request.WithMirostatEta(preset.MirostatEta)
	}
	if preset.Grammar != nil {
		request.WithGrammar(preset.Grammar)
	}
}

// executeToolForAgentic executes a tool with given parameters in the agentic loop context.
func (m *LLMModule) executeToolForAgentic(thread *starlark.Thread, tool *Tool, params map[string]any) (starlark.Value, error) {
	// Convert params map to Starlark values
	paramsMembers := make(map[string]starlark.Value)
	for key, value := range params {
		paramsMembers[key] = convertGoValueToStarlark(value)
	}

	// Fill in defaults for missing params
	for paramName, param := range tool.Params {
		if _, exists := paramsMembers[paramName]; !exists {
			if param.Default != nil {
				paramsMembers[paramName] = convertGoValueToStarlark(param.Default)
			} else {
				// Provide zero values for required types
				switch param.Type {
				case "bool":
					paramsMembers[paramName] = starlark.False
				case "int":
					paramsMembers[paramName] = starlark.MakeInt(0)
				case "float":
					paramsMembers[paramName] = starlark.Float(0.0)
				default:
					paramsMembers[paramName] = starlark.String("")
				}
			}
		}
	}

	// Validate parameters
	if err := ValidateToolParams(m.runtime, m.runtime.registry, tool, paramsMembers); err != nil {
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	// Create context for tool execution
	flagsMembers := make(starlark.StringDict)
	for k, v := range paramsMembers {
		flagsMembers[k] = v
	}

	childCtxMembers := starlark.StringDict{
		"flags":     starlarkstruct.FromStringDict(starlarkstruct.Default, flagsMembers),
		"args":      starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{}),
		"fs":        m.runtime.CreateFSModuleForCtx(),
		"git":       m.runtime.CreateGitModuleForCtx(),
		"llm":       m.runtime.CreateLLMModuleForCtx(m.currentSession),
		"shell":     m.runtime.CreateShellModuleForCtx(),
		"index":     m.runtime.CreateIndexModuleForCtx(),
		"output":    m.runtime.CreateOutputModuleForCtx(),
		"session":   m.runtime.CreateSessionModuleForCtx(m.currentSession),
		"json":      NewJSONModule(),
		"yaml":      NewYAMLModule(),
		"xml":       NewXMLModule(),
		"toml":      NewTOMLModule(),
		"csv":       NewCSVModule(),
		"env":       NewEnvModule(),
		"ui":        m.runtime.CreateUIModuleForCtx(0),
		"path":      NewPathModule(),
		"crypto":    NewCryptoModule(),
		"time":      NewTimeModule(),
		"regexp":    NewRegexpModule(),
		"http":      NewHTTPModule(),
		"template":  NewTemplateModule(m.runtime.WorkingDir()),
		"stdin":     m.runtime.CreateStdinModuleForCtx(),
		"workspace": starlark.String(m.runtime.WorkingDir()),
		"run":       CreateRunFunction(m.runtime.registry, m.runtime, m.currentSession, 0),
	}

	childCtxStruct := starlarkstruct.FromStringDict(starlarkstruct.Default, childCtxMembers)
	childCtx := CreateContextWithParams(childCtxStruct, paramsMembers)

	// Call the tool handler
	result, err := starlark.Call(thread, tool.Handler, starlark.Tuple{childCtx}, nil)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// splitAndComputeEmbeddings recursively splits batches that exceed rate limit capacity
func (m *LLMModule) splitAndComputeEmbeddings(
	ctx context.Context,
	embGateway ports.EmbeddingsGateway,
	model string,
	texts []string,
) (starlark.Value, error) {
	if len(texts) == 0 {
		return starlark.NewList(nil), nil
	}

	if len(texts) == 1 {
		return nil, fmt.Errorf("single text exceeds rate limit capacity - text is too large to embed")
	}

	// Split in half
	mid := len(texts) / 2

	// Process first half
	firstReq := gateway.NewComputeEmbeddingsRequest(model, texts[:mid], gateway.RetrievalDocument)
	firstEmbs, err := embGateway.ComputeEmbeddings(ctx, firstReq)
	if err != nil {
		if strings.Contains(err.Error(), "exceeds rate limit capacity") {
			return m.splitAndComputeEmbeddings(ctx, embGateway, model, texts[:mid])
		}
		return nil, fmt.Errorf("failed to compute embeddings (first half): %w", err)
	}

	// Process second half
	secondReq := gateway.NewComputeEmbeddingsRequest(model, texts[mid:], gateway.RetrievalDocument)
	secondEmbs, err := embGateway.ComputeEmbeddings(ctx, secondReq)
	if err != nil {
		if strings.Contains(err.Error(), "exceeds rate limit capacity") {
			return m.splitAndComputeEmbeddings(ctx, embGateway, model, texts[mid:])
		}
		return nil, fmt.Errorf("failed to compute embeddings (second half): %w", err)
	}

	// Combine results
	allEmbeddings := append(firstEmbs, secondEmbs...)

	// Convert to Starlark list of lists
	result := make([]starlark.Value, len(allEmbeddings))
	for i, emb := range allEmbeddings {
		embList := make([]starlark.Value, len(emb))
		for j, val := range emb {
			embList[j] = starlark.Float(val)
		}
		result[i] = starlark.NewList(embList)
	}

	return starlark.NewList(result), nil
}
