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

package executor

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()
	if policy.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts 3, got %d", policy.MaxAttempts)
	}
	if policy.InitialDelay != 100*time.Millisecond {
		t.Errorf("expected InitialDelay 100ms, got %v", policy.InitialDelay)
	}
	if policy.MaxDelay != 5*time.Second {
		t.Errorf("expected MaxDelay 5s, got %v", policy.MaxDelay)
	}
	if policy.Multiplier != 2.0 {
		t.Errorf("expected Multiplier 2.0, got %v", policy.Multiplier)
	}
}

func TestNoRetryPolicy(t *testing.T) {
	policy := NoRetryPolicy()
	if policy.MaxAttempts != 1 {
		t.Errorf("expected MaxAttempts 1, got %d", policy.MaxAttempts)
	}
	if policy.InitialDelay != 0 {
		t.Errorf("expected InitialDelay 0, got %v", policy.InitialDelay)
	}
	if policy.MaxDelay != 0 {
		t.Errorf("expected MaxDelay 0, got %v", policy.MaxDelay)
	}
	if policy.Multiplier != 1.0 {
		t.Errorf("expected Multiplier 1.0, got %v", policy.Multiplier)
	}
}

func TestNewExecutor(t *testing.T) {
	exec := NewExecutor()
	if exec.RetryPolicy == nil {
		t.Error("expected RetryPolicy to be set")
	}
	if exec.FeedbackHandler == nil {
		t.Error("expected FeedbackHandler to be set")
	}
}

func TestWithRetryPolicy(t *testing.T) {
	exec := NewExecutor()
	policy := NoRetryPolicy()
	result := exec.WithRetryPolicy(&policy)
	if result != exec {
		t.Error("expected WithRetryPolicy to return the executor")
	}
	if exec.RetryPolicy.MaxAttempts != 1 {
		t.Error("expected RetryPolicy to be updated")
	}
}

func TestWithFeedbackHandler(t *testing.T) {
	exec := NewExecutor()
	handler := func(f Feedback) {}
	result := exec.WithFeedbackHandler(handler)
	if result != exec {
		t.Error("expected WithFeedbackHandler to return the executor")
	}
	if exec.FeedbackHandler == nil {
		t.Error("expected FeedbackHandler to be updated")
	}
}

func TestRunFlow(t *testing.T) {
	exec := NewExecutor()
	ctx := context.Background()

	flow := func(ctx context.Context, activityCtx *ExecutorContext) error {
		return nil
	}

	err := exec.RunFlow(ctx, "test", flow)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestRunFlowWithError(t *testing.T) {
	exec := NewExecutor()
	ctx := context.Background()

	flow := func(ctx context.Context, activityCtx *ExecutorContext) error {
		return errors.New("test error")
	}

	err := exec.RunFlow(ctx, "test", flow)
	if err == nil {
		t.Error("expected error")
	}
	if !strings.Contains(err.Error(), "test error") {
		t.Errorf("expected error to contain 'test error', got %v", err)
	}
}

func TestRunActivity(t *testing.T) {
	exec := NewExecutor()
	ctx := context.Background()
	parentCtx := NewExecutorContext("parent", NoOpFeedbackHandler, exec)

	activity := func(ctx context.Context, activityCtx *ExecutorContext, input any) (any, error) {
		return "result", nil
	}

	fut := exec.RunActivity(ctx, parentCtx, "test", activity, "input")
	result, err := fut.Get(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "result" {
		t.Errorf("expected 'result', got %v", result)
	}
}

func TestRunActivityWithError(t *testing.T) {
	exec := NewExecutor()
	ctx := context.Background()
	parentCtx := NewExecutorContext("parent", NoOpFeedbackHandler, exec)

	activity := func(ctx context.Context, activityCtx *ExecutorContext, input any) (any, error) {
		return nil, errors.New("activity error")
	}

	fut := exec.RunActivity(ctx, parentCtx, "test", activity, "input")
	_, err := fut.Get(ctx)
	if err == nil {
		t.Error("expected error")
	}
}

func TestExecutorContext(t *testing.T) {
	exec := NewExecutor()
	ctx := NewExecutorContext("test", NoOpFeedbackHandler, exec)

	if ctx.GetExecutor() != exec {
		t.Error("expected GetExecutor to return the executor")
	}

	// Test feedback methods don't panic
	ctx.SendPending("pending")
	ctx.SendStarted("started")
	ctx.SendProgress(0.5, "progress")
	ctx.SendCompleted("completed")
	ctx.SendFailed(errors.New("error"), "failed")
	ctx.SendRetry(1, errors.New("retry error"))
}
