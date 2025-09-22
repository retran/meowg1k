package flows

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// Helper types for testing composite tasks

type IntDoubler struct{}

func (id *IntDoubler) Execute(ctx context.Context, input int) (string, Outcome[any], error) {
	return fmt.Sprintf("%d", input*2), Outcome[any]{Type: OutcomeSuccess}, nil
}

type SlowWorker struct {
	delay time.Duration
}

func (sw *SlowWorker) Execute(ctx context.Context, input int) (int, Outcome[any], error) {
	time.Sleep(sw.delay)
	return input * 2, Outcome[any]{Type: OutcomeSuccess}, nil
}

type ErrorWorker struct{}

func (ew *ErrorWorker) Execute(ctx context.Context, input int) (int, Outcome[any], error) {
	if input == 3 {
		return 0, Outcome[any]{}, errors.New("error on input 3")
	}
	return input * 2, Outcome[any]{Type: OutcomeSuccess}, nil
}

type IntTask struct {
	value int
}

func (it *IntTask) Execute(ctx context.Context, input interface{}) (int, Outcome[any], error) {
	return it.value, Outcome[any]{Type: OutcomeSuccess}, nil
}

type StringTask struct {
	value string
}

func (st *StringTask) Execute(ctx context.Context, input interface{}) (string, Outcome[any], error) {
	return st.value, Outcome[any]{Type: OutcomeSuccess}, nil
}

type NumberTask struct {
	value int
}

func (nt *NumberTask) Execute(ctx context.Context, input interface{}) (int, Outcome[any], error) {
	return nt.value, Outcome[any]{Type: OutcomeSuccess}, nil
}

func TestMapTaskComposite(t *testing.T) {
	t.Run("Basic Map Functionality", func(t *testing.T) {
		flow := NewFlow()

		doubler := &IntDoubler{}
		mapNode := AddMapTask(flow, "map", doubler, 0) // No concurrency limit
		finalTask := &TestTask{Name: "final", Result: "complete"}

		mapNode.LinkToID("final")
		AddTask(flow, "final", finalTask)

		flow.SetStart("map")

		ctx := context.Background()
		input := []int{1, 2, 3, 4, 5}
		result, err := flow.Run(ctx, input)

		if err != nil {
			t.Fatalf("Map flow execution failed: %v", err)
		}

		if result != "complete" {
			t.Errorf("Expected 'complete', got %v", result)
		}
	})

	t.Run("Map with Concurrency Limit", func(t *testing.T) {
		flow := NewFlow()

		worker := &SlowWorker{delay: 50 * time.Millisecond}
		AddMapTask(flow, "map", worker, 2) // Limit to 2 concurrent workers

		flow.SetStart("map")

		ctx := context.Background()
		input := []int{1, 2, 3, 4, 5}

		start := time.Now()
		_, err := flow.Run(ctx, input)
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Map with concurrency limit failed: %v", err)
		}

		// With 2 concurrent workers processing 5 items (50ms each),
		// should take roughly 150ms (3 rounds: 2+2+1)
		if duration < 140*time.Millisecond || duration > 200*time.Millisecond {
			t.Errorf("Expected ~150ms execution time, got %v", duration)
		}
	})

	t.Run("Map Error Handling", func(t *testing.T) {
		flow := NewFlow()

		worker := &ErrorWorker{}
		AddMapTask(flow, "map", worker, 0)

		flow.SetStart("map")

		ctx := context.Background()
		input := []int{1, 2, 3, 4, 5}
		_, err := flow.Run(ctx, input)

		if err == nil {
			t.Error("Expected error from map task with failing worker")
		}
	})
}

