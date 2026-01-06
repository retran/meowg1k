// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"fmt"
	"testing"

	"github.com/retran/meowg1k/internal/domain/preset"
	"github.com/retran/meowg1k/internal/domain/task"
)

// mockStandardInputReader is a mock implementation of StandardInputReader for testing.
type mockStandardInputReader struct {
	StdIn string
}

func (m *mockStandardInputReader) GetStdIn() (string, error) {
	return m.StdIn, nil
}

// mockTaskConfigurationProvider is a mock implementation of TaskConfigurationProvider for testing.
type mockTaskConfigurationProvider struct {
	config *task.ResolvedConfig
}

func (m *mockTaskConfigurationProvider) Get() (*task.ResolvedConfig, error) {
	return m.config, nil
}

func TestNewGeneratePromptService(t *testing.T) {
	mockTask := &mockTaskConfigurationProvider{
		config: &task.ResolvedConfig{
			Name:         "test-task",
			Preset:       &preset.ResolvedPreset{},
			SystemPrompt: "Test system prompt",
			UserPrompt:   "Test user prompt",
		},
	}

	mockCommand := &mockStandardInputReader{
		StdIn: "test stdin content",
	}

	service, err := NewGeneratePromptService(mockCommand, mockTask)
	if err != nil {
		t.Fatalf("NewGeneratePromptService failed: %v", err)
	}

	if service == nil {
		t.Fatal("Service should not be nil")
	}
}

func TestGetSystemPrompt(t *testing.T) {
	expectedSystemPrompt := "Test system prompt"
	mockTask := &mockTaskConfigurationProvider{
		config: &task.ResolvedConfig{
			SystemPrompt: expectedSystemPrompt,
			UserPrompt:   "Test user prompt",
		},
	}

	mockCommand := &mockStandardInputReader{
		StdIn: "",
	}

	service, err := NewGeneratePromptService(mockCommand, mockTask)
	if err != nil {
		t.Fatalf("NewGeneratePromptService failed: %v", err)
	}

	systemPrompt, err := service.GetSystemPrompt()
	if err != nil {
		t.Fatalf("GetSystemPrompt failed: %v", err)
	}

	if systemPrompt != expectedSystemPrompt {
		t.Errorf("Expected system prompt '%s', got '%s'", expectedSystemPrompt, systemPrompt)
	}
}

func TestGetUserPrompt(t *testing.T) {
	mockTask := &mockTaskConfigurationProvider{
		config: &task.ResolvedConfig{
			SystemPrompt: "Test system prompt",
			UserPrompt:   "Test user prompt",
		},
	}

	mockCommand := &mockStandardInputReader{
		StdIn: "",
	}

	service, err := NewGeneratePromptService(mockCommand, mockTask)
	if err != nil {
		t.Fatalf("NewGeneratePromptService failed: %v", err)
	}

	userPrompt, err := service.GetUserPrompt()
	if err != nil {
		t.Fatalf("GetUserPrompt failed: %v", err)
	}

	expectedUserPrompt := "Test user prompt\n"
	if userPrompt != expectedUserPrompt {
		t.Errorf("Expected user prompt '%s', got '%s'", expectedUserPrompt, userPrompt)
	}
}

func TestBuildUserPromptWithStdin(t *testing.T) {
	mockTask := &mockTaskConfigurationProvider{
		config: &task.ResolvedConfig{
			SystemPrompt: "Test system prompt",
			UserPrompt:   "Test user prompt",
		},
	}

	mockCommand := &mockStandardInputReader{
		StdIn: "stdin content here",
	}

	service, err := NewGeneratePromptService(mockCommand, mockTask)
	if err != nil {
		t.Fatalf("NewGeneratePromptService failed: %v", err)
	}

	userPrompt, err := service.GetUserPrompt()
	if err != nil {
		t.Fatalf("GetUserPrompt failed: %v", err)
	}

	expected := "Test user prompt\n```\nstdin content here\n```\n"
	if userPrompt != expected {
		t.Errorf("Expected user prompt '%s', got '%s'", expected, userPrompt)
	}
}

