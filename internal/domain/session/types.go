// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package session provides domain types for session management.
package session

import "time"

// Session represents a tool execution session with chat history.
type Session struct {
	ID        string
	ParentID  *string // nil for root sessions
	ToolName  string  // Name of the tool being executed
	Status    SessionStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SessionStatus represents the current state of a session.
type SessionStatus string

const (
	// SessionStatusRunning indicates the session is currently executing.
	SessionStatusRunning SessionStatus = "running"
	// SessionStatusCompleted indicates the session finished successfully.
	SessionStatusCompleted SessionStatus = "completed"
	// SessionStatusFailed indicates the session encountered an error.
	SessionStatusFailed SessionStatus = "failed"
)

// Event represents a single event in a session's history.
type Event struct {
	ID         string
	SessionID  string
	Type       EventType
	Content    string
	ToolCallID *string    // Only set for tool_result events
	ToolCalls  []ToolCall // Only set for assistant_message events with tool calls
	Obsolete   bool       // When true, event is marked for compaction (excluded from context)
	CreatedAt  time.Time
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
	ID     string                 // Unique identifier from LLM
	Name   string                 // Tool name
	Params map[string]interface{} // Tool parameters
}

// Metadata represents arbitrary key-value data stored with a session.
type Metadata struct {
	SessionID string
	Key       string
	Value     string // JSON-encoded value
}

// SessionFilter provides criteria for filtering sessions.
type SessionFilter struct {
	ParentID *string        // Filter by parent session ID
	ToolName *string        // Filter by tool name
	Status   *SessionStatus // Filter by status
	Limit    int            // Maximum number of results (0 = no limit)
}