func TestReduceTaskComposite(t *testing.T) {
	t.Run("Basic Reduce Sum", func(t *testing.T) {
		flow := NewFlow()

		reducer := func(acc int, item int) int {
			return acc + item
		}

		reduceNode := AddReduceTask(flow, "reduce", reducer, 0)
		finalTask := &TestTask{Name: "final", Result: "sum_complete"}

		reduceNode.LinkToID("final")
		AddTask(flow, "final", finalTask)

		flow.SetStart("reduce")

		ctx := context.Background()
		input := []int{1, 2, 3, 4, 5}
		result, err := flow.Run(ctx, input)

		if err != nil {
			t.Fatalf("Reduce flow execution failed: %v", err)
		}

		if result != "sum_complete" {
			t.Errorf("Expected 'sum_complete', got %v", result)
		}
	})

	t.Run("Reduce Product", func(t *testing.T) {
		flow := NewFlow()

		reducer := func(acc int, item int) int {
			return acc * item
		}

		AddReduceTask(flow, "reduce", reducer, 1) // Start with 1 for multiplication

		flow.SetStart("reduce")

		ctx := context.Background()
		input := []int{2, 3, 4}
		result, err := flow.Run(ctx, input)

		if err != nil {
			t.Fatalf("Reduce product failed: %v", err)
		}

		if result != 24 { // 1 * 2 * 3 * 4 = 24
			t.Errorf("Expected 24, got %v", result)
		}
	})

	t.Run("Reduce String Concatenation", func(t *testing.T) {
		flow := NewFlow()

		reducer := func(acc string, item string) string {
			if acc == "" {
				return item
			}
			return acc + "," + item
		}

		AddReduceTask(flow, "reduce", reducer, "")

		flow.SetStart("reduce")

		ctx := context.Background()
		input := []string{"a", "b", "c"}
		result, err := flow.Run(ctx, input)

		if err != nil {
			t.Fatalf("Reduce concatenation failed: %v", err)
		}

		if result != "a,b,c" {
			t.Errorf("Expected 'a,b,c', got %v", result)
		}
	})

	t.Run("Reduce Empty Input", func(t *testing.T) {
		flow := NewFlow()

		reducer := func(acc int, item int) int {
			return acc + item
		}

		AddReduceTask(flow, "reduce", reducer, 10)

		flow.SetStart("reduce")

		ctx := context.Background()
		input := []int{} // Empty slice
		result, err := flow.Run(ctx, input)

		if err != nil {
			t.Fatalf("Reduce with empty input failed: %v", err)
		}

		if result != 10 { // Should return initial value
			t.Errorf("Expected 10 (initial value), got %v", result)
		}
	})
}

