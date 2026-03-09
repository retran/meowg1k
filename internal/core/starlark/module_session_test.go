// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"

	domainsession "github.com/retran/meowg1k/internal/domain/session"
)

// ---------------------------------------------------------------------------
// Extended mockSessionService (full-featured for session module tests)
// ---------------------------------------------------------------------------

// fullMockSessionService is a more complete mock that tracks all operations.
type fullMockSessionService struct {
	metaErr   error
	eventsErr error
	listErr   error
	sessions  map[string]*domainsession.Session
	events    map[string][]*domainsession.Event
	metadata  map[string]map[string]string
	completed []string
	failed    []string
}

func newFullMockSessionService() *fullMockSessionService {
	return &fullMockSessionService{
		sessions: make(map[string]*domainsession.Session),
		events:   make(map[string][]*domainsession.Event),
		metadata: make(map[string]map[string]string),
	}
}

func (s *fullMockSessionService) addSession(sess *domainsession.Session) {
	s.sessions[sess.ID] = sess
	if s.metadata[sess.ID] == nil {
		s.metadata[sess.ID] = make(map[string]string)
	}
}

func (s *fullMockSessionService) CreateSession(_ context.Context, parentID *string, toolName string) (*domainsession.Session, error) {
	id := fmt.Sprintf("sess-%d", time.Now().UnixNano())
	sess := &domainsession.Session{
		ID:        id,
		ParentID:  parentID,
		ToolName:  toolName,
		Status:    domainsession.SessionStatusRunning,
		CreatedAt: time.Now(),
	}
	s.sessions[id] = sess
	s.metadata[id] = make(map[string]string)
	return sess, nil
}

func (s *fullMockSessionService) GetSession(_ context.Context, id string) (*domainsession.Session, error) {
	sess, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %q not found", id)
	}
	return sess, nil
}

func (s *fullMockSessionService) ListSessions(_ context.Context, filter *domainsession.Filter) ([]*domainsession.Session, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	result := make([]*domainsession.Session, 0)
	for _, sess := range s.sessions {
		if filter != nil {
			if filter.ToolName != nil && sess.ToolName != *filter.ToolName {
				continue
			}
			if filter.Status != nil && sess.Status != *filter.Status {
				continue
			}
			if filter.Limit > 0 && len(result) >= filter.Limit {
				break
			}
		}
		result = append(result, sess)
	}
	return result, nil
}

func (s *fullMockSessionService) GetChildSessions(_ context.Context, parentID string) ([]*domainsession.Session, error) {
	result := make([]*domainsession.Session, 0)
	for _, sess := range s.sessions {
		if sess.ParentID != nil && *sess.ParentID == parentID {
			result = append(result, sess)
		}
	}
	return result, nil
}

func (s *fullMockSessionService) CompleteSession(_ context.Context, id string) error {
	s.completed = append(s.completed, id)
	if sess, ok := s.sessions[id]; ok {
		sess.Status = domainsession.SessionStatusCompleted
	}
	return nil
}

func (s *fullMockSessionService) FailSession(_ context.Context, id string) error {
	s.failed = append(s.failed, id)
	if sess, ok := s.sessions[id]; ok {
		sess.Status = domainsession.SessionStatusFailed
	}
	return nil
}

func (s *fullMockSessionService) AddUserMessage(_ context.Context, sessionID, content string) error {
	event := &domainsession.Event{
		ID:        fmt.Sprintf("ev-%d", time.Now().UnixNano()),
		SessionID: sessionID,
		Type:      domainsession.EventTypeUserMessage,
		Content:   content,
	}
	s.events[sessionID] = append(s.events[sessionID], event)
	return nil
}

func (s *fullMockSessionService) AddAssistantMessage(_ context.Context, sessionID, content string, toolCalls []domainsession.ToolCall) error {
	event := &domainsession.Event{
		ID:        fmt.Sprintf("ev-%d", time.Now().UnixNano()),
		SessionID: sessionID,
		Type:      domainsession.EventTypeAssistantMessage,
		Content:   content,
		ToolCalls: toolCalls,
	}
	s.events[sessionID] = append(s.events[sessionID], event)
	return nil
}

