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
	"fmt"

	"github.com/retran/meowg1k/pkg/migrations"
	"github.com/retran/meowg1k/pkg/ratelimit"
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
	mainDBPath, err := dbPathService.GetMainDBPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get main db path: %w", err)
	}
	dbURL := fmt.Sprintf("file:%s?_foreign_keys=on", mainDBPath)
	db, err := sql.Open("sqlite3", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open main db at %s: %w", mainDBPath, err)
	}

	projectDB := db // TODO: For now, use the same DB for projects

	host := &localHostImpl{
		mainDB:    db,
		projectDB: projectDB,
	}

	if err := host.migrateDB(); err != nil {
		return nil, fmt.Errorf("failed to migrate db: %w", err)
	}

	if err := host.migrateProjectDB(); err != nil {
		return nil, fmt.Errorf("failed to migrate project db: %w", err)
	}

	return host, nil
}

func (h *localHostImpl) GetDB() *sql.DB {
	return h.mainDB
}

func (h *localHostImpl) GetProjectDB() *sql.DB {
	return h.projectDB
}

// getMainDBMigrations collects all migrations for the main database.
func (h *localHostImpl) getMainDBMigrations() []migrations.Migration {
	allMigrations := []migrations.Migration{}

	// Add rate limiting migrations
	allMigrations = append(allMigrations, ratelimit.Migrations...)

	// Future: add other subsystem migrations here
	// allMigrations = append(allMigrations, someother.Migrations...)

	return allMigrations
}

func (h *localHostImpl) migrateDB() error {
	allMigrations := h.getMainDBMigrations()
	if err := migrations.RunMigrations(h.mainDB, allMigrations); err != nil {
		return fmt.Errorf("failed to run main db migrations: %w", err)
	}
	return nil
}

func (h *localHostImpl) migrateProjectDB() error {
	return nil
}

func (h *localHostImpl) Close() error {
	mainDBErr := h.mainDB.Close()
	projectDBErr := h.projectDB.Close()

	return fmt.Errorf("failed to close db: %w, %w", mainDBErr, projectDBErr)
}
