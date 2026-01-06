// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package model

import model2 "github.com/retran/meowg1k/internal/domain/model"

// Registry is the private implementation of the Registry interface.
type Registry struct {
	models map[string]model2.Info
}

// NewRegistry creates a model registry without built-in definitions.
func NewRegistry() *Registry {
	return &Registry{
		models: map[string]model2.Info{},
	}
}

// Get returns information about a specific model.
func (r *Registry) Get(modelName string) model2.Info {
	if r == nil || r.models == nil {
		return model2.Info{
			Provider:         "unknown",
			MaxContextTokens: 0,
			MaxOutputTokens:  0,
			TokenizerType:    model2.TokenizerUnknown,
			Description:      "Unknown model",
		}
	}

	if info, exists := r.models[modelName]; exists {
		return info
	}

	return model2.Info{
		Provider:         "unknown",
		MaxContextTokens: 0,
		MaxOutputTokens:  0,
		TokenizerType:    model2.TokenizerUnknown,
		Description:      "Unknown model",
	}
}

// GetMaxContextTokens returns the maximum context tokens for a model.
func (r *Registry) GetMaxContextTokens(modelName string) int {
	return r.Get(modelName).MaxContextTokens
}

// GetTokenizerType returns the tokenizer type for a model.
func (r *Registry) GetTokenizerType(modelName string) model2.Tokenizer {
	return r.Get(modelName).TokenizerType
}

// GetDefaultEmbedDimension returns the default embedding dimension for a model.
func (r *Registry) GetDefaultEmbedDimension(modelName string) int {
	return r.Get(modelName).DefaultEmbedDimension
}

// GetProvider returns the provider for a model.
func (r *Registry) GetProvider(modelName string) string {
	return r.Get(modelName).Provider
}

// GetMaxOutputTokens returns the maximum output tokens for a model.
func (r *Registry) GetMaxOutputTokens(modelName string) int {
	return r.Get(modelName).MaxOutputTokens
}

// ListKnownModels returns a list of all models in the registry.
func (r *Registry) ListKnownModels() []string {
	if r == nil || r.models == nil {
		return nil
	}
	models := make([]string, 0, len(r.models))
	for model := range r.models {
		models = append(models, model)
	}

	return models
}
