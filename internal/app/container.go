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

// Package app contains the main application struct and orchestrates cross-cutting adapters.
package app

import (
	"context"
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

	"github.com/retran/meowg1k/internal/adapters/command"
	"github.com/retran/meowg1k/internal/adapters/config"
	"github.com/retran/meowg1k/internal/adapters/httpclient"
	"github.com/retran/meowg1k/internal/adapters/output"
	"github.com/retran/meowg1k/internal/adapters/sqlite"
	"github.com/retran/meowg1k/internal/adapters/sqlite/cache"
	"github.com/retran/meowg1k/internal/adapters/sqlite/path"
	"github.com/retran/meowg1k/internal/adapters/sqlite/ratelimit"
	"github.com/retran/meowg1k/internal/adapters/tracelog"
	"github.com/retran/meowg1k/internal/adapters/workspace"
	"github.com/retran/meowg1k/internal/core/shutdown"
	domainOutput "github.com/retran/meowg1k/internal/domain/output"
	"github.com/retran/meowg1k/internal/ports"
)

// Writer writes output to the user (used in activities).
type Writer interface {
	Print(content string) error
	PrintLine(content string) error
	Printf(format string, args ...any) error
	Flush() error
}

// Container is the main application struct that holds all cross-cutting adapters.
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
	OutputService Writer

	// TraceLogger provides context-aware trace logging for sessions.
	TraceLogger *tracelog.Logger

	// dbHost provides access to database connections (lazy initialized)
	dbHost ports.Host

	// dbPathService provides database path management
	dbPathService *path.Service

	// rateLimitRepo is the repository for rate limiting state (lazy initialized)
	rateLimitRepo *ratelimit.Repository

	// cacheRepo is the repository for LLM response caching (lazy initialized)
	cacheRepo ports.CacheRepository

	// httpClientService provides shared HTTP client for all gateways
	httpClientService ports.HTTPClientService

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
		return fmt.Errorf("log filename contains invalid characters or path separators: %s", fileName)
	}

	cleanLogDir := filepath.Clean(logDir)
	logPath := filepath.Join(cleanLogDir, fileName)

	if !strings.HasPrefix(logPath, cleanLogDir) {
		return fmt.Errorf("log path is outside log directory: %s is outside %s", logPath, cleanLogDir)
	}

	return nil
}

// AppContainerKey is the context key type for storing and retrieving the Container instance.
type appContainerKey struct{}

// AppContainerKey is the context key for storing and retrieving the Container instance.
var AppContainerKey = appContainerKey{}

