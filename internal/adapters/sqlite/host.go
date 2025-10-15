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

// Package sqlite provides SQLite database access and manages connections for index, cache, metadata, and rate limiting storage.
package sqlite

import (
	"database/sql"
	"fmt"

	"github.com/retran/meowg1k/internal/adapters/sqlite/cache"
	"github.com/retran/meowg1k/internal/adapters/sqlite/index"
	"github.com/retran/meowg1k/internal/adapters/sqlite/meta"
	"github.com/retran/meowg1k/internal/adapters/sqlite/migrations"
	"github.com/retran/meowg1k/internal/adapters/sqlite/ratelimit"
	"github.com/retran/meowg1k/internal/ports"
)

type DBPathService interface {
	GetMainDBPath() (string, error)
	GetProjectDBPath() (string, error)
}

type localHostImpl struct {
	mainDB    *sql.DB
	projectDB *sql.DB
}

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
	// Add busy_timeout and cache parameters for better concurrent access
	// Increased timeout to 30 seconds to handle high concurrency
	dbURL := fmt.Sprintf("file:%s?_foreign_keys=on&_busy_timeout=30000&cache=shared", path)
	db, err := sql.Open("sqlite3", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open main db at %s: %w", path, err)
	}

	// Configure connection pool to reduce contention
	// SQLite works best with limited concurrent writers in WAL mode
	db.SetMaxOpenConns(5)    // Limit total connections to reduce lock contention
	db.SetMaxIdleConns(2)    // Keep some connections ready
	db.SetConnMaxLifetime(0) // Reuse connections indefinitely

	// Enable Write-Ahead Logging for better concurrent access
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Set synchronous mode to NORMAL for better performance with WAL
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		return nil, fmt.Errorf("failed to set synchronous mode: %w", err)
	}

	// Increase cache size for better performance
	if _, err := db.Exec("PRAGMA cache_size=-64000"); err != nil { // 64MB cache
		return nil, fmt.Errorf("failed to set cache size: %w", err)
	}

	// Set busy timeout at PRAGMA level as well for extra safety
	if _, err := db.Exec("PRAGMA busy_timeout=30000"); err != nil {
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	// Set WAL autocheckpoint to run more frequently
	if _, err := db.Exec("PRAGMA wal_autocheckpoint=1000"); err != nil {
		return nil, fmt.Errorf("failed to set wal_autocheckpoint: %w", err)
	}

	// Configure connection pool for better concurrent access
	// SQLite with WAL can handle multiple readers, but only one writer at a time
	db.SetMaxOpenConns(10)   // Allow multiple connections
	db.SetMaxIdleConns(5)    // Keep some connections idle
	db.SetConnMaxLifetime(0) // No limit on connection lifetime
	db.SetConnMaxIdleTime(0) // No limit on idle time

	return db, nil
}

// getMainDBMigrations collects all migrations for the main database.
func (h *localHostImpl) getMainDBMigrations() ([]migrations.Migration, error) {
	if h == nil {
		return nil, fmt.Errorf("host is nil")
	}

	allMigrations := []migrations.Migration{}

	allMigrations = append(allMigrations, ratelimit.Migrations...)
	allMigrations = append(allMigrations, cache.Migrations...)

	// Future: add other subsystem migrations here
	// allMigrations = append(allMigrations, someother.Migrations...)

	return allMigrations, nil
}

func (h *localHostImpl) getProjectDBMigrations() ([]migrations.Migration, error) {
	if h == nil {
		return nil, fmt.Errorf("host is nil")
	}

	allMigrations := []migrations.Migration{}

	allMigrations = append(allMigrations, meta.Migrations...)
	allMigrations = append(allMigrations, index.Migrations...)

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
