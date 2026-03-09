// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package shutdown

import (
	"context"
	"log/slog"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestListenForSignals_SIGINT(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(ctx, logger, timeout)

	var callbackCalled bool
	service.Register(func(ctx context.Context) error {
		callbackCalled = true
		return nil
	})

	// Start listening in the background.
	done := make(chan bool)
	go func() {
		result := service.ListenForSignals()
		// Should return true when signal is received
		if !result {
			t.Error("ListenForSignals should return true when signal is received")
		}
		done <- true
	}()

	// Give listener time to start
	time.Sleep(50 * time.Millisecond)

	// Send SIGINT to current process
	pid := os.Getpid()
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatalf("Failed to find current process: %v", err)
	}

	err = process.Signal(syscall.SIGINT)
	if err != nil {
		t.Fatalf("Failed to send SIGINT: %v", err)
	}

	// Wait for ListenForSignals to return
	select {
	case <-done:
		// Expected - signal should trigger shutdown
	case <-time.After(time.Second):
		t.Error("ListenForSignals did not return after SIGINT")
	}

	// Give some time for callback to execute
	time.Sleep(100 * time.Millisecond)

	if !callbackCalled {
		t.Error("Callback should be called after SIGINT")
	}
}

func TestListenForSignals_SIGTERM(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()
	timeout := 5 * time.Second

	service := NewService(ctx, logger, timeout)

	var callbackCalled bool
	service.Register(func(ctx context.Context) error {
		callbackCalled = true
		return nil
	})

	// Start listening in the background.
	done := make(chan bool)
	go func() {
		result := service.ListenForSignals()
		// Should return true when signal is received
		if !result {
			t.Error("ListenForSignals should return true when signal is received")
		}
		done <- true
	}()

	// Give listener time to start
	time.Sleep(50 * time.Millisecond)

	// Send SIGTERM to current process
	pid := os.Getpid()
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatalf("Failed to find current process: %v", err)
	}

	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		t.Fatalf("Failed to send SIGTERM: %v", err)
	}

	// Wait for ListenForSignals to return
	select {
	case <-done:
		// Expected - signal should trigger shutdown
	case <-time.After(time.Second):
		t.Error("ListenForSignals did not return after SIGTERM")
	}

	// Give some time for callback to execute
	time.Sleep(100 * time.Millisecond)

	if !callbackCalled {
		t.Error("Callback should be called after SIGTERM")
	}
}
