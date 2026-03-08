// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

// Package session provides domain types for session management.
package session

import "time"

// Session represents a tool execution session with chat history.
type Session struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	ParentID  *string
	ID        string
	ToolName  string
	Status    Status
}

// Status represents the current state of a session.
type Status string

const (
	// SessionStatusRunning indicates the session is currently executing.
	SessionStatusRunning Status = "running"
	// SessionStatusCompleted indicates the session finished successfully.
	SessionStatusCompleted Status = "completed"
	// SessionStatusFailed indicates the session encountered an error.
	SessionStatusFailed Status = "failed"
)

// Event represents a single event in a session's history.
type Event struct {
	CreatedAt  time.Time
	ToolCallID *string
	ID         string
	SessionID  string
	Type       EventType
	Content    string
	ToolCalls  []ToolCall
	Obsolete   bool
}

// EventType represents the type of event in a session.
type EventType string

const (
	// EventTypeUserMessage represents a user input message.
	EventTypeUserMessage EventType = "user_message"
	// EventTypeAssistantMessage represents an LLM response or tool output.
	EventTypeAssistantMessage EventType = "assistant_message"
	// EventTypeToolResult represents the result of a tool invocation by the LLM.
	EventTypeToolResult EventType = "tool_result"
	// EventTypeSystem represents a system message (e.g., summaries for compaction).
	EventTypeSystem EventType = "system"
)

// ToolCall represents an LLM's request to invoke a tool.
type ToolCall struct {
	Params map[string]interface{} // Tool parameters
	ID     string                 // Unique identifier from LLM
	Name   string                 // Tool name
}

// Metadata represents arbitrary key-value data stored with a session.
type Metadata struct {
	SessionID string
	Key       string
	Value     string // JSON-encoded value
}

// Filter provides criteria for filtering sessions.
type Filter struct {
	ParentID *string // Filter by parent session ID
	ToolName *string // Filter by tool name
	Status   *Status // Filter by status
	Limit    int     // Maximum number of results (0 = no limit)
}
