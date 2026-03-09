// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/retran/meowg1k/internal/adapters/tracelog"
	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

// TraceLogger defines the interface for trace logging.
type TraceLogger interface {
	LogAPIInteraction(entry *tracelog.APIInteractionEntry) error
}

// loggingGenerationGateway wraps a GenerationGateway to log all API interactions.
type loggingGenerationGateway struct {
	inner    ports.GenerationGateway
	logger   TraceLogger
	command  string
	preset   string
	provider string
}

// newLoggingGenerationGateway creates a new logging wrapper for a generation gateway.
func newLoggingGenerationGateway(
	inner ports.GenerationGateway,
	logger TraceLogger,
	command string,
	preset string,
	provider string,
) ports.GenerationGateway {
	if logger == nil {
		return inner
	}

	return &loggingGenerationGateway{
		inner:    inner,
		logger:   logger,
		command:  command,
		preset:   preset,
		provider: provider,
	}
}

// GenerateContent wraps the inner gateway's GenerateContent and logs the interaction.
func (g *loggingGenerationGateway) GenerateContent(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
) (*gateway.GenerateContentResponse, error) {
	startTime := time.Now()

	response, err := g.inner.GenerateContent(ctx, request)

	duration := time.Since(startTime)

	// Log the interaction
	entry := &tracelog.APIInteractionEntry{
		Command:  g.command,
		Preset:   g.preset,
		Provider: g.provider,
		Model:    request.Model(),
		Request: tracelog.RequestData{
			SystemPrompt:    request.SystemPrompt(),
			UserPrompt:      request.UserPrompt(),
			MaxOutputTokens: request.MaxOutputTokens(),
		},
		Response: tracelog.ResponseData{
			Content: formatResponseContent(response),
		},
		DurationMs: duration.Milliseconds(),
	}

	// Add usage information if available
	if response != nil && response.Usage != nil {
		entry.Usage = tracelog.UsageData{
			PromptTokens:     response.Usage.PromptTokens,
			CompletionTokens: response.Usage.CompletionTokens,
			TotalTokens:      response.Usage.TotalTokens,
		}
	}

	if err != nil {
		entry.Response.Error = err.Error()
	}

	// Log synchronously - logging errors are not critical but should be fast
	if logErr := g.logger.LogAPIInteraction(entry); logErr != nil {
		// Ignore logging errors, but they're visible in debug mode
		_ = logErr
	}

	if err != nil {
		return response, fmt.Errorf("content generation failed: %w", err)
	}
	return response, nil
}

// GenerateContentStream wraps the inner gateway's GenerateContentStream and logs the interaction.
func (g *loggingGenerationGateway) GenerateContentStream(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
	callback gateway.StreamCallback,
) (*gateway.GenerateContentResponse, error) {
	startTime := time.Now()

	response, err := g.inner.GenerateContentStream(ctx, request, callback)

	duration := time.Since(startTime)

	entry := &tracelog.APIInteractionEntry{
		Command:  g.command,
		Preset:   g.preset,
		Provider: g.provider,
		Model:    request.Model(),
		Request: tracelog.RequestData{
			SystemPrompt:    request.SystemPrompt(),
			UserPrompt:      request.UserPrompt(),
			MaxOutputTokens: request.MaxOutputTokens(),
		},
		Response: tracelog.ResponseData{
			Content: formatResponseContent(response),
		},
		DurationMs: duration.Milliseconds(),
	}

	if response != nil && response.Usage != nil {
		entry.Usage = tracelog.UsageData{
			PromptTokens:     response.Usage.PromptTokens,
			CompletionTokens: response.Usage.CompletionTokens,
			TotalTokens:      response.Usage.TotalTokens,
		}
	}

	if err != nil {
		entry.Response.Error = err.Error()
	}

	if logErr := g.logger.LogAPIInteraction(entry); logErr != nil {
		_ = logErr
	}

	if err != nil {
		return response, fmt.Errorf("content streaming failed: %w", err)
	}
	return response, nil
}

// formatResponseContent formats the full response including all content blocks.
func formatResponseContent(response *gateway.GenerateContentResponse) string {
	if response == nil || len(response.Blocks) == 0 {
		return ""
	}

	var result strings.Builder

	for i, block := range response.Blocks {
		if i > 0 {
			result.WriteString("\n---\n")
		}
		formatContentBlock(&result, block)
	}

	return result.String()
}

