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

package migrations

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func TestRunMigrations_NilDB(t *testing.T) {
	migrations := []Migration{
		{Version: 1, Up: func(tx *sql.Tx) error { return nil }},
	}

	err := RunMigrations(nil, migrations)
	if err == nil {
		t.Fatal("expected error for nil database, got nil")
	}
	expectedMsg := "database connection is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestRunMigrations_NilMigrations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	err = RunMigrations(db, nil)
	if err == nil {
		t.Fatal("expected error for nil migrations, got nil")
	}
	expectedMsg := "migrations slice is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestRunMigrations_EmptyMigrations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	migrations := []Migration{}
	err = RunMigrations(db, migrations)
	if err != nil {
		t.Fatalf("unexpected error for empty migrations: %v", err)
	}

	// Verify schema_versions table was created
	var version uint
	err = db.QueryRow("SELECT version FROM schema_versions").Scan(&version)
	if err != nil {
		t.Fatalf("failed to query schema version: %v", err)
	}
	if version != 0 {
		t.Errorf("expected version 0, got %d", version)
	}
}

func TestRunMigrations_SingleMigration(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	migrations := []Migration{
		{
			Version: 1,
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY)")
				return err
			},
		},
	}

	err = RunMigrations(db, migrations)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify migration was applied
	var version uint
	err = db.QueryRow("SELECT version FROM schema_versions").Scan(&version)
	if err != nil {
		t.Fatalf("failed to query schema version: %v", err)
	}
	if version != 1 {
		t.Errorf("expected version 1, got %d", version)
	}

	// Verify table was created
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='test_table'").Scan(&tableName)
	if err != nil {
		t.Fatalf("test_table was not created: %v", err)
	}
}

func TestRunMigrations_MultipleMigrations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	migrations := []Migration{
		{
			Version: 1,
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY)")
				return err
			},
		},
		{
			Version: 2,
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec("CREATE TABLE posts (id INTEGER PRIMARY KEY)")
				return err
			},
		},
	}

	err = RunMigrations(db, migrations)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify final version
	var version uint
	err = db.QueryRow("SELECT version FROM schema_versions").Scan(&version)
	if err != nil {
		t.Fatalf("failed to query schema version: %v", err)
	}
	if version != 2 {
		t.Errorf("expected version 2, got %d", version)
	}

	// Verify both tables were created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('users', 'posts')").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count tables: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 tables, got %d", count)
	}
}

func TestRunMigrations_SkipsAppliedMigrations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	migrations1 := []Migration{
		{
			Version: 1,
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY)")
				return err
			},
		},
	}

	err = RunMigrations(db, migrations1)
	if err != nil {
		t.Fatalf("unexpected error on first run: %v", err)
	}

	// Run again with additional migration
	migrations2 := []Migration{
		{
			Version: 1,
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY)")
				return err
			},
		},
		{
			Version: 2,
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec("CREATE TABLE posts (id INTEGER PRIMARY KEY)")
				return err
			},
		},
	}

	err = RunMigrations(db, migrations2)
	if err != nil {
		t.Fatalf("unexpected error on second run: %v", err)
	}

	// Verify final version
	var version uint
	err = db.QueryRow("SELECT version FROM schema_versions").Scan(&version)
	if err != nil {
		t.Fatalf("failed to query schema version: %v", err)
	}
	if version != 2 {
		t.Errorf("expected version 2, got %d", version)
	}
}

func TestRunMigrations_UnorderedMigrations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Migrations out of order
	migrations := []Migration{
		{
			Version: 3,
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec("CREATE TABLE comments (id INTEGER PRIMARY KEY)")
				return err
			},
		},
		{
			Version: 1,
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY)")
				return err
			},
		},
		{
			Version: 2,
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec("CREATE TABLE posts (id INTEGER PRIMARY KEY)")
				return err
			},
		},
	}

	err = RunMigrations(db, migrations)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify final version
	var version uint
	err = db.QueryRow("SELECT version FROM schema_versions").Scan(&version)
	if err != nil {
		t.Fatalf("failed to query schema version: %v", err)
	}
	if version != 3 {
		t.Errorf("expected version 3, got %d", version)
	}
}

func TestRunMigrations_MigrationError(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	migrations := []Migration{
		{
			Version: 1,
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY)")
				return err
			},
		},
		{
			Version: 2,
			Up: func(tx *sql.Tx) error {
				// Intentional error: invalid SQL
				_, err := tx.Exec("INVALID SQL STATEMENT")
				return err
			},
		},
	}

	err = RunMigrations(db, migrations)
	if err == nil {
		t.Fatal("expected error for invalid migration, got nil")
	}

	// Verify version is still 0 (transaction rolled back)
	var version uint
	err = db.QueryRow("SELECT version FROM schema_versions").Scan(&version)
	if err != nil {
		t.Fatalf("failed to query schema version: %v", err)
	}
	if version != 0 {
		t.Errorf("expected version 0 after rollback, got %d", version)
	}
}

func TestRunMigrations_NilUpFunction(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	migrations := []Migration{
		{
			Version: 1,
			Up:      nil,
		},
	}

	err = RunMigrations(db, migrations)
	if err == nil {
		t.Fatal("expected error for nil Up function, got nil")
	}
}

func TestRunMigrations_InvalidDBPath(t *testing.T) {
	// Try to open database in non-existent directory
	db, err := sql.Open("sqlite3", "/nonexistent/path/db.sqlite")
	if err != nil {
		// This is expected in some cases
		return
	}
	defer db.Close()

	migrations := []Migration{
		{
			Version: 1,
			Up: func(tx *sql.Tx) error {
				_, err := tx.Exec("CREATE TABLE test (id INTEGER)")
				return err
			},
		},
	}

	err = RunMigrations(db, migrations)
	if err == nil {
		// The error might occur during actual execution
		// Just verify we can handle it gracefully
		os.Remove("/nonexistent/path/db.sqlite")
	}
}
