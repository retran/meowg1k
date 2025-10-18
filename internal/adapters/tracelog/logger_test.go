// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package tracelog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/retran/meowg1k/pkg/executor"
)

type mockWorkspaceResolver struct {
	workspaceDir string
}

func (m *mockWorkspaceResolver) Get() (string, error) {
	return m.workspaceDir, nil
}

func TestLogger_LogAPIInteraction(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	resolver := &mockWorkspaceResolver{workspaceDir: tempDir}

	logger := NewLogger(resolver)
	defer logger.Close()

	entry := &APIInteractionEntry{
		Command:  "commit",
		Profile:  "default",
		Provider: "openai",
		Model:    "gpt-4",
		Request: RequestData{
			SystemPrompt:    "Test system prompt",
			UserPrompt:      "Test user prompt",
			MaxOutputTokens: 1000,
		},
		Response: ResponseData{
			Content: "Test response",
		},
		DurationMs: 1500,
	}

	err := logger.LogAPIInteraction(entry)
	if err != nil {
		t.Fatalf("Failed to log API interaction: %v", err)
	}

	// Verify log file was created
	logsDir := filepath.Join(tempDir, logsSubDir)
	files, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatalf("Failed to read logs directory: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 log file, got %d", len(files))
	}

	// Verify log file content
	logFile := filepath.Join(logsDir, files[0].Name())
	f, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var loggedEntry APIInteractionEntry
	if scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &loggedEntry); err != nil {
			t.Fatalf("Failed to unmarshal log entry: %v", err)
		}
	} else {
		t.Fatalf("No log entries found")
	}

	if loggedEntry.LogEntryType != LogEntryTypeAPIInteraction {
		t.Errorf("Expected log_entry_type %s, got %s", LogEntryTypeAPIInteraction, loggedEntry.LogEntryType)
	}
	if loggedEntry.Command != "commit" {
		t.Errorf("Expected command 'commit', got %s", loggedEntry.Command)
	}
	if loggedEntry.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got %s", loggedEntry.Model)
	}
}

func TestLogger_LogExecutionEvent(t *testing.T) {
	tempDir := t.TempDir()
	resolver := &mockWorkspaceResolver{workspaceDir: tempDir}

	logger := NewLogger(resolver)
	defer logger.Close()

	entry := &ExecutionEventEntry{
		ExecutionName: "CommitFlow",
		Status:        "running",
		Message:       "Processing files",
		Metadata: map[string]any{
			"file_count": 5,
		},
	}

	err := logger.LogExecutionEvent(entry)
	if err != nil {
		t.Fatalf("Failed to log execution event: %v", err)
	}

	// Verify log file was created
	logsDir := filepath.Join(tempDir, logsSubDir)
	files, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatalf("Failed to read logs directory: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 log file, got %d", len(files))
	}
}

func TestLogger_LogApplicationError(t *testing.T) {
	tempDir := t.TempDir()
	resolver := &mockWorkspaceResolver{workspaceDir: tempDir}

	logger := NewLogger(resolver)
	defer logger.Close()

	entry := &ApplicationErrorEntry{
		Component: "ConfigService",
		Error:     "Failed to parse configuration",
	}

	err := logger.LogApplicationError(entry)
	if err != nil {
		t.Fatalf("Failed to log application error: %v", err)
	}

	// Verify log file was created
	logsDir := filepath.Join(tempDir, logsSubDir)
	files, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatalf("Failed to read logs directory: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 log file, got %d", len(files))
	}
}

func TestLogger_MultipleEntries(t *testing.T) {
	tempDir := t.TempDir()
	resolver := &mockWorkspaceResolver{workspaceDir: tempDir}

	logger := NewLogger(resolver)
	defer logger.Close()

	// Log multiple entries
	err := logger.LogExecutionEvent(&ExecutionEventEntry{
		ExecutionName: "CommitFlow",
		Status:        "running",
		Message:       "Starting",
	})
	if err != nil {
		t.Fatalf("Failed to log first event: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	err = logger.LogAPIInteraction(&APIInteractionEntry{
		Command:  "commit",
		Profile:  "default",
		Provider: "openai",
		Model:    "gpt-4",
		Request: RequestData{
			SystemPrompt:    "System",
			UserPrompt:      "User",
			MaxOutputTokens: 1000,
		},
		Response: ResponseData{
			Content: "Response",
		},
		DurationMs: 1000,
	})
	if err != nil {
		t.Fatalf("Failed to log API interaction: %v", err)
	}

	err = logger.LogExecutionEvent(&ExecutionEventEntry{
		ExecutionName: "CommitFlow",
		Status:        "completed",
		Message:       "Finished",
	})
	if err != nil {
		t.Fatalf("Failed to log second event: %v", err)
	}

	// Verify all entries are in the same file
	logsDir := filepath.Join(tempDir, logsSubDir)
	files, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatalf("Failed to read logs directory: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 log file, got %d", len(files))
	}

	// Count entries
	logFile := filepath.Join(logsDir, files[0].Name())
	f, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		count++
	}

	if count != 3 {
		t.Errorf("Expected 3 log entries, got %d", count)
	}
}

