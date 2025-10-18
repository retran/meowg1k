// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/retran/meowg1k/internal/adapters/sqlite/migrations"
	"github.com/retran/meowg1k/internal/ports"
)

// mockHost is a simple mock implementation of ports.Host for testing.
type mockHost struct {
	db *sql.DB
}

func newMockHost(db *sql.DB) ports.Host {
	return &mockHost{db: db}
}

func (m *mockHost) GetMainDB() (*sql.DB, error) {
	return m.db, nil
}

func (m *mockHost) GetProjectDB() (*sql.DB, error) {
	return m.db, nil
}

func (m *mockHost) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

func setupTestDB(t *testing.T) (*sql.DB, ports.Host) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Run migrations
	if err := migrations.RunMigrations(db, Migrations); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	host := newMockHost(db)
	return db, host
}

func TestNewRepository(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	if repo == nil {
		t.Fatal("NewRepository returned nil")
	}
}

func TestRepository_Get_Success(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Set a value first
	key := "test-key"
	expectedValue := "test-value"
	err := repo.Set(ctx, key, expectedValue)
	if err != nil {
		t.Fatalf("failed to set value: %v", err)
	}

	// Get the value
	value, found, err := repo.Get(ctx, key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected to find value, but got found=false")
	}
	if value != expectedValue {
		t.Errorf("expected value %q, got %q", expectedValue, value)
	}
}

func TestRepository_Get_NotFound(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Get non-existent key
	value, found, err := repo.Get(ctx, "non-existent-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected found=false for non-existent key")
	}
	if value != "" {
		t.Errorf("expected empty value for non-existent key, got %q", value)
	}
}

func TestRepository_Get_NilRepository(t *testing.T) {
	var repo *Repository
	ctx := context.Background()

	_, _, err := repo.Get(ctx, "test-key")
	if err == nil {
		t.Fatal("expected error for nil repository")
	}
	if err.Error() != "repository is nil" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRepository_Get_NilContext(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)

	//nolint:staticcheck // Testing nil context handling
	_, _, err := repo.Get(nil, "test-key")
	if err == nil {
		t.Fatal("expected error for nil context")
	}
	if err.Error() != "context is nil" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRepository_Get_EmptyKey(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	_, _, err := repo.Get(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	if err.Error() != "key cannot be empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRepository_Set_Success(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	key := "test-key"
	value := "test-value"

	err := repo.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the value was set
	gotValue, found, err := repo.Get(ctx, key)
	if err != nil {
		t.Fatalf("failed to get value: %v", err)
	}
	if !found {
		t.Error("value not found after set")
	}
	if gotValue != value {
		t.Errorf("expected value %q, got %q", value, gotValue)
	}
}

func TestRepository_Set_Replace(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	key := "test-key"
	value1 := "value-1"
	value2 := "value-2"

	// Set initial value
	err := repo.Set(ctx, key, value1)
	if err != nil {
		t.Fatalf("failed to set initial value: %v", err)
	}

	// Replace with new value
	err = repo.Set(ctx, key, value2)
	if err != nil {
		t.Fatalf("failed to replace value: %v", err)
	}

	// Verify the value was replaced
	gotValue, found, err := repo.Get(ctx, key)
	if err != nil {
		t.Fatalf("failed to get value: %v", err)
	}
	if !found {
		t.Error("value not found after replace")
	}
	if gotValue != value2 {
		t.Errorf("expected value %q, got %q", value2, gotValue)
	}
}

func TestRepository_Set_NilRepository(t *testing.T) {
	var repo *Repository
	ctx := context.Background()

	err := repo.Set(ctx, "test-key", "test-value")
	if err == nil {
		t.Fatal("expected error for nil repository")
	}
	if err.Error() != "repository is nil" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRepository_Set_NilContext(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)

	//nolint:staticcheck // Testing nil context handling
	err := repo.Set(nil, "test-key", "test-value")
	if err == nil {
		t.Fatal("expected error for nil context")
	}
	if err.Error() != "context is nil" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRepository_Set_EmptyKey(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	err := repo.Set(ctx, "", "test-value")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	if err.Error() != "key cannot be empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRepository_Purge_Success(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Set some entries with different timestamps
	now := time.Now().Unix()

	// Old entry (should be purged)
	_, err := db.ExecContext(ctx, `
		INSERT INTO llm_cache (key, value, created_at)
		VALUES (?, ?, ?)
	`, "old-key", "old-value", now-3600)
	if err != nil {
		t.Fatalf("failed to insert old entry: %v", err)
	}

	// Recent entry (should not be purged)
	err = repo.Set(ctx, "recent-key", "recent-value")
	if err != nil {
		t.Fatalf("failed to set recent entry: %v", err)
	}

	// Purge entries older than 30 minutes
	err = repo.Purge(ctx, 30*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify old entry was purged
	_, found, err := repo.Get(ctx, "old-key")
	if err != nil {
		t.Fatalf("failed to get old entry: %v", err)
	}
	if found {
		t.Error("old entry should have been purged")
	}

	// Verify recent entry still exists
	_, found, err = repo.Get(ctx, "recent-key")
	if err != nil {
		t.Fatalf("failed to get recent entry: %v", err)
	}
	if !found {
		t.Error("recent entry should not have been purged")
	}
}

func TestRepository_Purge_NilRepository(t *testing.T) {
	var repo *Repository
	ctx := context.Background()

	err := repo.Purge(ctx, time.Hour)
	if err == nil {
		t.Fatal("expected error for nil repository")
	}
	if err.Error() != "repository is nil" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRepository_Purge_NilContext(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)

	//nolint:staticcheck // Testing nil context handling
	err := repo.Purge(nil, time.Hour)
	if err == nil {
		t.Fatal("expected error for nil context")
	}
	if err.Error() != "context is nil" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRepository_Purge_InvalidTTL(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	tests := []struct {
		name string
		ttl  time.Duration
	}{
		{"zero ttl", 0},
		{"negative ttl", -time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Purge(ctx, tt.ttl)
			if err == nil {
				t.Fatal("expected error for invalid TTL")
			}
			if err.Error() != "TTL must be positive" {
				t.Errorf("unexpected error message: %v", err)
			}
		})
	}
}

func TestRepository_Purge_EmptyDatabase(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Purge from empty database should not error
	err := repo.Purge(ctx, time.Hour)
	if err != nil {
		t.Fatalf("unexpected error purging empty database: %v", err)
	}
}

func TestRepository_Set_EmptyValue(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	key := "test-key"
	value := ""

	err := repo.Set(ctx, key, value)
	if err != nil {
		t.Fatalf("unexpected error setting empty value: %v", err)
	}

	// Verify empty value was set
	gotValue, found, err := repo.Get(ctx, key)
	if err != nil {
		t.Fatalf("failed to get value: %v", err)
	}
	if !found {
		t.Error("value not found after set")
	}
	if gotValue != value {
		t.Errorf("expected empty value, got %q", gotValue)
	}
}

func TestRepository_MultipleOperations(t *testing.T) {
	db, host := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(host)
	ctx := context.Background()

	// Set multiple values
	entries := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for key, value := range entries {
		if err := repo.Set(ctx, key, value); err != nil {
			t.Fatalf("failed to set %s: %v", key, err)
		}
	}

	// Verify all values
	for key, expectedValue := range entries {
		value, found, err := repo.Get(ctx, key)
		if err != nil {
			t.Fatalf("failed to get %s: %v", key, err)
		}
		if !found {
			t.Errorf("key %s not found", key)
		}
		if value != expectedValue {
			t.Errorf("for key %s: expected %q, got %q", key, expectedValue, value)
		}
	}
}
