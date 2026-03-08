// Copyright © 2025 The meowg1k Authors.
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

// geminiResponseFormatText is the MIME type used when the response format is plain text.
const geminiResponseFormatText = "text"

// geminiMIMETypeJSON is the MIME type for JSON-structured responses.
const geminiMIMETypeJSON = "application/json"

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
		config.ResponseMIMEType = geminiMIMETypeJSON
		config.ResponseSchema = schema
		return
	}

	if responseFormat := request.ResponseFormat(); responseFormat != nil {
		switch *responseFormat {
		case "json_object", "json", "json_schema":
			config.ResponseMIMEType = geminiMIMETypeJSON
		case geminiResponseFormatText:
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
	applyGeminiSchemaStringFields(schema, schemaMap)
	applyGeminiSchemaListFields(schema, schemaMap)
	applyGeminiSchemaNumericFields(schema, schemaMap)
	applyGeminiSchemaNestedFields(schema, schemaMap)
	return schema
}

// applyGeminiSchemaStringFields sets simple string-typed schema fields.
func applyGeminiSchemaStringFields(schema *genai.Schema, m map[string]interface{}) {
	if t, ok := m["type"].(string); ok {
		schema.Type = genai.Type(t)
	}
	if desc, ok := m["description"].(string); ok {
		schema.Description = desc
	}
	if title, ok := m["title"].(string); ok {
		schema.Title = title
	}
	if format, ok := m["format"].(string); ok {
		schema.Format = format
	}
	if pattern, ok := m["pattern"].(string); ok {
		schema.Pattern = pattern
	}
	if nullable, ok := m["nullable"].(bool); ok {
		schema.Nullable = &nullable
	}
}

// applyGeminiSchemaListFields sets list-typed schema fields (enum, required).
func applyGeminiSchemaListFields(schema *genai.Schema, m map[string]interface{}) {
	if enum, ok := m["enum"].([]interface{}); ok {
		enumStrings := make([]string, 0, len(enum))
		for _, v := range enum {
			if s, ok := v.(string); ok {
				enumStrings = append(enumStrings, s)
			}
		}
		schema.Enum = enumStrings
	}
	if required, ok := m["required"].([]interface{}); ok {
		reqStrings := make([]string, 0, len(required))
		for _, v := range required {
			if s, ok := v.(string); ok {
				reqStrings = append(reqStrings, s)
			}
		}
		schema.Required = reqStrings
	}
}

// applyGeminiSchemaNumericFields sets numeric-typed schema constraint fields.
func applyGeminiSchemaNumericFields(schema *genai.Schema, m map[string]interface{}) {
	if minLength, ok := m["minLength"].(float64); ok {
		v := int64(minLength)
		schema.MinLength = &v
	}
	if maxLength, ok := m["maxLength"].(float64); ok {
		v := int64(maxLength)
		schema.MaxLength = &v
	}
	if minimum, ok := m["minimum"].(float64); ok {
		schema.Minimum = &minimum
	}
	if maximum, ok := m["maximum"].(float64); ok {
		schema.Maximum = &maximum
	}
	if minItems, ok := m["minItems"].(float64); ok {
		v := int64(minItems)
		schema.MinItems = &v
	}
	if maxItems, ok := m["maxItems"].(float64); ok {
		v := int64(maxItems)
		schema.MaxItems = &v
	}
}

// applyGeminiSchemaNestedFields sets nested and complex schema fields.
func applyGeminiSchemaNestedFields(schema *genai.Schema, m map[string]interface{}) {
	if properties, ok := m["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]*genai.Schema)
		for key, value := range properties {
			if propMap, ok := value.(map[string]interface{}); ok {
				schema.Properties[key] = convertToGeminiSchema(propMap)
			}
		}
	}
	if items, ok := m["items"].(map[string]interface{}); ok {
		schema.Items = convertToGeminiSchema(items)
	}
	if anyOf, ok := m["anyOf"].([]interface{}); ok {
		anyOfSchemas := make([]*genai.Schema, 0, len(anyOf))
		for _, v := range anyOf {
			if subSchema, ok := v.(map[string]interface{}); ok {
				anyOfSchemas = append(anyOfSchemas, convertToGeminiSchema(subSchema))
			}
		}
		schema.AnyOf = anyOfSchemas
	}
	if defaultVal, ok := m["default"]; ok {
		schema.Default = defaultVal
	}
	if example, ok := m["example"]; ok {
		schema.Example = example
	}
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

	userPrompt := genai.Text(g.mapMessagesToPrompt(request))
	return g.runGeminiStream(ctx, request.Model(), userPrompt, generationConfig, callback)
}

// runGeminiStream iterates over a Gemini stream and assembles the final response.
func (g *geminiGateway) runGeminiStream(
	ctx context.Context,
	model string,
	userPrompt []*genai.Content,
	config *genai.GenerateContentConfig,
	callback gateway.StreamCallback,
) (*gateway.GenerateContentResponse, error) {
	var (
		allBlocks  []gateway.ContentBlock
		lastUsage  *gateway.UsageMetadata
		totalCount int
	)

	for chunk, err := range g.client.Models.GenerateContentStream(ctx, model, userPrompt, config) {
		if err != nil {
			streamErr := fmt.Errorf("gemini stream error for model %q: %w", model, err)
			if cbErr := g.fireErrorCallback(callback, streamErr); cbErr != nil {
				return nil, cbErr
			}
			return nil, streamErr
		}

		if chunk == nil {
			continue
		}

		blocks, usage, count := extractGeminiChunk(chunk)
		if usage != nil {
			lastUsage = usage
			totalCount = count
		}

		allBlocks = append(allBlocks, blocks...)

		if cbErr := fireBlockCallbacks(blocks, callback); cbErr != nil {
			return nil, cbErr
		}
	}

	if callback != nil {
		if cbErr := callback(gateway.StreamEvent{Kind: gateway.StreamEventDone, Usage: lastUsage}); cbErr != nil {
			return nil, cbErr
		}
	}

	return &gateway.GenerateContentResponse{Blocks: allBlocks, TokenCount: totalCount, Usage: lastUsage}, nil
}

// extractGeminiChunk pulls blocks, usage metadata, and total token count from a stream chunk.
func extractGeminiChunk(chunk *genai.GenerateContentResponse) ([]gateway.ContentBlock, *gateway.UsageMetadata, int) {
	var usage *gateway.UsageMetadata
	var totalCount int

	if chunk.UsageMetadata != nil {
		totalCount = int(chunk.UsageMetadata.TotalTokenCount)
		usage = &gateway.UsageMetadata{
			PromptTokens:     int(chunk.UsageMetadata.PromptTokenCount),
			CompletionTokens: int(chunk.UsageMetadata.CandidatesTokenCount),
			TotalTokens:      int(chunk.UsageMetadata.TotalTokenCount),
		}
	}

	return parseGeminiBlocksOrdered(chunk), usage, totalCount
}

// fireErrorCallback sends a StreamEventError to the callback if it is non-nil.
func (g *geminiGateway) fireErrorCallback(callback gateway.StreamCallback, streamErr error) error {
	if callback == nil {
		return nil
	}
	return callback(gateway.StreamEvent{Kind: gateway.StreamEventError, Error: streamErr.Error(), Recoverable: false})
}

// fireBlockCallbacks sends stream events for each content block to the callback.
// Text and reasoning blocks are sent as delta events; tool call blocks are skipped.
func fireBlockCallbacks(blocks []gateway.ContentBlock, callback gateway.StreamCallback) error {
	if callback == nil {
		return nil
	}
	for _, block := range blocks {
		if cbErr := fireBlockCallback(block, callback); cbErr != nil {
			return cbErr
		}
	}
	return nil
}

// fireBlockCallback sends a stream event for a single content block.
func fireBlockCallback(block gateway.ContentBlock, callback gateway.StreamCallback) error {
	switch block.Kind {
	case gateway.ContentBlockText:
		if block.Text != "" {
			return callback(gateway.StreamEvent{Kind: gateway.StreamEventText, Delta: block.Text})
		}
	case gateway.ContentBlockReasoning:
		if block.Text != "" {
			return callback(gateway.StreamEvent{Kind: gateway.StreamEventThinking, Delta: block.Text})
		}
	case gateway.ContentBlockToolCall:
		// Tool call blocks are not streamed as incremental events;
		// they are available in the final response.
	}
	return nil
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
		return g.computeEmbeddingsOnce(ctx, request)
	}, fmt.Sprintf("Gemini ComputeEmbeddings for model %q", request.Model()))
}

