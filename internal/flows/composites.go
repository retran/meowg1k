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

package flows

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// --- MAP TASK ---

type mapTask[I any, O any] struct {
	worker         Executor[I, O, any]
	maxConcurrency int
}

func (mt *mapTask[I, O]) Execute(ctx context.Context, inputs []I) ([]O, Outcome[any], error) {
	results := make([]O, len(inputs))
	g, gCtx := errgroup.WithContext(ctx)
	if mt.maxConcurrency > 0 {
		g.SetLimit(mt.maxConcurrency)
	}

	for i, input := range inputs {
		i, input := i, input // classic Go loop variable capture fix
		g.Go(func() error {
			res, _, err := mt.worker.Execute(gCtx, input)
			if err == nil {
				results[i] = res
				return nil
			}
			return fmt.Errorf("worker failed for input at index %d: %w", i, err)
		})
	}

	if err := g.Wait(); err != nil {
		return nil, Outcome[any]{}, err
	}

	return results, Outcome[any]{Type: OutcomeSuccess}, nil
}

// AddMapTask adds a Map task to Flow.
// It takes a slice []I as input, applies worker to each element in parallel
// and returns a slice of results []O.
func AddMapTask[I any, O any](f *Flow, id TaskID, worker Executor[I, O, any], maxConcurrency int) *TaskNode[[]I, []O, any] {
	mapTask := &mapTask[I, O]{
		worker:         worker,
		maxConcurrency: maxConcurrency,
	}
	return AddTask[[]I, []O, any](f, id, mapTask)
}

// --- REDUCE TASK ---

type reduceTask[A any, I any] struct {
	reducer ReduceFunc[A, I]
	initial A
}

func (rt *reduceTask[A, I]) Execute(_ context.Context, inputs []I) (A, Outcome[any], error) {
	acc := rt.initial
	for _, item := range inputs {
		acc = rt.reducer(acc, item)
	}
	return acc, Outcome[any]{Type: OutcomeSuccess}, nil
}

// AddReduceTask adds a Reduce task to the Flow.
// It takes a slice []I and reduces it to a single value of type A.
func AddReduceTask[A any, I any](f *Flow, id TaskID, reducer ReduceFunc[A, I], initial A) *TaskNode[[]I, A, any] {
	reduceTask := &reduceTask[A, I]{reducer: reducer, initial: initial}
	return AddTask[[]I, A, any](f, id, reduceTask)
}

// --- JOIN TASK ---

// joinTask is an internal stateful task for merging (Fan-In).
type joinTask struct {
	mu             sync.Mutex
	expectedInputs int
	receivedInputs int
	results        []interface{}
	done           chan struct{} // Closed when all inputs are received
}

func newJoinTask(expectedInputs int) *joinTask {
	return &joinTask{
		expectedInputs: expectedInputs,
		results:        make([]interface{}, 0, expectedInputs),
		done:           make(chan struct{}),
	}
}

func (jt *joinTask) Execute(ctx context.Context, input interface{}) ([]interface{}, Outcome[any], error) {
	jt.mu.Lock()
	jt.receivedInputs++
	jt.results = append(jt.results, input)
	isLast := jt.receivedInputs == jt.expectedInputs
	if isLast {
		close(jt.done)
	}
	jt.mu.Unlock()

	if isLast {
		// Last goroutine returns collected results.
		return jt.results, Outcome[any]{Type: OutcomeSuccess}, nil
	}

	// Other goroutines block until the last one arrives or context is cancelled
	select {
	case <-jt.done:
		// Once the last goroutine arrives, others finish their branch.
		return nil, Outcome[any]{}, fmt.Errorf("join task completed with %d/%d inputs: %w", jt.receivedInputs, jt.expectedInputs, ErrBranchFinished)
	case <-ctx.Done():
		// Context cancelled - return appropriate error
		return nil, Outcome[any]{}, ctx.Err()
	}
}

// AddJoinTask adds a join node to the Flow that waits for `expectedInputs`
// incoming branches before continuing.
func AddJoinTask(f *Flow, id TaskID, expectedInputs int) *TaskNode[any, []interface{}, any] {
	if expectedInputs <= 0 {
		expectedInputs = 1
	}
	join := newJoinTask(expectedInputs)
	// Important: we add a custom task with `any` types for input and `[]interface{}` for output.
	return AddTask[any, []interface{}, any](f, id, join)
}

// --- TYPED JOIN HANDLERS ---

// findTypedInputs is a helper function that extracts typed values from a slice of interfaces.
// It returns a slice of reflect.Value objects for the found types and an error if any required type is missing.
func findTypedInputs(inputs []interface{}, expectedCount int, typeCheckers []func(interface{}) (interface{}, bool)) ([]interface{}, error) {
	if len(inputs) != expectedCount {
		return nil, NewWorkflowExecutionError("",
			fmt.Sprintf("expected %d inputs for typed join, got %d", expectedCount, len(inputs)), nil)
	}

	found := make([]interface{}, expectedCount)
	foundFlags := make([]bool, expectedCount)

	for _, item := range inputs {
		for i, checker := range typeCheckers {
			if foundFlags[i] {
				continue // Already found this type
			}
			if val, ok := checker(item); ok {
				found[i] = val
				foundFlags[i] = true
				break
			}
		}
	}

	// Check if all types were found
	for _, flag := range foundFlags {
		if !flag {
			return nil, NewWorkflowExecutionError("",
				"could not find all required types in joined inputs", nil)
		}
	}

	return found, nil
}

