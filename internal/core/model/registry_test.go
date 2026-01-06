// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"reflect"
	"testing"

	model2 "github.com/retran/meowg1k/internal/domain/model"
)

func TestNewService(t *testing.T) {
	service := NewRegistry()
	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestGetModelInfo(t *testing.T) {
	service := NewRegistry()

	info := service.Get("gpt-4o")
	expected := model2.Info{
		Provider:         "unknown",
		MaxContextTokens: 0,
		MaxOutputTokens:  0,
		TokenizerType:    model2.TokenizerUnknown,
		Description:      "Unknown model",
	}
	if !reflect.DeepEqual(info, expected) {
		t.Errorf("Expected %+v for unknown model, got %+v", expected, info)
	}
}

func TestGetMaxContextTokens(t *testing.T) {
	service := NewRegistry()

	tokens := service.GetMaxContextTokens("unknown-model")
	if tokens != 0 {
		t.Errorf("Expected 0 tokens for unknown model, got %d", tokens)
	}
}

func TestGetTokenizerType(t *testing.T) {
	service := NewRegistry()

	tokenizerType := service.GetTokenizerType("unknown-model")
	if tokenizerType != model2.TokenizerUnknown {
		t.Errorf("Expected TokenizerUnknown for unknown model, got %s", tokenizerType)
	}
}

func TestGetDefaultEmbedDimension(t *testing.T) {
	service := NewRegistry()

	dimension := service.GetDefaultEmbedDimension("unknown-model")
	if dimension != 0 {
		t.Errorf("Expected 0 for unknown model, got %d", dimension)
	}
}

func TestGetProvider(t *testing.T) {
	service := NewRegistry()

	provider := service.GetProvider("unknown-model")
	if provider != "unknown" {
		t.Errorf("Expected 'unknown' for unknown model, got '%s'", provider)
	}
}

func TestGetMaxOutputTokens(t *testing.T) {
	service := NewRegistry()

	tokens := service.GetMaxOutputTokens("unknown-model")
	if tokens != 0 {
		t.Errorf("Expected 0 for unknown model, got %d", tokens)
	}
}

func TestListKnownModels(t *testing.T) {
	service := NewRegistry()

	models := service.ListKnownModels()

	if len(models) != 0 {
		t.Fatalf("Expected empty model list, got %d", len(models))
	}
}
