// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/adapters/output"
	"github.com/retran/meowg1k/internal/adapters/tracelog"
	"github.com/retran/meowg1k/internal/core/shutdown"
	domainOutput "github.com/retran/meowg1k/internal/domain/output"
)

type stubHTTPClientService struct {
	closed bool
	err    error
}

func (s *stubHTTPClientService) Get() *http.Client {
	return &http.Client{}
}

func (s *stubHTTPClientService) GetWithTimeout(timeout time.Duration) *http.Client {
	_ = timeout
	return &http.Client{}
}

func (s *stubHTTPClientService) Validate() error {
	return nil
}

func (s *stubHTTPClientService) Close() error {
	s.closed = true
	return s.err
}

func TestCreateShutdownService(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	service, err := createShutdownService(context.Background(), logger, nil)
	require.NoError(t, err)
	require.NotNil(t, service)
	service.Shutdown()
}

func TestRegisterOutputShutdown(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	service := shutdown.NewService(context.Background(), logger, time.Second)
	outputService := output.NewService(domainOutput.Stdout)

	err := registerOutputShutdown(service, outputService)
	require.NoError(t, err)

	err = registerOutputShutdown((*shutdown.Service)(nil), outputService)
	assert.Error(t, err)
}

func TestRegisterTraceShutdown(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	service := shutdown.NewService(context.Background(), logger, time.Second)
	traceLogger := tracelog.NewDisabledLogger()

	err := registerTraceShutdown(service, traceLogger)
	require.NoError(t, err)

	err = registerTraceShutdown((*shutdown.Service)(nil), traceLogger)
	assert.Error(t, err)
}

func TestRegisterHTTPClientShutdown(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	service := shutdown.NewService(context.Background(), logger, time.Second)
	client := &stubHTTPClientService{}

	err := registerHTTPClientShutdown(service, client)
	require.NoError(t, err)

	err = registerHTTPClientShutdown((*shutdown.Service)(nil), client)
	assert.Error(t, err)
}
