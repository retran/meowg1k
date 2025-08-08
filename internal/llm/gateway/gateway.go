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

// Package gateway provides an interface for content generation with Large Language Models (LLMs).
package gateway

import (
	"context"
)

type Provider string

const (
	Llama Provider = "llama"
	Gemini Provider = "gemini"
)

// GenerationGateway defines the interface for content generation with a Large Language Model (LLM).
type GenerationGateway interface {
	// GenerateContent sends a request to the LLM and returns the generated content.
	GenerateContent(ctx context.Context, request *GenerateContentRequest) (string, error)
}

// GenerateContentRequest holds the parameters for a content generation request.
type GenerateContentRequest struct {
	model        string
	systemPrompt string
	userPrompt   string
}

// NewGenerateContentRequest creates a new GenerateContentRequest.
func NewGenerateContentRequest(model, systemPrompt, userPrompt string) *GenerateContentRequest {
	return &GenerateContentRequest{
		model:        model,
		systemPrompt: systemPrompt,
		userPrompt:   userPrompt,
	}
}

// Model returns the model name for the content generation request.
func (r *GenerateContentRequest) Model() string {
	return r.model
}

// SystemPrompt returns the system prompt for the content generation request.
func (r *GenerateContentRequest) SystemPrompt() string {
	return r.systemPrompt
}

// UserPrompt returns the user prompt for the content generation request.
func (r *GenerateContentRequest) UserPrompt() string {
	return r.userPrompt
}