func TestJoinTask(t *testing.T) {
	t.Run("Basic Join Two Branches", func(t *testing.T) {
		flow := NewFlow()

		// Start task that fans out
		startTask := &TestTask{Name: "start", Result: "start_result"}
		task1 := &TestTask{Name: "branch1", Result: "result1"}
		task2 := &TestTask{Name: "branch2", Result: "result2"}
		finalTask := &TestTask{Name: "final", Result: "final_result"}

		AddTask(flow, "start", startTask).
			LinkToID("task1").
			LinkToID("task2")

		AddTask(flow, "task1", task1).LinkToID("join")
		AddTask(flow, "task2", task2).LinkToID("join")

		joinNode := AddJoinTask(flow, "join", 2) // Expect 2 inputs
		joinNode.LinkToID("final")
		AddTask(flow, "final", finalTask)

		flow.SetStart("start")

		ctx := context.Background()
		result, err := flow.Run(ctx, "input")

		if err != nil {
			t.Fatalf("Join flow execution failed: %v", err)
		}

		if result != "final_result" {
			t.Errorf("Expected 'final_result', got %v", result)
		}
	})

	t.Run("Join Three Branches", func(t *testing.T) {
		flow := NewFlow()

		startTask := &TestTask{Name: "start", Result: "start"}
		task1 := &TestTask{Name: "task1", Result: "result1"}
		task2 := &TestTask{Name: "task2", Result: "result2"}
		task3 := &TestTask{Name: "task3", Result: "result3"}

		AddTask(flow, "start", startTask).
			LinkToID("task1").
			LinkToID("task2").
			LinkToID("task3")

		AddTask(flow, "task1", task1).LinkToID("join")
		AddTask(flow, "task2", task2).LinkToID("join")
		AddTask(flow, "task3", task3).LinkToID("join")

		AddJoinTask(flow, "join", 3) // Expect 3 inputs

		flow.SetStart("start")

		ctx := context.Background()
		result, err := flow.Run(ctx, "input")

		if err != nil {
			t.Fatalf("Three-way join failed: %v", err)
		}

		// Result should be the slice of joined results
		if results, ok := result.([]interface{}); ok {
			if len(results) != 3 {
				t.Errorf("Expected 3 joined results, got %d", len(results))
			}
		} else {
			t.Errorf("Expected slice of results, got %T", result)
		}
	})

	t.Run("Join with Different Delays", func(t *testing.T) {
		flow := NewFlow()

		startTask := &TestTask{Name: "start", Result: "start"}
		fastTask := &TestTask{Name: "fast", Result: "fast_result", Delay: 10 * time.Millisecond}
		slowTask := &TestTask{Name: "slow", Result: "slow_result", Delay: 50 * time.Millisecond}

		AddTask(flow, "start", startTask).
			LinkToID("fast").
			LinkToID("slow")

		AddTask(flow, "fast", fastTask).LinkToID("join")
		AddTask(flow, "slow", slowTask).LinkToID("join")

		AddJoinTask(flow, "join", 2)

		flow.SetStart("start")

		ctx := context.Background()
		start := time.Now()
		result, err := flow.Run(ctx, "input")
		duration := time.Since(start)

		if err != nil {
			t.Fatalf("Join with different delays failed: %v", err)
		}

		// Should wait for the slower task
		if duration < 45*time.Millisecond {
			t.Errorf("Join should wait for slower task, took only %v", duration)
		}

		if results, ok := result.([]interface{}); ok {
			if len(results) != 2 {
				t.Errorf("Expected 2 joined results, got %d", len(results))
			}
		} else {
			t.Errorf("Expected slice of results, got %T", result)
		}
	})
}

func TestTypedJoinHandlers(t *testing.T) {
	t.Run("TypedJoinHandler2", func(t *testing.T) {
		flow := NewFlow()

		startTask := &TestTask{Name: "start", Result: "start"}

		// Tasks that return different types
		intTask := &IntTask{value: 42}
		stringTask := &StringTask{value: "hello"}

		AddTask(flow, "start", startTask).
			LinkToID("int_task").
			LinkToID("string_task")

		AddTask(flow, "int_task", intTask).LinkToID("join")
		AddTask(flow, "string_task", stringTask).LinkToID("join")

		joinNode := AddJoinTask(flow, "join", 2)

		// Handler that combines int and string
		handler := func(i int, s string) (string, error) {
			return fmt.Sprintf("%s:%d", s, i), nil
		}

		AddTypedJoinHandler2(flow, "typed_join", handler)
		joinNode.LinkToID("typed_join")

		flow.SetStart("start")

		ctx := context.Background()
		result, err := flow.Run(ctx, "input")

		if err != nil {
			t.Fatalf("TypedJoinHandler2 failed: %v", err)
		}

		if result != "hello:42" {
			t.Errorf("Expected 'hello:42', got %v", result)
		}
	})

	t.Run("TypedJoinHandler3", func(t *testing.T) {
		flow := NewFlow()

		startTask := &TestTask{Name: "start", Result: "start"}

		task1 := &NumberTask{value: 10}
		task2 := &NumberTask{value: 20}
		task3 := &NumberTask{value: 30}

		AddTask(flow, "start", startTask).
			LinkToID("task1").
			LinkToID("task2").
			LinkToID("task3")

		AddTask(flow, "task1", task1).LinkToID("join")
		AddTask(flow, "task2", task2).LinkToID("join")
		AddTask(flow, "task3", task3).LinkToID("join")

		joinNode := AddJoinTask(flow, "join", 3)

		// Handler that sums three integers
		handler := func(a, b, c int) (int, error) {
			return a + b + c, nil
		}

		AddTypedJoinHandler3(flow, "typed_join", handler)
		joinNode.LinkToID("typed_join")

		flow.SetStart("start")

		ctx := context.Background()
		result, err := flow.Run(ctx, "input")

		if err != nil {
			t.Fatalf("TypedJoinHandler3 failed: %v", err)
		}

		if result != 60 { // 10 + 20 + 30
			t.Errorf("Expected 60, got %v", result)
		}
	})
}

