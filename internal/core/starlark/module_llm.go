// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	gatewayimpl "github.com/retran/meowg1k/internal/adapters/gateway"
	"github.com/retran/meowg1k/internal/core/model"
	"github.com/retran/meowg1k/internal/core/preset"
	"github.com/retran/meowg1k/internal/domain/gateway"
	domainpreset "github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/internal/domain/session"
	"github.com/retran/meowg1k/internal/ports"
)

// LLMServices holds references to LLM-related services.
type LLMServices struct {
	PresetService     *preset.Service
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
		"generate": starlark.NewBuiltin("generate", module.llmGenerate),
		"embed":    starlark.NewBuiltin("embed", module.llmEmbed),
		"agentic":  starlark.NewBuiltin("agentic", module.llmAgentic),
	})
}

// llmGenerate implements llm.generate().
func (m *LLMModule) llmGenerate(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var prompt, system, presetName string = "", "", "smart"

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "prompt", &prompt, "system?", &system, "preset?", &presetName); err != nil {
		return nil, err
	}

	if m.runtime.llmServices == nil {
		return nil, fmt.Errorf("llm services not configured")
	}

	ctx := context.Background()

	// Track start time for performance metrics
	startTime := time.Now()

	// Track system and user messages in session (if session exists)
	if m.runtime.sessionService != nil && m.currentSession != nil {
		if system != "" {
			_ = m.runtime.sessionService.AddSystemMessage(ctx, m.currentSession.ID, system)
		}
		_ = m.runtime.sessionService.AddUserMessage(ctx, m.currentSession.ID, prompt)
	}

	// Resolve preset
	presetObj, err := m.runtime.llmServices.PresetService.Get(domainpreset.Preset(presetName))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve preset %s: %w", presetName, err)
	}

	// Note: preset already contains resolved model info, no need to resolve again

	// Create gateway
	llmGateway, err := m.runtime.llmServices.GatewayFactory.NewGenerationGateway(ctx, presetObj)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM gateway for preset '%s': %w", presetName, err)
	}

	// Generate
	request := gateway.NewGenerateContentRequest(
		presetObj.Model, // Use actual model ID (e.g., "gemini-3-pro-preview"), not registered name
		system,
		prompt,
		presetObj.MaxOutputTokens,
	)

	response, err := llmGateway.GenerateContent(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed with preset '%s': %w", presetName, err)
	}

	// Calculate duration
	duration := time.Since(startTime)

	// Track assistant message in session (if session exists)
	if m.runtime.sessionService != nil && m.currentSession != nil {
		_ = m.runtime.sessionService.AddAssistantMessage(ctx, m.currentSession.ID, response.Text(), nil)

		// Store timing and token usage as metadata
		_ = m.runtime.sessionService.SetMetadata(ctx, m.currentSession.ID,
			fmt.Sprintf("llm_duration_ms_%d", time.Now().Unix()),
			fmt.Sprintf("%d", duration.Milliseconds()))

		if response.Usage != nil {
			_ = m.runtime.sessionService.SetMetadata(ctx, m.currentSession.ID,
				fmt.Sprintf("llm_tokens_%d", time.Now().Unix()),
				fmt.Sprintf("prompt=%d,completion=%d,total=%d",
					response.Usage.PromptTokens,
					response.Usage.CompletionTokens,
					response.Usage.TotalTokens))
		}
	}

	return starlark.String(response.Text()), nil
}

