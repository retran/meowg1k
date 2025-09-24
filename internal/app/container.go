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

// Package app contains the main application struct and orchestrates cross-cutting services.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/retran/meowg1k/internal/services/command"
	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/shutdown"
	"github.com/spf13/cobra"
)

// AppContainer is the main application struct that holds all cross-cutting services.
type AppContainer struct {
	// Logger is the structured logger for the application.
	Logger *slog.Logger

	// Context is the root context for the application.
	Context context.Context

	// ShutdownService handles graceful shutdown of the application.
	ShutdownService shutdown.Service

	// CommandService handles command-line parameters and flags.
	CommandService command.Service

	// ConfigService manages application configuration.
	ConfigService config.Service
}

const (
	logFileName = "meow.log"
)

// AppContainerKey is the context key type for storing and retrieving the AppContainer instance.
type appContainerKey struct{}

// AppContainerKey is the context key for storing and retrieving the AppContainer instance.
var AppContainerKey = appContainerKey{}

// NewAppContainer initializes the main application struct with all necessary services.
func NewAppContainer(cmd *cobra.Command) (*AppContainer, error) {
	container := &AppContainer{}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, AppContainerKey, container)

	// Create logs directory in user's cache directory
	logDir, err := getLogDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get log directory: %w", err)
	}

	// Ensure log directory exists
	if err = os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	logPath := filepath.Join(logDir, logFileName)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	shutdownService := shutdown.NewService(logger, ctx, 10*time.Second)

	shutdownService.Register(func(ctx context.Context) error {
		if logFile != nil {
			if err = logFile.Close(); err != nil {
				return fmt.Errorf("failed to close log file: %w", err)
			}
		}
		return nil
	})

	commandService, err := command.NewService(cmd)
	if err != nil {
		return nil, err
	}

	configService, err := config.NewService(commandService)
	if err != nil {
		return nil, err
	}

	container.Logger = logger
	container.ShutdownService = shutdownService
	container.CommandService = commandService
	container.ConfigService = configService
	container.Context = shutdownService.Context()

	return container, nil
}

// getLogDir returns the appropriate log directory for the current OS.
func getLogDir() (string, error) {
	// TODO review if this is the best location for logs
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDir, "Library", "Logs", "meow"), nil
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(homeDir, "AppData", "Local")
		}
		return filepath.Join(localAppData, "meow", "logs"), nil
	default:
		xdgCache := os.Getenv("XDG_CACHE_HOME")
		if xdgCache == "" {
			xdgCache = filepath.Join(homeDir, ".cache")
		}
		return filepath.Join(xdgCache, "meow", "logs"), nil
	}
}
