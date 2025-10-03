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

// Package gatewaymocks provides mock implementations for gateway services.
package gatewaymocks

import (
	"context"

	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/internal/services/profile"
)

// MockGenerationGateway is a mock implementation of gateway.GenerationGateway for testing.
type MockGenerationGateway struct {
	Content string
	Err     error
}

// GenerateContent implements gateway.GenerationGateway.
func (m *MockGenerationGateway) GenerateContent(ctx context.Context, request *gateway.GenerateContentRequest) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	return m.Content, nil
}

// MockGatewayFactory is a mock implementation of gateway.Factory for testing.
type MockGatewayFactory struct {
	GenerationGateway gateway.GenerationGateway
	Err               error
}

// NewGenerationGateway implements gateway.Factory.
func (m *MockGatewayFactory) NewGenerationGateway(ctx context.Context, profile *profile.ResolvedProfile) (gateway.GenerationGateway, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.GenerationGateway, nil
}

// NewEmbeddingsGateway implements gateway.Factory.
func (m *MockGatewayFactory) NewEmbeddingsGateway(ctx context.Context, profile *profile.ResolvedProfile) (gateway.EmbeddingsGateway, error) {
	return nil, nil
}
