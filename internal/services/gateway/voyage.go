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

	"github.com/retran/meowg1k/internal/models/gateway"
	"github.com/retran/meowg1k/internal/services/llm/voyage"
)

var _ EmbeddingsGateway = (*voyageGateway)(nil)

// voyageGateway is a unified client for the Voyage AI API, implementing EmbeddingGateway.
// It only supports embeddings, not content generation.
type voyageGateway struct {
	ComputeDistanceMixin
	client voyage.Service
}

// NewVoyageGateway creates and initializes a new VoyageGateway.
func newVoyageGateway(apiKey string) (EmbeddingsGateway, error) {
	client, err := voyage.NewService(voyage.Config{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create voyage client: %w", err)
	}

	return &voyageGateway{
		ComputeDistanceMixin: ComputeDistanceMixin{},
		client:               client,
	}, nil
}

// mapTaskTypeToInputType maps our generic TaskType to Voyage AI's input_type parameter.
func mapTaskTypeToInputType(taskType gateway.TaskType) string {
	switch taskType {
	case gateway.RetrievalDocument:
		return "document"
	case gateway.RetrievalQuery:
		return "query"
	case gateway.CodeRetrievalQuery:
		return "query"
	case gateway.Classification:
		return "classification"
	case gateway.Clustering:
		return "clustering"
	case gateway.SemanticSimilarity:
		return "query"
	case gateway.QuestionAnswering:
		return "query"
	case gateway.FactVerification:
		return "query"
	default:
		return "query" // default to query for unknown task types
	}
}

// ComputeEmbeddings sends a request to the Voyage AI API to compute embeddings for the given text chunks.
func (g *voyageGateway) ComputeEmbeddings(ctx context.Context, request *gateway.ComputeEmbeddingsRequest) ([]gateway.Embedding, error) {
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
		return []gateway.Embedding{}, fmt.Errorf("failed to compute embedding with Voyage AI: %w", err)
	}

	embeddings := make([]gateway.Embedding, 0, len(response.Data))
	for _, data := range response.Data {
		embeddings = append(embeddings, gateway.Embedding(data.Embedding))
	}

	return embeddings, nil
}
