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

	"github.com/retran/meowg1k/internal/llm/client/voyage"
)

// Compile-time check to ensure VoyageGateway implements EmbeddingGateway.
var _ EmbeddingGateway = (*VoyageGateway)(nil)

// VoyageGateway is a unified client for the Voyage AI API, implementing EmbeddingGateway.
// It only supports embeddings, not content generation.
type VoyageGateway struct {
	ComputeDistanceMixin
	client *voyage.Client
}

// NewVoyageGateway creates and initializes a new VoyageGateway.
func NewVoyageGateway(apiKey string) (*VoyageGateway, error) {
	client, err := voyage.NewClient(voyage.Config{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create voyage client: %w", err)
	}

	return &VoyageGateway{
		ComputeDistanceMixin: ComputeDistanceMixin{},
		client: client,
	}, nil
}

// mapTaskTypeToInputType maps our generic TaskType to Voyage AI's input_type parameter.
func mapTaskTypeToInputType(taskType TaskType) string {
	switch taskType {
	case RetrievalDocument:
		return "document"
	case RetrievalQuery:
		return "query"
	case CodeRetrievalQuery:
		return "query"
	case Classification:
		return "document"
	case Clustering:
		return "document"
	case SemanticSimilarity:
		return "document"
	case QuestionAnswering:
		return "query"
	case FactVerification:
		return "query"
	default:
		return "document" // Default to document
	}
}

// ComputeEmbeddings sends a request to the Voyage AI API to compute embeddings for the given text chunks.
func (g *VoyageGateway) ComputeEmbeddings(ctx context.Context, request *ComputeEmbeddingsRequest) ([]Embedding, error) {
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
		return []Embedding{}, fmt.Errorf("failed to compute embedding with Voyage AI: %w", err)
	}

	embeddings := make([]Embedding, 0, len(response.Data))
	for _, data := range response.Data {
		embeddings = append(embeddings, Embedding(data.Embedding))
	}

	return embeddings, nil
}
