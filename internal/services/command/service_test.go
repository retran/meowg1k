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

package command

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNewService(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	
	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	
	if service == nil {
		t.Fatal("Service should not be nil")
	}
}

func TestNewServicePanicsWithNilCommand(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when command is nil")
		}
	}()
	
	NewService(nil)
}

func TestGetCommand(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	
	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	
	if service.GetCommand() != cmd {
		t.Error("GetCommand should return the original command")
	}
}

func TestGetCommandName(t *testing.T) {
	cmd := &cobra.Command{
		Use: "testcmd",
	}
	
	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	
	if service.GetCommandName() != "testcmd" {
		t.Errorf("Expected command name 'testcmd', got '%s'", service.GetCommandName())
	}
}

func TestGetConfigPath(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().String("config", "", "config file path")
	cmd.Flags().Set("config", "/path/to/config.yaml")
	
	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	
	configPath, err := service.GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath failed: %v", err)
	}
	
	if configPath != "/path/to/config.yaml" {
		t.Errorf("Expected config path '/path/to/config.yaml', got '%s'", configPath)
	}
}

func TestGetConfigPathUndefined(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	
	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	
	_, err = service.GetConfigPath()
	if err == nil {
		t.Error("Expected error when config flag is not defined")
	}
}

func TestGetTaskName(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().String("task", "", "task name")
	cmd.Flags().Set("task", "mytask")
	
	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	
	taskName, err := service.GetTaskName()
	if err != nil {
		t.Fatalf("GetTaskName failed: %v", err)
	}
	
	if taskName != "mytask" {
		t.Errorf("Expected task name 'mytask', got '%s'", taskName)
	}
}

func TestGetUserPrompt(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().String("user-prompt", "", "user prompt")
	cmd.Flags().Set("user-prompt", "hello world")
	
	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	
	userPrompt, err := service.GetUserPrompt()
	if err != nil {
		t.Fatalf("GetUserPrompt failed: %v", err)
	}
	
	if userPrompt != "hello world" {
		t.Errorf("Expected user prompt 'hello world', got '%s'", userPrompt)
	}
}

func TestGetSilentFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().Bool("silent", false, "silent mode")
	cmd.Flags().Set("silent", "true")
	
	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	
	silent, err := service.GetSilentFlag()
	if err != nil {
		t.Fatalf("GetSilentFlag failed: %v", err)
	}
	
	if !silent {
		t.Error("Expected silent flag to be true")
	}
}

func TestGetStdIn(t *testing.T) {
	// This test is tricky because it depends on stdin state
	// We'll test the basic functionality
	cmd := &cobra.Command{
		Use: "test",
	}
	
	service, err := NewService(cmd)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	
	// When run in test environment, stdin is typically empty
	stdin := service.GetStdIn()
	if stdin == "" {
		// This is expected in test environment
		t.Log("StdIn is empty as expected in test environment")
	}
	
	// Test that GetStdIn returns a string (not nil)
	_ = service.GetStdIn() // Should not panic
}