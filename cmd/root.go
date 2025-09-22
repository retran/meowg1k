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

// Package cmd provides commands for the meow CLI application.
package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/retran/meowg1k/internal/config"
	"github.com/retran/meowg1k/internal/services/config/loader"
	"github.com/retran/meowg1k/internal/services/config/registry"
	"github.com/retran/meowg1k/internal/services/config/validator"
	"github.com/retran/meowg1k/internal/utils/shutdown"
	"github.com/spf13/cobra"
)

var (
	configPath  string
	appConfig   *config.Config
	logFile     *os.File
	verbosity   int  // Verbosity level: 0=ERROR, 1=WARN, 2=INFO, 3=DEBUG
	quietMode   bool // Suppress all log output except errors
	shutdownMgr *shutdown.Manager
)

func Execute() error {
	// Create shutdown manager with 10 second timeout
	shutdownMgr = shutdown.NewManager(10 * time.Second)

	// Setup basic logging to file with default level
	// Will be reconfigured after config is loaded
	if err := setupLogging(); err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}

	// Configure shutdown manager to use the same logger
	shutdownMgr = shutdownMgr.WithLogger(slog.Default())

	// Start listening for shutdown signals in background with error handling
	signalCtx, signalCancel := context.WithCancel(context.Background())
	defer signalCancel() // Ensure signal goroutine is cancelled on exit

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(signalCtx, "Signal handler panicked, attempting controlled shutdown", "panic", r)
				// Attempt controlled shutdown on panic
				if shutdownMgr != nil {
					shutdownMgr.Shutdown()
				}
			}
		}()

		signalReceived := shutdownMgr.ListenForSignals()
		if !signalReceived {
			slog.DebugContext(signalCtx, "Signal listener cancelled by context")
		}
	}()

	// Execute the command and ensure cleanup on exit
	err := rootCmd.Execute()

	// Cancel signal listener when command execution completes
	signalCancel()

	return err
}

var rootCmd = &cobra.Command{
	Use:   "meow",
	Short: "'meow' — your fast, script-friendly AI companion",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for commands that don't need it
		if cmd.Name() == "version" || cmd.Name() == "help" || cmd.Name() == "meow" || cmd.Name() == "completion" {
			return nil
		}

		var err error
		// Load configuration using individual services
		registryService := registry.NewService()
		validatorService := validator.NewService(registryService)
		loaderService := loader.NewService()

		appConfig, err = loaderService.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Validate the loaded configuration
		if err := validatorService.ValidateConfig(appConfig); err != nil {
			return fmt.Errorf("configuration validation failed: %w", err)
		}

		// Reconfigure logging with the loaded configuration
		if err := reconfigureLogging(appConfig); err != nil {
			return fmt.Errorf("failed to reconfigure logging: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "config file path (overrides project/user configs when specified)")
	rootCmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "increase verbosity (can be used multiple times: -v=WARN, -vv=INFO, -vvv=DEBUG)")
	rootCmd.PersistentFlags().BoolVarP(&quietMode, "quiet", "q", false, "suppress all log output except errors")
}

// GetShutdownContext returns the application shutdown context.
// This context is cancelled when the application receives a shutdown signal.
func GetShutdownContext() context.Context {
	if shutdownMgr != nil {
		return shutdownMgr.Context()
	}
	return context.Background()
}

// RegisterShutdownCallback adds a callback to be executed during graceful shutdown.
func RegisterShutdownCallback(callback func(context.Context) error) {
	if shutdownMgr != nil {
		shutdownMgr.Register(callback)
	}
}

// setupLogging configures logging to write to a file instead of stderr.
func setupLogging() error {
	return reconfigureLogging(nil)
}

// reconfigureLogging updates logging configuration based on CLI flags and config file.
func reconfigureLogging(cfg *config.Config) error {
	// Determine log level based on verbosity, quiet mode, and config
	var logLevel slog.Level

	// CLI flags take precedence over config
	if quietMode {
		logLevel = slog.LevelError
	} else if verbosity > 0 {
		switch verbosity {
		case 1:
			logLevel = slog.LevelWarn
		case 2:
			logLevel = slog.LevelInfo
		case 3:
			logLevel = slog.LevelDebug
		default:
			// Cap at debug level for excessive verbosity
			logLevel = slog.LevelDebug
		}
	} else if cfg != nil && cfg.Logging != nil {
		// Use config file settings if no CLI flags
		if cfg.Logging.Quiet {
			logLevel = slog.LevelError
		} else {
			switch cfg.Logging.Level {
			case "debug":
				logLevel = slog.LevelDebug
			case "info":
				logLevel = slog.LevelInfo
			case "warn":
				logLevel = slog.LevelWarn
			case "error":
				logLevel = slog.LevelError
			default:
				// Default to info level
				logLevel = slog.LevelInfo
			}
		}
	} else {
		// Default to error level when no configuration is provided
		logLevel = slog.LevelError
	}

	// Create logs directory in user's cache directory
	logDir, err := getLogDir()
	if err != nil {
		return fmt.Errorf("failed to get log directory: %w", err)
	}

	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create log file (close existing one if reopening)
	if logFile != nil {
		// Close existing log file to prevent resource leak
		if err := logFile.Close(); err != nil {
			// Return the error to propagate critical file closing failures
			return fmt.Errorf("failed to close previous log file: %w", err)
		}
		logFile = nil
	}

	logPath := filepath.Join(logDir, "meow.log")
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// If log file creation fails, fall back to stderr to ensure logging continues
		fmt.Fprintf(os.Stderr, "Warning: failed to open log file %s, falling back to stderr: %v\n", logPath, err)

		// Create logger that writes to stderr as fallback
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: logLevel,
		}))
		slog.SetDefault(logger)

		// Don't register shutdown callback since we're not managing a file
		return nil
	}

	// Create structured logger that writes to file with configurable level
	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Set as default logger
	slog.SetDefault(logger)

	// Register shutdown callback to close log file
	RegisterShutdownCallback(func(ctx context.Context) error {
		if logFile != nil {
			if err := logFile.Close(); err != nil {
				return fmt.Errorf("failed to close log file: %w", err)
			}
		}
		return nil
	})

	return nil
}

// getLogDir returns the appropriate log directory for the current OS.
func getLogDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "darwin": // macOS
		return filepath.Join(homeDir, "Library", "Logs", "meow"), nil
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(homeDir, "AppData", "Local")
		}
		return filepath.Join(localAppData, "meow", "logs"), nil
	default: // Linux and others
		xdgCache := os.Getenv("XDG_CACHE_HOME")
		if xdgCache == "" {
			xdgCache = filepath.Join(homeDir, ".cache")
		}
		return filepath.Join(xdgCache, "meow", "logs"), nil
	}
}
