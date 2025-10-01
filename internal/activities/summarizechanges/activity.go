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

package summarizechanges

import (
	"context"

	"github.com/retran/meowg1k/pkg/executor"
)

// Factory creates instances of the generate activity with injected dependencies.
type Factory struct{}

// NewFactory creates a new generate activity factory with injected services.
func NewFactory() *Factory {
	return &Factory{}
}

// NewActivity creates and returns the generate activity function with added progress reporting.
func (f *Factory) NewActivity() executor.Activity[any, any] {
	return func(ctx context.Context, executorCtx *executor.Context, activityInput any) (any, error) {
		return nil, nil
	}
}
