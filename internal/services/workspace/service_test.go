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

package workspace

import (
	"os"
	"testing"
)

func TestNewService(t *testing.T) {
	service := NewService()
	if service == nil {
		t.Error("NewService returned nil")
	}
}

func TestGetWorkspaceDir(t *testing.T) {
	service := NewService()

	dir, err := service.GetWorkspaceDir()
	if err != nil {
		t.Errorf("GetWorkspaceDir failed: %v", err)
	}

	expected, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd failed: %v", err)
	}

	if dir != expected {
		t.Errorf("Expected %s, got %s", expected, dir)
	}
}
