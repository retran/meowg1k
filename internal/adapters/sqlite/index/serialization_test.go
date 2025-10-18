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

package index

import (
	"testing"

	"github.com/retran/meowg1k/internal/domain/gateway"
)

func TestEncodeDecodeEmbedding(t *testing.T) {
	tests := []struct {
		name      string
		embedding gateway.Embedding
	}{
		{
			name:      "empty embedding",
			embedding: gateway.Embedding{},
		},
		{
			name:      "single value",
			embedding: gateway.Embedding{1.0},
		},
		{
			name:      "multiple values",
			embedding: gateway.Embedding{0.1, 0.2, 0.3, 0.4, 0.5},
		},
		{
			name:      "negative values",
			embedding: gateway.Embedding{-1.0, -0.5, 0.0, 0.5, 1.0},
		},
		{
			name: "large embedding",
			embedding: func() gateway.Embedding {
				e := make(gateway.Embedding, 1536)
				for i := range e {
					e[i] = float64(i) * 0.001
				}
				return e
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode the embedding
			encoded, err := encodeEmbedding(tt.embedding)
			if err != nil {
				t.Fatalf("encodeEmbedding() error = %v", err)
			}

			// Verify the encoded data has the correct length
			expectedLen := len(tt.embedding) * float64Size
			if len(encoded) != expectedLen {
				t.Errorf("encoded data length = %d, want %d", len(encoded), expectedLen)
			}

			// Decode the embedding
			decoded, err := decodeEmbedding(encoded)
			if err != nil {
				t.Fatalf("decodeEmbedding() error = %v", err)
			}

			// Verify the decoded embedding matches the original
			if len(decoded) != len(tt.embedding) {
				t.Errorf("decoded length = %d, want %d", len(decoded), len(tt.embedding))
			}

			for i := range tt.embedding {
				if decoded[i] != tt.embedding[i] {
					t.Errorf("decoded[%d] = %f, want %f", i, decoded[i], tt.embedding[i])
				}
			}
		})
	}
}

func TestDecodeEmbedding_InvalidData(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "invalid length - not multiple of 8",
			data:    []byte{1, 2, 3, 4, 5},
			wantErr: true,
		},
		{
			name:    "invalid length - 1 byte",
			data:    []byte{1},
			wantErr: true,
		},
		{
			name:    "valid empty data",
			data:    []byte{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeEmbedding(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeEmbedding() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEncodeEmbedding_SpecialValues(t *testing.T) {
	tests := []struct {
		name      string
		embedding gateway.Embedding
	}{
		{
			name:      "very small values",
			embedding: gateway.Embedding{1e-300, 1e-200, 1e-100},
		},
		{
			name:      "very large values",
			embedding: gateway.Embedding{1e100, 1e200, 1e300},
		},
		{
			name:      "zero values",
			embedding: gateway.Embedding{0.0, 0.0, 0.0},
		},
		{
			name:      "mixed precision",
			embedding: gateway.Embedding{0.123456789012345, -0.987654321098765},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := encodeEmbedding(tt.embedding)
			if err != nil {
				t.Fatalf("encodeEmbedding() error = %v", err)
			}

			decoded, err := decodeEmbedding(encoded)
			if err != nil {
				t.Fatalf("decodeEmbedding() error = %v", err)
			}

			if len(decoded) != len(tt.embedding) {
				t.Fatalf("length mismatch: got %d, want %d", len(decoded), len(tt.embedding))
			}

			for i := range tt.embedding {
				if decoded[i] != tt.embedding[i] {
					t.Errorf("value mismatch at index %d: got %v, want %v", i, decoded[i], tt.embedding[i])
				}
			}
		})
	}
}

func TestDecodeEmbedding_InvalidLength7Bytes(t *testing.T) {
	// 7 bytes is not a multiple of 8
	data := []byte{1, 2, 3, 4, 5, 6, 7}
	_, err := decodeEmbedding(data)
	if err == nil {
		t.Error("expected error for 7-byte data, got nil")
	}
}

func TestDecodeEmbedding_ValidSingleFloat(t *testing.T) {
	// Create a valid 8-byte encoding of a single float64
	original := gateway.Embedding{42.42}
	encoded, err := encodeEmbedding(original)
	if err != nil {
		t.Fatalf("encodeEmbedding() error = %v", err)
	}

	decoded, err := decodeEmbedding(encoded)
	if err != nil {
		t.Fatalf("decodeEmbedding() error = %v", err)
	}

	if len(decoded) != 1 {
		t.Errorf("expected length 1, got %d", len(decoded))
	}

	if decoded[0] != 42.42 {
		t.Errorf("expected 42.42, got %f", decoded[0])
	}
}
