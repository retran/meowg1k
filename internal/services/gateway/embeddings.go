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
	"errors"
	"math"

	mdGateway "github.com/retran/meowg1k/internal/models/gateway"
)

var (
	ErrVectorsDifferentLength = errors.New("vectors must have the same length")
	ErrVectorsEmpty           = errors.New("vectors must not be empty")
)

// EmbeddingsGateway defines the contract for a client that computes text embeddings
// and measures the distance between them.
type EmbeddingsGateway interface {
	// ComputeEmbeddings computes the vector embedding for the given text.
	ComputeEmbeddings(ctx context.Context, request *mdGateway.ComputeEmbeddingsRequest) ([]mdGateway.Embedding, error)
	// ComputeDistance calculates a similarity or distance score between two embeddings.
	// The exact metric (e.g., cosine similarity) depends on the implementation.
	ComputeDistance(first, second mdGateway.Embedding) (float64, error)
}

type ComputeDistanceMixin struct {
}

// ComputeDistance calculates the cosine similarity between two embeddings.
// It returns a value between -1 (opposite) and 1 (identical), where 0 indicates orthogonality.
func (g *ComputeDistanceMixin) ComputeDistance(a, b mdGateway.Embedding) (float64, error) {
	if len(a) != len(b) {
		return 0, ErrVectorsDifferentLength
	}

	if len(a) == 0 || len(b) == 0 {
		return 0, ErrVectorsEmpty
	}

	var dotProduct, aMagnitude, bMagnitude float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		aMagnitude += float64(a[i]) * float64(a[i])
		bMagnitude += float64(b[i]) * float64(b[i])
	}

	if aMagnitude == 0 || bMagnitude == 0 {
		return 0, nil
	}

	return dotProduct / (math.Sqrt(aMagnitude) * math.Sqrt(bMagnitude)), nil
}
