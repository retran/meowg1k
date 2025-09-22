/*package shutdown

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

// Package shutdown provides utilities for graceful application shutdown.
package shutdown

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Manager handles graceful shutdown of the application.
// It listens for system signals and coordinates shutdown of all registered components.
type Manager struct {
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	logger    *slog.Logger
	callbacks []ShutdownCallback
	timeout   time.Duration
}

// ShutdownCallback is a function called during graceful shutdown.
// It should complete cleanup and return within a reasonable time.
type ShutdownCallback func(ctx context.Context) error

// NewManager creates a new shutdown manager with the specified timeout.
// timeout sets the maximum time to wait for all callbacks to complete.
func NewManager(timeout time.Duration) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		ctx:       ctx,
		cancel:    cancel,
		logger:    slog.Default(),
		callbacks: make([]ShutdownCallback, 0),
		timeout:   timeout,
	}
}

// WithLogger sets a custom logger for the shutdown manager.
func (m *Manager) WithLogger(logger *slog.Logger) *Manager {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger = logger
	return m
}

// Context returns the shutdown context.
// This context is cancelled when shutdown begins.
func (m *Manager) Context() context.Context {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ctx
}

// Register adds a callback to be executed during shutdown.
// Callbacks are executed in the order they were registered.
func (m *Manager) Register(callback ShutdownCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()

	callbackIndex := len(m.callbacks)
	m.callbacks = append(m.callbacks, callback)

	m.logger.DebugContext(m.ctx, "Registered shutdown callback",
		"callback_index", callbackIndex,
		"total_callbacks", len(m.callbacks))
}

// ListenForSignals starts listening for shutdown signals (SIGINT, SIGTERM).
// This function blocks until a signal is received or the context is cancelled.
// Returns true if shutdown was triggered by a signal, false if context was cancelled.
func (m *Manager) ListenForSignals() bool {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	m.logger.DebugContext(m.ctx, "Starting to listen for shutdown signals",
		"signals", []string{"SIGINT", "SIGTERM"},
		"timeout", m.timeout.String())

	select {
	case sig := <-sigChan:
		m.logger.InfoContext(m.ctx, "Received shutdown signal",
			"signal", sig.String(),
			"signal_number", int(sig.(syscall.Signal)))
		m.shutdown()
		return true
	case <-m.ctx.Done():
		m.logger.DebugContext(context.Background(), "Signal listener cancelled by context")
		return false
	}
}

// Shutdown triggers graceful shutdown manually.
// This can be called programmatically instead of waiting for signals.
func (m *Manager) Shutdown() {
	m.shutdown()
}

// shutdown performs the actual shutdown process.
func (m *Manager) shutdown() {
	m.logger.InfoContext(context.Background(), "Beginning graceful shutdown",
		"timeout", m.timeout.String(),
		"registered_callbacks", len(m.callbacks))

	// Cancel the main context to signal shutdown
	m.cancel()

	// Create a timeout context for the shutdown process
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), m.timeout)
	defer shutdownCancel()

	m.mu.RLock()
	callbacks := make([]ShutdownCallback, len(m.callbacks))
	copy(callbacks, m.callbacks)
	m.mu.RUnlock()

	// Execute all shutdown callbacks
	for i, callback := range callbacks {
		callbackStart := time.Now()
		m.logger.DebugContext(shutdownCtx, "Executing shutdown callback",
			"callback_index", i,
			"remaining_callbacks", len(callbacks)-i-1)

		if err := callback(shutdownCtx); err != nil {
			m.logger.ErrorContext(shutdownCtx, "Shutdown callback failed",
				"callback_index", i,
				"error", err.Error(),
				"execution_time", time.Since(callbackStart))
		} else {
			m.logger.DebugContext(shutdownCtx, "Shutdown callback completed successfully",
				"callback_index", i,
				"execution_time", time.Since(callbackStart))
		}

		// Check if we're running out of time
		select {
		case <-shutdownCtx.Done():
			m.logger.WarnContext(shutdownCtx, "Shutdown timeout reached, cancelling remaining callbacks",
				"completed_callbacks", i+1,
				"remaining_callbacks", len(callbacks)-i-1)
			return
		default:
			// Continue with next callback
		}
	}

	m.logger.InfoContext(context.Background(), "Graceful shutdown completed successfully",
		"total_callbacks", len(callbacks),
		"total_time", m.timeout.String())
}

// CreateShutdownContext creates a new context that is cancelled when shutdown begins.
// This is a convenience function for components that need shutdown-aware contexts.
func (m *Manager) CreateShutdownContext(parent context.Context) context.Context {
	// Create a context that is cancelled when either the parent or shutdown context is cancelled
	ctx, cancel := context.WithCancel(parent)

	go func() {
		select {
		case <-m.ctx.Done():
			cancel()
		case <-parent.Done():
			cancel()
		}
	}()

	return ctx
}
