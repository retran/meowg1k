// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"context"
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/retran/meowg1k/internal/domain/session"
	"github.com/retran/meowg1k/internal/ports"
)

// SessionModule provides session management functionality to Starlark scripts.
type SessionModule struct {
	sessionService ports.SessionService
	currentSession *session.Session // The session this context belongs to
}

// NewSessionModule creates a new session module.
// Returns a Starlark struct with session methods exposed directly as attributes.
func NewSessionModule(sessionService ports.SessionService, currentSession *session.Session) starlark.Value {
	module := &SessionModule{
		sessionService: sessionService,
		currentSession: currentSession,
	}
	return module.toStarlarkStruct()
}

// toStarlarkStruct converts the session module to a Starlark struct with methods
func (m *SessionModule) toStarlarkStruct() *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"id":                 m.idMethod(),
		"tool_name":          m.toolNameMethod(),
		"parent_id":          m.parentIDMethod(),
		"status":             m.statusMethod(),
		"set_metadata":       m.setMetadataMethod(),
		"get_metadata":       m.getMetadataMethod(),
		"get_all_metadata":   m.getAllMetadataMethod(),
		"get_children":       m.getChildrenMethod(),
		"get_child_metadata": m.getChildMetadataMethod(),
		"get_events":         m.getEventsMethod(),
		"mark_obsolete":      m.markObsoleteMethod(),
		"insert_summary":     m.insertSummaryMethod(),
		"list_all":           m.listAllMethod(),
		"get_by_id":          m.getByIDMethod(),
		"set_system":         m.setSystemMethod(),
		"get_system":         m.getSystemMethod(),
	})
}

// idMethod returns the current session ID
func (m *SessionModule) idMethod() *starlark.Builtin {
	return starlark.NewBuiltin("id", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if m.currentSession == nil {
			return starlark.None, nil
		}
		return starlark.String(m.currentSession.ID), nil
	})
}

// toolNameMethod returns the current session tool name
func (m *SessionModule) toolNameMethod() *starlark.Builtin {
	return starlark.NewBuiltin("tool_name", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if m.currentSession == nil {
			return starlark.None, nil
		}
		return starlark.String(m.currentSession.ToolName), nil
	})
}

// parentIDMethod returns the current session parent ID
func (m *SessionModule) parentIDMethod() *starlark.Builtin {
	return starlark.NewBuiltin("parent_id", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if m.currentSession == nil || m.currentSession.ParentID == nil {
			return starlark.None, nil
		}
		return starlark.String(*m.currentSession.ParentID), nil
	})
}

// statusMethod returns the current session status
func (m *SessionModule) statusMethod() *starlark.Builtin {
	return starlark.NewBuiltin("status", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if m.currentSession == nil {
			return starlark.None, nil
		}
		return starlark.String(string(m.currentSession.Status)), nil
	})
}

// setMetadataMethod sets metadata for the current session
func (m *SessionModule) setMetadataMethod() *starlark.Builtin {
	return starlark.NewBuiltin("set_metadata", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if m.currentSession == nil {
			return nil, fmt.Errorf("no active session")
		}

		var key, value string
		if err := starlark.UnpackArgs("set_metadata", args, kwargs, "key", &key, "value", &value); err != nil {
			return nil, err
		}

		ctx := context.Background()
		if err := m.sessionService.SetMetadata(ctx, m.currentSession.ID, key, value); err != nil {
			return nil, fmt.Errorf("failed to set metadata: %w", err)
		}

		return starlark.None, nil
	})
}

// getMetadataMethod retrieves metadata for the current session
func (m *SessionModule) getMetadataMethod() *starlark.Builtin {
	return starlark.NewBuiltin("get_metadata", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if m.currentSession == nil {
			return nil, fmt.Errorf("no active session")
		}

		var key string
		if err := starlark.UnpackArgs("get_metadata", args, kwargs, "key", &key); err != nil {
			return nil, err
		}

		ctx := context.Background()
		value, err := m.sessionService.GetMetadata(ctx, m.currentSession.ID, key)
		if err != nil {
			return starlark.None, nil // Return None if metadata not found
		}

		return starlark.String(value), nil
	})
}

// getAllMetadataMethod retrieves all metadata for the current session
func (m *SessionModule) getAllMetadataMethod() *starlark.Builtin {
	return starlark.NewBuiltin("get_all_metadata", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if m.currentSession == nil {
			return nil, fmt.Errorf("no active session")
		}

		ctx := context.Background()
		metadata, err := m.sessionService.GetAllMetadata(ctx, m.currentSession.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get metadata: %w", err)
		}

		// Convert to Starlark dict
		dict := starlark.NewDict(len(metadata))
		for k, v := range metadata {
			if err := dict.SetKey(starlark.String(k), starlark.String(v)); err != nil {
				return nil, fmt.Errorf("failed to build metadata dict: %w", err)
			}
		}

		return dict, nil
	})
}

