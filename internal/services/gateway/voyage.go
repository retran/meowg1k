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

	"github.com/retran/meowg1k/internal/core/gateway"
	"github.com/retran/meowg1k/pkg/voyage"
)

var _ EmbeddingsGateway = (*voyageGateway)(nil)

// voyageGateway is a unified client for the Voyage AI API, implementing EmbeddingGateway.
type voyageGateway struct {
	gateway.ComputeDistanceMixin
	client *voyage.Client
}

// NewVoyageGateway creates and initializes a new VoyageGateway.
func newVoyageGateway(apiKey string) (EmbeddingsGateway, error) {
	client, err := voyage.NewClient("", apiKey, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to create voyage client: %w", err)
	}

	return &voyageGateway{
		ComputeDistanceMixin: gateway.ComputeDistanceMixin{},
		client:               client,
	}, nil
}

const (
	DefaultTaskType = "query"
)

// mapTaskTypeToInputType maps our generic gateway.TaskType to Voyage AI's input_type parameter.
func mapTaskTypeToInputType(taskType gateway.TaskType) string {
	switch taskType {
	case gateway.RetrievalDocument:
		return "document"
	case gateway.RetrievalQuery:
		return DefaultTaskType
	case gateway.CodeRetrievalQuery:
		return DefaultTaskType
	case gateway.Classification:
		return "classification"
	case gateway.Clustering:
		return "clustering"
	case gateway.SemanticSimilarity:
		return DefaultTaskType
	case gateway.QuestionAnswering:
		return DefaultTaskType
	case gateway.FactVerification:
		return DefaultTaskType
	default:
		return DefaultTaskType // default to query for unknown task types
	}
}

// ComputeEmbeddings sends a request to the Voyage AI API to compute embeddings for the given text chunks.
func (g *voyageGateway) ComputeEmbeddings(
	ctx context.Context,
	request *gateway.ComputeEmbeddingsRequest,
) ([]gateway.Embedding, error) {
	if g == nil {
		return nil, fmt.Errorf("voyage gateway is nil")
	}

	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	inputType := mapTaskTypeToInputType(request.TaskType())

	req := voyage.EmbeddingRequest{
		Input:     request.Chunks(),
		Model:     request.Model(),
		InputType: inputType,
	}

	// Set output dimension if specified
	if request.Dimensions() > 0 {
		dims := request.Dimensions()
		req.OutputDimension = &dims
	}

	response, err := g.client.CreateEmbeddings(ctx, req)
	if err != nil {
		return []gateway.Embedding{}, fmt.Errorf("failed to compute embeddings from Voyage AI for model %q: %w", request.Model(), err)
	}

	embeddings := make([]gateway.Embedding, 0, len(response.Data))
	for _, data := range response.Data {
		embeddings = append(embeddings, data.Embedding)
	}

	return embeddings, nil
}
