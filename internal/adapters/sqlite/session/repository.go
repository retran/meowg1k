// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package session provides a SQLite-based repository for storing and querying sessions, events, and metadata.
package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/retran/meowg1k/internal/domain/session"
	"github.com/retran/meowg1k/internal/ports"
)

// Repository persists sessions, events, and metadata in SQLite.
type Repository struct {
	host ports.Host
}

var _ ports.SessionRepository = (*Repository)(nil)

// NewRepository creates a session repository backed by SQLite.
func NewRepository(host ports.Host) *Repository {
	return &Repository{host: host}
}

// CreateSession inserts a new session.
func (r *Repository) CreateSession(ctx context.Context, s *session.Session) error {
	if s == nil {
		return fmt.Errorf("session cannot be nil")
	}

	db, err := r.host.GetProjectDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO sessions (id, parent_id, tool_name, status, created_at, updated_at) 
		 VALUES (?, ?, ?, ?, ?, ?)`,
		s.ID, s.ParentID, s.ToolName, s.Status, s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert session: %w", err)
	}

	return nil
}

// GetSession retrieves a session by ID.
func (r *Repository) GetSession(ctx context.Context, id string) (*session.Session, error) {
	if id == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	db, err := r.host.GetProjectDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	var s session.Session
	var parentID sql.NullString

	err = db.QueryRowContext(ctx,
		`SELECT id, parent_id, tool_name, status, created_at, updated_at
		 FROM sessions WHERE id = ?`,
		id,
	).Scan(&s.ID, &parentID, &s.ToolName, &s.Status, &s.CreatedAt, &s.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query session: %w", err)
	}

	if parentID.Valid {
		s.ParentID = &parentID.String
	}

	return &s, nil
}

// UpdateSession updates an existing session's status and updated_at timestamp.
func (r *Repository) UpdateSession(ctx context.Context, s *session.Session) error {
	if s == nil {
		return fmt.Errorf("session cannot be nil")
	}

	db, err := r.host.GetProjectDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	_, err = db.ExecContext(ctx,
		`UPDATE sessions SET status = ?, updated_at = ? WHERE id = ?`,
		s.Status, s.UpdatedAt, s.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// ListSessions retrieves sessions with optional filters.
func (r *Repository) ListSessions(ctx context.Context, filter *session.SessionFilter) ([]*session.Session, error) {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	query := `SELECT id, parent_id, tool_name, status, created_at, updated_at FROM sessions WHERE 1=1`
	args := []interface{}{}

	if filter != nil {
		if filter.ParentID != nil {
			query += ` AND parent_id = ?`
			args = append(args, *filter.ParentID)
		}
		if filter.ToolName != nil {
			query += ` AND tool_name = ?`
			args = append(args, *filter.ToolName)
		}
		if filter.Status != nil {
			query += ` AND status = ?`
			args = append(args, *filter.Status)
		}
		if filter.Limit > 0 {
			query += ` ORDER BY created_at DESC LIMIT ?`
			args = append(args, filter.Limit)
		}
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*session.Session
	for rows.Next() {
		var s session.Session
		var parentID sql.NullString

		err := rows.Scan(&s.ID, &parentID, &s.ToolName, &s.Status, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		if parentID.Valid {
			s.ParentID = &parentID.String
		}

		sessions = append(sessions, &s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}

	return sessions, nil
}

// AddEvent inserts a new event into a session.
func (r *Repository) AddEvent(ctx context.Context, e *session.Event) error {
	if e == nil {
		return fmt.Errorf("event cannot be nil")
	}

	db, err := r.host.GetProjectDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO events (id, session_id, type, content, tool_call_id, obsolete, created_at) 
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.SessionID, e.Type, e.Content, e.ToolCallID, e.Obsolete, e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	for _, tc := range e.ToolCalls {
		paramsJSON, err := json.Marshal(tc.Params)
		if err != nil {
			return fmt.Errorf("failed to marshal tool call params: %w", err)
		}

		_, err = tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO tool_calls (id, event_id, name, params) VALUES (?, ?, ?, ?)`,
			tc.ID, e.ID, tc.Name, string(paramsJSON),
		)
		if err != nil {
			return fmt.Errorf("failed to insert tool call: %w", err)
		}
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE sessions SET updated_at = ? WHERE id = ?`,
		time.Now(), e.SessionID,
	)
	if err != nil {
		return fmt.Errorf("failed to update session timestamp: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetEvents retrieves events for a session with pagination.
func (r *Repository) GetEvents(ctx context.Context, sessionID string, limit, offset int) ([]*session.Event, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	db, err := r.host.GetProjectDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	rows, err := db.QueryContext(ctx,
		`SELECT id, session_id, type, content, tool_call_id, obsolete, created_at
		 FROM events WHERE session_id = ? AND obsolete = 0 
		 ORDER BY created_at ASC LIMIT ? OFFSET ?`,
		sessionID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []*session.Event
	for rows.Next() {
		var e session.Event
		var toolCallID sql.NullString

		err := rows.Scan(&e.ID, &e.SessionID, &e.Type, &e.Content, &toolCallID, &e.Obsolete, &e.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		if toolCallID.Valid {
			e.ToolCallID = &toolCallID.String
		}

		events = append(events, &e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating events: %w", err)
	}

	// Load tool calls for all events
	for _, e := range events {
		toolCalls, err := r.getToolCallsForEvent(ctx, db, e.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get tool calls for event %s: %w", e.ID, err)
		}
		e.ToolCalls = toolCalls
	}

	return events, nil
}

func (r *Repository) getToolCallsForEvent(ctx context.Context, db *sql.DB, eventID string) ([]session.ToolCall, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, name, params FROM tool_calls WHERE event_id = ?`,
		eventID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query tool calls: %w", err)
	}
	defer rows.Close()

	var toolCalls []session.ToolCall
	for rows.Next() {
		var tc session.ToolCall
		var paramsJSON string

		err := rows.Scan(&tc.ID, &tc.Name, &paramsJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tool call: %w", err)
		}

		if err := json.Unmarshal([]byte(paramsJSON), &tc.Params); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool call params: %w", err)
		}

		toolCalls = append(toolCalls, tc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tool calls: %w", err)
	}

	return toolCalls, nil
}