// getChildrenMethod retrieves child sessions
func (m *SessionModule) getChildrenMethod() *starlark.Builtin {
	return starlark.NewBuiltin("get_children", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if m.currentSession == nil {
			return nil, fmt.Errorf("no active session")
		}

		ctx := context.Background()
		children, err := m.sessionService.GetChildSessions(ctx, m.currentSession.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get child sessions: %w", err)
		}

		// Convert to Starlark list of dicts
		result := make([]starlark.Value, len(children))
		for i, child := range children {
			childDict := starlark.NewDict(4)
			childDict.SetKey(starlark.String("id"), starlark.String(child.ID))
			childDict.SetKey(starlark.String("tool_name"), starlark.String(child.ToolName))
			childDict.SetKey(starlark.String("status"), starlark.String(string(child.Status)))
			if child.ParentID != nil {
				childDict.SetKey(starlark.String("parent_id"), starlark.String(*child.ParentID))
			}
			result[i] = childDict
		}

		return starlark.NewList(result), nil
	})
}

// getChildMetadataMethod retrieves metadata from a child session
func (m *SessionModule) getChildMetadataMethod() *starlark.Builtin {
	return starlark.NewBuiltin("get_child_metadata", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if m.currentSession == nil {
			return nil, fmt.Errorf("no active session")
		}

		var childID, key string
		if err := starlark.UnpackArgs("get_child_metadata", args, kwargs, "child_id", &childID, "key", &key); err != nil {
			return nil, err
		}

		ctx := context.Background()
		value, err := m.sessionService.GetChildMetadata(ctx, m.currentSession.ID, childID, key)
		if err != nil {
			return starlark.None, nil // Return None if metadata not found
		}

		return starlark.String(value), nil
	})
}

// getEventsMethod retrieves events for the current session
func (m *SessionModule) getEventsMethod() *starlark.Builtin {
	return starlark.NewBuiltin("get_events", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if m.currentSession == nil {
			return nil, fmt.Errorf("no active session")
		}

		var limit int = 100
		var offset int = 0
		if err := starlark.UnpackArgs("get_events", args, kwargs, "limit?", &limit, "offset?", &offset); err != nil {
			return nil, err
		}

		ctx := context.Background()
		events, err := m.sessionService.GetEvents(ctx, m.currentSession.ID, limit, offset)
		if err != nil {
			return nil, fmt.Errorf("failed to get events: %w", err)
		}

		// Convert to Starlark list of dicts
		result := make([]starlark.Value, len(events))
		for i, event := range events {
			eventDict := starlark.NewDict(5)
			eventDict.SetKey(starlark.String("id"), starlark.String(event.ID))
			eventDict.SetKey(starlark.String("type"), starlark.String(string(event.Type)))
			eventDict.SetKey(starlark.String("content"), starlark.String(event.Content))
			if event.ToolCallID != nil {
				eventDict.SetKey(starlark.String("tool_call_id"), starlark.String(*event.ToolCallID))
			}
			result[i] = eventDict
		}

		return starlark.NewList(result), nil
	})
}

// markObsoleteMethod marks events as obsolete
func (m *SessionModule) markObsoleteMethod() *starlark.Builtin {
	return starlark.NewBuiltin("mark_obsolete", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if m.currentSession == nil {
			return nil, fmt.Errorf("no active session")
		}

		var eventIDsList *starlark.List
		if err := starlark.UnpackArgs("mark_obsolete", args, kwargs, "event_ids", &eventIDsList); err != nil {
			return nil, err
		}

		// Convert Starlark list to Go slice
		eventIDs := make([]string, eventIDsList.Len())
		iter := eventIDsList.Iterate()
		defer iter.Done()
		var i int
		var val starlark.Value
		for iter.Next(&val) {
			eventID, ok := starlark.AsString(val)
			if !ok {
				return nil, fmt.Errorf("event_ids must be a list of strings")
			}
			eventIDs[i] = eventID
			i++
		}

		ctx := context.Background()
		if err := m.sessionService.MarkEventsObsolete(ctx, eventIDs); err != nil {
			return nil, fmt.Errorf("failed to mark events obsolete: %w", err)
		}

		return starlark.None, nil
	})
}

