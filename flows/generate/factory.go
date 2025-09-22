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
	"github.com/retran/meowg1k/internal/flows"
	"github.com/retran/meowg1k/internal/services/config/loader"
	"github.com/retran/meowg1k/internal/services/config/resolver"
	"github.com/retran/meowg1k/internal/services/gateway"
	"github.com/retran/meowg1k/internal/services/prompt"
)

// FlowFactory creates generate workflows with pre-configured dependencies.
type FlowFactory struct {
	loaderService   loader.Service
	resolverService resolver.Service
	promptBuilder   prompt.Builder
	gatewayFactory  gateway.GatewayFactory
}

// NewFlowFactory creates a new FlowFactory with all required dependencies.
func NewFlowFactory(
	loaderService loader.Service,
	resolverService resolver.Service,
	promptBuilder prompt.Builder,
	gatewayFactory gateway.GatewayFactory,
) *FlowFactory {
	if loaderService == nil {
		panic("loaderService cannot be nil")
	}
	if resolverService == nil {
		panic("resolverService cannot be nil")
	}
	if promptBuilder == nil {
		panic("promptBuilder cannot be nil")
	}
	if gatewayFactory == nil {
		panic("gatewayFactory cannot be nil")
	}

	return &FlowFactory{
		loaderService:   loaderService,
		resolverService: resolverService,
		promptBuilder:   promptBuilder,
		gatewayFactory:  gatewayFactory,
	}
}

// CreateFlow creates a new generate workflow using the pre-configured dependencies.
func (f *FlowFactory) CreateFlow(feedbackHandler flows.FeedbackHandler) *flows.Flow {
	flow := flows.NewFlow()

	// Create task executors using injected dependencies
	resolveParamsExecutor := NewResolveParamsExecutor(
		f.loaderService,
		f.resolverService,
		f.promptBuilder,
	)
	createGatewayExecutor := NewCreateGatewayExecutor(f.gatewayFactory)
	generateContentExecutor := &GenerateContentExecutor{}

	// Build the workflow
	flows.AddTask(flow, "resolve-params", resolveParamsExecutor).
		LinkToID("create-gateway")

	flows.AddTask(flow, "create-gateway", createGatewayExecutor).
		LinkToID("generate-content")

	flows.AddTask(flow, "generate-content", generateContentExecutor)

	// Configure feedback handler if provided
	if feedbackHandler != nil {
		flow.WithFeedbackHandler(feedbackHandler)
	}

	return flow.SetStart("resolve-params")
}
