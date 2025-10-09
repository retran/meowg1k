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
	"errors"
	"sync"
)

var (
	// ErrNoFuturesProvided indicates that no futures were provided to a wait operation.
	ErrNoFuturesProvided = errors.New("no futures provided")
	// ErrFutureIsNil indicates that the future is nil.
	ErrFutureIsNil = errors.New("future is nil")
	// ErrContextIsNil indicates that the context is nil.
	ErrContextIsNil = errors.New("context is nil")
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
		return ErrFutureIsNil
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
		return ErrFutureIsNil
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
		return zero, ErrFutureIsNil
	}

	if ctx == nil {
		return zero, ErrContextIsNil
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
		// TODO proper error
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
		return zero, ErrFutureIsNil, false
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
		return nil, []error{ErrContextIsNil}
	}

	results := make([]T, len(futures))
	errs := make([]error, len(futures))

	for i, future := range futures {
		if future == nil {
			errs[i] = ErrFutureIsNil
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
		return value, -1, ErrContextIsNil
	}

	if len(futures) == 0 {
		return value, -1, ErrNoFuturesProvided
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
		errors[*new(K)] = ErrContextIsNil
		return results, errors
	}

	for key, future := range futures {
		if future == nil {
			errors[key] = ErrFutureIsNil
			continue
		}
		result, err := future.Get(ctx)
		results[key] = result
		errors[key] = err
	}

	return results, errors
}
