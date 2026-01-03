// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package app provides the application container that orchestrates all services and dependencies.
// It manages initialization, configuration, and lifecycle of adapters and services.
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

	_ "github.com/ncruces/go-sqlite3/driver" // Register SQLite driver.
	_ "github.com/ncruces/go-sqlite3/embed"  // Embed SQLite shared library.
	"github.com/spf13/cobra"

	"github.com/retran/meowg1k/internal/adapters/command"
	adapterConfig "github.com/retran/meowg1k/internal/adapters/config"
	"github.com/retran/meowg1k/internal/adapters/httpclient"
	"github.com/retran/meowg1k/internal/adapters/output"
	"github.com/retran/meowg1k/internal/adapters/sqlite"
	"github.com/retran/meowg1k/internal/adapters/sqlite/cache"
	"github.com/retran/meowg1k/internal/adapters/sqlite/path"
	"github.com/retran/meowg1k/internal/adapters/sqlite/ratelimit"
	"github.com/retran/meowg1k/internal/adapters/tracelog"
	"github.com/retran/meowg1k/internal/adapters/workspace"
	"github.com/retran/meowg1k/internal/core/shutdown"
	domainConfig "github.com/retran/meowg1k/internal/domain/config"
	domainOutput "github.com/retran/meowg1k/internal/domain/output"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
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
	ConfigService *adapterConfig.Service

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

// validateLogPath validates the log path to prevent directory traversal attacks.
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

	logger, logFile, err := buildLogger()
	if err != nil {
		return nil, err
	}

	shutdownService, err := createShutdownService(ctx, logger, logFile)
	if err != nil {
		return nil, err
	}

	commandService, err := command.NewService(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to create command service: %w", err)
	}

	workspaceService := workspace.NewService(commandService)

	configService, err := adapterConfig.NewService(commandService, workspaceService)
	if err != nil {
		return nil, fmt.Errorf("failed to create config service: %w", err)
	}

	outputService := output.NewService(domainOutput.Stdout)
	if err := registerOutputShutdown(shutdownService, outputService); err != nil {
		return nil, err
	}

	traceLogger := tracelog.NewLogger(workspaceService)
	if err := registerTraceShutdown(shutdownService, traceLogger); err != nil {
		return nil, err
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
	if err := registerHTTPClientShutdown(shutdownService, httpClientService); err != nil {
		return nil, err
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

func buildLogger() (*slog.Logger, *os.File, error) {
	logDir, err := getLogDir()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get log directory: %w", err)
	}

	if err = os.MkdirAll(logDir, 0o750); err != nil {
		return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	if err = validateLogPath(logDir, logFileName); err != nil {
		return nil, nil, fmt.Errorf("invalid log path: %w", err)
	}

	root, err := os.OpenRoot(logDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open root directory: %w", err)
	}
	defer func() { _ = root.Close() }() //nolint:errcheck // Defer close errors are not critical

	logFile, err := root.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log file: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	return logger, logFile, nil
}

func createShutdownService(ctx context.Context, logger *slog.Logger, logFile *os.File) (*shutdown.Service, error) {
	shutdownService := shutdown.NewService(ctx, logger, 10*time.Second)
	err := shutdownService.Register(func(_ context.Context) error {
		if logFile == nil {
			return nil
		}
		if err := logFile.Close(); err != nil {
			return fmt.Errorf("failed to close log file: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register log file shutdown callback: %w", err)
	}
	return shutdownService, nil
}

func registerOutputShutdown(shutdownService *shutdown.Service, outputService *output.Service) error {
	if err := shutdownService.Register(func(_ context.Context) error {
		return outputService.Flush()
	}); err != nil {
		return fmt.Errorf("failed to register output service shutdown callback: %w", err)
	}
	return nil
}

func registerTraceShutdown(shutdownService *shutdown.Service, traceLogger *tracelog.Logger) error {
	if err := shutdownService.Register(func(_ context.Context) error {
		return traceLogger.Close()
	}); err != nil {
		return fmt.Errorf("failed to register trace logger shutdown callback: %w", err)
	}
	return nil
}

func registerHTTPClientShutdown(shutdownService *shutdown.Service, httpClientService ports.HTTPClientService) error {
	if err := shutdownService.Register(func(_ context.Context) error {
		return httpClientService.Close()
	}); err != nil {
		return fmt.Errorf("failed to register HTTP client service shutdown callback: %w", err)
	}
	return nil
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

		rateLimitRepo := ratelimit.NewRepository(dbHost)
		cacheRepo := cache.NewRepository(dbHost)

		// Purge expired cache entries on startup if caching is configured
		cfg, err := c.ConfigService.Get()
		if err == nil && cfg != nil {
			cacheTTL := maxPresetCacheTTL(cfg)
			if cacheTTL > 0 {
				ctx := c.ShutdownService.Context()
				if err := cacheRepo.Purge(ctx, cacheTTL); err != nil {
					c.Logger.Error("failed to purge expired cache entries on startup", "error", err)
					// Don't fail initialization - just log the error
				}
			}
		}

		if err := c.ShutdownService.Register(func(_ context.Context) error {
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

func maxPresetCacheTTL(cfg *domainConfig.Config) time.Duration {
	if cfg == nil || cfg.Presets == nil {
		return 0
	}

	var maxTTL time.Duration
	for _, preset := range cfg.Presets {
		if preset == nil || preset.Cache == nil || !preset.Cache.Enabled {
			continue
		}
		if preset.Cache.TTL > maxTTL {
			maxTTL = preset.Cache.TTL
		}
	}

	return maxTTL
}

// GetHTTPClientService returns the HTTP client service.
func (c *Container) GetHTTPClientService() ports.HTTPClientService {
	return c.httpClientService
}

// GetSilentFlag returns the silent flag from the command service.
func (c *Container) GetSilentFlag() (bool, error) {
	flag, err := c.CommandService.GetSilentFlag()
	if err != nil {
		return false, fmt.Errorf("failed to get silent flag: %w", err)
	}
	return flag, nil
}

// GetExecutor returns a new executor with the default configuration.
func (c *Container) GetExecutor() executor.Executor {
	return executor.NewExecutor(runtime.NumCPU() * 2)
}

// getLogDir returns the appropriate log directory for the current OS.
func getLogDir() (string, error) {
	// TODO review if this is the best location for logs
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
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
