/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
