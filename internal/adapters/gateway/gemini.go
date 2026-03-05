// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"google.golang.org/genai"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

var (
	_ ports.GenerationGateway = (*geminiGateway)(nil)
	_ ports.EmbeddingsGateway = (*geminiGateway)(nil)
)

// geminiGateway is a unified client for the Google Gemini API,
// implementing both GenerationGateway and EmbeddingGateway.
type geminiGateway struct {
	gateway.ComputeDistanceMixin
	client *genai.Client
}

// NewGeminiGateway creates and initializes a new unified GeminiGateway.
func newGeminiGateway(ctx context.Context, apiKey string) (ports.Gateway, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &geminiGateway{
		client: client,
	}, nil
}

// GenerateContent sends a content generation request to the Google Gemini API.
func (g *geminiGateway) GenerateContent(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
) (*gateway.GenerateContentResponse, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if g == nil {
		return nil, fmt.Errorf("gemini gateway is nil")
	}

	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	return RetryWithBackoff(ctx, DefaultRetryConfig(), func(ctx context.Context) (*gateway.GenerateContentResponse, error) {
		generationConfig := buildGeminiGenerationConfig(request)
		if tools := request.Tools(); len(tools) > 0 {
			generationConfig.Tools = buildGeminiTools(tools)
			generationConfig.ToolConfig = &genai.ToolConfig{
				FunctionCallingConfig: &genai.FunctionCallingConfig{Mode: genai.FunctionCallingConfigModeAuto},
			}
		}

		userPromptText := g.mapMessagesToPrompt(request)
		userPrompt := genai.Text(userPromptText)

		result, err := g.client.Models.GenerateContent(ctx, request.Model(), userPrompt, generationConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch response from Gemini API for model %q: %w", request.Model(), err)
		}

		if err := validateGeminiResponse(result, request.Model()); err != nil {
			return nil, err
		}

		blocks := parseGeminiBlocksOrdered(result)

		// Extract usage information
		tokenCount := 0
		var usage *gateway.UsageMetadata
		if result.UsageMetadata != nil {
			tokenCount = int(result.UsageMetadata.TotalTokenCount)
			usage = &gateway.UsageMetadata{
				PromptTokens:     int(result.UsageMetadata.PromptTokenCount),
				CompletionTokens: int(result.UsageMetadata.CandidatesTokenCount),
				TotalTokens:      int(result.UsageMetadata.TotalTokenCount),
			}
		}

		return &gateway.GenerateContentResponse{Blocks: blocks, TokenCount: tokenCount, Usage: usage}, nil
	}, fmt.Sprintf("Gemini GenerateContent for model %q", request.Model()))
}

func (g *geminiGateway) mapMessagesToPrompt(request *gateway.GenerateContentRequest) string {
	msgs := request.Messages()
	if len(msgs) == 0 {
		return request.UserPrompt()
	}

	var b strings.Builder
	for i := range msgs {
		g.writeGeminiMessage(&b, &msgs[i])
	}
	return strings.TrimSpace(b.String())
}

func (g *geminiGateway) writeGeminiMessage(b *strings.Builder, m *gateway.Message) {
	role := string(m.Role)
	if role == "" {
		role = "unknown"
	}
	if len(m.ToolCalls) > 0 {
		g.writeGeminiToolCalls(b, role, m.ToolCalls)
		return
	}
	if m.Role == gateway.MessageRoleTool {
		g.writeGeminiToolResult(b, m)
		return
	}
	b.WriteString(strings.ToUpper(role))
	b.WriteString(":\n")
	b.WriteString(strings.TrimSpace(m.Content))
	b.WriteString("\n\n")
}

