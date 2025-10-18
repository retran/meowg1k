// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	domainGateway "github.com/retran/meowg1k/internal/domain/gateway"
	"github.com/retran/meowg1k/internal/ports"
)

func TestNewWorkerPoolGateway(t *testing.T) {
	mockGateway := &mockGenerationGateway{
		response: "test response",
	}

	tests := []struct {
		name           string
		maxConcurrency int
	}{
		{
			name:           "with positive concurrency",
			maxConcurrency: 5,
		},
		{
			name:           "with zero concurrency (should default to 1)",
			maxConcurrency: 0,
		},
		{
			name:           "with negative concurrency (should default to 1)",
			maxConcurrency: -1,
		},
		{
			name:           "with concurrency of 1",
			maxConcurrency: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := newWorkerPoolGateway(mockGateway, tt.maxConcurrency)
			if gateway == nil {
				t.Fatal("newWorkerPoolGateway returned nil")
			}
		})
	}
}

func TestWorkerPoolGateway_GenerateContent(t *testing.T) {
	mockGateway := &mockGenerationGateway{
		response: "test response",
	}

	gateway := newWorkerPoolGateway(mockGateway, 2)
	request := domainGateway.NewGenerateContentRequest("test-model", "System", "User", 1000)

	ctx := context.Background()
	response, err := gateway.GenerateContent(ctx, request)
	if err != nil {
		t.Errorf("GenerateContent() unexpected error = %v", err)
	}

	if response != "test response" {
		t.Errorf("GenerateContent() = %v, want %v", response, "test response")
	}
}

func TestWorkerPoolGateway_Concurrency(t *testing.T) {
	const maxConcurrency = 2
	const numRequests = 5

	activeRequests := 0
	maxActive := 0
	var mu sync.Mutex

	slowGateway := &mockGenerationGateway{
		response: "test response",
	}

	// Wrap the mock to track concurrency
	trackingGateway := &struct {
		*mockGenerationGateway
	}{slowGateway}

	// Override GenerateContent to track concurrency
	originalMock := trackingGateway.mockGenerationGateway
	trackingGateway.mockGenerationGateway = &mockGenerationGateway{
		response: originalMock.response,
		err:      originalMock.err,
	}

	// Create a custom gateway that tracks active requests
	type trackingWorkerPool struct {
		gateway   ports.GenerationGateway
		semaphore chan struct{}
		tracker   func()
	}

	tracker := func() {
		mu.Lock()
		activeRequests++
		if activeRequests > maxActive {
			maxActive = activeRequests
		}
		mu.Unlock()

		time.Sleep(50 * time.Millisecond) // Simulate work

		mu.Lock()
		activeRequests--
		mu.Unlock()
	}

	customGateway := &trackingWorkerPool{
		gateway:   originalMock,
		semaphore: make(chan struct{}, maxConcurrency),
		tracker:   tracker,
	}

	// Implement GenerateContent
	generateFunc := func(ctx context.Context, request *domainGateway.GenerateContentRequest) (string, error) {
		select {
		case customGateway.semaphore <- struct{}{}:
			defer func() {
				<-customGateway.semaphore
			}()
		case <-ctx.Done():
			return "", ctx.Err()
		}

		customGateway.tracker()
		return customGateway.gateway.GenerateContent(ctx, request)
	}

	var wg sync.WaitGroup
	ctx := context.Background()

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			request := domainGateway.NewGenerateContentRequest("test-model", "System", "User", 1000)
			_, err := generateFunc(ctx, request)
			if err != nil {
				t.Errorf("GenerateContent() unexpected error = %v", err)
			}
		}()
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()

	if maxActive > maxConcurrency {
		t.Errorf("Max active requests %d exceeded max concurrency %d", maxActive, maxConcurrency)
	}
}

// blockingMockGateway is a mock that blocks until unblocked
type blockingMockGateway struct {
	blockChan chan struct{}
}

func (b *blockingMockGateway) GenerateContent(ctx context.Context, req *domainGateway.GenerateContentRequest) (string, error) {
	select {
	case <-b.blockChan:
		return "completed", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func TestWorkerPoolGateway_ContextCancellation(t *testing.T) {
	// Create a blocking gateway
	blockChan := make(chan struct{})
	blockingGateway := &blockingMockGateway{
		blockChan: blockChan,
	}

	// Create worker pool with 1 slot
	gateway := newWorkerPoolGateway(blockingGateway, 1)
	request := domainGateway.NewGenerateContentRequest("test-model", "System", "User", 1000)

	// Start first request to occupy the worker slot
	go func() {
		gateway.GenerateContent(context.Background(), request)
	}()

	// Give it time to acquire the semaphore slot
	time.Sleep(50 * time.Millisecond)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Second request should fail because context is cancelled and slot is occupied
	_, err := gateway.GenerateContent(ctx, request)

	// Unblock the first request
	close(blockChan)

	if err == nil {
		t.Error("Expected context cancellation error, got nil")
	}

	if err != nil && !strings.Contains(err.Error(), "context") {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
}

func TestWorkerPoolGateway_MultipleRequests(t *testing.T) {
	mockGateway := &mockGenerationGateway{
		response: "test response",
	}

	gateway := newWorkerPoolGateway(mockGateway, 3)

	const numRequests = 10
	var wg sync.WaitGroup
	errors := make([]error, numRequests)
	responses := make([]string, numRequests)

	ctx := context.Background()

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			request := domainGateway.NewGenerateContentRequest("test-model", "System", "User", 1000)
			response, err := gateway.GenerateContent(ctx, request)
			errors[idx] = err
			responses[idx] = response
		}(i)
	}

	wg.Wait()

	for i := 0; i < numRequests; i++ {
		if errors[i] != nil {
			t.Errorf("Request %d failed with error: %v", i, errors[i])
		}
		if responses[i] != "test response" {
			t.Errorf("Request %d got response %v, want %v", i, responses[i], "test response")
		}
	}
}
