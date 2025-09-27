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

package app

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/cobra"
)

func TestGetLogDir(t *testing.T) {
	// Test getLogDir function
	dir, err := getLogDir()
	if err != nil {
		t.Errorf("getLogDir returned error: %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get user home dir")
	}

	switch runtime.GOOS {
	case "darwin":
		expected := filepath.Join(home, "Library", "Logs", "meow")
		if dir != expected {
			t.Errorf("expected %s, got %s", expected, dir)
		}
	case "windows":
		expected := filepath.Join(home, "AppData", "Local", "meow", "logs")
		if dir != expected {
			t.Errorf("expected %s, got %s", expected, dir)
		}
	default:
		expected := filepath.Join(home, ".cache", "meow", "logs")
		if dir != expected {
			t.Errorf("expected %s, got %s", expected, dir)
		}
	}
}

func TestNewAppContainer(t *testing.T) {
	// Create a test cobra command
	cmd := &cobra.Command{
		Use: "test",
	}

	// Call NewAppContainer
	container, err := NewAppContainer(cmd)
	if err != nil {
		t.Errorf("NewAppContainer returned error: %v", err)
	}

	if container == nil {
		t.Error("container is nil")
	}

	if container.Logger == nil {
		t.Error("Logger is nil")
	}

	if container.ShutdownService == nil {
		t.Error("ShutdownService is nil")
	}

	if container.CommandService == nil {
		t.Error("CommandService is nil")
	}

	if container.ConfigService == nil {
		t.Error("ConfigService is nil")
	}

	if container.Context == nil {
		t.Error("Context is nil")
	}

	// Check that AppContainerKey is set in context
	val := container.Context.Value(AppContainerKey)
	if val != container {
		t.Error("AppContainerKey not set correctly in context")
	}
}
