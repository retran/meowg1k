// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/domain/session"
)

// mockSessionRepository is a mock implementation of ports.SessionRepository.
type mockSessionRepository struct {
	mock.Mock
}

func (m *mockSessionRepository) CreateSession(ctx context.Context, s *session.Session) error {
	args := m.Called(ctx, s)
	return args.Error(0)
}

func (m *mockSessionRepository) GetSession(ctx context.Context, id string) (*session.Session, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*session.Session), args.Error(1)
}

func (m *mockSessionRepository) UpdateSession(ctx context.Context, s *session.Session) error {
	args := m.Called(ctx, s)
	return args.Error(0)
}

func (m *mockSessionRepository) ListSessions(ctx context.Context, filter *session.SessionFilter) ([]*session.Session, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*session.Session), args.Error(1)
}

func (m *mockSessionRepository) AddEvent(ctx context.Context, e *session.Event) error {
	args := m.Called(ctx, e)
	return args.Error(0)
}

func (m *mockSessionRepository) GetEvents(ctx context.Context, sessionID string, limit, offset int) ([]*session.Event, error) {
	args := m.Called(ctx, sessionID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*session.Event), args.Error(1)
}

func (m *mockSessionRepository) MarkEventsObsolete(ctx context.Context, eventIDs []string) error {
	args := m.Called(ctx, eventIDs)
	return args.Error(0)
}

func (m *mockSessionRepository) InsertSummary(ctx context.Context, sessionID, afterEventID, summaryContent string) error {
	args := m.Called(ctx, sessionID, afterEventID, summaryContent)
	return args.Error(0)
}

func (m *mockSessionRepository) SetMetadata(ctx context.Context, sessionID, key, value string) error {
	args := m.Called(ctx, sessionID, key, value)
	return args.Error(0)
}

func (m *mockSessionRepository) GetMetadata(ctx context.Context, sessionID, key string) (string, error) {
	args := m.Called(ctx, sessionID, key)
	return args.String(0), args.Error(1)
}

func (m *mockSessionRepository) GetAllMetadata(ctx context.Context, sessionID string) (map[string]string, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

func TestNewService(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := new(mockSessionRepository)
		svc, err := NewService(repo)
		require.NoError(t, err)
		require.NotNil(t, svc)
	})

	t.Run("nil repository", func(t *testing.T) {
		svc, err := NewService(nil)
		require.Error(t, err)
		require.Nil(t, svc)
		assert.Contains(t, err.Error(), "repository is nil")
	})
}

func TestService_CreateSession(t *testing.T) {
	t.Run("root session", func(t *testing.T) {
		repo := new(mockSessionRepository)
		svc, _ := NewService(repo)
		ctx := context.Background()

		repo.On("CreateSession", ctx, mock.MatchedBy(func(s *session.Session) bool {
			return s.ToolName == "test-tool" && s.ParentID == nil && s.Status == session.SessionStatusRunning
		})).Return(nil)

		sess, err := svc.CreateSession(ctx, nil, "test-tool")
		require.NoError(t, err)
		require.NotNil(t, sess)
		assert.NotEmpty(t, sess.ID)
		assert.Equal(t, "test-tool", sess.ToolName)
		assert.Nil(t, sess.ParentID)
		assert.Equal(t, session.SessionStatusRunning, sess.Status)

		repo.AssertExpectations(t)
	})

	t.Run("child session", func(t *testing.T) {
		repo := new(mockSessionRepository)
		svc, _ := NewService(repo)
		ctx := context.Background()

		parentID := "parent-123"
		parentSession := &session.Session{
			ID:       parentID,
			ToolName: "parent-tool",
			Status:   session.SessionStatusRunning,
		}

		repo.On("GetSession", ctx, parentID).Return(parentSession, nil)
		repo.On("CreateSession", ctx, mock.MatchedBy(func(s *session.Session) bool {
			return s.ParentID != nil && *s.ParentID == parentID
		})).Return(nil)

		sess, err := svc.CreateSession(ctx, &parentID, "child-tool")
		require.NoError(t, err)
		require.NotNil(t, sess)
		require.NotNil(t, sess.ParentID)
		assert.Equal(t, parentID, *sess.ParentID)

		repo.AssertExpectations(t)
	})

	t.Run("parent not found", func(t *testing.T) {
		repo := new(mockSessionRepository)
		svc, _ := NewService(repo)
		ctx := context.Background()

		parentID := "nonexistent"
		repo.On("GetSession", ctx, parentID).Return(nil, errors.New("not found"))

		sess, err := svc.CreateSession(ctx, &parentID, "child-tool")
		require.Error(t, err)
		require.Nil(t, sess)
		assert.Contains(t, err.Error(), "parent session not found")

		repo.AssertExpectations(t)
	})

	t.Run("empty tool name", func(t *testing.T) {
		repo := new(mockSessionRepository)
		svc, _ := NewService(repo)
		ctx := context.Background()

		sess, err := svc.CreateSession(ctx, nil, "")
		require.Error(t, err)
		require.Nil(t, sess)
		assert.Contains(t, err.Error(), "tool name cannot be empty")
	})
}

