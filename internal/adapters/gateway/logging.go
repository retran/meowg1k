/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gateway

import (
	"context"
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
	inner      ports.GenerationGateway
	logger     TraceLogger
	command    string
	profile    string
	provider   string
}

// newLoggingGenerationGateway creates a new logging wrapper for a generation gateway.
func newLoggingGenerationGateway(
	inner ports.GenerationGateway,
	logger TraceLogger,
	command string,
	profile string,
	provider string,
) ports.GenerationGateway {
	if logger == nil {
		return inner
	}

	return &loggingGenerationGateway{
		inner:    inner,
		logger:   logger,
		command:  command,
		profile:  profile,
		provider: provider,
	}
}

// GenerateContent wraps the inner gateway's GenerateContent and logs the interaction.
func (g *loggingGenerationGateway) GenerateContent(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
) (string, error) {
	startTime := time.Now()

	content, err := g.inner.GenerateContent(ctx, request)

	duration := time.Since(startTime)

	// Log the interaction
	entry := &tracelog.APIInteractionEntry{
		Command:  g.command,
		Profile:  g.profile,
		Provider: g.provider,
		Model:    request.Model(),
		Request: tracelog.RequestData{
			SystemPrompt:    request.SystemPrompt(),
			UserPrompt:      request.UserPrompt(),
			MaxOutputTokens: request.MaxOutputTokens(),
		},
		Response: tracelog.ResponseData{
			Content: content,
		},
		DurationMs: duration.Milliseconds(),
	}

	if err != nil {
		entry.Response.Error = err.Error()
	}

	// Log asynchronously to avoid blocking (ignore errors)
	go g.logger.LogAPIInteraction(entry)

	return content, err
}