// MarkEventsObsolete marks events as obsolete (soft delete for compaction).
func (r *Repository) MarkEventsObsolete(ctx context.Context, eventIDs []string) error {
	if len(eventIDs) == 0 {
		return nil
	}

	db, err := r.host.GetProjectDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	placeholders := ""
	args := make([]interface{}, len(eventIDs))
	for i, id := range eventIDs {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
		args[i] = id
	}

	query := fmt.Sprintf(`UPDATE events SET obsolete = 1 WHERE id IN (%s)`, placeholders)
	_, err = db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to mark events obsolete: %w", err)
	}

	return nil
}

// InsertSummary inserts a system summary event after a specific event.
func (r *Repository) InsertSummary(ctx context.Context, sessionID, afterEventID, summaryContent string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}
	if summaryContent == "" {
		return fmt.Errorf("summary content cannot be empty")
	}

	db, err := r.host.GetProjectDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	var afterTimestamp time.Time
	if afterEventID != "" {
		err = db.QueryRowContext(ctx,
			`SELECT created_at FROM events WHERE id = ?`,
			afterEventID,
		).Scan(&afterTimestamp)
		if err != nil {
			return fmt.Errorf("failed to get after event timestamp: %w", err)
		}
	} else {
		afterTimestamp = time.Now()
	}

	// Insert summary with timestamp slightly after the reference event
	summaryTime := afterTimestamp.Add(time.Microsecond)
	summaryEvent := &session.Event{
		ID:        generateID(),
		SessionID: sessionID,
		Type:      session.EventTypeSystem,
		Content:   summaryContent,
		Obsolete:  false,
		CreatedAt: summaryTime,
	}

	return r.AddEvent(ctx, summaryEvent)
}

// SetMetadata sets a metadata key-value pair for a session.
func (r *Repository) SetMetadata(ctx context.Context, sessionID, key, value string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}
	if key == "" {
		return fmt.Errorf("metadata key cannot be empty")
	}

	db, err := r.host.GetProjectDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO session_metadata (session_id, key, value) 
		 VALUES (?, ?, ?) 
		 ON CONFLICT(session_id, key) DO UPDATE SET value = excluded.value`,
		sessionID, key, value,
	)
	if err != nil {
		return fmt.Errorf("failed to set metadata: %w", err)
	}

	return nil
}

// GetMetadata retrieves a metadata value for a session.
func (r *Repository) GetMetadata(ctx context.Context, sessionID, key string) (string, error) {
	if sessionID == "" {
		return "", fmt.Errorf("session ID cannot be empty")
	}
	if key == "" {
		return "", fmt.Errorf("metadata key cannot be empty")
	}

	db, err := r.host.GetProjectDB()
	if err != nil {
		return "", fmt.Errorf("failed to get database: %w", err)
	}

	var value string
	err = db.QueryRowContext(ctx,
		`SELECT value FROM session_metadata WHERE session_id = ? AND key = ?`,
		sessionID, key,
	).Scan(&value)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("metadata not found for key: %s", key)
	}
	if err != nil {
		return "", fmt.Errorf("failed to query metadata: %w", err)
	}

	return value, nil
}

// GetAllMetadata retrieves all metadata for a session.
func (r *Repository) GetAllMetadata(ctx context.Context, sessionID string) (map[string]string, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	db, err := r.host.GetProjectDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	rows, err := db.QueryContext(ctx,
		`SELECT key, value FROM session_metadata WHERE session_id = ?`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query metadata: %w", err)
	}
	defer rows.Close()

	metadata := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan metadata: %w", err)
		}
		metadata[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating metadata: %w", err)
	}

	return metadata, nil
}

// generateID generates a unique ID for events/sessions.
// In production, this should use UUID generation.
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
