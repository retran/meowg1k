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

// Package savedocumentversion implements an activity that saves a document version with its chunks to storage.
package savedocumentversion

import (
	"context"
	"fmt"

	"github.com/retran/meowg1k/internal/core/index"
	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/pkg/executor"
)

type Input struct {
	FilePath    string
	Content     []byte
	ContentHash string
	Chunks      []domainindex.ChunkData
	Embeddings  []gateway.Embedding
}

type Output struct {
	FilePath  string
	VersionID int64
}

type Factory struct {
	indexService *index.Service
}

var _ executor.ActivityFactory[*Input, *Output] = (*Factory)(nil)

func NewFactory(indexService *index.Service) (executor.ActivityFactory[*Input, *Output], error) {
	if indexService == nil {
		return nil, fmt.Errorf("savedocumentversion.NewFactory: indexService cannot be nil")
	}

	return &Factory{
		indexService: indexService,
	}, nil
}

func (f *Factory) NewActivity() executor.Activity[*Input, *Output] {
	return func(ctx context.Context, executorCtx *executor.Context, input *Input) (*Output, error) {
		executorCtx.SendRunning(fmt.Sprintf("Saving: %s", input.FilePath))

		serviceInput := &index.SaveVersionInput{
			FilePath:    input.FilePath,
			Content:     input.Content,
			ContentHash: input.ContentHash,
			Chunks:      input.Chunks,
			Embeddings:  input.Embeddings,
		}

		result, err := f.indexService.SaveNewVersion(ctx, serviceInput)
		if err != nil {
			return nil, fmt.Errorf("failed to save document version: %w", err)
		}

		executorCtx.SendCompleted(fmt.Sprintf("Saved: %s (%d)", input.FilePath, len(input.Chunks)))
		return &Output{
			FilePath:  result.FilePath,
			VersionID: result.VersionID,
		}, nil
	}
}