// typedJoinHandler2 is an internal task wrapper for a function that processes 2 joined results.
type typedJoinHandler2[I1, I2, O any] struct {
	handler func(res1 I1, res2 I2) (O, error)
}

func (t *typedJoinHandler2[I1, I2, O]) Execute(_ context.Context, inputs []interface{}) (O, Outcome[any], error) {
	var zero O

	typeCheckers := []func(interface{}) (interface{}, bool){
		func(item interface{}) (interface{}, bool) {
			if v, ok := item.(I1); ok {
				return v, true
			}
			return nil, false
		},
		func(item interface{}) (interface{}, bool) {
			if v, ok := item.(I2); ok {
				return v, true
			}
			return nil, false
		},
	}

	found, err := findTypedInputs(inputs, 2, typeCheckers)
	if err != nil {
		return zero, Outcome[any]{}, err
	}

	val1 := found[0].(I1)
	val2 := found[1].(I2)

	res, err := t.handler(val1, val2)
	if err != nil {
		return zero, Outcome[any]{}, err
	}
	return res, Outcome[any]{Type: OutcomeSuccess}, nil
}

// AddTypedJoinHandler2 adds a task that is a type-safe wrapper
// over a function that processes 2 results from different branches.
func AddTypedJoinHandler2[I1, I2, O any](f *Flow, id TaskID, handler func(I1, I2) (O, error)) *TaskNode[[]interface{}, O, any] {
	task := &typedJoinHandler2[I1, I2, O]{handler: handler}
	return AddTask[[]interface{}, O, any](f, id, task)
}

// typedJoinHandler3 - wrapper for 3 results.
type typedJoinHandler3[A any, B any, C any, O any] struct {
	handler func(A, B, C) (O, error)
}

func (t *typedJoinHandler3[I1, I2, I3, O]) Execute(_ context.Context, inputs []interface{}) (O, Outcome[any], error) {
	var zero O

	typeCheckers := []func(interface{}) (interface{}, bool){
		func(item interface{}) (interface{}, bool) {
			if v, ok := item.(I1); ok {
				return v, true
			}
			return nil, false
		},
		func(item interface{}) (interface{}, bool) {
			if v, ok := item.(I2); ok {
				return v, true
			}
			return nil, false
		},
		func(item interface{}) (interface{}, bool) {
			if v, ok := item.(I3); ok {
				return v, true
			}
			return nil, false
		},
	}

	found, err := findTypedInputs(inputs, 3, typeCheckers)
	if err != nil {
		return zero, Outcome[any]{}, err
	}

	val1 := found[0].(I1)
	val2 := found[1].(I2)
	val3 := found[2].(I3)

	res, err := t.handler(val1, val2, val3)
	if err != nil {
		return zero, Outcome[any]{}, err
	}
	return res, Outcome[any]{Type: OutcomeSuccess}, nil
}

// AddTypedJoinHandler3 - DSL function for 3 results.
func AddTypedJoinHandler3[I1, I2, I3, O any](f *Flow, id TaskID, handler func(I1, I2, I3) (O, error)) *TaskNode[[]interface{}, O, any] {
	task := &typedJoinHandler3[I1, I2, I3, O]{handler: handler}
	return AddTask[[]interface{}, O, any](f, id, task)
}

// --- SUB-WORKFLOW ---

// SubWorkflowTask is a wrapper that allows using one Flow as a Task inside another.
type subWorkflowTask struct {
	subFlow *Flow
}

func (t *subWorkflowTask) Execute(ctx context.Context, input interface{}) (interface{}, Outcome[any], error) {
	// Run the nested workflow, passing the context and input data.
	result, err := t.subFlow.Run(ctx, input)
	if err != nil {
		return nil, Outcome[any]{}, err
	}
	// Successful completion of sub-workflow is equivalent to OutcomeSuccess for the parent.
	return result, Outcome[any]{Type: OutcomeSuccess}, nil
}

// AddSubWorkflow embeds one workflow (subFlow) as a task in another (f).
func AddSubWorkflow(f *Flow, id TaskID, subFlow *Flow) *TaskNode[any, any, any] {
	// Sub-workflow inherits RetryPolicy from the parent.
	f.internal.RLock()
	policy := f.internal.retryPolicy
	logger := f.internal.logger
	feedbackHandler := f.internal.feedbackHandler
	f.internal.RUnlock()

	subFlow.WithRetryPolicy(policy).WithLogger(logger).WithFeedbackHandler(feedbackHandler)

	task := &subWorkflowTask{subFlow: subFlow}
	// Input and output of sub-workflow are `any`, since their types are not known at compile time of the parent.
	return AddTask[any, any, any](f, id, task)
}