// computeEmbeddingsOnce performs a single (non-retried) embedding request to the Gemini API.
func (g *geminiGateway) computeEmbeddingsOnce(
	ctx context.Context,
	request *gateway.ComputeEmbeddingsRequest,
) ([]gateway.Embedding, error) {
	contents := make([]*genai.Content, 0, len(request.Chunks()))
	for _, value := range request.Chunks() {
		contents = append(contents, genai.NewContentFromText(value, genai.RoleUser))
	}

	config, err := buildGeminiEmbedConfig(request)
	if err != nil {
		return nil, err
	}

	response, err := g.client.Models.EmbedContent(ctx, request.Model(), contents, config)
	if err != nil {
		return nil, fmt.Errorf("failed to compute embeddings for model %q: %w", request.Model(), err)
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
}

// buildGeminiEmbedConfig constructs the embedding config from a ComputeEmbeddingsRequest.
// Returns an error if the dimensions value exceeds int32 range.
func buildGeminiEmbedConfig(request *gateway.ComputeEmbeddingsRequest) (*genai.EmbedContentConfig, error) {
	config := &genai.EmbedContentConfig{
		TaskType: string(request.TaskType()),
	}

	if dimensions := request.Dimensions(); dimensions > 0 {
		if dimensions > math.MaxInt32 {
			return nil, fmt.Errorf("dimensions value %d exceeds int32 range for model %q", dimensions, request.Model())
		}
		dims := int32(dimensions) // #nosec G115 // overflow checked above
		config.OutputDimensionality = &dims
	}

	return config, nil
}
