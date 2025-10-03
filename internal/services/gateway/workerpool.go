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

package gateway

import (
	"context"
	"fmt"
)

// workerPoolGateway wraps a GenerationGateway with a worker pool to limit concurrency.
type workerPoolGateway struct {
	gateway   GenerationGateway
	semaphore chan struct{}
}

// newWorkerPoolGateway creates a new gateway with worker pool concurrency control.
func newWorkerPoolGateway(gateway GenerationGateway, maxConcurrency int) GenerationGateway {
	if maxConcurrency <= 0 {
		maxConcurrency = 1 // At least one worker
	}

	return &workerPoolGateway{
		gateway:   gateway,
		semaphore: make(chan struct{}, maxConcurrency),
	}
}

// GenerateContent implements GenerationGateway with worker pool concurrency control.
func (g *workerPoolGateway) GenerateContent(
	ctx context.Context,
	request *GenerateContentRequest,
) (string, error) {
	select {
	case g.semaphore <- struct{}{}:
		defer func() {
			<-g.semaphore
		}()
	case <-ctx.Done():
		return "", fmt.Errorf("context cancelled while waiting for worker pool slot: %w", ctx.Err())
	}

	return g.gateway.GenerateContent(ctx, request)
}
