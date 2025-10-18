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

package indexservice

import (
	"github.com/retran/meowg1k/internal/activities/scanworkspacestate"
	"github.com/retran/meowg1k/internal/domain/gateway"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
)

// PrepareForProcessingOutput represents the output of the PrepareForProcessing operation.
type PrepareForProcessingOutput struct {
	ExistingVersions map[string]int64
	FilesToProcess   map[string]domainindex.FileState
	ContentHashMap   map[string]string
}

// SaveVersionInput represents the input for saving a new version.
type SaveVersionInput struct {
	FilePath    string
	Content     []byte
	ContentHash string
	Chunks      []domainindex.ChunkData
	Embeddings  []gateway.Embedding
}

// SaveVersionOutput represents the output of saving a new version.
type SaveVersionOutput struct {
	FilePath  string
	VersionID int64
}

// FinalizeInput represents the input for finalizing live snapshots.
type FinalizeInput struct {
	ScanResult       *scanworkspacestate.Output
	ExistingVersions map[string]int64
	NewVersions      map[string]int64
}
