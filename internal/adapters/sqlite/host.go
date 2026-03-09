// Copyright © 2025 The meowg1k Authors.
// SPDX-License-Identifier: Apache-2.0

// Package sqlite provides SQLite database access and manages connections for index, cache, metadata, and rate limiting storage.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/retran/meowg1k/internal/adapters/sqlite/cache"
	"github.com/retran/meowg1k/internal/adapters/sqlite/index"
	"github.com/retran/meowg1k/internal/adapters/sqlite/meta"
	"github.com/retran/meowg1k/internal/adapters/sqlite/migrations"
	"github.com/retran/meowg1k/internal/adapters/sqlite/session"
	"github.com/retran/meowg1k/internal/ports"
)

// DBPathService resolves locations for main and project databases.
type DBPathService interface {
	GetMainDBPath() (string, error)
	GetProjectDBPath() (string, error)
}

type localHostImpl struct {
	mainDB    *sql.DB
	projectDB *sql.DB
}

// NewLocalHost creates a SQLite host using local filesystem paths.
func NewLocalHost(dbPathService DBPathService) (ports.Host, error) {
	if dbPathService == nil {
		return nil, fmt.Errorf("db path service is nil")
	}

	mainDBPath, err := dbPathService.GetMainDBPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get main db path: %w", err)
	}

	mainDB, err := getDB(mainDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get main db: %w", err)
	}

	projectDBPath, err := dbPathService.GetProjectDBPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get project db path: %w", err)
	}

	projectDB, err := getDB(projectDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get project db: %w", err)
	}

	host := &localHostImpl{
		mainDB:    mainDB,
		projectDB: projectDB,
	}

	if err := host.migrateMainDB(); err != nil {
		return nil, fmt.Errorf("failed to migrate main db: %w", err)
	}

	if err := host.migrateProjectDB(); err != nil {
		return nil, fmt.Errorf("failed to migrate project db: %w", err)
	}

	return host, nil
}

func (h *localHostImpl) GetMainDB() (*sql.DB, error) {
	if h == nil {
		return nil, fmt.Errorf("host is nil")
	}

	return h.mainDB, nil
}

func (h *localHostImpl) GetProjectDB() (*sql.DB, error) {
	if h == nil {
		return nil, fmt.Errorf("host is nil")
	}

	return h.projectDB, nil
}

func getDB(path string) (*sql.DB, error) {
	// _busy_timeout and cache=shared improve multi-client access.
	dbURL := fmt.Sprintf("file:%s?_foreign_keys=on&_busy_timeout=30000&cache=shared", path)
	db, err := sql.Open("sqlite3", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open main db at %s: %w", path, err)
	}

	// SQLite works best with limited writers in WAL mode.
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(0)

	ctx := context.Background()

	// Enable Write-Ahead Logging for better multi-client access.
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// NORMAL synchronous mode is safe with WAL and gives better performance.
	if _, err := db.ExecContext(ctx, "PRAGMA synchronous=NORMAL"); err != nil {
		return nil, fmt.Errorf("failed to set synchronous mode: %w", err)
	}

	if _, err := db.ExecContext(ctx, "PRAGMA cache_size=-64000"); err != nil { // 64MB cache
		return nil, fmt.Errorf("failed to set cache size: %w", err)
	}

	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=30000"); err != nil {
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	if _, err := db.ExecContext(ctx, "PRAGMA wal_autocheckpoint=1000"); err != nil {
		return nil, fmt.Errorf("failed to set wal_autocheckpoint: %w", err)
	}

	// WAL supports multiple concurrent readers with a single writer.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)

	return db, nil
}

// getMainDBMigrations collects all migrations for the main database.
func (h *localHostImpl) getMainDBMigrations() ([]migrations.Migration, error) {
	if h == nil {
		return nil, fmt.Errorf("host is nil")
	}

	allMigrations := make([]migrations.Migration, 0, len(cache.Migrations))

	allMigrations = append(allMigrations, cache.Migrations...)

	// Future: add other subsystem migrations here
	// allMigrations = append(allMigrations, someother.Migrations...)

	return allMigrations, nil
}

func (h *localHostImpl) getProjectDBMigrations() ([]migrations.Migration, error) {
	if h == nil {
		return nil, fmt.Errorf("host is nil")
	}

	allMigrations := make([]migrations.Migration, 0, len(meta.Migrations)+len(index.Migrations)+len(session.Migrations))

	allMigrations = append(allMigrations, meta.Migrations...)
	allMigrations = append(allMigrations, index.Migrations...)
	allMigrations = append(allMigrations, session.Migrations...)

	// Future: add other subsystem migrations here
	// allMigrations = append(allMigrations, someother.Migrations...)

	return allMigrations, nil
}

func (h *localHostImpl) migrateMainDB() error {
	if h == nil {
		return fmt.Errorf("host is nil")
	}

	allMigrations, err := h.getMainDBMigrations()
	if err != nil {
		return fmt.Errorf("failed to get main db migrations: %w", err)
	}

	if err := migrations.RunMigrations(h.mainDB, allMigrations); err != nil {
		return fmt.Errorf("failed to run main db migrations: %w", err)
	}

	return nil
}

func (h *localHostImpl) migrateProjectDB() error {
	if h == nil {
		return fmt.Errorf("host is nil")
	}

	allMigrations, err := h.getProjectDBMigrations()
	if err != nil {
		return fmt.Errorf("failed to get project db migrations: %w", err)
	}

	if err := migrations.RunMigrations(h.projectDB, allMigrations); err != nil {
		return fmt.Errorf("failed to run project db migrations: %w", err)
	}

	return nil
}

func (h *localHostImpl) Close() error {
	if h == nil {
		return fmt.Errorf("host is nil")
	}

	var errs []error

	if h.mainDB != nil {
		if err := h.mainDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close main db: %w", err))
		}
	}

	// Only close projectDB if it's different from mainDB
	if h.projectDB != nil && h.projectDB != h.mainDB {
		if err := h.projectDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close project db: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to close db: %v", errs)
	}

	return nil
}
