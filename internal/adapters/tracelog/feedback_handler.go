// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package tracelog

import "github.com/retran/meowg1k/pkg/executor"

// FeedbackHandler creates a feedback handler that logs execution events to the trace logger.
func (l *Logger) FeedbackHandler(inner executor.FeedbackHandler) executor.FeedbackHandler {
	if l.disabled {
		return inner
	}

	return func(feedback *executor.Feedback) {
		// Call the original handler first
		if inner != nil {
			inner(feedback)
		}

		// Log the execution event
		entry := &ExecutionEventEntry{
			ExecutionName: feedback.ActivityName,
			Status:        string(feedback.Status),
			Message:       feedback.Message,
			Metadata:      feedback.Metadata,
		}

		if feedback.Error != nil {
			entry.Error = feedback.Error.Error()
		}

		// Log asynchronously to avoid blocking (ignore errors)
		go l.LogExecutionEvent(entry)
	}
}
