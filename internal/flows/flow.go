/*
Copyright © 2025 Andrew Vasilyev <me@retran.me>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in type executorWrapper[I any, O any, OT any] struct {
	exec Executor[I, O, OT]
}liance with the License.
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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
)

// Flow is the main container for the task graph.
// All interaction happens through typed DSL functions.
type Flow struct {
	internal *flowInternal
}

// TaskNode represents a typed node in the task graph.
// It is returned by AddTask and used to create links with other nodes.
// Generics in TaskNode ensure type safety when linking.
type TaskNode[I any, O any, OT any] struct {
	flow *Flow
	id   TaskID
}

// NewFlow creates a new empty workflow.
func NewFlow() *Flow {
	return &Flow{
		internal: &flowInternal{
			id:              generateFlowID(),
			tasks:           make(map[TaskID]executable),
			taskMetadata:    make(map[TaskID]taskMetadata),
			links:           make(map[TaskID][]link),
			retryPolicy:     DefaultRetryPolicy(),
			timeoutConfig:   DefaultTimeoutConfig(),
			feedbackHandler: func(Feedback) {}, // no-op by default
			logger:          slog.Default(),
		},
	}
}

// SetStart sets the starting task for the workflow.
func (f *Flow) SetStart(id TaskID) *Flow {
	f.internal.setStart(id)
	return f
}

// WithRetryPolicy sets custom retry policy for the entire workflow.
func (f *Flow) WithRetryPolicy(policy RetryPolicy) *Flow {
	f.internal.setRetryPolicy(policy)
	return f
}

// WithFeedbackHandler sets feedback handler for tasks.
func (f *Flow) WithFeedbackHandler(handler FeedbackHandler) *Flow {
	f.internal.setFeedbackHandler(handler)
	return f
}

// WithLogger sets logger for the workflow.
func (f *Flow) WithLogger(logger *slog.Logger) *Flow {
	f.internal.setLogger(logger)
	return f
}

// WithTimeoutConfig sets timeout configuration for the workflow.
func (f *Flow) WithTimeoutConfig(config TimeoutConfig) *Flow {
	f.internal.setTimeoutConfig(config)
	return f
}

// AddTask adds a new task to Flow and returns a typed node (TaskNode).
func AddTask[I any, O any, OT any](f *Flow, id TaskID, task Executor[I, O, OT]) *TaskNode[I, O, OT] {
	wrapper := &executorWrapper[I, O, OT]{executor: task}
	f.internal.addTask(id, wrapper)
	return &TaskNode[I, O, OT]{flow: f, id: id}
}

// WithDescription sets a human-readable description for the task.
// This description is used in progress reporting and logging.
func (n *TaskNode[I, O, OT]) WithDescription(description string) *TaskNode[I, O, OT] {
	n.flow.internal.setTaskDescription(n.id, description)
	return n
}

// LinkTo creates an unconditional link from current node to target.
// This link is used when Outcome equals OutcomeSuccess.
// The target node's input type must match this node's output type for compile-time type safety.
func (n *TaskNode[I, O, OT]) LinkTo(target *TaskNode[O, any, any]) *TaskNode[I, O, OT] {
	n.flow.internal.addLink(n.id, link{to: target.id, on: OutcomeSuccess})
	return n
}

// LinkToID creates an unconditional link from current node to target by ID.
// This is a backward compatibility method. Use LinkTo with typed nodes for type safety.
// Deprecated: Use LinkTo(*TaskNode) instead for compile-time type checking.
func (n *TaskNode[I, O, OT]) LinkToID(targetID TaskID) *TaskNode[I, O, OT] {
	n.flow.internal.addLink(n.id, link{to: targetID, on: OutcomeSuccess})
	return n
}

// When creates a conditional link.
// The `condition` function takes Outcome.Data (type OT) and returns bool.
// Compiler will check that data type in source task's Outcome matches
// the condition function argument type. This is the core of DSL type safety.
func (n *TaskNode[I, O, OT]) When(condition func(data OT) bool, target *TaskNode[O, any, any]) *TaskNode[I, O, OT] {
	// Wrap typed function in untyped for internal storage.
	genericCondition := func(data interface{}) bool {
		if typedData, ok := data.(OT); ok {
			return condition(typedData)
		}
		return false
	}
	n.flow.internal.addLink(n.id, link{to: target.id, on: OutcomeConditional, condition: genericCondition})
	return n
}

// WhenID creates a conditional link by target ID.
// This is a backward compatibility method. Use When with typed nodes for type safety.
// Deprecated: Use When(*TaskNode) instead for compile-time type checking.
func (n *TaskNode[I, O, OT]) WhenID(condition func(data OT) bool, targetID TaskID) *TaskNode[I, O, OT] {
	// Wrap typed function in untyped for internal storage.
	genericCondition := func(data interface{}) bool {
		if typedData, ok := data.(OT); ok {
			return condition(typedData)
		}
		return false
	}
	n.flow.internal.addLink(n.id, link{to: targetID, on: OutcomeConditional, condition: genericCondition})
	return n
}

// Validate performs workflow graph validation.
func (f *Flow) Validate() error {
	return f.internal.validate()
}

// Run executes workflow asynchronously and in parallel.
func (f *Flow) Run(ctx context.Context, initialInput interface{}) (interface{}, error) {
	// Validation before execution
	if err := f.Validate(); err != nil {
		return nil, err
	}

	return f.internal.run(ctx, initialInput)
}

// ID returns unique workflow identifier.
func (f *Flow) ID() string {
	f.internal.RLock()
	defer f.internal.RUnlock()
	return f.internal.id
}

// generateFlowID generates unique identifier for workflow.
func generateFlowID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "flow_" + hex.EncodeToString(bytes)
}

// --- INTERNAL IMPLEMENTATION ---

type executable interface {
	execute(ctx context.Context, input interface{}) (output interface{}, outcome Outcome[any], err error)
}

// executorWrapper wraps typed Executor in untyped interface for internal storage.
type executorWrapper[I any, O any, OT any] struct {
	executor Executor[I, O, OT]
}

func (w *executorWrapper[I, O, OT]) execute(ctx context.Context, input interface{}) (interface{}, Outcome[any], error) {
	// Perform type assertion to convert interface{} to I.
	typedInput, ok := input.(I)
	if !ok {
		return nil, Outcome[any]{}, fmt.Errorf("type assertion failed: expected %T, got %T", *new(I), input)
	}

	result, outcome, err := w.executor.Execute(ctx, typedInput)
	if err != nil {
		return nil, Outcome[any]{}, err
	}

	// Convert typed outcome to untyped outcome for internal flow processing
	// The type safety is maintained at the executor level, but the flow engine
	// needs to work with mixed outcome types from different tasks
	return result, Outcome[any]{Type: outcome.Type, Data: outcome.Data}, nil
}

// link represents a connection (edge) in the graph.
type link struct {
	to        TaskID
	on        OutcomeType
	condition func(data interface{}) bool // used only for OutcomeConditional
}

// taskMetadata stores additional information about a task.
type taskMetadata struct {
	description string
}

// flowInternal - untyped storage for task graph and links.
type flowInternal struct {
	sync.RWMutex
	id              string
	tasks           map[TaskID]executable
	taskMetadata    map[TaskID]taskMetadata
	links           map[TaskID][]link
	startTask       TaskID
	retryPolicy     RetryPolicy
	timeoutConfig   TimeoutConfig
	feedbackHandler FeedbackHandler
	logger          *slog.Logger
}

func (f *flowInternal) addTask(id TaskID, task executable) {
	f.Lock()
	defer f.Unlock()
	f.tasks[id] = task
}

func (f *flowInternal) setTaskDescription(id TaskID, description string) {
	f.Lock()
	defer f.Unlock()
	f.taskMetadata[id] = taskMetadata{description: description}
}

func (f *flowInternal) getTaskDescription(id TaskID, status string) string {
	f.RLock()
	defer f.RUnlock()

	if metadata, exists := f.taskMetadata[id]; exists && metadata.description != "" {
		// Return custom description with status
		switch status {
		case "started":
			return metadata.description
		case "completed":
			return metadata.description + " - completed"
		case "failed":
			return metadata.description + " - failed"
		case "retrying":
			return metadata.description + " - retrying"
		default:
			return metadata.description
		}
	}

	// Fallback to default descriptions or task ID
	return getTaskDescription(id, status)
}

func (f *flowInternal) addLink(from TaskID, l link) {
	f.Lock()
	defer f.Unlock()
	f.links[from] = append(f.links[from], l)
}

func (f *flowInternal) setStart(id TaskID) {
	f.Lock()
	defer f.Unlock()
	f.startTask = id
}

func (f *flowInternal) setRetryPolicy(policy RetryPolicy) {
	f.Lock()
	defer f.Unlock()
	// Validate policy values
	if policy.Multiplier < 1.0 {
		policy.Multiplier = 1.0
	}
	if policy.MaxRetries < 0 {
		policy.MaxRetries = 0
	}
	f.retryPolicy = policy
}

func (f *flowInternal) setFeedbackHandler(handler FeedbackHandler) {
	f.Lock()
	defer f.Unlock()
	if handler != nil {
		f.feedbackHandler = handler
	} else {
		f.feedbackHandler = func(Feedback) {} // no-op
	}
}

func (f *flowInternal) setLogger(logger *slog.Logger) {
	f.Lock()
	defer f.Unlock()
	if logger != nil {
		f.logger = logger
	} else {
		f.logger = slog.Default()
	}
}

func (f *flowInternal) setTimeoutConfig(config TimeoutConfig) {
	f.Lock()
	defer f.Unlock()
	f.timeoutConfig = config
}