func TestBuildUserPromptStdinOnly(t *testing.T) {
	mockTask := &mockTaskConfigurationProvider{
		config: &task.ResolvedConfig{
			SystemPrompt: "Test system prompt",
			UserPrompt:   "", // Empty user prompt
		},
	}

	mockCommand := &mockStandardInputReader{
		StdIn: "stdin only content",
	}

	service, err := NewGeneratePromptService(mockCommand, mockTask)
	if err != nil {
		t.Fatalf("NewGeneratePromptService failed: %v", err)
	}

	userPrompt, err := service.GetUserPrompt()
	if err != nil {
		t.Fatalf("GetUserPrompt failed: %v", err)
	}

	expected := "stdin only content\n"
	if userPrompt != expected {
		t.Errorf("Expected user prompt '%s', got '%s'", expected, userPrompt)
	}
}

func TestBuildUserPromptEmpty(t *testing.T) {
	mockTask := &mockTaskConfigurationProvider{
		config: &task.ResolvedConfig{
			SystemPrompt: "Test system prompt",
			UserPrompt:   "", // Empty user prompt
		},
	}

	mockCommand := &mockStandardInputReader{
		StdIn: "", // Empty stdin
	}

	service, err := NewGeneratePromptService(mockCommand, mockTask)
	if err != nil {
		t.Fatalf("NewGeneratePromptService failed: %v", err)
	}

	userPrompt, err := service.GetUserPrompt()
	if err != nil {
		t.Fatalf("GetUserPrompt failed: %v", err)
	}

	expected := ""
	if userPrompt != expected {
		t.Errorf("Expected empty user prompt, got '%s'", userPrompt)
	}
}

func TestNewGeneratePromptServiceWithNilCommandParametersReader(t *testing.T) {
	mockTask := &mockTaskConfigurationProvider{
		config: &task.ResolvedConfig{
			SystemPrompt: "Test",
		},
	}

	service, err := NewGeneratePromptService(nil, mockTask)
	if err == nil {
		t.Error("Expected error when command parameters reader is nil")
	}
	if service != nil {
		t.Error("Expected nil service when command parameters reader is nil")
	}
}

func TestNewGeneratePromptServiceWithNilTaskConfigProvider(t *testing.T) {
	mockCommand := &mockStandardInputReader{
		StdIn: "test",
	}

	service, err := NewGeneratePromptService(mockCommand, nil)
	if err == nil {
		t.Error("Expected error when task config provider is nil")
	}
	if service != nil {
		t.Error("Expected nil service when task config provider is nil")
	}
}

func TestNewGeneratePromptServiceWithTaskConfigError(t *testing.T) {
	mockTask := &mockTaskConfigProviderWithError{}
	mockCommand := &mockStandardInputReader{}

	service, err := NewGeneratePromptService(mockCommand, mockTask)
	if err == nil {
		t.Error("Expected error when task config provider returns error during service creation")
	}
	if service != nil {
		t.Error("Expected nil service when task config provider returns error")
	}
}

func TestNewGeneratePromptServiceWithStdinError(t *testing.T) {
	mockTask := &mockTaskConfigurationProvider{
		config: &task.ResolvedConfig{
			SystemPrompt: "Test",
			UserPrompt:   "Test",
		},
	}
	mockCommand := &mockStandardInputReaderWithError{}

	service, err := NewGeneratePromptService(mockCommand, mockTask)
	if err == nil {
		t.Error("Expected error when stdin reader returns error during service creation")
	}
	if service != nil {
		t.Error("Expected nil service when stdin reader returns error")
	}
}

type mockTaskConfigProviderWithError struct{}

func (m *mockTaskConfigProviderWithError) Get() (*task.ResolvedConfig, error) {
	return nil, fmt.Errorf("task config error")
}

type mockStandardInputReaderWithError struct{}

func (m *mockStandardInputReaderWithError) GetStdIn() (string, error) {
	return "", fmt.Errorf("stdin error")
}
