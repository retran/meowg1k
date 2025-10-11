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

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	logsSubDir = ".meowg1k/logs"
)

// WorkspaceResolver resolves the workspace root directory.
type WorkspaceResolver interface {
	Get() (string, error)
}

// Logger provides trace logging for a single session.
type Logger struct {
	workspaceResolver WorkspaceResolver
	file              *os.File
	encoder           *json.Encoder
	mu                sync.Mutex
	disabled          bool
}

// NewLogger creates a new trace logger instance.
// The log file is created lazily on the first log entry.
func NewLogger(workspaceResolver WorkspaceResolver) *Logger {
	return &Logger{
		workspaceResolver: workspaceResolver,
	}
}

// NewDisabledLogger creates a logger that does nothing (for testing or when logging is disabled).
func NewDisabledLogger() *Logger {
	return &Logger{
		disabled: true,
	}
}

// ensureLogFile ensures the log file is created and ready for writing.
func (l *Logger) ensureLogFile() error {
	if l.file != nil {
		return nil
	}

	if l.workspaceResolver == nil {
		return fmt.Errorf("workspace resolver is nil")
	}

	workspaceDir, err := l.workspaceResolver.Get()
	if err != nil {
		return fmt.Errorf("failed to get workspace directory: %w", err)
	}

	logsDir := filepath.Join(workspaceDir, logsSubDir)
	if err := os.MkdirAll(logsDir, 0o750); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Generate unique filename with timestamp and random suffix
	timestamp := time.Now().Format("20060102_150405")
	randomBytes := make([]byte, 3)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Errorf("failed to generate random suffix: %w", err)
	}
	randomSuffix := hex.EncodeToString(randomBytes)

	filename := fmt.Sprintf("%s_%s.log.jsonl", timestamp, randomSuffix)
	logPath := filepath.Join(logsDir, filename)

	// #nosec G304 -- logPath is constructed from workspace dir (validated), fixed subdir, timestamp, and random suffix
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	l.file = file
	l.encoder = json.NewEncoder(file)

	return nil
}

// LogAPIInteraction logs an LLM API interaction.
func (l *Logger) LogAPIInteraction(entry *APIInteractionEntry) error {
	if l.disabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.ensureLogFile(); err != nil {
		return err
	}

	entry.LogEntryType = LogEntryTypeAPIInteraction
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	return l.encoder.Encode(entry)
}

// LogExecutionEvent logs an executor framework event.
func (l *Logger) LogExecutionEvent(entry *ExecutionEventEntry) error {
	if l.disabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.ensureLogFile(); err != nil {
		return err
	}

	entry.LogEntryType = LogEntryTypeExecutionEvent
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	return l.encoder.Encode(entry)
}

// LogApplicationError logs a critical application error.
func (l *Logger) LogApplicationError(entry *ApplicationErrorEntry) error {
	if l.disabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.ensureLogFile(); err != nil {
		return err
	}

	entry.LogEntryType = LogEntryTypeApplicationError
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	return l.encoder.Encode(entry)
}

// Close closes the log file.
func (l *Logger) Close() error {
	if l.disabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		if err := l.file.Close(); err != nil {
			return fmt.Errorf("failed to close log file: %w", err)
		}
		l.file = nil
		l.encoder = nil
	}

	return nil
}
