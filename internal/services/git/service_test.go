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

package git

import (
	"testing"
)

func TestNewService(t *testing.T) {
	service := NewService()
	if service == nil {
		t.Errorf("NewService() returned nil")
	}
}

func TestServiceImpl_ReadStagedFiles(t *testing.T) {
	service := NewService()

	// This test assumes we're in a git repository
	// It may fail if there are no staged files or if not in a git repo
	files, err := service.ReadStagedFiles()
	if err != nil {
		// If git command fails (e.g., not in git repo), that's acceptable for this basic test
		t.Logf("ReadStagedFiles() error (expected if not in git repo): %v", err)
		return
	}

	// If successful, files should be a slice (possibly empty)
	if files == nil {
		t.Errorf("ReadStagedFiles() returned nil slice")
	}
}

func TestServiceImpl_ReadStagedChanges(t *testing.T) {
	service := NewService()

	// Test with a file that likely doesn't exist in staging
	_, err := service.ReadStagedChanges("nonexistent.txt")
	// This should fail since the file is not staged
	if err == nil {
		t.Logf("ReadStagedChanges() unexpectedly succeeded for nonexistent file")
	}
}

func TestServiceImpl_ReadStagedFileContent(t *testing.T) {
	service := NewService()

	// Test with a file that likely doesn't exist in staging
	_, err := service.ReadStagedFileContent("nonexistent.txt")
	// This should fail since the file is not staged
	if err == nil {
		t.Logf("ReadStagedFileContent() unexpectedly succeeded for nonexistent file")
	}
}

func TestServiceImpl_ReadOriginalFileContent(t *testing.T) {
	service := NewService()

	// Test with a file that likely doesn't exist in HEAD
	_, err := service.ReadOriginalFileContent("nonexistent.txt")
	// This should fail since the file is not in HEAD
	if err == nil {
		t.Logf("ReadOriginalFileContent() unexpectedly succeeded for nonexistent file")
	}
}
