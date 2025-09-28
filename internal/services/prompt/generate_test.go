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

package prompt

import (
	"testing"

	mdProfile "github.com/retran/meowg1k/internal/models/profile"
	"github.com/retran/meowg1k/internal/services/command"
	"github.com/retran/meowg1k/internal/services/task"
)

// Mock implementations for testing

type mockTaskService struct {
	task.Service
	config *task.Configuration
}

func (m *mockTaskService) Get() *task.Configuration {
	return m.config
}

type mockCommandService struct {
	command.Service
	stdin string
}

func (m *mockCommandService) GetStdIn() string {
	return m.stdin
}

func TestNewGeneratePromptService(t *testing.T) {
	mockTask := &mockTaskService{
		config: &task.Configuration{
			Name:         "test-task",
			Profile:      &mdProfile.ResolvedProfile{},
			SystemPrompt: "Test system prompt",
			UserPrompt:   "Test user prompt",
		},
	}

	mockCommand := &mockCommandService{
		stdin: "test stdin content",
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
	mockTask := &mockTaskService{
		config: &task.Configuration{
			SystemPrompt: expectedSystemPrompt,
			UserPrompt:   "Test user prompt",
		},
	}

	mockCommand := &mockCommandService{
		stdin: "",
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
	mockTask := &mockTaskService{
		config: &task.Configuration{
			SystemPrompt: "Test system prompt",
			UserPrompt:   "Test user prompt",
		},
	}

	mockCommand := &mockCommandService{
		stdin: "",
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
	mockTask := &mockTaskService{
		config: &task.Configuration{
			SystemPrompt: "Test system prompt",
			UserPrompt:   "Test user prompt",
		},
	}

	mockCommand := &mockCommandService{
		stdin: "stdin content here",
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
	mockTask := &mockTaskService{
		config: &task.Configuration{
			SystemPrompt: "Test system prompt",
			UserPrompt:   "", // Empty user prompt
		},
	}

	mockCommand := &mockCommandService{
		stdin: "stdin only content",
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
	mockTask := &mockTaskService{
		config: &task.Configuration{
			SystemPrompt: "Test system prompt",
			UserPrompt:   "", // Empty user prompt
		},
	}

	mockCommand := &mockCommandService{
		stdin: "", // Empty stdin
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

func TestInterfaceImplementation(t *testing.T) {
	mockTask := &mockTaskService{
		config: &task.Configuration{
			SystemPrompt: "Test system prompt",
			UserPrompt:   "Test user prompt",
		},
	}

	mockCommand := &mockCommandService{
		stdin: "",
	}

	service, err := NewGeneratePromptService(mockCommand, mockTask)
	if err != nil {
		t.Fatalf("NewGeneratePromptService failed: %v", err)
	}

	// Test that service implements both interfaces
	var _ SystemPromptProvider = service
	var _ UserPromptProvider = service
}
