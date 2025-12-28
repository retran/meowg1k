// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

// workerPoolGateway wraps a ports.GenerationGateway with a worker pool to limit concurrency.
type workerPoolGateway struct {
	gateway   ports.GenerationGateway
	semaphore chan struct{}
}

// newWorkerPoolGateway creates a new gateway with worker pool concurrency control.
func newWorkerPoolGateway(innerGateway ports.GenerationGateway, maxConcurrency int) ports.GenerationGateway {
	if maxConcurrency <= 0 {
		maxConcurrency = 1 // At least one worker
	}

	return &workerPoolGateway{
		gateway:   innerGateway,
		semaphore: make(chan struct{}, maxConcurrency),
	}
}

// GenerateContent implements GenerationGateway with worker pool concurrency control.
func (g *workerPoolGateway) GenerateContent(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
) (string, error) {
	if g == nil {
		return "", fmt.Errorf("worker pool gateway is nil")
	}

	if ctx == nil {
		return "", fmt.Errorf("context cannot be nil")
	}

	if request == nil {
		return "", fmt.Errorf("request cannot be nil")
	}

	select {
	case g.semaphore <- struct{}{}:
		defer func() {
			<-g.semaphore
		}()
	case <-ctx.Done():
		return "", fmt.Errorf("context cancelled while waiting for worker pool slot: %w", ctx.Err())
	}

	content, err := g.gateway.GenerateContent(ctx, request)
	if err != nil {
		return "", fmt.Errorf("failed to write content: %w", err)
	}
	return content, nil
}

// GenerateContentWithTools implements tool calling with worker pool concurrency control.
func (g *workerPoolGateway) GenerateContentWithTools(
	ctx context.Context,
	request *gateway.GenerateContentRequest,
	tools []gateway.ToolDefinition,
) (*gateway.GenerateContentResponse, error) {
	if g == nil {
		return nil, fmt.Errorf("worker pool gateway is nil")
	}

	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	inner, ok := g.gateway.(ports.ToolCallingGateway)
	if !ok {
		return nil, gateway.ErrToolCallingNotSupported
	}

	select {
	case g.semaphore <- struct{}{}:
		defer func() {
			<-g.semaphore
		}()
	case <-ctx.Done():
		return nil, fmt.Errorf("context cancelled while waiting for worker pool slot: %w", ctx.Err())
	}

	response, err := inner.GenerateContentWithTools(ctx, request, tools)
	if err != nil {
		return nil, fmt.Errorf("failed to write content: %w", err)
	}
	return response, nil
}