func (s *fullMockSessionService) AddToolResult(_ context.Context, sessionID, toolCallID, content string) error {
	event := &domainsession.Event{
		ID:         fmt.Sprintf("ev-%d", time.Now().UnixNano()),
		SessionID:  sessionID,
		Type:       domainsession.EventTypeToolResult,
		Content:    content,
		ToolCallID: &toolCallID,
	}
	s.events[sessionID] = append(s.events[sessionID], event)
	return nil
}

func (s *fullMockSessionService) AddSystemMessage(_ context.Context, sessionID, content string) error {
	event := &domainsession.Event{
		ID:        fmt.Sprintf("ev-%d", time.Now().UnixNano()),
		SessionID: sessionID,
		Type:      domainsession.EventTypeSystem,
		Content:   content,
	}
	s.events[sessionID] = append(s.events[sessionID], event)
	return nil
}

func (s *fullMockSessionService) GetEvents(_ context.Context, sessionID string, limit, offset int) ([]*domainsession.Event, error) {
	if s.eventsErr != nil {
		return nil, s.eventsErr
	}
	all := s.events[sessionID]
	if offset >= len(all) {
		return nil, nil
	}
	end := offset + limit
	if end > len(all) || limit == 0 {
		end = len(all)
	}
	return all[offset:end], nil
}

func (s *fullMockSessionService) GetAllEvents(_ context.Context, sessionID string) ([]*domainsession.Event, error) {
	if s.eventsErr != nil {
		return nil, s.eventsErr
	}
	return s.events[sessionID], nil
}

func (s *fullMockSessionService) MarkEventsObsolete(_ context.Context, eventIDs []string) error {
	for _, id := range eventIDs {
		for _, evList := range s.events {
			for _, ev := range evList {
				if ev.ID == id {
					ev.Obsolete = true
				}
			}
		}
	}
	return nil
}

func (s *fullMockSessionService) InsertSummary(_ context.Context, sessionID, _, content string) error {
	event := &domainsession.Event{
		ID:        fmt.Sprintf("summary-%d", time.Now().UnixNano()),
		SessionID: sessionID,
		Type:      domainsession.EventTypeSystem,
		Content:   content,
	}
	s.events[sessionID] = append(s.events[sessionID], event)
	return nil
}

func (s *fullMockSessionService) SetMetadata(_ context.Context, sessionID, key, value string) error {
	if s.metaErr != nil {
		return s.metaErr
	}
	if s.metadata[sessionID] == nil {
		s.metadata[sessionID] = make(map[string]string)
	}
	s.metadata[sessionID][key] = value
	return nil
}

func (s *fullMockSessionService) GetMetadata(_ context.Context, sessionID, key string) (string, error) {
	if s.metaErr != nil {
		return "", s.metaErr
	}
	m := s.metadata[sessionID]
	if m == nil {
		return "", fmt.Errorf("not found")
	}
	v, ok := m[key]
	if !ok {
		return "", fmt.Errorf("key %q not found", key)
	}
	return v, nil
}

func (s *fullMockSessionService) GetAllMetadata(_ context.Context, sessionID string) (map[string]string, error) {
	if s.metaErr != nil {
		return nil, s.metaErr
	}
	if m := s.metadata[sessionID]; m != nil {
		return m, nil
	}
	return map[string]string{}, nil
}

func (s *fullMockSessionService) GetChildMetadata(_ context.Context, _, childID, key string) (string, error) {
	m := s.metadata[childID]
	if m == nil {
		return "", fmt.Errorf("child session not found")
	}
	v, ok := m[key]
	if !ok {
		return "", fmt.Errorf("key not found")
	}
	return v, nil
}

// ---------------------------------------------------------------------------
// Helper: build a session module with a live session
// ---------------------------------------------------------------------------

func makeSessionModule(svc *fullMockSessionService, sess *domainsession.Session) starlark.Value {
	return NewSessionModule(svc, sess)
}

func callSessionMethod(t *testing.T, mod starlark.Value, method string, kwargs []starlark.Tuple) (starlark.Value, error) {
	t.Helper()
	fn := getAttr(t, mod, method)
	thread := &starlark.Thread{Name: "test"}
	return starlark.Call(thread, fn, starlark.Tuple{}, kwargs)
}

// ---------------------------------------------------------------------------
// NewSessionModule – creation
// ---------------------------------------------------------------------------

