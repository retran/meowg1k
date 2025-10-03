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

package summarizefile

import (
"context"
"testing"
"github.com/retran/meowg1k/pkg/executor"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory(nil, nil)
	if factory == nil {
		t.Error("NewFactory returned nil")
	}
}

func TestActivityNilInput(t *testing.T) {
	factory := NewFactory(nil, nil)
	activity := factory.NewActivity()
	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	_, err := activity(ctx, execCtx, nil)
	if err != executor.ErrInputCannotBeNil {
		t.Errorf("Expected ErrInputCannotBeNil, got %v", err)
	}
}

func TestActivityInvalidInput(t *testing.T) {
	factory := NewFactory(nil, nil)
	activity := factory.NewActivity()
	ctx := context.Background()
	execCtx := executor.NewContext("test", nil, nil)
	_, err := activity(ctx, execCtx, "invalid")
	if err == nil {
		t.Error("Expected error for invalid input type")
	}
}
