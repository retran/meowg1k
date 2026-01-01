// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package state manages flow execution state including memory and task planning.
package state

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type key int

const flowStateKey key = 0

// Fact represents a piece of information stored in the flow memory.
type Fact struct {
	Content string
}

// TaskStatus represents the status of a task.
type TaskStatus string

const (
	// StatusPending indicates a task that has not been started.
	StatusPending TaskStatus = "pending"
	// StatusDone indicates a completed task.
	StatusDone TaskStatus = "done"
	// StatusFailed indicates a task that failed.
	StatusFailed TaskStatus = "failed"
	// StatusSkipped indicates a task that was skipped.
	StatusSkipped TaskStatus = "skipped"
)

// Task represents a unit of work in the plan.
type Task struct {
	ID          string
	Description string
	Status      TaskStatus
}

// FlowState holds the mutable state for a flow execution (Memory + Plan).
type FlowState struct {
	RestartRequest *string
	Facts          []Fact
	Tasks          []Task
	RestartCount   int
	mu             sync.RWMutex
}

// NewFlowState creates a new empty state.
func NewFlowState() *FlowState {
	return &FlowState{
		Facts:          make([]Fact, 0),
		Tasks:          make([]Task, 0),
		RestartRequest: nil,
		RestartCount:   0,
	}
}

// SetRestartRequest sets the restart instruction.
func (s *FlowState) SetRestartRequest(instruction string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RestartRequest = &instruction
}

// GetRestartRequest returns the restart instruction and clears it.
func (s *FlowState) GetRestartRequest() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.RestartRequest == nil {
		return "", false
	}
	req := *s.RestartRequest
	s.RestartRequest = nil
	return req, true
}

// IncrementRestartCount increments the restart counter.
func (s *FlowState) IncrementRestartCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RestartCount++
	return s.RestartCount
}

// ResetPlan clears the task board while keeping memory facts.
// Useful when restarting a flow so the planner can rebuild from scratch.
func (s *FlowState) ResetPlan() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Tasks = make([]Task, 0)
}

// AddFact adds a fact to the memory.
func (s *FlowState) AddFact(content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Facts = append(s.Facts, Fact{Content: content})
}

// GetFacts returns all facts.
func (s *FlowState) GetFacts() []Fact {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy
	result := make([]Fact, len(s.Facts))
	copy(result, s.Facts)
	return result
}

// SearchFacts returns facts containing the query (simple substring search for now).
func (s *FlowState) SearchFacts(query string) []Fact {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Fact
	queryLower := strings.ToLower(query)
	for _, f := range s.Facts {
		if strings.Contains(strings.ToLower(f.Content), queryLower) {
			result = append(result, f)
		}
	}
	return result
}

// SetTasks initializes or overwrites the task board.
func (s *FlowState) SetTasks(tasks []Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Tasks = tasks
}

// GetTasks returns all tasks.
func (s *FlowState) GetTasks() []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Task, len(s.Tasks))
	copy(result, s.Tasks)
	return result
}

// UpdateTaskStatus updates the status of a task by ID.
func (s *FlowState) UpdateTaskStatus(id string, status TaskStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Tasks {
		if s.Tasks[i].ID == id {
			s.Tasks[i].Status = status
			return nil
		}
	}
	return fmt.Errorf("task not found: %s", id)
}

// WithFlowState returns a context with the FlowState attached.
func WithFlowState(ctx context.Context, s *FlowState) context.Context {
	return context.WithValue(ctx, flowStateKey, s)
}

// GetFlowState retrieves the FlowState from the context.
func GetFlowState(ctx context.Context) (*FlowState, error) {
	val := ctx.Value(flowStateKey)
	if val == nil {
		return nil, fmt.Errorf("flow state not found in context")
	}
	s, ok := val.(*FlowState)
	if !ok {
		return nil, fmt.Errorf("invalid flow state type in context")
	}
	return s, nil
}