func TestDisabledLogger(t *testing.T) {
	logger := NewDisabledLogger()
	defer logger.Close()

	// Should not fail
	err := logger.LogAPIInteraction(&APIInteractionEntry{
		Command: "test",
	})
	if err != nil {
		t.Errorf("Disabled logger should not return error: %v", err)
	}

	err = logger.LogExecutionEvent(&ExecutionEventEntry{
		ExecutionName: "test",
	})
	if err != nil {
		t.Errorf("Disabled logger should not return error: %v", err)
	}

	err = logger.LogApplicationError(&ApplicationErrorEntry{
		Component: "test",
	})
	if err != nil {
		t.Errorf("Disabled logger should not return error: %v", err)
	}
}

func TestLogger_FeedbackHandler(t *testing.T) {
	tempDir := t.TempDir()
	resolver := &mockWorkspaceResolver{workspaceDir: tempDir}

	logger := NewLogger(resolver)
	defer logger.Close()

	// Track if inner handler was called
	innerCalled := false
	innerHandler := func(feedback *executor.Feedback) {
		innerCalled = true
	}

	// Create wrapped handler
	handler := logger.FeedbackHandler(innerHandler)

	// Test with successful feedback
	feedback := &executor.Feedback{
		ActivityName: "TestActivity",
		Status:       executor.StatusRunning,
		Message:      "Processing",
		Metadata: map[string]any{
			"progress": 50,
		},
	}

	handler(feedback)

	// Wait a bit for async logging
	time.Sleep(50 * time.Millisecond)

	if !innerCalled {
		t.Error("Inner handler was not called")
	}

	// Verify log was created
	logsDir := filepath.Join(tempDir, logsSubDir)
	files, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatalf("Failed to read logs directory: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 log file, got %d", len(files))
	}

	// Verify log content
	logFile := filepath.Join(logsDir, files[0].Name())
	f, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var loggedEntry ExecutionEventEntry
	if scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &loggedEntry); err != nil {
			t.Fatalf("Failed to unmarshal log entry: %v", err)
		}
	} else {
		t.Fatalf("No log entries found")
	}

	if loggedEntry.LogEntryType != LogEntryTypeExecutionEvent {
		t.Errorf("Expected log_entry_type %s, got %s", LogEntryTypeExecutionEvent, loggedEntry.LogEntryType)
	}
	if loggedEntry.ExecutionName != "TestActivity" {
		t.Errorf("Expected execution_name 'TestActivity', got %s", loggedEntry.ExecutionName)
	}
	if loggedEntry.Status != string(executor.StatusRunning) {
		t.Errorf("Expected status 'running', got %s", loggedEntry.Status)
	}
}

func TestLogger_FeedbackHandlerWithError(t *testing.T) {
	tempDir := t.TempDir()
	resolver := &mockWorkspaceResolver{workspaceDir: tempDir}

	logger := NewLogger(resolver)
	defer logger.Close()

	handler := logger.FeedbackHandler(nil)

	// Test with error feedback
	feedback := &executor.Feedback{
		ActivityName: "FailedActivity",
		Status:       executor.StatusFailed,
		Message:      "Operation failed",
		Error:        fmt.Errorf("test error"),
	}

	handler(feedback)

	// Wait a bit for async logging
	time.Sleep(50 * time.Millisecond)

	// Verify log was created with error
	logsDir := filepath.Join(tempDir, logsSubDir)
	logFile := filepath.Join(logsDir, func() string {
		files, _ := os.ReadDir(logsDir)
		return files[0].Name()
	}())

	f, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var loggedEntry ExecutionEventEntry
	if scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &loggedEntry); err != nil {
			t.Fatalf("Failed to unmarshal log entry: %v", err)
		}
	}

	if loggedEntry.Error != "test error" {
		t.Errorf("Expected error 'test error', got %s", loggedEntry.Error)
	}
}

func TestLogger_FeedbackHandlerDisabled(t *testing.T) {
	logger := NewDisabledLogger()
	defer logger.Close()

	innerCalled := false
	innerHandler := func(feedback *executor.Feedback) {
		innerCalled = true
	}

	handler := logger.FeedbackHandler(innerHandler)

	feedback := &executor.Feedback{
		ActivityName: "TestActivity",
		Status:       executor.StatusRunning,
	}

	handler(feedback)

	if !innerCalled {
		t.Error("Inner handler should still be called for disabled logger")
	}
}