func (g *geminiGateway) writeGeminiToolCalls(b *strings.Builder, role string, toolCalls []gateway.ToolCall) {
	b.WriteString(strings.ToUpper(role))
	b.WriteString(" TOOL_CALLS:\n")
	for _, c := range toolCalls {
		args, err := json.Marshal(c.Arguments)
		if err != nil {
			args = []byte("{}")
		}
		b.WriteString("- ")
		b.WriteString(c.Name)
		if c.ID != "" {
			b.WriteString(" (id=")
			b.WriteString(c.ID)
			b.WriteString(")")
		}
		b.WriteString(" args=")
		b.Write(args)
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func (g *geminiGateway) writeGeminiToolResult(b *strings.Builder, m *gateway.Message) {
	b.WriteString("TOOL ")
	b.WriteString(m.ToolName)
	if m.ToolCallID != "" {
		b.WriteString(" (tool_call_id=")
		b.WriteString(m.ToolCallID)
		b.WriteString(")")
	}
	b.WriteString(":\n")
	b.WriteString(strings.TrimSpace(m.Content))
	b.WriteString("\n\n")
}

func parseGeminiBlocksOrdered(resp *genai.GenerateContentResponse) []gateway.ContentBlock {
	if resp == nil || len(resp.Candidates) == 0 {
		return nil
	}

	parts := resp.Candidates[0].Content.Parts
	if len(parts) == 0 {
		return nil
	}

	blocks := make([]gateway.ContentBlock, 0, len(parts))
	for _, part := range parts {
		if part == nil {
			continue
		}
		if part.FunctionCall != nil {
			call := gateway.ToolCall{ID: part.FunctionCall.ID, Name: part.FunctionCall.Name, Arguments: part.FunctionCall.Args}
			blocks = append(blocks, gateway.ContentBlock{Kind: gateway.ContentBlockToolCall, ToolCall: &call})
			continue
		}
		if part.Text != "" {
			kind := gateway.ContentBlockText
			if part.Thought {
				kind = gateway.ContentBlockReasoning
			}
			blocks = append(blocks, gateway.ContentBlock{Kind: kind, Text: part.Text})
		}
	}
	return blocks
}

func buildGeminiGenerationConfig(request *gateway.GenerateContentRequest) *genai.GenerateContentConfig {
	config := &genai.GenerateContentConfig{}

	applyGeminiSystemPrompt(config, request)
	applyGeminiSamplingConfig(config, request)
	applyGeminiPenaltyConfig(config, request)
	applyGeminiResponseConfig(config, request)
	applyGeminiCandidateConfig(config, request)
	applyGeminiLogprobConfig(config, request)

	return config
}

func buildGeminiTools(tools []gateway.ToolDefinition) []*genai.Tool {
	if len(tools) == 0 {
		return nil
	}

	functions := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		functions = append(functions, &genai.FunctionDeclaration{
			Name:                 tool.Name,
			Description:          tool.Description,
			ParametersJsonSchema: tool.Parameters,
		})
	}

	return []*genai.Tool{
		{
			FunctionDeclarations: functions,
		},
	}
}

func applyGeminiSystemPrompt(config *genai.GenerateContentConfig, request *gateway.GenerateContentRequest) {
	if request.SystemPrompt() == "" {
		return
	}

	parts := genai.Text(request.SystemPrompt())
	if len(parts) > 0 {
		config.SystemInstruction = parts[0]
	}
}

func applyGeminiSamplingConfig(config *genai.GenerateContentConfig, request *gateway.GenerateContentRequest) {
	if temperature := request.Temperature(); temperature != nil {
		temp := float32(*temperature)
		config.Temperature = &temp
	}

	if topP := request.TopP(); topP != nil {
		p := float32(*topP)
		config.TopP = &p
	}

	if topK := request.TopK(); topK != nil {
		k := float32(*topK)
		config.TopK = &k
	}

	if maxTokens := request.MaxOutputTokens(); maxTokens > 0 {
		config.MaxOutputTokens = clampToInt32(maxTokens)
	}

	if seed := request.Seed(); seed != nil {
		config.Seed = toInt32Pointer(*seed)
	}

	if stop := request.Stop(); len(stop) > 0 {
		config.StopSequences = stop
	}
}

func applyGeminiPenaltyConfig(config *genai.GenerateContentConfig, request *gateway.GenerateContentRequest) {
	if frequencyPenalty := request.FrequencyPenalty(); frequencyPenalty != nil {
		fp := float32(*frequencyPenalty)
		config.FrequencyPenalty = &fp
	}

	if presencePenalty := request.PresencePenalty(); presencePenalty != nil {
		pp := float32(*presencePenalty)
		config.PresencePenalty = &pp
	}
}

func applyGeminiResponseConfig(config *genai.GenerateContentConfig, request *gateway.GenerateContentRequest) {
	if responseSchema := request.ResponseSchema(); responseSchema != nil {
		// Convert map[string]interface{} to *genai.Schema
		schema := convertToGeminiSchema(responseSchema)
		config.ResponseMIMEType = "application/json"
		config.ResponseSchema = schema
		return
	}

	if responseFormat := request.ResponseFormat(); responseFormat != nil {
		switch *responseFormat {
		case "json_object", "json", "json_schema":
			config.ResponseMIMEType = "application/json"
		case "text":
			config.ResponseMIMEType = "text/plain"
		}
	}
}

// convertToGeminiSchema converts a map[string]interface{} JSON schema to a *genai.Schema.
func convertToGeminiSchema(schemaMap map[string]interface{}) *genai.Schema {
	if schemaMap == nil {
		return nil
	}

	schema := &genai.Schema{}

	if t, ok := schemaMap["type"].(string); ok {
		schema.Type = genai.Type(t)
	}

	if desc, ok := schemaMap["description"].(string); ok {
		schema.Description = desc
	}

	if title, ok := schemaMap["title"].(string); ok {
		schema.Title = title
	}

	if enum, ok := schemaMap["enum"].([]interface{}); ok {
		enumStrings := make([]string, 0, len(enum))
		for _, v := range enum {
			if s, ok := v.(string); ok {
				enumStrings = append(enumStrings, s)
			}
		}
		schema.Enum = enumStrings
	}

	if format, ok := schemaMap["format"].(string); ok {
		schema.Format = format
	}

	if required, ok := schemaMap["required"].([]interface{}); ok {
		reqStrings := make([]string, 0, len(required))
		for _, v := range required {
			if s, ok := v.(string); ok {
				reqStrings = append(reqStrings, s)
			}
		}
		schema.Required = reqStrings
	}

	if properties, ok := schemaMap["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]*genai.Schema)
		for key, value := range properties {
			if propMap, ok := value.(map[string]interface{}); ok {
				schema.Properties[key] = convertToGeminiSchema(propMap)
			}
		}
	}

	if items, ok := schemaMap["items"].(map[string]interface{}); ok {
		schema.Items = convertToGeminiSchema(items)
	}

	if nullable, ok := schemaMap["nullable"].(bool); ok {
		schema.Nullable = &nullable
	}

	if minLength, ok := schemaMap["minLength"].(float64); ok {
		minLengthInt := int64(minLength)
		schema.MinLength = &minLengthInt
	}

	if maxLength, ok := schemaMap["maxLength"].(float64); ok {
		maxLengthInt := int64(maxLength)
		schema.MaxLength = &maxLengthInt
	}

	if minimum, ok := schemaMap["minimum"].(float64); ok {
		schema.Minimum = &minimum
	}

	if maximum, ok := schemaMap["maximum"].(float64); ok {
		schema.Maximum = &maximum
	}

	if minItems, ok := schemaMap["minItems"].(float64); ok {
		minItemsInt := int64(minItems)
		schema.MinItems = &minItemsInt
	}

	if maxItems, ok := schemaMap["maxItems"].(float64); ok {
		maxItemsInt := int64(maxItems)
		schema.MaxItems = &maxItemsInt
	}

	if pattern, ok := schemaMap["pattern"].(string); ok {
		schema.Pattern = pattern
	}

	if defaultVal, ok := schemaMap["default"]; ok {
		schema.Default = defaultVal
	}

	if example, ok := schemaMap["example"]; ok {
		schema.Example = example
	}

	if anyOf, ok := schemaMap["anyOf"].([]interface{}); ok {
		anyOfSchemas := make([]*genai.Schema, 0, len(anyOf))
		for _, v := range anyOf {
			if subSchema, ok := v.(map[string]interface{}); ok {
				anyOfSchemas = append(anyOfSchemas, convertToGeminiSchema(subSchema))
			}
		}
		schema.AnyOf = anyOfSchemas
	}

	return schema
}

func applyGeminiCandidateConfig(config *genai.GenerateContentConfig, request *gateway.GenerateContentRequest) {
	if candidateCount := request.CandidateCount(); candidateCount != nil {
		count := *candidateCount
		if count < 0 {
			count = 0
		}
		config.CandidateCount = clampToInt32(count)
	}
}

func applyGeminiLogprobConfig(config *genai.GenerateContentConfig, request *gateway.GenerateContentRequest) {
	if logProbs := request.LogProbs(); logProbs != nil {
		config.ResponseLogprobs = *logProbs
	}

	if topLogProbs := request.TopLogProbs(); topLogProbs != nil {
		count := *topLogProbs
		if count < 0 {
			count = 0
		}
		config.Logprobs = toInt32Pointer(count)
	}
}

func clampToInt32(value int) int32 {
	if value > math.MaxInt32 {
		return math.MaxInt32
	}
	if value < math.MinInt32 {
		return math.MinInt32
	}
	return int32(value)
}

func toInt32Pointer(value int) *int32 {
	clamped := clampToInt32(value)
	return &clamped
}

func validateGeminiResponse(result *genai.GenerateContentResponse, model string) error {
	if len(result.Candidates) > 0 && result.Candidates[0].FinishReason != genai.FinishReasonStop &&
		result.Candidates[0].FinishReason != genai.FinishReasonMaxTokens {
		return fmt.Errorf("generation stopped for model %q with reason: %s", model, result.Candidates[0].FinishReason)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		if result.PromptFeedback != nil && result.PromptFeedback.BlockReason != genai.BlockedReasonUnspecified {
			return fmt.Errorf("request was blocked by Gemini API for model %q with reason: %s", model, result.PromptFeedback.BlockReason)
		}

		return fmt.Errorf("gemini API returned an empty response for model %q", model)
	}

	return nil
}

// GenerateContentStream implements native streaming for Gemini using the SDK's
// GenerateContentStream iterator. Each chunk is delivered to callback as a
// StreamEventText event so the caller sees tokens as they arrive. A final
// StreamEventDone event is fired after all chunks have been processed.
func (g *geminiGateway) GenerateContentStream(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
	callback gateway.StreamCallback,
) (*gateway.GenerateContentResponse, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if g == nil {
		return nil, fmt.Errorf("gemini gateway is nil")
	}
	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	generationConfig := buildGeminiGenerationConfig(request)
	if tools := request.Tools(); len(tools) > 0 {
		generationConfig.Tools = buildGeminiTools(tools)
		generationConfig.ToolConfig = &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{Mode: genai.FunctionCallingConfigModeAuto},
		}
	}

	userPromptText := g.mapMessagesToPrompt(request)
	userPrompt := genai.Text(userPromptText)

	var (
		allBlocks  []gateway.ContentBlock
		lastUsage  *gateway.UsageMetadata
		totalCount int
		streamErr  error
	)

	for chunk, err := range g.client.Models.GenerateContentStream(ctx, request.Model(), userPrompt, generationConfig) {
		if err != nil {
			streamErr = fmt.Errorf("gemini stream error for model %q: %w", request.Model(), err)
			if callback != nil {
				_ = callback(gateway.StreamEvent{Kind: gateway.StreamEventError, Error: streamErr.Error(), Recoverable: false})
			}
			return nil, streamErr
		}

		if chunk == nil {
			continue
		}

		// Accumulate usage from each chunk (last non-nil wins).
		if chunk.UsageMetadata != nil {
			totalCount = int(chunk.UsageMetadata.TotalTokenCount)
			lastUsage = &gateway.UsageMetadata{
				PromptTokens:     int(chunk.UsageMetadata.PromptTokenCount),
				CompletionTokens: int(chunk.UsageMetadata.CandidatesTokenCount),
				TotalTokens:      int(chunk.UsageMetadata.TotalTokenCount),
			}
		}

		blocks := parseGeminiBlocksOrdered(chunk)
		allBlocks = append(allBlocks, blocks...)

		if callback != nil {
			for _, block := range blocks {
				switch block.Kind {
				case gateway.ContentBlockText:
					if block.Text != "" {
						if cbErr := callback(gateway.StreamEvent{Kind: gateway.StreamEventText, Delta: block.Text}); cbErr != nil {
							return nil, cbErr
						}
					}
				case gateway.ContentBlockReasoning:
					if block.Text != "" {
						if cbErr := callback(gateway.StreamEvent{Kind: gateway.StreamEventThinking, Delta: block.Text}); cbErr != nil {
							return nil, cbErr
						}
					}
				}
			}
		}
	}

	if callback != nil {
		_ = callback(gateway.StreamEvent{Kind: gateway.StreamEventDone, Usage: lastUsage})
	}

	return &gateway.GenerateContentResponse{Blocks: allBlocks, TokenCount: totalCount, Usage: lastUsage}, nil
}

