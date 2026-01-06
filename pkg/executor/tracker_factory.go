// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package executor

// Compile-time check to ensure BubbleTeaTracker implements ProgressTracker.
var _ ProgressTracker = (*BubbleTeaTracker)(nil)

// ProgressTracker is the interface for tracking execution progress.
type ProgressTracker interface {
	Start()
	Stop()
	FeedbackHandler() FeedbackHandler
	GetExecution(name string) *Execution
	GetExecutionCount() int
}

// NewProgressTracker creates a new Bubbletea-based progress tracker.
func NewProgressTracker(silent bool) ProgressTracker {
	return NewBubbleTeaTracker(silent)
}
