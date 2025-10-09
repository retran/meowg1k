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

package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/retran/meowg1k/pkg/migrations"
	"github.com/retran/meowg1k/pkg/ratelimit"
)

var (
	// ErrDBPathServiceIsNil indicates that the DBPathService is nil.
	ErrDBPathServiceIsNil = errors.New("db path service is nil")
	// ErrHostIsNil indicates that the host is nil.
	ErrHostIsNil = errors.New("host is nil")
)

// DBPathService defines the interface for determining database paths.
type DBPathService interface {
	GetMainDBPath() (string, error)
}

type localHostImpl struct {
	mainDB    *sql.DB
	projectDB *sql.DB
}

// NewLocalHost creates a new local host with databases using the provided path service.
func NewLocalHost(dbPathService DBPathService) (Host, error) {
	if dbPathService == nil {
		return nil, ErrDBPathServiceIsNil
	}

	mainDBPath, err := dbPathService.GetMainDBPath()
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to get main db path: %w", err)
	}

	dbURL := fmt.Sprintf("file:%s?_foreign_keys=on", mainDBPath)
	db, err := sql.Open("sqlite3", dbURL)
	if err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to open main db at %s: %w", mainDBPath, err)
	}

	projectDB := db // TODO: For now, use the same DB for projects

	host := &localHostImpl{
		mainDB:    db,
		projectDB: projectDB,
	}

	if err := host.migrateDB(); err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to migrate db: %w", err)
	}

	if err := host.migrateProjectDB(); err != nil {
		// TODO proper error
		return nil, fmt.Errorf("failed to migrate project db: %w", err)
	}

	return host, nil
}

func (h *localHostImpl) GetDB() (*sql.DB, error) {
	if h == nil {
		return nil, ErrHostIsNil
	}

	return h.mainDB, nil
}

func (h *localHostImpl) GetProjectDB() (*sql.DB, error) {
	if h == nil {
		return nil, ErrHostIsNil
	}

	return h.projectDB, nil
}

// getMainDBMigrations collects all migrations for the main database.
func (h *localHostImpl) getMainDBMigrations() ([]migrations.Migration, error) {
	if h == nil {
		return nil, ErrHostIsNil
	}

	allMigrations := []migrations.Migration{}

	// Add rate limiting migrations
	allMigrations = append(allMigrations, ratelimit.Migrations...)

	// Future: add other subsystem migrations here
	// allMigrations = append(allMigrations, someother.Migrations...)

	return allMigrations, nil
}

func (h *localHostImpl) migrateDB() error {
	if h == nil {
		return ErrHostIsNil
	}

	allMigrations, err := h.getMainDBMigrations()
	if err != nil {
		// TODO proper error
		return fmt.Errorf("failed to get main db migrations: %w", err)
	}

	if err := migrations.RunMigrations(h.mainDB, allMigrations); err != nil {
		// TODO proper error
		return fmt.Errorf("failed to run main db migrations: %w", err)
	}

	return nil
}

func (h *localHostImpl) migrateProjectDB() error {
	if h == nil {
		return ErrHostIsNil
	}
	// No migrations for project DB yet
	return nil
}

func (h *localHostImpl) Close() error {
	if h == nil {
		return ErrHostIsNil
	}

	var errs []error

	if h.mainDB != nil {
		if err := h.mainDB.Close(); err != nil {
			// TODO proper error
			errs = append(errs, fmt.Errorf("failed to close main db: %w", err))
		}
	}

	// Only close projectDB if it's different from mainDB
	if h.projectDB != nil && h.projectDB != h.mainDB {
		if err := h.projectDB.Close(); err != nil {
			// TODO proper error
			errs = append(errs, fmt.Errorf("failed to close project db: %w", err))
		}
	}

	if len(errs) > 0 {
		// TODO proper error
		return fmt.Errorf("failed to close db: %v", errs)
	}

	return nil
}
