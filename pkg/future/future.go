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

// Package future provides a simple implementation of futures in Go.
package future

import (
	"context"
	"fmt"
	"sync"
)

// Future represents a value that will be available in the future
type Future[T any] struct {
	ch   chan result[T]
	mu   sync.RWMutex
	done bool
	val  T
	err  error
}

type result[T any] struct {
	value T
	error error
}

// NewFuture creates a new Future
func NewFuture[T any]() *Future[T] {
	return &Future[T]{
		ch: make(chan result[T], 1),
	}
}

// Complete completes the future with a value
func (f *Future[T]) Complete(value T) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.done {
		return
	}
	f.done = true
	f.val = value
	f.ch <- result[T]{value: value}
	close(f.ch)
}

// CompleteWithError completes the future with an error
func (f *Future[T]) CompleteWithError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.done {
		return
	}
	f.done = true
	f.err = err
	f.ch <- result[T]{error: err}
	close(f.ch)
}

// Get waits for the future to complete and returns the result
func (f *Future[T]) Get(ctx context.Context) (T, error) {
	f.mu.RLock()
	if f.done {
		val, err := f.val, f.err
		f.mu.RUnlock()
		return val, err
	}
	f.mu.RUnlock()

	select {
	case res := <-f.ch:
		return res.value, res.error
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

// IsDone returns true if the future is completed
func (f *Future[T]) IsDone() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.done
}

// TryGet returns the result if available, or nil if not ready
func (f *Future[T]) TryGet() (T, error, bool) {
	f.mu.RLock()
	if f.done {
		val, err := f.val, f.err
		f.mu.RUnlock()
		return val, err, true
	}
	f.mu.RUnlock()

	select {
	case res := <-f.ch:
		return res.value, res.error, true
	default:
		var zero T
		return zero, nil, false
	}
}

// WaitAll waits for all futures to complete and returns all results
func WaitAll[T any](ctx context.Context, futures ...*Future[T]) ([]T, []error) {
	results := make([]T, len(futures))
	errors := make([]error, len(futures))

	for i, future := range futures {
		result, err := future.Get(ctx)
		results[i] = result
		errors[i] = err
	}

	return results, errors
}

// WaitAny waits for any future to complete and returns its result and index
// The returned index indicates which future completed first
func WaitAny[T any](ctx context.Context, futures ...*Future[T]) (T, int, error) {
	if len(futures) == 0 {
		var zero T
		return zero, -1, fmt.Errorf("no futures provided")
	}

	// Create a combined channel for all futures
	type indexedResult struct {
		value T
		err   error
		index int
	}

	resultCh := make(chan indexedResult, len(futures))

	// Start goroutines to monitor each future
	for i, future := range futures {
		go func(idx int, f *Future[T]) {
			result, err := f.Get(ctx)
			resultCh <- indexedResult{result, err, idx}
		}(i, future)
	}

	// Wait for the first result
	select {
	case res := <-resultCh:
		return res.value, res.index, res.err
	case <-ctx.Done():
		var zero T
		return zero, -1, ctx.Err()
	}
}

// WaitAllMap waits for all futures in a map and returns results with the same keys
func WaitAllMap[K comparable, T any](ctx context.Context, futures map[K]*Future[T]) (map[K]T, map[K]error) {
	results := make(map[K]T)
	errors := make(map[K]error)

	for key, future := range futures {
		result, err := future.Get(ctx)
		results[key] = result
		errors[key] = err
	}

	return results, errors
}
