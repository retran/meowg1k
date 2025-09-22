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
	"testing"

	"github.com/retran/meowg1k/internal/flows"
	"github.com/retran/meowg1k/internal/services/config/loader"
	"github.com/retran/meowg1k/internal/services/config/resolver"
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/internal/services/prompt"
)

// Tests for FlowFactory

func TestNewFlowFactory_Creation(t *testing.T) {
	loaderService := &mockConfigLoaderService{}
	resolverService := &mockProfileResolver{}
	promptBuilder := &mockPromptBuilder{}
	gatewayFactory := &mockGatewayFactory{}

	factory := NewFlowFactory(loaderService, resolverService, promptBuilder, gatewayFactory)

	if factory == nil {
		t.Fatal("Expected factory to be created, got nil")
	}

	if factory.loaderService != loaderService {
		t.Error("LoaderService not set correctly")
	}
	if factory.resolverService != resolverService {
		t.Error("ResolverService not set correctly")
	}
	if factory.promptBuilder != promptBuilder {
		t.Error("PromptBuilder not set correctly")
	}
	if factory.gatewayFactory != gatewayFactory {
		t.Error("GatewayFactory not set correctly")
	}
}

func TestNewFlowFactory_NilDependencies(t *testing.T) {
	tests := []struct {
		name            string
		loaderService   loader.Service
		resolverService resolver.Service
		promptBuilder   prompt.Builder
		gatewayFactory  gateway.GatewayFactory
		expectPanic     bool
	}{
		{
			name:            "nil loaderService",
			loaderService:   nil,
			resolverService: &mockProfileResolver{},
			promptBuilder:   &mockPromptBuilder{},
			gatewayFactory:  &mockGatewayFactory{},
			expectPanic:     true,
		},
		{
			name:            "nil resolverService",
			loaderService:   &mockConfigLoaderService{},
			resolverService: nil,
			promptBuilder:   &mockPromptBuilder{},
			gatewayFactory:  &mockGatewayFactory{},
			expectPanic:     true,
		},
		{
			name:            "nil promptBuilder",
			loaderService:   &mockConfigLoaderService{},
			resolverService: &mockProfileResolver{},
			promptBuilder:   nil,
			gatewayFactory:  &mockGatewayFactory{},
			expectPanic:     true,
		},
		{
			name:            "nil gatewayFactory",
			loaderService:   &mockConfigLoaderService{},
			resolverService: &mockProfileResolver{},
			promptBuilder:   &mockPromptBuilder{},
			gatewayFactory:  nil,
			expectPanic:     true,
		},
		{
			name:            "all dependencies valid",
			loaderService:   &mockConfigLoaderService{},
			resolverService: &mockProfileResolver{},
			promptBuilder:   &mockPromptBuilder{},
			gatewayFactory:  &mockGatewayFactory{},
			expectPanic:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.expectPanic && r == nil {
					t.Error("Expected panic but none occurred")
				}
				if !tt.expectPanic && r != nil {
					t.Errorf("Unexpected panic: %v", r)
				}
			}()

			NewFlowFactory(tt.loaderService, tt.resolverService, tt.promptBuilder, tt.gatewayFactory)
		})
	}
}

func TestFlowFactory_CreateFlow(t *testing.T) {
	loaderService := &mockConfigLoaderService{}
	resolverService := &mockProfileResolver{}
	promptBuilder := &mockPromptBuilder{}
	gatewayFactory := &mockGatewayFactory{}

	factory := NewFlowFactory(loaderService, resolverService, promptBuilder, gatewayFactory)

	// Test without feedback handler
	flow := factory.CreateFlow(nil)

	if flow == nil {
		t.Fatal("Expected flow to be created, got nil")
	}

	// Test with feedback handler
	feedbackHandler := func(feedback flows.Feedback) {
		// Mock feedback handler
	}

	flowWithFeedback := factory.CreateFlow(feedbackHandler)

	if flowWithFeedback == nil {
		t.Fatal("Expected flow with feedback handler to be created, got nil")
	}
}

func TestFlowFactory_CreateFlow_Integration(t *testing.T) {
	// Create real services for integration test
	loaderService := &integrationConfigLoaderService{config: createTestConfig()}
	resolverService := &integrationResolverService{}
	promptBuilder := prompt.NewBuilder()
	gatewayFactory := &mockGatewayFactory{
		gateway: &mockGenerationGateway{
			content: "test generated content",
		},
	}

	factory := NewFlowFactory(loaderService, resolverService, promptBuilder, gatewayFactory)
	flow := factory.CreateFlow(nil)

	if flow == nil {
		t.Fatal("Expected flow to be created, got nil")
	}

	// Test that the flow can be executed (basic structural test)
	// More detailed execution testing is done in other test files
}

func TestFlowFactory_UsageExample(t *testing.T) {
	// This test demonstrates how to use the factory pattern in practice

	// Step 1: Create all your service dependencies
	loaderService := &mockConfigLoaderService{config: createTestConfig()}
	resolverService := &mockProfileResolver{
		profile: createTestProfile(),
		prompt:  "test user prompt",
	}
	promptBuilder := &mockPromptBuilder{}
	gatewayFactory := &mockGatewayFactory{
		gateway: &mockGenerationGateway{
			content: "generated test content",
		},
	}

	// Step 2: Create the factory with all dependencies
	factory := NewFlowFactory(loaderService, resolverService, promptBuilder, gatewayFactory)

	// Step 3: Use the factory to create flows as needed
	flow1 := factory.CreateFlow(nil)
	flow2 := factory.CreateFlow(func(feedback flows.Feedback) {
		// Custom feedback handler
	})

	// Both flows share the same dependencies but can have different configurations
	if flow1 == nil || flow2 == nil {
		t.Fatal("Expected both flows to be created")
	}

	// The factory ensures consistent workflow creation with pre-configured dependencies
	// This makes testing easier and reduces the chance of misconfiguration
}
