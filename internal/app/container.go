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
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/db"
	"github.com/retran/meowg1k/internal/services/command"
	"github.com/retran/meowg1k/internal/services/config"
	"github.com/retran/meowg1k/internal/services/dbpath"
	"github.com/retran/meowg1k/internal/services/output"
	"github.com/retran/meowg1k/internal/services/shutdown"
	"github.com/retran/meowg1k/pkg/ratelimit"
)

var (
	// ErrInvalidLogFilename is returned when a log filename contains invalid characters.
	ErrInvalidLogFilename = errors.New("log filename contains invalid characters or path separators")
	// ErrLogPathOutsideDirectory is returned when a log path is outside the expected directory.
	ErrLogPathOutsideDirectory = errors.New("log path is outside log directory")
	// ErrCmdIsNil is returned when the cobra command is nil.
	ErrCmdIsNil = errors.New("cobra command is nil")
)

// Container is the main application struct that holds all cross-cutting services.
type Container struct {
	// Logger is the structured logger for the application.
	Logger *slog.Logger

	// ShutdownService handles graceful shutdown of the application.
	ShutdownService *shutdown.Service

	// CommandService handles command-line parameters and flags.
	CommandService *command.Service

	// ConfigService manages application configuration.
	ConfigService *config.Service

	// OutputService handles application output to stdout/stderr.
	OutputService output.Writer

	// dbHost provides access to database connections (lazy initialized)
	dbHost db.Host

	// dbPathService provides database path management
	dbPathService *dbpath.Service

	// rateLimitRepo is the repository for rate limiting state (lazy initialized)
	rateLimitRepo ratelimit.Repository

	// dbInitOnce ensures database is initialized only once
	dbInitOnce sync.Once
}

const (
	logFileName = "meow.log"
	osWindows   = "windows"
	osDarwin    = "darwin"
)

// validateLogPath validates the log path to prevent directory traversal attacks
func validateLogPath(logDir, fileName string) error {
	if strings.Contains(fileName, "/") || strings.Contains(fileName, "\\") || strings.Contains(fileName, "..") {
		// TODO proper error
		return fmt.Errorf("%w: %s", ErrInvalidLogFilename, fileName)
	}

	cleanLogDir := filepath.Clean(logDir)
	logPath := filepath.Join(cleanLogDir, fileName)

	if !strings.HasPrefix(logPath, cleanLogDir) {
		// TODO proper error
		return fmt.Errorf("%w: %s is outside %s", ErrLogPathOutsideDirectory, logPath, cleanLogDir)
	}

	return nil
}

// AppContainerKey is the context key type for storing and retrieving the Container instance.
type appContainerKey struct{}

// AppContainerKey is the context key for storing and retrieving the Container instance.
var AppContainerKey = appContainerKey{}

// NewAppContainer initializes the main application struct with all necessary services.
func NewAppContainer(cmd *cobra.Command) (*Container, error) {
	if cmd == nil {
		return nil, ErrCmdIsNil
	}

	container := &Container{}

	ctx := cmd.Context()
	if ctx == nil {
		// TODO proper error
		ctx = context.Background()
	}

	logDir, err := getLogDir()
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to get log directory: %w", err)
	}

	if err = os.MkdirAll(logDir, 0o750); err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	if err = validateLogPath(logDir, logFileName); err != nil {
		// TODO proper error
		return nil, fmt.Errorf("invalid log path: %w", err)
	}

	root, err := os.OpenRoot(logDir)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to open root directory: %w", err)
	}
	defer root.Close()

	logFile, err := root.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	shutdownService := shutdown.NewService(logger, ctx, 10*time.Second)

	err = shutdownService.Register(func(ctx context.Context) error {
		if logFile != nil {
			if err = logFile.Close(); err != nil {
				// TODO proper error
				return fmt.Errorf("failed to close log file: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to register log file shutdown callback: %w", err)
	}

	commandService, err := command.NewService(cmd)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	configService, err := config.NewService(commandService)
	if err != nil {
		// TODO proper error
		return nil, err
	}

	outputService := output.NewService(output.Stdout)
	err = shutdownService.Register(func(ctx context.Context) error {
		return outputService.Flush()
	})
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to register output service shutdown callback: %w", err)
	}

	dbPathService := dbpath.NewService()

	container.Logger = logger
	container.ShutdownService = shutdownService
	container.CommandService = commandService
	container.ConfigService = configService
	container.OutputService = outputService
	container.dbPathService = dbPathService

	shutdownCtx := context.WithValue(shutdownService.Context(), AppContainerKey, container)
	cmd.SetContext(shutdownCtx)

	return container, nil
}

// initDB initializes the database host and rate limit repository if not already initialized.
// This method is thread-safe and will only initialize once.
func (c *Container) initDB() error {
	var initErr error
	c.dbInitOnce.Do(func() {
		dbHost, err := db.NewLocalHost(c.dbPathService)
		if err != nil {
			// TODO proper error
			initErr = fmt.Errorf("failed to initialize database host: %w", err)
			return
		}

		mainDB, err := dbHost.GetDB()
		if err != nil {
			// TODO proper error
			initErr = fmt.Errorf("failed to get main database: %w", err)
			return
		}

		rateLimitRepo := ratelimit.NewRepository(mainDB)

		if err := c.ShutdownService.Register(func(ctx context.Context) error {
			if err := dbHost.Close(); err != nil {
				// TODO proper error
				return fmt.Errorf("failed to close database host: %w", err)
			}
			return nil
		}); err != nil {
			// TODO proper error
			initErr = fmt.Errorf("failed to register database shutdown callback: %w", err)
			return
		}

		c.dbHost = dbHost
		c.rateLimitRepo = rateLimitRepo
	})
	return initErr
}

// GetRateLimitRepo returns the rate limit repository, initializing the database if needed.
func (c *Container) GetRateLimitRepo() ratelimit.Repository {
	if err := c.initDB(); err != nil {
		// TODO proper error
		c.Logger.Error("failed to initialize database", "error", err)
		return nil
	}
	return c.rateLimitRepo
}

// getLogDir returns the appropriate log directory for the current OS.
func getLogDir() (string, error) {
	// TODO review if this is the best location for logs
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// TODO proper error
		return "", err
	}

	switch runtime.GOOS {
	case osDarwin:
		return filepath.Join(homeDir, "Library", "Logs", "meow"), nil
	case osWindows:
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
