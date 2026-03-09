// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

// Package session provides services for managing sessions, events, and metadata.
package session

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/retran/meowg1k/internal/domain/session"
	"github.com/retran/meowg1k/internal/ports"
)

// Service manages session lifecycle and operations.
type Service struct {
	repository ports.SessionRepository
}

var _ ports.SessionService = (*Service)(nil)

// NewService creates a new session service.
func NewService(repository ports.SessionRepository) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("session repository is nil")
	}

	return &Service{
		repository: repository,
	}, nil
}

// CreateSession creates a new session with auto-generated UUID.
func (s *Service) CreateSession(ctx context.Context, parentID *string, toolName string) (*session.Session, error) {
	if s == nil {
		return nil, fmt.Errorf("session service is nil")
	}

	if toolName == "" {
		return nil, fmt.Errorf("tool name cannot be empty")
	}

	if parentID != nil && *parentID != "" {
		_, err := s.repository.GetSession(ctx, *parentID)
		if err != nil {
			return nil, fmt.Errorf("parent session not found: %w", err)
		}
	}

	now := time.Now().UTC()
	sess := &session.Session{
		ID:        uuid.New().String(),
		ParentID:  parentID,
		ToolName:  toolName,
		Status:    session.SessionStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repository.CreateSession(ctx, sess); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return sess, nil
}

// GetSession retrieves a session by ID.
func (s *Service) GetSession(ctx context.Context, id string) (*session.Session, error) {
	if s == nil {
		return nil, fmt.Errorf("session service is nil")
	}

	if id == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	sess, err := s.repository.GetSession(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return sess, nil
}

// ListSessions retrieves sessions with optional filters.
func (s *Service) ListSessions(ctx context.Context, filter *session.Filter) ([]*session.Session, error) {
	if s == nil {
		return nil, fmt.Errorf("session service is nil")
	}

	sessions, err := s.repository.ListSessions(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	return sessions, nil
}

// GetChildSessions retrieves all child sessions for a parent session.
func (s *Service) GetChildSessions(ctx context.Context, parentID string) ([]*session.Session, error) {
	if s == nil {
		return nil, fmt.Errorf("session service is nil")
	}

	if parentID == "" {
		return nil, fmt.Errorf("parent ID cannot be empty")
	}

	filter := &session.Filter{
		ParentID: &parentID,
	}

	return s.ListSessions(ctx, filter)
}

// updateSessionStatus retrieves a session and sets its status to the given value.
func (s *Service) updateSessionStatus(ctx context.Context, id string, status session.Status) error {
	if s == nil {
		return fmt.Errorf("session service is nil")
	}

	if id == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	sess, err := s.repository.GetSession(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	sess.Status = status
	sess.UpdatedAt = time.Now().UTC()

	if err := s.repository.UpdateSession(ctx, sess); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// CompleteSession marks a session as completed.
func (s *Service) CompleteSession(ctx context.Context, id string) error {
	return s.updateSessionStatus(ctx, id, session.SessionStatusCompleted)
}

// FailSession marks a session as failed.
func (s *Service) FailSession(ctx context.Context, id string) error {
	return s.updateSessionStatus(ctx, id, session.SessionStatusFailed)
}

// addSimpleEvent creates a simple event (no tool calls or tool result IDs) and persists it.
func (s *Service) addSimpleEvent(ctx context.Context, sessionID, content string, eventType session.EventType, failMsg string) error {
	if s == nil {
		return fmt.Errorf("session service is nil")
	}

	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	event := &session.Event{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Type:      eventType,
		Content:   content,
		Obsolete:  false,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repository.AddEvent(ctx, event); err != nil {
		return fmt.Errorf("%s: %w", failMsg, err)
	}

	return nil
}

// AddUserMessage adds a user message event to the session.
func (s *Service) AddUserMessage(ctx context.Context, sessionID, content string) error {
	return s.addSimpleEvent(ctx, sessionID, content, session.EventTypeUserMessage, "failed to add user message")
}

// AddAssistantMessage adds an assistant message event to the session.
func (s *Service) AddAssistantMessage(ctx context.Context, sessionID, content string, toolCalls []session.ToolCall) error {
	if s == nil {
		return fmt.Errorf("session service is nil")
	}

	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	event := &session.Event{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Type:      session.EventTypeAssistantMessage,
		Content:   content,
		ToolCalls: toolCalls,
		Obsolete:  false,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repository.AddEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to add assistant message: %w", err)
	}

	return nil
}

// AddToolResult adds a tool result event to the session.
func (s *Service) AddToolResult(ctx context.Context, sessionID, toolCallID, content string) error {
	if s == nil {
		return fmt.Errorf("session service is nil")
	}

	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	if toolCallID == "" {
		return fmt.Errorf("tool call ID cannot be empty")
	}

	event := &session.Event{
		ID:         uuid.New().String(),
		SessionID:  sessionID,
		Type:       session.EventTypeToolResult,
		Content:    content,
		ToolCallID: &toolCallID,
		Obsolete:   false,
		CreatedAt:  time.Now().UTC(),
	}

	if err := s.repository.AddEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to add tool result: %w", err)
	}

	return nil
}

// AddSystemMessage adds a system message event to the session.
func (s *Service) AddSystemMessage(ctx context.Context, sessionID, content string) error {
	return s.addSimpleEvent(ctx, sessionID, content, session.EventTypeSystem, "failed to add system message")
}

// GetEvents retrieves events for a session with pagination.
func (s *Service) GetEvents(ctx context.Context, sessionID string, limit, offset int) ([]*session.Event, error) {
	if s == nil {
		return nil, fmt.Errorf("session service is nil")
	}

	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than 0")
	}

	events, err := s.repository.GetEvents(ctx, sessionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %w", err)
	}

	return events, nil
}

// GetAllEvents retrieves all non-obsolete events for a session.
func (s *Service) GetAllEvents(ctx context.Context, sessionID string) ([]*session.Event, error) {
	if s == nil {
		return nil, fmt.Errorf("session service is nil")
	}

	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	// Use a large limit to get all events
	const maxEvents = 100000
	events, err := s.repository.GetEvents(ctx, sessionID, maxEvents, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get all events: %w", err)
	}

	return events, nil
}

// MarkEventsObsolete marks events as obsolete for compaction.
func (s *Service) MarkEventsObsolete(ctx context.Context, eventIDs []string) error {
	if s == nil {
		return fmt.Errorf("session service is nil")
	}

	if len(eventIDs) == 0 {
		return nil
	}

	if err := s.repository.MarkEventsObsolete(ctx, eventIDs); err != nil {
		return fmt.Errorf("failed to mark events obsolete: %w", err)
	}

	return nil
}

// InsertSummary inserts a summary event after a specific event.
func (s *Service) InsertSummary(ctx context.Context, sessionID, afterEventID, summaryContent string) error {
	if s == nil {
		return fmt.Errorf("session service is nil")
	}

	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	if summaryContent == "" {
		return fmt.Errorf("summary content cannot be empty")
	}

	if err := s.repository.InsertSummary(ctx, sessionID, afterEventID, summaryContent); err != nil {
		return fmt.Errorf("failed to insert summary: %w", err)
	}

	return nil
}

// SetMetadata sets session metadata.
func (s *Service) SetMetadata(ctx context.Context, sessionID, key, value string) error {
	if s == nil {
		return fmt.Errorf("session service is nil")
	}

	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	if key == "" {
		return fmt.Errorf("metadata key cannot be empty")
	}

	if err := s.repository.SetMetadata(ctx, sessionID, key, value); err != nil {
		return fmt.Errorf("failed to set metadata: %w", err)
	}

	return nil
}

// GetMetadata retrieves session metadata.
func (s *Service) GetMetadata(ctx context.Context, sessionID, key string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("session service is nil")
	}

	if sessionID == "" {
		return "", fmt.Errorf("session ID cannot be empty")
	}

	if key == "" {
		return "", fmt.Errorf("metadata key cannot be empty")
	}

	value, err := s.repository.GetMetadata(ctx, sessionID, key)
	if err != nil {
		return "", fmt.Errorf("failed to get metadata: %w", err)
	}

	return value, nil
}

// GetAllMetadata retrieves all session metadata.
func (s *Service) GetAllMetadata(ctx context.Context, sessionID string) (map[string]string, error) {
	if s == nil {
		return nil, fmt.Errorf("session service is nil")
	}

	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	metadata, err := s.repository.GetAllMetadata(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get all metadata: %w", err)
	}

	return metadata, nil
}

// GetChildMetadata retrieves metadata from a child session.
func (s *Service) GetChildMetadata(ctx context.Context, sessionID, childID, key string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("session service is nil")
	}

	if sessionID == "" {
		return "", fmt.Errorf("session ID cannot be empty")
	}

	if childID == "" {
		return "", fmt.Errorf("child ID cannot be empty")
	}

	if key == "" {
		return "", fmt.Errorf("metadata key cannot be empty")
	}

	// Verify the child session is actually a child of the parent
	child, err := s.repository.GetSession(ctx, childID)
	if err != nil {
		return "", fmt.Errorf("failed to get child session: %w", err)
	}

	if child.ParentID == nil || *child.ParentID != sessionID {
		return "", fmt.Errorf("session %s is not a child of %s", childID, sessionID)
	}

	value, err := s.repository.GetMetadata(ctx, childID, key)
	if err != nil {
		return "", fmt.Errorf("failed to get child metadata: %w", err)
	}

	return value, nil
}
