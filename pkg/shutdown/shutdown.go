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

// Package shutdown provides service for graceful application shutdown.
package shutdown

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Service handles graceful shutdown of the application.
// It listens for system signals and coordinates shutdown of all registered components.
type Service struct {
	mu        sync.RWMutex
	ctx       context.Context //nolint:containedctx // stored to expose shutdown context to consumers
	cancel    context.CancelFunc
	logger    *slog.Logger
	callbacks []Callback
	timeout   time.Duration
}

// Callback is a function called during graceful shutdown.
// It should complete cleanup and return within a reasonable time.
type Callback func(ctx context.Context) error

// NewService creates a new shutdown manager with the specified timeout.
// timeout sets the maximum time to wait for all callbacks to complete.
func NewService(logger *slog.Logger, ctx context.Context, timeout time.Duration) *Service {
	ctx, cancel := context.WithCancel(ctx)

	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		ctx:       ctx,
		cancel:    cancel,
		logger:    logger,
		callbacks: make([]Callback, 0),
		timeout:   timeout,
	}
}

// Context returns the shutdown context.
// This context is canceled when shutdown begins.
func (m *Service) Context() context.Context {
	if m == nil {
		return context.Background()
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.ctx
}

// Register adds a callback to be executed during shutdown.
// Callbacks are executed in the order they were registered.
func (m *Service) Register(callback Callback) error {
	if m == nil {
		return fmt.Errorf("shutdown service is nil")
	}

	if callback == nil {
		return fmt.Errorf("shutdown callback is nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	callbackIndex := len(m.callbacks)
	m.callbacks = append(m.callbacks, callback)

	m.logger.DebugContext(m.ctx, "Registered shutdown callback",
		"callback_index", callbackIndex,
		"total_callbacks", len(m.callbacks))

	return nil
}

// ListenForSignals starts listening for shutdown signals (SIGINT, SIGTERM).
// This function blocks until a signal is received or the context is canceled.
// Returns true if shutdown was triggered by a signal, false if context was canceled.
func (m *Service) ListenForSignals() bool {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	m.logger.DebugContext(m.ctx, "Starting to listen for shutdown signals",
		"signals", []string{"SIGINT", "SIGTERM"},
		"timeout", m.timeout.String())

	select {
	case sig := <-sigChan:
		if syscallSig, ok := sig.(syscall.Signal); ok {
			m.logger.InfoContext(m.ctx, "Received shutdown signal",
				"signal", sig.String(),
				"signal_number", int(syscallSig))
		} else {
			m.logger.InfoContext(m.ctx, "Received shutdown signal",
				"signal", sig.String())
		}

		m.shutdown()

		return true
	case <-m.ctx.Done():
		m.logger.DebugContext(context.Background(), "Signal listener canceled by context")
		return false
	}
}

// Shutdown triggers graceful shutdown manually.
func (m *Service) Shutdown() {
	m.shutdown()
}

// shutdown performs the actual shutdown process.
func (m *Service) shutdown() {
	m.logger.InfoContext(context.Background(), "Beginning graceful shutdown",
		"timeout", m.timeout.String(),
		"registered_callbacks", len(m.callbacks))

	m.cancel()

	// TODO proper context?
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), m.timeout)
	defer shutdownCancel()

	m.mu.RLock()
	callbacks := make([]Callback, len(m.callbacks))
	copy(callbacks, m.callbacks)
	m.mu.RUnlock()

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

		if shutdownCtx.Err() != nil {
			m.logger.WarnContext(shutdownCtx, "Shutdown timeout reached, canceling remaining callbacks",
				"completed_callbacks", i+1,
				"remaining_callbacks", len(callbacks)-i-1)

			return
		}
	}

	m.logger.InfoContext(context.Background(), "Graceful shutdown completed successfully",
		"total_callbacks", len(callbacks),
		"total_time", m.timeout.String())
}