// insertSummaryMethod inserts a summary event
func (m *SessionModule) insertSummaryMethod() *starlark.Builtin {
	return starlark.NewBuiltin("insert_summary", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if m.currentSession == nil {
			return nil, fmt.Errorf("no active session")
		}

		var afterEventID, content string
		if err := starlark.UnpackArgs("insert_summary", args, kwargs, "after_event_id", &afterEventID, "content", &content); err != nil {
			return nil, err
		}

		ctx := context.Background()
		if err := m.sessionService.InsertSummary(ctx, m.currentSession.ID, afterEventID, content); err != nil {
			return nil, fmt.Errorf("failed to insert summary: %w", err)
		}

		return starlark.None, nil
	})
}

// listAllMethod retrieves all sessions with filters (global query)
func (m *SessionModule) listAllMethod() *starlark.Builtin {
	return starlark.NewBuiltin("list_all", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var toolName string
		var status string
		var limit int = 0

		if err := starlark.UnpackArgs("list_all", args, kwargs,
			"tool_name?", &toolName,
			"status?", &status,
			"limit?", &limit,
		); err != nil {
			return nil, err
		}

		filter := &session.SessionFilter{}
		if toolName != "" {
			filter.ToolName = &toolName
		}
		if status != "" {
			sessionStatus := session.SessionStatus(status)
			filter.Status = &sessionStatus
		}
		if limit > 0 {
			filter.Limit = limit
		}

		ctx := context.Background()
		sessions, err := m.sessionService.ListSessions(ctx, filter)
		if err != nil {
			return nil, fmt.Errorf("failed to list sessions: %w", err)
		}

		// Convert to Starlark list of dicts
		result := make([]starlark.Value, len(sessions))
		for i, sess := range sessions {
			sessDict := starlark.NewDict(5)
			sessDict.SetKey(starlark.String("id"), starlark.String(sess.ID))
			sessDict.SetKey(starlark.String("tool_name"), starlark.String(sess.ToolName))
			sessDict.SetKey(starlark.String("status"), starlark.String(string(sess.Status)))
			if sess.ParentID != nil {
				sessDict.SetKey(starlark.String("parent_id"), starlark.String(*sess.ParentID))
			}
			sessDict.SetKey(starlark.String("created_at"), starlark.String(sess.CreatedAt.Format("2006-01-02T15:04:05Z")))
			result[i] = sessDict
		}

		return starlark.NewList(result), nil
	})
}

const systemPromptMetadataKey = "__system_prompt__"

// setSystemMethod sets the system prompt for the current session.
func (m *SessionModule) setSystemMethod() *starlark.Builtin {
	return starlark.NewBuiltin("set_system", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if m.currentSession == nil {
			return nil, fmt.Errorf("no active session")
		}

		var prompt string
		if err := starlark.UnpackArgs("set_system", args, kwargs, "prompt", &prompt); err != nil {
			return nil, err
		}

		ctx := context.Background()
		if err := m.sessionService.SetMetadata(ctx, m.currentSession.ID, systemPromptMetadataKey, prompt); err != nil {
			return nil, fmt.Errorf("failed to set system prompt: %w", err)
		}

		return starlark.None, nil
	})
}

// getSystemMethod retrieves the system prompt for the current session.
// Returns None if no system prompt has been set.
func (m *SessionModule) getSystemMethod() *starlark.Builtin {
	return starlark.NewBuiltin("get_system", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if m.currentSession == nil {
			return nil, fmt.Errorf("no active session")
		}

		ctx := context.Background()
		value, err := m.sessionService.GetMetadata(ctx, m.currentSession.ID, systemPromptMetadataKey)
		if err != nil {
			return starlark.None, nil // Return None if not set
		}

		return starlark.String(value), nil
	})
}

// getByIDMethod retrieves a session by ID (global query)
func (m *SessionModule) getByIDMethod() *starlark.Builtin {
	return starlark.NewBuiltin("get_by_id", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var sessionID string
		if err := starlark.UnpackArgs("get_by_id", args, kwargs, "session_id", &sessionID); err != nil {
			return nil, err
		}

		ctx := context.Background()
		sess, err := m.sessionService.GetSession(ctx, sessionID)
		if err != nil {
			return starlark.None, nil // Return None if not found
		}

		// Convert to Starlark dict
		sessDict := starlark.NewDict(5)
		sessDict.SetKey(starlark.String("id"), starlark.String(sess.ID))
		sessDict.SetKey(starlark.String("tool_name"), starlark.String(sess.ToolName))
		sessDict.SetKey(starlark.String("status"), starlark.String(string(sess.Status)))
		if sess.ParentID != nil {
			sessDict.SetKey(starlark.String("parent_id"), starlark.String(*sess.ParentID))
		}
		sessDict.SetKey(starlark.String("created_at"), starlark.String(sess.CreatedAt.Format("2006-01-02T15:04:05Z")))

		return sessDict, nil
	})
}
