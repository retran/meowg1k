// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/retran/meowg1k/internal/adapters/sqlite/migrations"
)

func TestMigrations_NotEmpty(t *testing.T) {
	if len(Migrations) == 0 {
		t.Fatal("Migrations slice is empty")
	}
}

func TestMigrations_Version2Exists(t *testing.T) {
	found := false
	for _, m := range Migrations {
		if m.Version == 2 {
			found = true
			break
		}
	}
	if !found {
		t.Error("Migration version 2 not found")
	}
}

func TestMigrations_CreateTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Run migrations
	err = migrations.RunMigrations(db, Migrations)
	if err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Verify table exists
	var tableName string
	err = db.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='llm_cache'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("table llm_cache does not exist: %v", err)
	}
	if tableName != "llm_cache" {
		t.Errorf("expected table name 'llm_cache', got %q", tableName)
	}
}

func TestMigrations_TableSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Run migrations
	err = migrations.RunMigrations(db, Migrations)
	if err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Check table schema
	rows, err := db.Query(`PRAGMA table_info(llm_cache)`)
	if err != nil {
		t.Fatalf("failed to get table info: %v", err)
	}
	defer rows.Close()

	type columnInfo struct {
		cid       int
		name      string
		colType   string
		notNull   int
		dfltValue sql.NullString
		pk        int
	}

	var columns []columnInfo
	for rows.Next() {
		var col columnInfo
		err := rows.Scan(&col.cid, &col.name, &col.colType, &col.notNull, &col.dfltValue, &col.pk)
		if err != nil {
			t.Fatalf("failed to scan column info: %v", err)
		}
		columns = append(columns, col)
	}

	expectedColumns := map[string]struct {
		colType string
		notNull bool
		pk      bool
	}{
		"key":        {colType: "TEXT", notNull: false, pk: true},
		"value":      {colType: "TEXT", notNull: true, pk: false},
		"created_at": {colType: "INTEGER", notNull: true, pk: false},
	}

	if len(columns) != len(expectedColumns) {
		t.Errorf("expected %d columns, got %d", len(expectedColumns), len(columns))
	}

	for _, col := range columns {
		expected, ok := expectedColumns[col.name]
		if !ok {
			t.Errorf("unexpected column: %s", col.name)
			continue
		}

		if col.colType != expected.colType {
			t.Errorf("column %s: expected type %s, got %s", col.name, expected.colType, col.colType)
		}

		isPk := col.pk == 1
		if isPk != expected.pk {
			t.Errorf("column %s: expected pk=%v, got pk=%v", col.name, expected.pk, isPk)
		}

		isNotNull := col.notNull == 1
		if isNotNull != expected.notNull {
			t.Errorf("column %s: expected notNull=%v, got notNull=%v", col.name, expected.notNull, isNotNull)
		}
	}
}

func TestMigrations_IndexExists(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Run migrations
	err = migrations.RunMigrations(db, Migrations)
	if err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Verify index exists
	var indexName string
	err = db.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='index' AND name='idx_llm_cache_created_at'
	`).Scan(&indexName)
	if err != nil {
		t.Fatalf("index idx_llm_cache_created_at does not exist: %v", err)
	}
	if indexName != "idx_llm_cache_created_at" {
		t.Errorf("expected index name 'idx_llm_cache_created_at', got %q", indexName)
	}
}

func TestMigrations_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Run migrations twice
	err = migrations.RunMigrations(db, Migrations)
	if err != nil {
		t.Fatalf("first migration run failed: %v", err)
	}

	err = migrations.RunMigrations(db, Migrations)
	if err != nil {
		t.Fatalf("second migration run failed (should be idempotent): %v", err)
	}

	// Verify table still exists and is usable
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM llm_cache`).Scan(&count)
	if err != nil {
		t.Fatalf("table is not usable after repeated migrations: %v", err)
	}
}

func TestMigrations_TableUsable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Run migrations
	err = migrations.RunMigrations(db, Migrations)
	if err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Test inserting data
	_, err = db.Exec(`
		INSERT INTO llm_cache (key, value, created_at)
		VALUES (?, ?, ?)
	`, "test-key", "test-value", 123456789)
	if err != nil {
		t.Fatalf("failed to insert into llm_cache: %v", err)
	}

	// Test querying data
	var key, value string
	var createdAt int64
	err = db.QueryRow(`
		SELECT key, value, created_at FROM llm_cache WHERE key = ?
	`, "test-key").Scan(&key, &value, &createdAt)
	if err != nil {
		t.Fatalf("failed to query llm_cache: %v", err)
	}

	if key != "test-key" || value != "test-value" || createdAt != 123456789 {
		t.Errorf("unexpected data: key=%s, value=%s, created_at=%d", key, value, createdAt)
	}
}
