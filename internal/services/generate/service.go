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

package generate

import (
	"context"

	"github.com/retran/meowg1k/internal/services/gateway"
)

// Service encapsulates the generation logic.
type Service interface {
	Generate(ctx context.Context, params *Params) (string, error)
}

// service encapsulates the generation logic.
type service struct {
	gateway gateway.GenerationGateway
}

// NewService creates a new generation service with the given gateway.
func NewService(gw gateway.GenerationGateway) Service {
	return &service{
		gateway: gw,
	}
}

// Generate generates content using the configured gateway.
func (s *service) Generate(ctx context.Context, params *Params) (string, error) {
	request := gateway.NewGenerateContentRequest(params.Profile.Model, params.SystemPrompt, params.UserPrompt, params.Profile.MaxOutputTokens)

	ctx, cancel := context.WithTimeout(ctx, params.Profile.Timeout)
	defer cancel()

	content, err := s.gateway.GenerateContent(ctx, request)
	if err != nil {
		return "", err
	}

	return content, nil
}
