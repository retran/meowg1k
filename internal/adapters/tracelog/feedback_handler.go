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
		if inner != nil {
			inner(feedback)
		}

		entry := &ExecutionEventEntry{
			ExecutionName: feedback.ActivityName,
			Status:        string(feedback.Status),
			Message:       feedback.Message,
		}

		if feedback.Error != nil {
			entry.Error = feedback.Error.Error()
		}

		go func() {
			_ = l.LogExecutionEvent(entry) //nolint:errcheck // Async logging errors are not critical
		}()
	}
}
