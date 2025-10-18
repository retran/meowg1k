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

// Package future provides asynchronous computation primitives with support for chaining, error handling, and concurrent execution.
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

// Complete completes the future with a value.
// Returns an error if the future is nil.
func (f *Future[T]) Complete(value T) error {
	if f == nil {
		return fmt.Errorf("future is nil")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.done {
		return nil
	}

	f.done = true

	f.val = value
	f.ch <- result[T]{value: value}

	close(f.ch)
	return nil
}

// CompleteWithError completes the future with an error.
// Returns an error if the future is nil.
func (f *Future[T]) CompleteWithError(err error) error {
	if f == nil {
		return fmt.Errorf("future is nil")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.done {
		return nil
	}

	f.done = true

	f.err = err
	f.ch <- result[T]{error: err}

	close(f.ch)
	return nil
}

// Get waits for the future to complete and returns the result
func (f *Future[T]) Get(ctx context.Context) (T, error) {
	var zero T
	if f == nil {
		return zero, fmt.Errorf("future is nil")
	}

	if ctx == nil {
		return zero, fmt.Errorf("context is nil")
	}

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
		return zero, ctx.Err()
	}
}

// IsDone returns true if the future is completed
func (f *Future[T]) IsDone() bool {
	if f == nil {
		return false
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.done
}

// TryGet returns the result if available, or nil if not ready
func (f *Future[T]) TryGet() (T, error, bool) {
	var zero T
	if f == nil {
		return zero, fmt.Errorf("future is nil"), false
	}

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
		return zero, nil, false
	}
}

// WaitAll waits for all futures to complete and returns all results
func WaitAll[T any](ctx context.Context, futures ...*Future[T]) ([]T, []error) {
	if ctx == nil {
		return nil, []error{fmt.Errorf("context is nil")}
	}

	results := make([]T, len(futures))
	errs := make([]error, len(futures))

	for i, future := range futures {
		if future == nil {
			errs[i] = fmt.Errorf("future at index %d is nil", i)
			continue
		}
		result, err := future.Get(ctx)
		results[i] = result
		errs[i] = err
	}

	return results, errs
}

// WaitAny waits for any future to complete and returns its result and index
// The returned index indicates which future completed first
func WaitAny[T any](ctx context.Context, futures ...*Future[T]) (value T, index int, err error) {
	if ctx == nil {
		return value, -1, fmt.Errorf("context is nil")
	}

	if len(futures) == 0 {
		return value, -1, fmt.Errorf("no futures provided")
	}

	type indexedResult struct {
		value T
		err   error
		index int
	}

	resultCh := make(chan indexedResult, len(futures))

	for i, future := range futures {
		if future == nil {
			continue
		}
		go func(idx int, f *Future[T]) {
			result, err := f.Get(ctx)
			resultCh <- indexedResult{result, err, idx}
		}(i, future)
	}

	select {
	case res := <-resultCh:
		return res.value, res.index, res.err
	case <-ctx.Done():
		return value, -1, ctx.Err()
	}
}

// WaitAllMap waits for all futures in a map and returns results with the same keys
func WaitAllMap[K comparable, T any](ctx context.Context, futures map[K]*Future[T]) (
	results map[K]T, errors map[K]error,
) {
	results = make(map[K]T)
	errors = make(map[K]error)

	if ctx == nil {
		errors[*new(K)] = fmt.Errorf("context is nil")
		return results, errors
	}

	for key, future := range futures {
		if future == nil {
			errors[key] = fmt.Errorf("future for key %v is nil", key)
			continue
		}
		result, err := future.Get(ctx)
		results[key] = result
		errors[key] = err
	}

	return results, errors
}
