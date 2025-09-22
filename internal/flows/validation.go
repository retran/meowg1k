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

// validate performs workflow graph validation before execution.
func (f *flowInternal) validate() error {
	f.RLock()
	defer f.RUnlock()

	// 1. Check that start task is set
	if f.startTask == "" {
		return NewWorkflowValidationError("start task is not set")
	}

	// 2. Check that start task exists
	if _, exists := f.tasks[f.startTask]; !exists {
		return NewWorkflowValidationError("start task does not exist", map[string]interface{}{
			"start_task": f.startTask,
		})
	}

	// 3. Check that all referenced tasks exist
	for fromTask, links := range f.links {
		for _, link := range links {
			if _, exists := f.tasks[link.to]; !exists {
				return NewWorkflowValidationError("referenced task does not exist", map[string]interface{}{
					"from_task": fromTask,
					"to_task":   link.to,
				})
			}
		}
	}

	// 4. Check for circular dependencies
	if err := f.detectCycles(); err != nil {
		return err
	}

	// 5. Find unreachable tasks (optional check)
	unreachable := f.findUnreachableTasks()
	if len(unreachable) > 0 {
		// This is a warning, not an error
		details := map[string]interface{}{
			"unreachable_tasks": unreachable,
		}
		f.logger.Warn("Found unreachable tasks in workflow", "details", details)
	}

	return nil
}

// detectCycles detects circular dependencies in the graph.
func (f *flowInternal) detectCycles() error {
	// Use DFS algorithm with three node states:
	// 0 - unvisited, 1 - in progress, 2 - processed
	state := make(map[TaskID]int)

	var dfs func(TaskID, []TaskID) error
	dfs = func(current TaskID, path []TaskID) error {
		if state[current] == 1 {
			// Found cycle
			cycleStart := -1
			for i, task := range path {
				if task == current {
					cycleStart = i
					break
				}
			}
			cycle := append(path[cycleStart:], current)
			return NewWorkflowValidationError("circular dependency detected", map[string]interface{}{
				"cycle": cycle,
			})
		}

		if state[current] == 2 {
			// Already processed
			return nil
		}

		state[current] = 1 // Start processing
		newPath := append(path, current)

		// Visit all neighbors
		for _, link := range f.links[current] {
			if err := dfs(link.to, newPath); err != nil {
				return err
			}
		}

		state[current] = 2 // Finish processing
		return nil
	}

	// Start DFS from the start task
	return dfs(f.startTask, nil)
}

// findUnreachableTasks finds tasks unreachable from the start task.
func (f *flowInternal) findUnreachableTasks() []TaskID {
	visited := make(map[TaskID]bool)

	var dfs func(TaskID)
	dfs = func(current TaskID) {
		if visited[current] {
			return
		}
		visited[current] = true

		// Visit all neighbors
		for _, link := range f.links[current] {
			dfs(link.to)
		}
	}

	// Start DFS from the start task
	dfs(f.startTask)

	// Collect unreachable tasks
	var unreachable []TaskID
	for taskID := range f.tasks {
		if !visited[taskID] {
			unreachable = append(unreachable, taskID)
		}
	}

	return unreachable
}

// validateTaskTypes checks type compatibility between linked tasks.
// This is a static check at compile time through generics,
// but here we can add additional runtime checks.
func (f *flowInternal) validateTaskTypes() error {
	// In the current implementation, type checking happens at compile time
	// through generics in the DSL. Here we can add additional checks
	// for special cases if needed.
	return nil
}