func TestNewSessionModule_HasExpectedMethods(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "s1", ToolName: "test-tool", Status: domainsession.SessionStatusRunning}
	mod := makeSessionModule(svc, sess)

	expectedMethods := []string{
		"id", "tool_name", "parent_id", "status",
		"set_metadata", "get_metadata", "get_all_metadata",
		"get_children", "get_child_metadata",
		"get_events", "mark_obsolete", "insert_summary",
		"list_all", "get_by_id",
		"set_system", "get_system",
	}

	type attrNamer interface {
		AttrNames() []string
	}
	an, ok := mod.(attrNamer)
	require.True(t, ok)

	nameSet := make(map[string]bool)
	for _, n := range an.AttrNames() {
		nameSet[n] = true
	}

	for _, m := range expectedMethods {
		assert.True(t, nameSet[m], "module should expose method %q", m)
	}
}

// ---------------------------------------------------------------------------
// id, tool_name, parent_id, status
// ---------------------------------------------------------------------------

func TestSessionModule_ID(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "session-abc", ToolName: "mytool", Status: domainsession.SessionStatusRunning}
	mod := makeSessionModule(svc, sess)

	val, err := callSessionMethod(t, mod, "id", nil)
	require.NoError(t, err)
	assert.Equal(t, starlark.String("session-abc"), val)
}

func TestSessionModule_ID_NilSession(t *testing.T) {
	svc := newFullMockSessionService()
	mod := makeSessionModule(svc, nil)

	val, err := callSessionMethod(t, mod, "id", nil)
	require.NoError(t, err)
	assert.Equal(t, starlark.None, val)
}

func TestSessionModule_ToolName(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "s1", ToolName: "my-awesome-tool", Status: domainsession.SessionStatusRunning}
	mod := makeSessionModule(svc, sess)

	val, err := callSessionMethod(t, mod, "tool_name", nil)
	require.NoError(t, err)
	assert.Equal(t, starlark.String("my-awesome-tool"), val)
}

func TestSessionModule_ToolName_NilSession(t *testing.T) {
	svc := newFullMockSessionService()
	mod := makeSessionModule(svc, nil)

	val, err := callSessionMethod(t, mod, "tool_name", nil)
	require.NoError(t, err)
	assert.Equal(t, starlark.None, val)
}

func TestSessionModule_ParentID_WithParent(t *testing.T) {
	svc := newFullMockSessionService()
	parentID := "parent-123"
	sess := &domainsession.Session{ID: "s1", ParentID: &parentID, Status: domainsession.SessionStatusRunning}
	mod := makeSessionModule(svc, sess)

	val, err := callSessionMethod(t, mod, "parent_id", nil)
	require.NoError(t, err)
	assert.Equal(t, starlark.String("parent-123"), val)
}

func TestSessionModule_ParentID_NoParent(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "s1", ParentID: nil, Status: domainsession.SessionStatusRunning}
	mod := makeSessionModule(svc, sess)

	val, err := callSessionMethod(t, mod, "parent_id", nil)
	require.NoError(t, err)
	assert.Equal(t, starlark.None, val)
}

func TestSessionModule_ParentID_NilSession(t *testing.T) {
	svc := newFullMockSessionService()
	mod := makeSessionModule(svc, nil)

	val, err := callSessionMethod(t, mod, "parent_id", nil)
	require.NoError(t, err)
	assert.Equal(t, starlark.None, val)
}

func TestSessionModule_Status(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "s1", Status: domainsession.SessionStatusRunning}
	mod := makeSessionModule(svc, sess)

	val, err := callSessionMethod(t, mod, "status", nil)
	require.NoError(t, err)
	assert.Equal(t, starlark.String("running"), val)
}

func TestSessionModule_Status_NilSession(t *testing.T) {
	svc := newFullMockSessionService()
	mod := makeSessionModule(svc, nil)

	val, err := callSessionMethod(t, mod, "status", nil)
	require.NoError(t, err)
	assert.Equal(t, starlark.None, val)
}

// ---------------------------------------------------------------------------
// set_metadata / get_metadata / get_all_metadata
// ---------------------------------------------------------------------------

func TestSessionModule_SetMetadata(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "s1"}
	svc.addSession(sess)
	mod := makeSessionModule(svc, sess)

	_, err := callSessionMethod(t, mod, "set_metadata", []starlark.Tuple{
		{starlark.String("key"), starlark.String("mykey")},
		{starlark.String("value"), starlark.String("myvalue")},
	})
	require.NoError(t, err)
	assert.Equal(t, "myvalue", svc.metadata["s1"]["mykey"])
}