func TestSubWorkflow(t *testing.T) {
	t.Run("Basic SubWorkflow", func(t *testing.T) {
		// Create main workflow
		mainFlow := NewFlow()

		// Create sub-workflow
		subFlow := NewFlow()
		subTask := &TestTask{Name: "sub_task", Result: "sub_result"}
		AddTask(subFlow, "sub_task", subTask)
		subFlow.SetStart("sub_task")

		// Add sub-workflow to main workflow
		mainTask := &TestTask{Name: "main_task", Result: "main_result"}
		AddTask(mainFlow, "main_task", mainTask).LinkToID("sub_workflow")

		subWorkflowNode := AddSubWorkflow(mainFlow, "sub_workflow", subFlow)

		finalTask := &TestTask{Name: "final", Result: "final_result"}
		subWorkflowNode.LinkToID("final")
		AddTask(mainFlow, "final", finalTask)

		mainFlow.SetStart("main_task")

		ctx := context.Background()
		result, err := mainFlow.Run(ctx, "input")

		if err != nil {
			t.Fatalf("SubWorkflow execution failed: %v", err)
		}

		if result != "final_result" {
			t.Errorf("Expected 'final_result', got %v", result)
		}
	})

	t.Run("SubWorkflow Error Propagation", func(t *testing.T) {
		mainFlow := NewFlow()

		// Create sub-workflow with error
		subFlow := NewFlow()
		errorTask := &ErrorTestTask{ErrorMessage: "sub workflow error"}
		AddTask(subFlow, "error_task", errorTask)
		subFlow.SetStart("error_task")

		// Add sub-workflow to main workflow
		AddSubWorkflow(mainFlow, "sub_workflow", subFlow)
		mainFlow.SetStart("sub_workflow")

		ctx := context.Background()
		_, err := mainFlow.Run(ctx, "input")

		if err == nil {
			t.Error("Expected error from sub-workflow to be propagated")
		}
	})

	t.Run("Nested SubWorkflows", func(t *testing.T) {
		// Create deeply nested workflows
		innerFlow := NewFlow()
		innerTask := &TestTask{Name: "inner", Result: "inner_result"}
		AddTask(innerFlow, "inner", innerTask)
		innerFlow.SetStart("inner")

		middleFlow := NewFlow()
		middleTask := &TestTask{Name: "middle", Result: "middle_result"}
		AddTask(middleFlow, "middle", middleTask).LinkToID("inner_sub")
		AddSubWorkflow(middleFlow, "inner_sub", innerFlow)
		middleFlow.SetStart("middle")

		outerFlow := NewFlow()
		outerTask := &TestTask{Name: "outer", Result: "outer_result"}
		AddTask(outerFlow, "outer", outerTask).LinkToID("middle_sub")
		AddSubWorkflow(outerFlow, "middle_sub", middleFlow)
		outerFlow.SetStart("outer")

		ctx := context.Background()
		result, err := outerFlow.Run(ctx, "input")

		if err != nil {
			t.Fatalf("Nested SubWorkflow execution failed: %v", err)
		}

		if result != "inner_result" {
			t.Errorf("Expected 'inner_result', got %v", result)
		}
	})
}
