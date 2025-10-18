// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package tracelog provides structured logging for tracing LLM API interactions and activity execution.
package tracelog

import "time"

// LogEntryType represents the type of log entry.
type LogEntryType string

const (
	// LogEntryTypeAPIInteraction represents an LLM API interaction.
	LogEntryTypeAPIInteraction LogEntryType = "api_interaction"
	// LogEntryTypeExecutionEvent represents an execution event from the executor framework.
	LogEntryTypeExecutionEvent LogEntryType = "execution_event"
	// LogEntryTypeApplicationError represents a critical application error.
	LogEntryTypeApplicationError LogEntryType = "application_error"
)

// BaseLogEntry contains fields common to all log entries.
type BaseLogEntry struct {
	LogEntryType LogEntryType `json:"log_entry_type"`
	Timestamp    time.Time    `json:"timestamp"`
}

// APIInteractionEntry logs an LLM API interaction.
type APIInteractionEntry struct {
	BaseLogEntry
	Command    string       `json:"command"`
	Profile    string       `json:"profile"`
	Provider   string       `json:"provider"`
	Model      string       `json:"model"`
	Request    RequestData  `json:"request"`
	Response   ResponseData `json:"response"`
	Usage      UsageData    `json:"usage,omitempty"`
	DurationMs int64        `json:"duration_ms"`
}

// RequestData contains details about the API request.
type RequestData struct {
	SystemPrompt    string `json:"system_prompt"`
	UserPrompt      string `json:"user_prompt"`
	MaxOutputTokens int    `json:"max_output_tokens"`
}

// ResponseData contains details about the API response.
type ResponseData struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

// UsageData contains token usage information.
type UsageData struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

// ExecutionEventEntry logs an executor framework event.
type ExecutionEventEntry struct {
	BaseLogEntry
	ExecutionName string         `json:"execution_name"`
	Status        string         `json:"status"`
	Message       string         `json:"message,omitempty"`
	Error         string         `json:"error,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// ApplicationErrorEntry logs a critical application error.
type ApplicationErrorEntry struct {
	BaseLogEntry
	Component  string `json:"component"`
	Error      string `json:"error"`
	StackTrace string `json:"stack_trace,omitempty"`
}
