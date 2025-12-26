// Package index provides serialization helpers for embeddings stored in SQLite.
package index

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/retran/meowg1k/internal/domain/gateway"
)

const float64Size = 8

// encodeEmbedding serializes an embedding vector to bytes for storage.
func encodeEmbedding(embedding gateway.Embedding) ([]byte, error) {
	buf := new(bytes.Buffer)
	buf.Grow(len(embedding) * float64Size)

	err := binary.Write(buf, binary.LittleEndian, embedding)
	if err != nil {
		return nil, fmt.Errorf("failed to encode embedding: %w", err)
	}
	return buf.Bytes(), nil
}

// decodeEmbedding deserializes an embedding vector from stored bytes.
func decodeEmbedding(data []byte) (gateway.Embedding, error) {
	if len(data)%float64Size != 0 {
		return nil, fmt.Errorf("invalid data length: must be a multiple of %d", float64Size)
	}

	embeddingLen := len(data) / float64Size
	embedding := make(gateway.Embedding, embeddingLen)

	reader := bytes.NewReader(data)

	err := binary.Read(reader, binary.LittleEndian, &embedding)
	if err != nil {
		return nil, fmt.Errorf("failed to decode embedding: %w", err)
	}
	return embedding, nil
}
