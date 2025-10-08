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

package testutil

import (
	"database/sql"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/retran/meowg1k/internal/db"
	"github.com/retran/meowg1k/pkg/migrations"
	"github.com/retran/meowg1k/pkg/ratelimit"
)

// NewInMemoryDB creates a new in-memory SQLite database for testing.
// The database is isolated and will be destroyed when closed.
func NewInMemoryDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}
	return db, nil
}

// mockDBHost is a mock implementation of db.Host for testing.
// It uses in-memory SQLite databases that are isolated and destroyed after tests.
type mockDBHost struct {
	mainDB    *sql.DB
	projectDB *sql.DB
}

// NewMockDBHost creates a new mock db.Host with in-memory databases for testing.
// The databases are already migrated and ready to use.
func NewMockDBHost() (db.Host, error) {
	mainDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}

	// Run migrations for main DB
	mainMigrations := []migrations.Migration{}
	mainMigrations = append(mainMigrations, ratelimit.Migrations...)

	if err := migrations.RunMigrations(mainDB, mainMigrations); err != nil {
		mainDB.Close()
		return nil, err
	}

	// For tests, use the same in-memory DB for project DB
	projectDB := mainDB

	return &mockDBHost{
		mainDB:    mainDB,
		projectDB: projectDB,
	}, nil
}

// GetDB returns the main database connection.
func (m *mockDBHost) GetDB() *sql.DB {
	return m.mainDB
}

// GetProjectDB returns the project database connection.
func (m *mockDBHost) GetProjectDB() *sql.DB {
	return m.projectDB
}

// Close closes both database connections.
func (m *mockDBHost) Close() error {
	return m.mainDB.Close()
}