// formatContentBlock writes a single content block to the string builder.
func formatContentBlock(result *strings.Builder, block gateway.ContentBlock) {
	switch block.Kind {
	case gateway.ContentBlockText:
		result.WriteString(block.Text)
	case gateway.ContentBlockReasoning:
		result.WriteString("[REASONING]\n")
		result.WriteString(block.Text)
	case gateway.ContentBlockToolCall:
		formatToolCallBlock(result, block.ToolCall)
	}
}

// formatToolCallBlock writes a tool call block to the string builder.
func formatToolCallBlock(result *strings.Builder, call *gateway.ToolCall) {
	if call == nil {
		return
	}
	fmt.Fprintf(result, "[TOOL_CALL: %s]\n", call.Name)
	if len(call.Arguments) > 0 {
		if args, err := json.Marshal(call.Arguments); err == nil {
			result.WriteString(string(args))
		}
	}
}

// loggingEmbeddingsGateway wraps an EmbeddingsGateway to log all API interactions.
type loggingEmbeddingsGateway struct {
	inner    ports.EmbeddingsGateway
	logger   TraceLogger
	command  string
	preset   string
	provider string
}

// newLoggingEmbeddingsGateway creates a new logging wrapper for an embeddings gateway.
func newLoggingEmbeddingsGateway(
	inner ports.EmbeddingsGateway,
	logger TraceLogger,
	command string,
	preset string,
	provider string,
) ports.EmbeddingsGateway {
	if logger == nil {
		return inner
	}

	return &loggingEmbeddingsGateway{
		inner:    inner,
		logger:   logger,
		command:  command,
		preset:   preset,
		provider: provider,
	}
}

// ComputeEmbeddings wraps the inner gateway's ComputeEmbeddings and logs the interaction.
func (g *loggingEmbeddingsGateway) ComputeEmbeddings(
	ctx context.Context,
	request *gateway.ComputeEmbeddingsRequest,
) ([]gateway.Embedding, error) {
	startTime := time.Now()

	embeddings, err := g.inner.ComputeEmbeddings(ctx, request)

	duration := time.Since(startTime)

	// Log the interaction
	entry := &tracelog.APIInteractionEntry{
		Command:  g.command,
		Preset:   g.preset,
		Provider: g.provider,
		Model:    request.Model(),
		Request: tracelog.RequestData{
			SystemPrompt:    string(request.TaskType()), // Use TaskType as context
			UserPrompt:      formatChunks(request.Chunks()),
			MaxOutputTokens: request.Dimensions(),
		},
		Response: tracelog.ResponseData{
			Content: formatEmbeddingsResult(embeddings),
		},
		DurationMs: duration.Milliseconds(),
	}

	if err != nil {
		entry.Response.Error = err.Error()
	}

	// Log synchronously - logging errors are not critical but should be fast
	if logErr := g.logger.LogAPIInteraction(entry); logErr != nil {
		// Ignore logging errors, but they're visible in debug mode
		_ = logErr
	}

	if err != nil {
		return embeddings, fmt.Errorf("embeddings computation failed: %w", err)
	}
	return embeddings, nil
}

// ComputeDistance delegates to the inner gateway without logging (pure computation).
func (g *loggingEmbeddingsGateway) ComputeDistance(first, second gateway.Embedding) (float64, error) {
	dist, err := g.inner.ComputeDistance(first, second)
	if err != nil {
		return 0, fmt.Errorf("distance computation failed: %w", err)
	}
	return dist, nil
}

// formatChunks formats the chunks for logging (truncate if too many).
func formatChunks(chunks []string) string {
	const maxChunks = 10
	const maxChunkLen = 100

	if len(chunks) == 0 {
		return ""
	}

	result := ""
	displayCount := len(chunks)
	if displayCount > maxChunks {
		displayCount = maxChunks
	}

	for i, chunk := range chunks[:displayCount] {
		if len(chunk) > maxChunkLen {
			chunk = chunk[:maxChunkLen] + "..."
		}
		if i > 0 {
			result += "\n---\n"
		}
		result += chunk
	}

	if len(chunks) > maxChunks {
		result += fmt.Sprintf("\n... and %d more chunks", len(chunks)-maxChunks)
	}

	return result
}

// formatEmbeddingsResult formats the embeddings result for logging.
func formatEmbeddingsResult(embeddings []gateway.Embedding) string {
	if len(embeddings) == 0 {
		return "0 embeddings"
	}

	dimensions := 0
	if len(embeddings) > 0 && len(embeddings[0]) > 0 {
		dimensions = len(embeddings[0])
	}

	return fmt.Sprintf("%d embeddings with %d dimensions each", len(embeddings), dimensions)
}