func TestService_CompleteSession(t *testing.T) {
	repo := new(mockSessionRepository)
	svc, _ := NewService(repo)
	ctx := context.Background()

	sessionID := "session-123"
	existingSession := &session.Session{
		ID:       sessionID,
		ToolName: "test-tool",
		Status:   session.SessionStatusRunning,
	}

	repo.On("GetSession", ctx, sessionID).Return(existingSession, nil)
	repo.On("UpdateSession", ctx, mock.MatchedBy(func(s *session.Session) bool {
		return s.ID == sessionID && s.Status == session.SessionStatusCompleted
	})).Return(nil)

	err := svc.CompleteSession(ctx, sessionID)
	require.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestService_FailSession(t *testing.T) {
	repo := new(mockSessionRepository)
	svc, _ := NewService(repo)
	ctx := context.Background()

	sessionID := "session-123"
	existingSession := &session.Session{
		ID:       sessionID,
		ToolName: "test-tool",
		Status:   session.SessionStatusRunning,
	}

	repo.On("GetSession", ctx, sessionID).Return(existingSession, nil)
	repo.On("UpdateSession", ctx, mock.MatchedBy(func(s *session.Session) bool {
		return s.ID == sessionID && s.Status == session.SessionStatusFailed
	})).Return(nil)

	err := svc.FailSession(ctx, sessionID)
	require.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestService_AddUserMessage(t *testing.T) {
	repo := new(mockSessionRepository)
	svc, _ := NewService(repo)
	ctx := context.Background()

	sessionID := "session-123"
	content := "Hello, world!"

	repo.On("AddEvent", ctx, mock.MatchedBy(func(e *session.Event) bool {
		return e.SessionID == sessionID &&
			e.Type == session.EventTypeUserMessage &&
			e.Content == content &&
			!e.Obsolete
	})).Return(nil)

	err := svc.AddUserMessage(ctx, sessionID, content)
	require.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestService_AddAssistantMessage(t *testing.T) {
	repo := new(mockSessionRepository)
	svc, _ := NewService(repo)
	ctx := context.Background()

	sessionID := "session-123"
	content := "I'll help you with that"
	toolCalls := []session.ToolCall{
		{
			ID:   "call-1",
			Name: "search",
			Params: map[string]interface{}{
				"query": "test",
			},
		},
	}

	repo.On("AddEvent", ctx, mock.MatchedBy(func(e *session.Event) bool {
		return e.SessionID == sessionID &&
			e.Type == session.EventTypeAssistantMessage &&
			e.Content == content &&
			len(e.ToolCalls) == 1 &&
			e.ToolCalls[0].Name == "search"
	})).Return(nil)

	err := svc.AddAssistantMessage(ctx, sessionID, content, toolCalls)
	require.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestService_AddToolResult(t *testing.T) {
	repo := new(mockSessionRepository)
	svc, _ := NewService(repo)
	ctx := context.Background()

	sessionID := "session-123"
	toolCallID := "call-1"
	content := "Search results: ..."

	repo.On("AddEvent", ctx, mock.MatchedBy(func(e *session.Event) bool {
		return e.SessionID == sessionID &&
			e.Type == session.EventTypeToolResult &&
			e.Content == content &&
			e.ToolCallID != nil &&
			*e.ToolCallID == toolCallID
	})).Return(nil)

	err := svc.AddToolResult(ctx, sessionID, toolCallID, content)
	require.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestService_AddSystemMessage(t *testing.T) {
	repo := new(mockSessionRepository)
	svc, _ := NewService(repo)
	ctx := context.Background()

	sessionID := "session-123"
	content := "System notification"

	repo.On("AddEvent", ctx, mock.MatchedBy(func(e *session.Event) bool {
		return e.SessionID == sessionID &&
			e.Type == session.EventTypeSystem &&
			e.Content == content
	})).Return(nil)

	err := svc.AddSystemMessage(ctx, sessionID, content)
	require.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestService_GetEvents(t *testing.T) {
	repo := new(mockSessionRepository)
	svc, _ := NewService(repo)
	ctx := context.Background()

	sessionID := "session-123"
	expectedEvents := []*session.Event{
		{
			ID:        "event-1",
			SessionID: sessionID,
			Type:      session.EventTypeUserMessage,
			Content:   "Message 1",
		},
	}

	repo.On("GetEvents", ctx, sessionID, 10, 0).Return(expectedEvents, nil)

	events, err := svc.GetEvents(ctx, sessionID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "event-1", events[0].ID)

	repo.AssertExpectations(t)
}

func TestService_GetAllEvents(t *testing.T) {
	repo := new(mockSessionRepository)
	svc, _ := NewService(repo)
	ctx := context.Background()

	sessionID := "session-123"
	expectedEvents := []*session.Event{
		{ID: "event-1", SessionID: sessionID},
		{ID: "event-2", SessionID: sessionID},
	}

	repo.On("GetEvents", ctx, sessionID, 100000, 0).Return(expectedEvents, nil)

	events, err := svc.GetAllEvents(ctx, sessionID)
	require.NoError(t, err)
	assert.Len(t, events, 2)

	repo.AssertExpectations(t)
}

func TestService_GetChildSessions(t *testing.T) {
	repo := new(mockSessionRepository)
	svc, _ := NewService(repo)
	ctx := context.Background()

	parentID := "parent-123"
	expectedSessions := []*session.Session{
		{ID: "child-1", ParentID: &parentID},
		{ID: "child-2", ParentID: &parentID},
	}

	repo.On("ListSessions", ctx, mock.MatchedBy(func(f *session.SessionFilter) bool {
		return f.ParentID != nil && *f.ParentID == parentID
	})).Return(expectedSessions, nil)

	children, err := svc.GetChildSessions(ctx, parentID)
	require.NoError(t, err)
	assert.Len(t, children, 2)

	repo.AssertExpectations(t)
}

func TestService_MarkEventsObsolete(t *testing.T) {
	repo := new(mockSessionRepository)
	svc, _ := NewService(repo)
	ctx := context.Background()

	eventIDs := []string{"event-1", "event-2"}

	repo.On("MarkEventsObsolete", ctx, eventIDs).Return(nil)

	err := svc.MarkEventsObsolete(ctx, eventIDs)
	require.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestService_InsertSummary(t *testing.T) {
	repo := new(mockSessionRepository)
	svc, _ := NewService(repo)
	ctx := context.Background()

	sessionID := "session-123"
	afterEventID := "event-1"
	summary := "Summary of previous conversation"

	repo.On("InsertSummary", ctx, sessionID, afterEventID, summary).Return(nil)

	err := svc.InsertSummary(ctx, sessionID, afterEventID, summary)
	require.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestService_SetAndGetMetadata(t *testing.T) {
	repo := new(mockSessionRepository)
	svc, _ := NewService(repo)
	ctx := context.Background()

	sessionID := "session-123"
	key := "user_name"
	value := "Alice"

	repo.On("SetMetadata", ctx, sessionID, key, value).Return(nil)
	repo.On("GetMetadata", ctx, sessionID, key).Return(value, nil)

	err := svc.SetMetadata(ctx, sessionID, key, value)
	require.NoError(t, err)

	retrieved, err := svc.GetMetadata(ctx, sessionID, key)
	require.NoError(t, err)
	assert.Equal(t, value, retrieved)

	repo.AssertExpectations(t)
}

func TestService_GetAllMetadata(t *testing.T) {
	repo := new(mockSessionRepository)
	svc, _ := NewService(repo)
	ctx := context.Background()

	sessionID := "session-123"
	expectedMetadata := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	repo.On("GetAllMetadata", ctx, sessionID).Return(expectedMetadata, nil)

	metadata, err := svc.GetAllMetadata(ctx, sessionID)
	require.NoError(t, err)
	assert.Len(t, metadata, 2)
	assert.Equal(t, "value1", metadata["key1"])

	repo.AssertExpectations(t)
}

func TestService_GetChildMetadata(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := new(mockSessionRepository)
		svc, _ := NewService(repo)
		ctx := context.Background()

		sessionID := "parent-123"
		childID := "child-456"
		key := "result"
		value := "success"

		childSession := &session.Session{
			ID:       childID,
			ParentID: &sessionID,
			ToolName: "child-tool",
		}

		repo.On("GetSession", ctx, childID).Return(childSession, nil)
		repo.On("GetMetadata", ctx, childID, key).Return(value, nil)

		retrieved, err := svc.GetChildMetadata(ctx, sessionID, childID, key)
		require.NoError(t, err)
		assert.Equal(t, value, retrieved)

		repo.AssertExpectations(t)
	})

	t.Run("not a child", func(t *testing.T) {
		repo := new(mockSessionRepository)
		svc, _ := NewService(repo)
		ctx := context.Background()

		sessionID := "parent-123"
		childID := "child-456"
		wrongParent := "other-parent"
		key := "result"

		childSession := &session.Session{
			ID:       childID,
			ParentID: &wrongParent,
			ToolName: "child-tool",
		}

		repo.On("GetSession", ctx, childID).Return(childSession, nil)

		retrieved, err := svc.GetChildMetadata(ctx, sessionID, childID, key)
		require.Error(t, err)
		assert.Empty(t, retrieved)
		assert.Contains(t, err.Error(), "not a child")

		repo.AssertExpectations(t)
	})
}