// CountTokens counts tokens for the given content chunks using the Gemini API.
func (g *geminiGateway) CountTokens(
	ctx context.Context,
	model string,
	chunks []string,
) (int, error) {
	if ctx == nil {
		return 0, fmt.Errorf("context cannot be nil")
	}

	if g == nil {
		return 0, fmt.Errorf("gemini gateway is nil")
	}

	if len(chunks) == 0 {
		return 0, nil
	}

	contents := make([]*genai.Content, 0, len(chunks))
	for _, chunk := range chunks {
		contents = append(contents, genai.NewContentFromText(chunk, genai.RoleUser))
	}

	response, err := g.client.Models.CountTokens(ctx, model, contents, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to count tokens for model %q: %w", model, err)
	}

	return int(response.TotalTokens), nil
}

// ComputeEmbeddings sends a request to the Google Gemini API to compute embeddings for the given text chunks.
func (g *geminiGateway) ComputeEmbeddings(
	ctx context.Context,
	request *gateway.ComputeEmbeddingsRequest,
) ([]gateway.Embedding, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if g == nil {
		return nil, fmt.Errorf("gemini gateway is nil")
	}

	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	return RetryWithBackoff(ctx, DefaultRetryConfig(), func(ctx context.Context) ([]gateway.Embedding, error) {
		contents := make([]*genai.Content, 0, len(request.Chunks()))
		for _, value := range request.Chunks() {
			contents = append(contents, genai.NewContentFromText(value, genai.RoleUser))
		}

		config := &genai.EmbedContentConfig{
			TaskType: string(request.TaskType()),
		}

		if request.Dimensions() > 0 {
			dimensions := request.Dimensions()
			if dimensions > math.MaxInt32 {
				return nil, fmt.Errorf("dimensions value %d exceeds int32 range for model %q", dimensions, request.Model())
			}

			dims := int32(dimensions) // #nosec G115 // overflow checked above
			config.OutputDimensionality = &dims
		}

		response, err := g.client.Models.EmbedContent(ctx,
			request.Model(),
			contents,
			config,
		)
		if err != nil {
			return nil, err
		}

		embeddings := make([]gateway.Embedding, 0, len(response.Embeddings))
		for _, value := range response.Embeddings {
			values := make([]float64, len(value.Values))
			for i, v := range value.Values {
				values[i] = float64(v)
			}
			embeddings = append(embeddings, values)
		}

		return embeddings, nil
	}, fmt.Sprintf("Gemini ComputeEmbeddings for model %q", request.Model()))
}