// NewAppContainer initializes the main application struct with all necessary adapters.
func NewAppContainer(cmd *cobra.Command) (*Container, error) {
	if cmd == nil {
		return nil, fmt.Errorf("cobra command is nil")
	}

	container := &Container{}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	logDir, err := getLogDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get log directory: %w", err)
	}

	if err = os.MkdirAll(logDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	if err = validateLogPath(logDir, logFileName); err != nil {
		return nil, fmt.Errorf("invalid log path: %w", err)
	}

	root, err := os.OpenRoot(logDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open root directory: %w", err)
	}
	defer root.Close()

	logFile, err := root.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	shutdownService := shutdown.NewService(logger, ctx, 10*time.Second)

	err = shutdownService.Register(func(ctx context.Context) error {
		if logFile != nil {
			if err = logFile.Close(); err != nil {
				return fmt.Errorf("failed to close log file: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register log file shutdown callback: %w", err)
	}

	commandService, err := command.NewService(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to create command service: %w", err)
	}

	workspaceService := workspace.NewService(commandService)

	configService, err := config.NewService(commandService, workspaceService)
	if err != nil {
		return nil, fmt.Errorf("failed to create config service: %w", err)
	}

	outputService := output.NewService(domainOutput.Stdout)
	err = shutdownService.Register(func(ctx context.Context) error {
		return outputService.Flush()
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register output service shutdown callback: %w", err)
	}

	// Initialize trace logger
	traceLogger := tracelog.NewLogger(workspaceService)
	err = shutdownService.Register(func(ctx context.Context) error {
		return traceLogger.Close()
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register trace logger shutdown callback: %w", err)
	}

	dbPathService, err := path.NewService(workspaceService)
	if err != nil {
		return nil, fmt.Errorf("failed to create db path service: %w", err)
	}

	// Initialize HTTP client service
	httpClientService, err := httpclient.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client service: %w", err)
	}

	// Register HTTP client cleanup on shutdown
	err = shutdownService.Register(func(ctx context.Context) error {
		return httpClientService.Close()
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register HTTP client service shutdown callback: %w", err)
	}

	container.Logger = logger
	container.ShutdownService = shutdownService
	container.CommandService = commandService
	container.ConfigService = configService
	container.OutputService = outputService
	container.TraceLogger = traceLogger
	container.dbPathService = dbPathService
	container.httpClientService = httpClientService

	shutdownCtx := context.WithValue(shutdownService.Context(), AppContainerKey, container)
	cmd.SetContext(shutdownCtx)

	return container, nil
}

// initDB initializes the database host and rate limit repository if not already initialized.
// This method is thread-safe and will only initialize once.
func (c *Container) initDB() error {
	var initErr error
	c.dbInitOnce.Do(func() {
		dbHost, err := sqlite.NewLocalHost(c.dbPathService)
		if err != nil {
			initErr = fmt.Errorf("failed to initialize database host: %w", err)
			return
		}

		mainDB, err := dbHost.GetDB()
		if err != nil {
			initErr = fmt.Errorf("failed to get main database: %w", err)
			return
		}

		rateLimitRepo := ratelimit.NewRepository(mainDB)
		cacheRepo := cache.NewRepository(mainDB)

		// Purge expired cache entries on startup if caching is configured
		config, err := c.ConfigService.Get()
		if err == nil && config != nil && config.Cache != nil && config.Cache.TTL > 0 {
			ctx := c.ShutdownService.Context()
			if err := cacheRepo.Purge(ctx, config.Cache.TTL); err != nil {
				c.Logger.Error("failed to purge expired cache entries on startup", "error", err)
				// Don't fail initialization - just log the error
			}
		}

		if err := c.ShutdownService.Register(func(ctx context.Context) error {
			if err := dbHost.Close(); err != nil {
				return fmt.Errorf("failed to close database host: %w", err)
			}
			return nil
		}); err != nil {
			initErr = fmt.Errorf("failed to register database shutdown callback: %w", err)
			return
		}

		c.dbHost = dbHost
		c.rateLimitRepo = rateLimitRepo
		c.cacheRepo = cacheRepo
	})
	return initErr
}

// GetRateLimitRepo returns the rate limit repository, initializing the database if needed.
func (c *Container) GetRateLimitRepo() *ratelimit.Repository {
	// If already set (e.g., in tests), return it directly
	if c.rateLimitRepo != nil {
		return c.rateLimitRepo
	}
	if err := c.initDB(); err != nil {
		c.Logger.Error("failed to initialize database", "error", err)
		return nil
	}
	return c.rateLimitRepo
}

// GetCacheRepo returns the cache repository, initializing the database if needed.
func (c *Container) GetCacheRepo() ports.CacheRepository {
	// If already set (e.g., in tests), return it directly
	if c.cacheRepo != nil {
		return c.cacheRepo
	}
	if err := c.initDB(); err != nil {
		c.Logger.Error("failed to initialize database", "error", err)
		return nil
	}
	return c.cacheRepo
}

// GetHTTPClientService returns the HTTP client service.
func (c *Container) GetHTTPClientService() ports.HTTPClientService {
	return c.httpClientService
}

// getLogDir returns the appropriate log directory for the current OS.
func getLogDir() (string, error) {
	// TODO review if this is the best location for logs
	homeDir, err := os.UserHomeDir()
	if err != nil {
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