// llmEmbed implements llm.embed().
func (m *LLMModule) llmEmbed(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var texts *starlark.List
	var presetName string = "embeddings"

	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "texts", &texts, "preset?", &presetName); err != nil {
		return nil, err
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
		// Check if error is about exceeding capacity (not just temporarily exhausted)
		if strings.Contains(errMsg, "exceeds rate limit capacity") {
			// Recursively split batch until each piece fits within capacity
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

// llmAgentic implements llm.agentic() - an agentic loop with native tool calling.
func (m *LLMModule) llmAgentic(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var toolsList *starlark.List
	var prompt, system, presetName string = "", "", "smart"
	var onToolError string = "return"
	var maxIterations int = 50

	if err := starlark.UnpackArgs(b.Name(), args, kwargs,
		"tools", &toolsList,
		"prompt", &prompt,
		"system?", &system,
		"preset?", &presetName,
		"on_tool_error?", &onToolError,
		"max_iterations?", &maxIterations,
	); err != nil {
		return nil, err
	}

	// Validate on_tool_error
	if onToolError != "return" && onToolError != "retry" && onToolError != "abort" {
		return nil, fmt.Errorf("on_tool_error must be 'return', 'retry', or 'abort', got '%s'", onToolError)
	}

	if m.runtime.llmServices == nil {
		return nil, fmt.Errorf("llm services not configured")
	}

	ctx := context.Background()

	// Convert Starlark tools list to gateway.ToolDefinition
	toolDefinitions := make([]gateway.ToolDefinition, 0, toolsList.Len())
	toolsByName := make(map[string]*Tool)

	for i := 0; i < toolsList.Len(); i++ {
		toolValue, ok := toolsList.Index(i).(*ToolValue)
		if !ok {
			return nil, fmt.Errorf("tools[%d] is not a tool object", i)
		}

		tool := toolValue.Tool
		toolsByName[tool.Name] = tool

		// Generate tool schema using existing method
		schema := tool.GenerateToolSchema()

		// Convert to gateway.ToolDefinition
		toolDef := gateway.ToolDefinition{
			Name:        schema.Name,
			Description: schema.Description,
			Parameters:  schema.Parameters,
		}
		toolDefinitions = append(toolDefinitions, toolDef)
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

	// Track initial system and user messages
	if m.runtime.sessionService != nil && m.currentSession != nil {
		if system != "" {
			_ = m.runtime.sessionService.AddSystemMessage(ctx, m.currentSession.ID, system)
		}
		_ = m.runtime.sessionService.AddUserMessage(ctx, m.currentSession.ID, prompt)
	}

	// Build messages array for multi-turn conversation
	messages := []gateway.Message{
		{Role: gateway.MessageRoleUser, Content: prompt},
	}

	// Agentic loop
	var finalResponse string
	for iteration := 0; iteration < maxIterations; iteration++ {
		// Create request with tools
		request := gateway.NewGenerateContentRequest(
			presetObj.Model,
			system,
			"", // Empty user prompt since we use messages
			presetObj.MaxOutputTokens,
		)
		request.WithMessages(messages).WithTools(toolDefinitions)

		// Call LLM
		startTime := time.Now()
		response, err := llmGateway.GenerateContent(ctx, request)
		if err != nil {
			// Check if tool calling is not supported
			if err == gateway.ErrToolCallingNotSupported {
				return nil, fmt.Errorf("agentic loop requires tool calling support, but gateway does not support it")
			}
			return nil, fmt.Errorf("LLM generation failed: %w", err)
		}
		duration := time.Since(startTime)

		// Extract tool calls from response
		toolCalls := response.ToolCalls()
		responseText := response.Text()

		// Track assistant message with tool calls (if any)
		if m.runtime.sessionService != nil && m.currentSession != nil {
			// Convert gateway.ToolCall to session.ToolCall
			sessionToolCalls := make([]session.ToolCall, 0, len(toolCalls))
			for _, tc := range toolCalls {
				sessionToolCalls = append(sessionToolCalls, session.ToolCall{
					ID:     tc.ID,
					Name:   tc.Name,
					Params: tc.Arguments,
				})
			}

			_ = m.runtime.sessionService.AddAssistantMessage(ctx, m.currentSession.ID, responseText, sessionToolCalls)

			// Store timing and token usage
			_ = m.runtime.sessionService.SetMetadata(ctx, m.currentSession.ID,
				fmt.Sprintf("llm_duration_ms_%d", time.Now().Unix()),
				fmt.Sprintf("%d", duration.Milliseconds()))

			if response.Usage != nil {
				_ = m.runtime.sessionService.SetMetadata(ctx, m.currentSession.ID,
					fmt.Sprintf("llm_tokens_%d", time.Now().Unix()),
					fmt.Sprintf("prompt=%d,completion=%d,total=%d",
						response.Usage.PromptTokens,
						response.Usage.CompletionTokens,
						response.Usage.TotalTokens))
			}
		}

		// If no tool calls, we're done
		if len(toolCalls) == 0 {
			finalResponse = responseText
			break
		}

		// Add assistant message with tool calls to conversation
		assistantMsg := gateway.Message{
			Role:      gateway.MessageRoleAssistant,
			Content:   responseText,
			ToolCalls: toolCalls,
		}
		messages = append(messages, assistantMsg)

		// Execute each tool call
		for _, toolCall := range toolCalls {
			tool, exists := toolsByName[toolCall.Name]
			if !exists {
				errorMsg := fmt.Sprintf("Tool '%s' not found", toolCall.Name)
				if onToolError == "abort" {
					return nil, fmt.Errorf("tool '%s' not found", toolCall.Name)
				}
				// Track error as tool result
				if m.runtime.sessionService != nil && m.currentSession != nil {
					_ = m.runtime.sessionService.AddToolResult(ctx, m.currentSession.ID, toolCall.ID, errorMsg)
				}
				// Add error to messages
				messages = append(messages, gateway.Message{
					Role:       gateway.MessageRoleTool,
					Content:    errorMsg,
					ToolCallID: toolCall.ID,
					ToolName:   toolCall.Name,
				})
				continue
			}

			// Execute tool
			result, toolErr := m.executeToolForAgentic(thread, tool, toolCall.Arguments)

			var resultContent string
			if toolErr != nil {
				resultContent = fmt.Sprintf("Error: %s", toolErr.Error())
				if onToolError == "abort" {
					return nil, fmt.Errorf("tool '%s' failed: %w", toolCall.Name, toolErr)
				}
				// For "retry" mode, we'll just continue and let LLM try again
				// For "return" mode, we return the error as tool result
			} else {
				// Convert result to string
				resultContent = result.String()
			}

			// Track tool result
			if m.runtime.sessionService != nil && m.currentSession != nil {
				_ = m.runtime.sessionService.AddToolResult(ctx, m.currentSession.ID, toolCall.ID, resultContent)
			}

			// Add tool result to messages
			messages = append(messages, gateway.Message{
				Role:       gateway.MessageRoleTool,
				Content:    resultContent,
				ToolCallID: toolCall.ID,
				ToolName:   toolCall.Name,
			})
		}

		// Continue loop to let LLM process tool results
	}

	// If we exhausted iterations, return what we have
	if finalResponse == "" {
		finalResponse = "Maximum iterations reached without final response"
	}

	return starlark.String(finalResponse), nil
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
	// Note: We create a minimal context since tools called by LLM shouldn't have full CLI context
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
		"env":       NewEnvModule(),
		"ui":        NewIndentedUIModule(0),
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
		// Can't split further - this single text exceeds capacity
		return nil, fmt.Errorf("single text exceeds rate limit capacity - text is too large to embed")
	}

	// Split in half
	mid := len(texts) / 2

	// Process first half
	firstReq := gateway.NewComputeEmbeddingsRequest(model, texts[:mid], gateway.RetrievalDocument)
	firstEmbs, err := embGateway.ComputeEmbeddings(ctx, firstReq)
	if err != nil {
		// If first half still exceeds capacity, recursively split it
		if strings.Contains(err.Error(), "exceeds rate limit capacity") {
			return m.splitAndComputeEmbeddings(ctx, embGateway, model, texts[:mid])
		}
		return nil, fmt.Errorf("failed to compute embeddings (first half): %w", err)
	}

	// Process second half
	secondReq := gateway.NewComputeEmbeddingsRequest(model, texts[mid:], gateway.RetrievalDocument)
	secondEmbs, err := embGateway.ComputeEmbeddings(ctx, secondReq)
	if err != nil {
		// If second half still exceeds capacity, recursively split it
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
