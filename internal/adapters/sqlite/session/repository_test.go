// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/domain/session"
	"github.com/retran/meowg1k/internal/ports"
)

// mockHost is a simple mock implementation of ports.Host for testing.
type mockHost struct {
	db *sql.DB
}

func newMockHost(db *sql.DB) ports.Host {
	return &mockHost{db: db}
}

func (m *mockHost) GetMainDB() (*sql.DB, error) {
	return m.db, nil
}

func (m *mockHost) GetProjectDB() (*sql.DB, error) {
	return m.db, nil
}

func (m *mockHost) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// setupTestDB creates an in-memory SQLite database for testing.
func setupTestDB(t *testing.T) (*sql.DB, ports.Host) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err, "failed to open test database")

	// Run migrations
	for _, migration := range Migrations {
		tx, err := db.BeginTx(context.Background(), nil)
		require.NoError(t, err, "failed to begin transaction for migration %d", migration.Version)

		err = migration.Up(tx)
		if err != nil {
			tx.Rollback()
			require.NoError(t, err, "failed to run migration %d", migration.Version)
		}

		err = tx.Commit()
		require.NoError(t, err, "failed to commit migration %d", migration.Version)
	}

	host := newMockHost(db)
	return db, host
}

func TestRepository_CreateAndGetSession(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	now := time.Now().UTC()
	s := &session.Session{
		ID:        "session-1",
		ParentID:  nil,
		ToolName:  "test-tool",
		Status:    session.SessionStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := repo.CreateSession(ctx, s)
	require.NoError(t, err, "CreateSession should not return error")

	// Retrieve session
	retrieved, err := repo.GetSession(ctx, "session-1")
	require.NoError(t, err, "GetSession should not return error")
	require.NotNil(t, retrieved, "retrieved session should not be nil")

	assert.Equal(t, s.ID, retrieved.ID)
	assert.Equal(t, s.ToolName, retrieved.ToolName)
	assert.Equal(t, s.Status, retrieved.Status)
	assert.Nil(t, retrieved.ParentID)
}

func TestRepository_CreateChildSession(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Create parent session
	now := time.Now().UTC()
	parent := &session.Session{
		ID:        "parent-1",
		ParentID:  nil,
		ToolName:  "parent-tool",
		Status:    session.SessionStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err := repo.CreateSession(ctx, parent)
	require.NoError(t, err)

	// Create child session
	parentID := "parent-1"
	child := &session.Session{
		ID:        "child-1",
		ParentID:  &parentID,
		ToolName:  "child-tool",
		Status:    session.SessionStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err = repo.CreateSession(ctx, child)
	require.NoError(t, err)

	// Retrieve child
	retrieved, err := repo.GetSession(ctx, "child-1")
	require.NoError(t, err)
	require.NotNil(t, retrieved.ParentID)
	assert.Equal(t, "parent-1", *retrieved.ParentID)
}

func TestRepository_UpdateSession(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	now := time.Now().UTC()
	s := &session.Session{
		ID:        "session-1",
		ParentID:  nil,
		ToolName:  "test-tool",
		Status:    session.SessionStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err := repo.CreateSession(ctx, s)
	require.NoError(t, err)

	// Update status
	s.Status = session.SessionStatusCompleted
	s.UpdatedAt = time.Now().UTC()
	err = repo.UpdateSession(ctx, s)
	require.NoError(t, err)

	// Retrieve and verify
	retrieved, err := repo.GetSession(ctx, "session-1")
	require.NoError(t, err)
	assert.Equal(t, session.SessionStatusCompleted, retrieved.Status)
}

func TestRepository_ListSessions(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	now := time.Now().UTC()

	// Create multiple sessions
	sessions := []*session.Session{
		{
			ID:        "session-1",
			ParentID:  nil,
			ToolName:  "tool-a",
			Status:    session.SessionStatusRunning,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        "session-2",
			ParentID:  nil,
			ToolName:  "tool-b",
			Status:    session.SessionStatusCompleted,
			CreatedAt: now.Add(time.Minute),
			UpdatedAt: now.Add(time.Minute),
		},
		{
			ID:        "session-3",
			ParentID:  nil,
			ToolName:  "tool-a",
			Status:    session.SessionStatusFailed,
			CreatedAt: now.Add(2 * time.Minute),
			UpdatedAt: now.Add(2 * time.Minute),
		},
	}

	for _, s := range sessions {
		err := repo.CreateSession(ctx, s)
		require.NoError(t, err)
	}

	// List all
	all, err := repo.ListSessions(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// Filter by tool name
	toolNameA := "tool-a"
	filtered, err := repo.ListSessions(ctx, &session.SessionFilter{
		ToolName: &toolNameA,
	})
	require.NoError(t, err)
	assert.Len(t, filtered, 2)

	// Filter by status
	statusCompleted := session.SessionStatusCompleted
	filtered, err = repo.ListSessions(ctx, &session.SessionFilter{
		Status: &statusCompleted,
	})
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "session-2", filtered[0].ID)

	// Limit results
	limited, err := repo.ListSessions(ctx, &session.SessionFilter{
		Limit: 2,
	})
	require.NoError(t, err)
	assert.Len(t, limited, 2)
}

func TestRepository_AddAndGetEvents(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Create session
	now := time.Now().UTC()
	s := &session.Session{
		ID:        "session-1",
		ParentID:  nil,
		ToolName:  "test-tool",
		Status:    session.SessionStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err := repo.CreateSession(ctx, s)
	require.NoError(t, err)

	// Add event
	event := &session.Event{
		ID:        "event-1",
		SessionID: "session-1",
		Type:      session.EventTypeUserMessage,
		Content:   "Hello, world!",
		Obsolete:  false,
		CreatedAt: now,
	}
	err = repo.AddEvent(ctx, event)
	require.NoError(t, err)

	// Retrieve events
	events, err := repo.GetEvents(ctx, "session-1", 10, 0)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "event-1", events[0].ID)
	assert.Equal(t, "Hello, world!", events[0].Content)
	assert.Equal(t, session.EventTypeUserMessage, events[0].Type)
}

func TestRepository_AddEventWithToolCalls(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Create session
	now := time.Now().UTC()
	s := &session.Session{
		ID:        "session-1",
		ParentID:  nil,
		ToolName:  "test-tool",
		Status:    session.SessionStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err := repo.CreateSession(ctx, s)
	require.NoError(t, err)

	// Add event with tool calls
	event := &session.Event{
		ID:        "event-1",
		SessionID: "session-1",
		Type:      session.EventTypeAssistantMessage,
		Content:   "I'll call these tools",
		ToolCalls: []session.ToolCall{
			{
				ID:   "call-1",
				Name: "search",
				Params: map[string]interface{}{
					"query": "test query",
				},
			},
			{
				ID:   "call-2",
				Name: "read_file",
				Params: map[string]interface{}{
					"path": "/tmp/test.txt",
				},
			},
		},
		Obsolete:  false,
		CreatedAt: now,
	}
	err = repo.AddEvent(ctx, event)
	require.NoError(t, err)

	// Retrieve events
	events, err := repo.GetEvents(ctx, "session-1", 10, 0)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Len(t, events[0].ToolCalls, 2)
	assert.Equal(t, "call-1", events[0].ToolCalls[0].ID)
	assert.Equal(t, "search", events[0].ToolCalls[0].Name)
	assert.Equal(t, "test query", events[0].ToolCalls[0].Params["query"])
}

func TestRepository_MarkEventsObsolete(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Create session
	now := time.Now().UTC()
	s := &session.Session{
		ID:        "session-1",
		ParentID:  nil,
		ToolName:  "test-tool",
		Status:    session.SessionStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err := repo.CreateSession(ctx, s)
	require.NoError(t, err)

	// Add events
	for i := 1; i <= 3; i++ {
		event := &session.Event{
			ID:        generateID(),
			SessionID: "session-1",
			Type:      session.EventTypeUserMessage,
			Content:   "Message content",
			Obsolete:  false,
			CreatedAt: now.Add(time.Duration(i) * time.Second),
		}
		err = repo.AddEvent(ctx, event)
		require.NoError(t, err)
	}

	// Get all events
	events, err := repo.GetEvents(ctx, "session-1", 10, 0)
	require.NoError(t, err)
	require.Len(t, events, 3)

	// Mark first two events as obsolete
	err = repo.MarkEventsObsolete(ctx, []string{events[0].ID, events[1].ID})
	require.NoError(t, err)

	// GetEvents should only return non-obsolete events
	remaining, err := repo.GetEvents(ctx, "session-1", 10, 0)
	require.NoError(t, err)
	assert.Len(t, remaining, 1)
	assert.Equal(t, events[2].ID, remaining[0].ID)
}

func TestRepository_SetAndGetMetadata(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Create session
	now := time.Now().UTC()
	s := &session.Session{
		ID:        "session-1",
		ParentID:  nil,
		ToolName:  "test-tool",
		Status:    session.SessionStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err := repo.CreateSession(ctx, s)
	require.NoError(t, err)

	// Set metadata
	err = repo.SetMetadata(ctx, "session-1", "user_name", "Alice")
	require.NoError(t, err)

	err = repo.SetMetadata(ctx, "session-1", "model", "gpt-4")
	require.NoError(t, err)

	// Get metadata
	value, err := repo.GetMetadata(ctx, "session-1", "user_name")
	require.NoError(t, err)
	assert.Equal(t, "Alice", value)

	// Get all metadata
	all, err := repo.GetAllMetadata(ctx, "session-1")
	require.NoError(t, err)
	assert.Len(t, all, 2)
	assert.Equal(t, "Alice", all["user_name"])
	assert.Equal(t, "gpt-4", all["model"])

	// Update metadata
	err = repo.SetMetadata(ctx, "session-1", "user_name", "Bob")
	require.NoError(t, err)

	value, err = repo.GetMetadata(ctx, "session-1", "user_name")
	require.NoError(t, err)
	assert.Equal(t, "Bob", value)
}

func TestRepository_InsertSummary(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Create session
	now := time.Now().UTC()
	s := &session.Session{
		ID:        "session-1",
		ParentID:  nil,
		ToolName:  "test-tool",
		Status:    session.SessionStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err := repo.CreateSession(ctx, s)
	require.NoError(t, err)

	// Add events
	event1 := &session.Event{
		ID:        "event-1",
		SessionID: "session-1",
		Type:      session.EventTypeUserMessage,
		Content:   "First message",
		Obsolete:  false,
		CreatedAt: now,
	}
	err = repo.AddEvent(ctx, event1)
	require.NoError(t, err)

	// Insert summary after first event
	err = repo.InsertSummary(ctx, "session-1", "event-1", "Summary of conversation so far")
	require.NoError(t, err)

	// Get events
	events, err := repo.GetEvents(ctx, "session-1", 10, 0)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, session.EventTypeUserMessage, events[0].Type)
	assert.Equal(t, session.EventTypeSystem, events[1].Type)
	assert.Equal(t, "Summary of conversation so far", events[1].Content)
}