func TestSessionModule_SetMetadata_NilSession(t *testing.T) {
	svc := newFullMockSessionService()
	mod := makeSessionModule(svc, nil)

	_, err := callSessionMethod(t, mod, "set_metadata", []starlark.Tuple{
		{starlark.String("key"), starlark.String("k")},
		{starlark.String("value"), starlark.String("v")},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
}

func TestSessionModule_GetMetadata(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "s1"}
	svc.addSession(sess)
	svc.metadata["s1"]["thekey"] = "thevalue"
	mod := makeSessionModule(svc, sess)

	val, err := callSessionMethod(t, mod, "get_metadata", []starlark.Tuple{
		{starlark.String("key"), starlark.String("thekey")},
	})
	require.NoError(t, err)
	assert.Equal(t, starlark.String("thevalue"), val)
}

func TestSessionModule_GetMetadata_NotFound_ReturnsNone(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "s1"}
	svc.addSession(sess)
	mod := makeSessionModule(svc, sess)

	val, err := callSessionMethod(t, mod, "get_metadata", []starlark.Tuple{
		{starlark.String("key"), starlark.String("nonexistent")},
	})
	require.NoError(t, err)
	assert.Equal(t, starlark.None, val)
}

func TestSessionModule_GetMetadata_NilSession(t *testing.T) {
	svc := newFullMockSessionService()
	mod := makeSessionModule(svc, nil)

	_, err := callSessionMethod(t, mod, "get_metadata", []starlark.Tuple{
		{starlark.String("key"), starlark.String("k")},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
}

func TestSessionModule_GetAllMetadata(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "s1"}
	svc.addSession(sess)
	svc.metadata["s1"]["alpha"] = "1"
	svc.metadata["s1"]["beta"] = "2"
	mod := makeSessionModule(svc, sess)

	val, err := callSessionMethod(t, mod, "get_all_metadata", nil)
	require.NoError(t, err)

	d, ok := val.(*starlark.Dict)
	require.True(t, ok)
	assert.Equal(t, 2, d.Len())

	alphaVal, _, _ := d.Get(starlark.String("alpha"))
	assert.Equal(t, starlark.String("1"), alphaVal)
}

func TestSessionModule_GetAllMetadata_NilSession(t *testing.T) {
	svc := newFullMockSessionService()
	mod := makeSessionModule(svc, nil)

	_, err := callSessionMethod(t, mod, "get_all_metadata", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
}

func TestSessionModule_GetAllMetadata_ServiceError(t *testing.T) {
	svc := newFullMockSessionService()
	svc.metaErr = fmt.Errorf("db failure")
	sess := &domainsession.Session{ID: "s1"}
	mod := makeSessionModule(svc, sess)

	_, err := callSessionMethod(t, mod, "get_all_metadata", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get metadata")
}

// ---------------------------------------------------------------------------
// get_children
// ---------------------------------------------------------------------------

func TestSessionModule_GetChildren(t *testing.T) {
	svc := newFullMockSessionService()
	parentSess := &domainsession.Session{ID: "parent"}
	svc.addSession(parentSess)

	parentID := "parent"
	child1 := &domainsession.Session{ID: "child1", ParentID: &parentID, ToolName: "tool1", Status: domainsession.SessionStatusCompleted}
	child2 := &domainsession.Session{ID: "child2", ParentID: &parentID, ToolName: "tool2", Status: domainsession.SessionStatusRunning}
	svc.addSession(child1)
	svc.addSession(child2)

	mod := makeSessionModule(svc, parentSess)
	val, err := callSessionMethod(t, mod, "get_children", nil)
	require.NoError(t, err)

	list, ok := val.(*starlark.List)
	require.True(t, ok)
	assert.Equal(t, 2, list.Len())
}

func TestSessionModule_GetChildren_NilSession(t *testing.T) {
	svc := newFullMockSessionService()
	mod := makeSessionModule(svc, nil)

	_, err := callSessionMethod(t, mod, "get_children", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
}

// ---------------------------------------------------------------------------
// get_child_metadata
// ---------------------------------------------------------------------------

func TestSessionModule_GetChildMetadata(t *testing.T) {
	svc := newFullMockSessionService()
	parentSess := &domainsession.Session{ID: "parent"}
	childSess := &domainsession.Session{ID: "child"}
	svc.addSession(parentSess)
	svc.addSession(childSess)
	svc.metadata["child"]["info"] = "child-value"

	mod := makeSessionModule(svc, parentSess)
	val, err := callSessionMethod(t, mod, "get_child_metadata", []starlark.Tuple{
		{starlark.String("child_id"), starlark.String("child")},
		{starlark.String("key"), starlark.String("info")},
	})
	require.NoError(t, err)
	assert.Equal(t, starlark.String("child-value"), val)
}

func TestSessionModule_GetChildMetadata_NotFound_ReturnsNone(t *testing.T) {
	svc := newFullMockSessionService()
	parentSess := &domainsession.Session{ID: "parent"}
	svc.addSession(parentSess)

	mod := makeSessionModule(svc, parentSess)
	val, err := callSessionMethod(t, mod, "get_child_metadata", []starlark.Tuple{
		{starlark.String("child_id"), starlark.String("nonexistent-child")},
		{starlark.String("key"), starlark.String("k")},
	})
	require.NoError(t, err)
	assert.Equal(t, starlark.None, val)
}

func TestSessionModule_GetChildMetadata_NilSession(t *testing.T) {
	svc := newFullMockSessionService()
	mod := makeSessionModule(svc, nil)

	_, err := callSessionMethod(t, mod, "get_child_metadata", []starlark.Tuple{
		{starlark.String("child_id"), starlark.String("c")},
		{starlark.String("key"), starlark.String("k")},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
}

// ---------------------------------------------------------------------------
// get_events
// ---------------------------------------------------------------------------

func TestSessionModule_GetEvents(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "s1"}
	svc.addSession(sess)

	// Add some events manually
	svc.events["s1"] = []*domainsession.Event{
		{ID: "e1", SessionID: "s1", Type: domainsession.EventTypeUserMessage, Content: "hello"},
		{ID: "e2", SessionID: "s1", Type: domainsession.EventTypeAssistantMessage, Content: "world"},
	}

	mod := makeSessionModule(svc, sess)
	val, err := callSessionMethod(t, mod, "get_events", []starlark.Tuple{
		{starlark.String("limit"), starlark.MakeInt(10)},
	})
	require.NoError(t, err)

	list, ok := val.(*starlark.List)
	require.True(t, ok)
	assert.Equal(t, 2, list.Len())
}

func TestSessionModule_GetEvents_WithToolCallID(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "s1"}
	svc.addSession(sess)

	toolCallID := "tc-1"
	svc.events["s1"] = []*domainsession.Event{
		{ID: "e1", SessionID: "s1", Type: domainsession.EventTypeToolResult, Content: "result", ToolCallID: &toolCallID},
	}

	mod := makeSessionModule(svc, sess)
	val, err := callSessionMethod(t, mod, "get_events", nil)
	require.NoError(t, err)

	list, ok := val.(*starlark.List)
	require.True(t, ok)
	require.Equal(t, 1, list.Len())

	item, _ := list.Index(0).(*starlark.Dict)
	require.NotNil(t, item)
	tcIDVal, _, _ := item.Get(starlark.String("tool_call_id"))
	assert.Equal(t, starlark.String("tc-1"), tcIDVal)
}

func TestSessionModule_GetEvents_NilSession(t *testing.T) {
	svc := newFullMockSessionService()
	mod := makeSessionModule(svc, nil)

	_, err := callSessionMethod(t, mod, "get_events", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
}

func TestSessionModule_GetEvents_ServiceError(t *testing.T) {
	svc := newFullMockSessionService()
	svc.eventsErr = fmt.Errorf("events db failure")
	sess := &domainsession.Session{ID: "s1"}
	mod := makeSessionModule(svc, sess)

	_, err := callSessionMethod(t, mod, "get_events", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get events")
}

// ---------------------------------------------------------------------------
// mark_obsolete
// ---------------------------------------------------------------------------

func TestSessionModule_MarkObsolete(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "s1"}
	svc.addSession(sess)
	svc.events["s1"] = []*domainsession.Event{
		{ID: "ev-to-delete", SessionID: "s1", Type: domainsession.EventTypeUserMessage, Content: "old"},
	}

	mod := makeSessionModule(svc, sess)
	thread := &starlark.Thread{Name: "test"}
	fn := getAttr(t, mod, "mark_obsolete")

	eventIDsList := starlark.NewList([]starlark.Value{starlark.String("ev-to-delete")})
	_, err := starlark.Call(thread, fn, starlark.Tuple{}, []starlark.Tuple{
		{starlark.String("event_ids"), eventIDsList},
	})
	require.NoError(t, err)

	// Verify the event was marked obsolete
	assert.True(t, svc.events["s1"][0].Obsolete)
}

func TestSessionModule_MarkObsolete_NilSession(t *testing.T) {
	svc := newFullMockSessionService()
	mod := makeSessionModule(svc, nil)
	thread := &starlark.Thread{Name: "test"}
	fn := getAttr(t, mod, "mark_obsolete")

	eventIDsList := starlark.NewList([]starlark.Value{starlark.String("ev1")})
	_, err := starlark.Call(thread, fn, starlark.Tuple{}, []starlark.Tuple{
		{starlark.String("event_ids"), eventIDsList},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
}

// ---------------------------------------------------------------------------
// insert_summary
// ---------------------------------------------------------------------------

func TestSessionModule_InsertSummary(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "s1"}
	svc.addSession(sess)
	svc.events["s1"] = []*domainsession.Event{
		{ID: "ev1", SessionID: "s1", Type: domainsession.EventTypeUserMessage, Content: "hello"},
	}

	mod := makeSessionModule(svc, sess)
	_, err := callSessionMethod(t, mod, "insert_summary", []starlark.Tuple{
		{starlark.String("after_event_id"), starlark.String("ev1")},
		{starlark.String("content"), starlark.String("Summary: one user message")},
	})
	require.NoError(t, err)

	// A summary event should have been inserted
	events := svc.events["s1"]
	assert.Equal(t, 2, len(events))
}

func TestSessionModule_InsertSummary_NilSession(t *testing.T) {
	svc := newFullMockSessionService()
	mod := makeSessionModule(svc, nil)

	_, err := callSessionMethod(t, mod, "insert_summary", []starlark.Tuple{
		{starlark.String("after_event_id"), starlark.String("ev1")},
		{starlark.String("content"), starlark.String("summary")},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
}

// ---------------------------------------------------------------------------
// list_all
// ---------------------------------------------------------------------------

func TestSessionModule_ListAll(t *testing.T) {
	svc := newFullMockSessionService()
	sess1 := &domainsession.Session{ID: "s1", ToolName: "tool-a", Status: domainsession.SessionStatusRunning, CreatedAt: time.Now()}
	sess2 := &domainsession.Session{ID: "s2", ToolName: "tool-b", Status: domainsession.SessionStatusCompleted, CreatedAt: time.Now()}
	svc.addSession(sess1)
	svc.addSession(sess2)

	// list_all is a global query – does not require a currentSession
	mod := makeSessionModule(svc, nil)
	val, err := callSessionMethod(t, mod, "list_all", nil)
	require.NoError(t, err)

	list, ok := val.(*starlark.List)
	require.True(t, ok)
	assert.Equal(t, 2, list.Len())
}

func TestSessionModule_ListAll_FilterByToolName(t *testing.T) {
	svc := newFullMockSessionService()
	sess1 := &domainsession.Session{ID: "s1", ToolName: "alpha", Status: domainsession.SessionStatusRunning, CreatedAt: time.Now()}
	sess2 := &domainsession.Session{ID: "s2", ToolName: "beta", Status: domainsession.SessionStatusRunning, CreatedAt: time.Now()}
	svc.addSession(sess1)
	svc.addSession(sess2)

	mod := makeSessionModule(svc, nil)
	val, err := callSessionMethod(t, mod, "list_all", []starlark.Tuple{
		{starlark.String("tool_name"), starlark.String("alpha")},
	})
	require.NoError(t, err)

	list, ok := val.(*starlark.List)
	require.True(t, ok)
	assert.Equal(t, 1, list.Len())
}

func TestSessionModule_ListAll_ServiceError(t *testing.T) {
	svc := newFullMockSessionService()
	svc.listErr = fmt.Errorf("list failure")
	mod := makeSessionModule(svc, nil)

	_, err := callSessionMethod(t, mod, "list_all", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list sessions")
}

// ---------------------------------------------------------------------------
// get_by_id
// ---------------------------------------------------------------------------

func TestSessionModule_GetByID(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "target-sess", ToolName: "mytool", Status: domainsession.SessionStatusCompleted, CreatedAt: time.Now()}
	svc.addSession(sess)

	mod := makeSessionModule(svc, nil)
	val, err := callSessionMethod(t, mod, "get_by_id", []starlark.Tuple{
		{starlark.String("session_id"), starlark.String("target-sess")},
	})
	require.NoError(t, err)

	d, ok := val.(*starlark.Dict)
	require.True(t, ok)

	idVal, _, _ := d.Get(starlark.String("id"))
	assert.Equal(t, starlark.String("target-sess"), idVal)

	statusVal, _, _ := d.Get(starlark.String("status"))
	assert.Equal(t, starlark.String("completed"), statusVal)
}

func TestSessionModule_GetByID_NotFound_ReturnsNone(t *testing.T) {
	svc := newFullMockSessionService()
	mod := makeSessionModule(svc, nil)

	val, err := callSessionMethod(t, mod, "get_by_id", []starlark.Tuple{
		{starlark.String("session_id"), starlark.String("does-not-exist")},
	})
	require.NoError(t, err)
	assert.Equal(t, starlark.None, val)
}

func TestSessionModule_GetByID_WithParentID(t *testing.T) {
	svc := newFullMockSessionService()
	parentID := "parent-sess"
	sess := &domainsession.Session{ID: "child-sess", ParentID: &parentID, ToolName: "child-tool", Status: domainsession.SessionStatusRunning, CreatedAt: time.Now()}
	svc.addSession(sess)

	mod := makeSessionModule(svc, nil)
	val, err := callSessionMethod(t, mod, "get_by_id", []starlark.Tuple{
		{starlark.String("session_id"), starlark.String("child-sess")},
	})
	require.NoError(t, err)

	d, ok := val.(*starlark.Dict)
	require.True(t, ok)

	parentIDVal, _, _ := d.Get(starlark.String("parent_id"))
	assert.Equal(t, starlark.String("parent-sess"), parentIDVal)
}

// ---------------------------------------------------------------------------
// set_system / get_system
// ---------------------------------------------------------------------------

func TestSessionModule_SetSystem(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "s1"}
	svc.addSession(sess)
	mod := makeSessionModule(svc, sess)

	_, err := callSessionMethod(t, mod, "set_system", []starlark.Tuple{
		{starlark.String("prompt"), starlark.String("You are a helpful assistant.")},
	})
	require.NoError(t, err)

	// Verify the metadata was stored under the system prompt key
	assert.Equal(t, "You are a helpful assistant.", svc.metadata["s1"][systemPromptMetadataKey])
}

func TestSessionModule_SetSystem_NilSession(t *testing.T) {
	svc := newFullMockSessionService()
	mod := makeSessionModule(svc, nil)

	_, err := callSessionMethod(t, mod, "set_system", []starlark.Tuple{
		{starlark.String("prompt"), starlark.String("system prompt")},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
}

func TestSessionModule_GetSystem(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "s1"}
	svc.addSession(sess)
	svc.metadata["s1"][systemPromptMetadataKey] = "Be concise."
	mod := makeSessionModule(svc, sess)

	val, err := callSessionMethod(t, mod, "get_system", nil)
	require.NoError(t, err)
	assert.Equal(t, starlark.String("Be concise."), val)
}

func TestSessionModule_GetSystem_NotSet_ReturnsNone(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "s1"}
	svc.addSession(sess)
	mod := makeSessionModule(svc, sess)

	val, err := callSessionMethod(t, mod, "get_system", nil)
	require.NoError(t, err)
	assert.Equal(t, starlark.None, val)
}

func TestSessionModule_GetSystem_NilSession(t *testing.T) {
	svc := newFullMockSessionService()
	mod := makeSessionModule(svc, nil)

	_, err := callSessionMethod(t, mod, "get_system", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active session")
}

func TestSessionModule_SetSystem_ThenGetSystem_RoundTrip(t *testing.T) {
	svc := newFullMockSessionService()
	sess := &domainsession.Session{ID: "s1"}
	svc.addSession(sess)
	mod := makeSessionModule(svc, sess)

	systemPrompt := "You are an expert Go developer."

	_, err := callSessionMethod(t, mod, "set_system", []starlark.Tuple{
		{starlark.String("prompt"), starlark.String(systemPrompt)},
	})
	require.NoError(t, err)

	val, err := callSessionMethod(t, mod, "get_system", nil)
	require.NoError(t, err)
	assert.Equal(t, starlark.String(systemPrompt), val)
}
