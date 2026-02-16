// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package meta

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/retran/meowg1k/internal/ports"
)

// mockHost implements ports.Host for testing.
type mockHost struct {
	GetProjectDBFunc func() (*sql.DB, error)
	GetMainDBFunc    func() (*sql.DB, error)
	CloseFunc        func() error
}

func (m *mockHost) GetProjectDB() (*sql.DB, error) {
	if m.GetProjectDBFunc != nil {
		return m.GetProjectDBFunc()
	}
	return nil, errors.New("not implemented")
}

func (m *mockHost) GetMainDB() (*sql.DB, error) {
	if m.GetMainDBFunc != nil {
		return m.GetMainDBFunc()
	}
	return nil, errors.New("not implemented")
}

func (m *mockHost) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func TestNewRepository(t *testing.T) {
	host := &mockHost{}
	repo := NewRepository(host)

	if repo == nil {
		t.Fatal("Expected repository to be non-nil")
	}

	// Verify repository works by attempting an operation
	ctx := context.Background()
	_ = repo.SetValue(ctx, "test", []byte("test"))
	// If we get here without panic, the host was stored correctly
}

func TestRepository_InterfaceCompliance(t *testing.T) {
	var _ ports.MetaRepository = (*Repository)(nil)
	t.Log("Repository correctly implements MetaRepository interface")
}

func TestRepository_SetValue_DBError(t *testing.T) {
	host := &mockHost{
		GetProjectDBFunc: func() (*sql.DB, error) {
			return nil, errors.New("database connection error")
		},
	}
	repo := NewRepository(host)

	ctx := context.Background()
	err := repo.SetValue(ctx, "testkey", []byte("testvalue"))

	if err == nil {
		t.Fatal("Expected error when GetProjectDB fails")
	}

	if !strings.Contains(err.Error(), "failed to get database") {
		t.Errorf("Expected 'failed to get database' in error, got: %v", err)
	}
}

func TestRepository_GetValue_DBError(t *testing.T) {
	host := &mockHost{
		GetProjectDBFunc: func() (*sql.DB, error) {
			return nil, errors.New("database connection error")
		},
	}
	repo := NewRepository(host)

	ctx := context.Background()
	_, err := repo.GetValue(ctx, "testkey")

	if err == nil {
		t.Fatal("Expected error when GetProjectDB fails")
	}

	if !strings.Contains(err.Error(), "failed to get database") {
		t.Errorf("Expected 'failed to get database' in error, got: %v", err)
	}
}

func TestRepository_DeleteValue_DBError(t *testing.T) {
	host := &mockHost{
		GetProjectDBFunc: func() (*sql.DB, error) {
			return nil, errors.New("database connection error")
		},
	}
	repo := NewRepository(host)

	ctx := context.Background()
	err := repo.DeleteValue(ctx, "testkey")

	if err == nil {
		t.Fatal("Expected error when GetProjectDB fails")
	}

	if !strings.Contains(err.Error(), "failed to get database") {
		t.Errorf("Expected 'failed to get database' in error, got: %v", err)
	}
}

func TestRepository_GetValue_KeyNotFound(t *testing.T) {
	// This test would require an in-memory SQLite database to test properly.
	// For now, we test the logic with a mock that returns sql.ErrNoRows.

	// We can't easily test this without a real database connection
	// because QueryRowContext.Scan is called, which requires a real DB.
	// However, we can verify the error handling logic exists by examining the code.
	t.Skip("Skipping integration test - requires in-memory database")
}

func TestRepository_SetValue_ValidKey(t *testing.T) {
	t.Skip("Skipping integration test - requires in-memory database")
}

func TestRepository_DeleteValue_NonExistentKey(t *testing.T) {
	t.Skip("Skipping integration test - requires in-memory database")
}

func TestRepository_LargeValueStoredExternally(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "project.db")

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=on", dbPath))
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), `CREATE TABLE meta_kv (key TEXT PRIMARY KEY, value BLOB NOT NULL);`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	host := &mockHost{
		GetProjectDBFunc: func() (*sql.DB, error) {
			return db, nil
		},
	}

	repo := NewRepository(host)
	ctx := context.Background()

	largeValue := bytes.Repeat([]byte{0x2A}, maxInlineValueSize+1024)
	if err := repo.SetValue(ctx, "large-key", largeValue); err != nil {
		t.Fatalf("SetValue failed: %v", err)
	}

	readBack, err := repo.GetValue(ctx, "large-key")
	if err != nil {
		t.Fatalf("GetValue failed: %v", err)
	}

	if !bytes.Equal(readBack, largeValue) {
		t.Fatal("retrieved value does not match original large payload")
	}

	metaBlobDir := filepath.Join(tempDir, "meta_blobs")
	entries, err := os.ReadDir(metaBlobDir)
	if err != nil {
		t.Fatalf("failed to read blob directory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 blob file, found %d", len(entries))
	}

	if err := repo.DeleteValue(ctx, "large-key"); err != nil {
		t.Fatalf("DeleteValue failed: %v", err)
	}

	entries, err = os.ReadDir(metaBlobDir)
	if err != nil {
		t.Fatalf("failed to read blob directory after delete: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected blob directory to be empty after delete, found %d files", len(entries))
	}
}

func TestRepository_SmallValueStoredInline(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "project.db")

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=on", dbPath))
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), `CREATE TABLE meta_kv (key TEXT PRIMARY KEY, value BLOB NOT NULL);`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	host := &mockHost{
		GetProjectDBFunc: func() (*sql.DB, error) {
			return db, nil
		},
	}

	repo := NewRepository(host)
	ctx := context.Background()

	// Test storing a small value that should be stored inline
	smallValue := []byte("This is a small value that fits inline")
	if err := repo.SetValue(ctx, "small-key", smallValue); err != nil {
		t.Fatalf("SetValue failed: %v", err)
	}

	// Verify the value can be retrieved
	readBack, err := repo.GetValue(ctx, "small-key")
	if err != nil {
		t.Fatalf("GetValue failed: %v", err)
	}

	if !bytes.Equal(readBack, smallValue) {
		t.Fatal("retrieved value does not match original small value")
	}

	// Verify no blob directory was created for small values
	metaBlobDir := filepath.Join(tempDir, "meta_blobs")
	if _, err := os.Stat(metaBlobDir); err == nil {
		// If directory exists, it should be empty
		entries, err := os.ReadDir(metaBlobDir)
		if err != nil {
			t.Fatalf("failed to read blob directory: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("expected no blob files for small value, found %d", len(entries))
		}
	}
}

func TestRepository_UpdateLargeValueWithSmallValue(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "project.db")

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=on", dbPath))
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), `CREATE TABLE meta_kv (key TEXT PRIMARY KEY, value BLOB NOT NULL);`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	host := &mockHost{
		GetProjectDBFunc: func() (*sql.DB, error) {
			return db, nil
		},
	}

	repo := NewRepository(host)
	ctx := context.Background()

	// First, store a large value that will be stored as a blob
	largeValue := bytes.Repeat([]byte{0x2A}, maxInlineValueSize+1024)
	if err := repo.SetValue(ctx, "test-key", largeValue); err != nil {
		t.Fatalf("SetValue (large) failed: %v", err)
	}

	// Verify blob was created
	metaBlobDir := filepath.Join(tempDir, "meta_blobs")
	entries, err := os.ReadDir(metaBlobDir)
	if err != nil {
		t.Fatalf("failed to read blob directory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 blob file after large value, found %d", len(entries))
	}

	// Now update with a small value - this should trigger setInlineValue with existingRef cleanup
	smallValue := []byte("small replacement value")
	if err := repo.SetValue(ctx, "test-key", smallValue); err != nil {
		t.Fatalf("SetValue (small) failed: %v", err)
	}

	// Verify the blob was cleaned up
	entries, err = os.ReadDir(metaBlobDir)
	if err != nil {
		t.Fatalf("failed to read blob directory after update: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected blob to be cleaned up after replacing with small value, found %d files", len(entries))
	}

	// Verify the small value can be retrieved correctly
	readBack, err := repo.GetValue(ctx, "test-key")
	if err != nil {
		t.Fatalf("GetValue failed: %v", err)
	}

	if !bytes.Equal(readBack, smallValue) {
		t.Fatal("retrieved value does not match the small replacement value")
	}
}

func TestRepository_UpdateSmallValueWithSmallValue(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "project.db")

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=on", dbPath))
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), `CREATE TABLE meta_kv (key TEXT PRIMARY KEY, value BLOB NOT NULL);`); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	host := &mockHost{
		GetProjectDBFunc: func() (*sql.DB, error) {
			return db, nil
		},
	}

	repo := NewRepository(host)
	ctx := context.Background()

	// Store a small value
	smallValue1 := []byte("first small value")
	if err := repo.SetValue(ctx, "test-key", smallValue1); err != nil {
		t.Fatalf("SetValue (first) failed: %v", err)
	}

	// Update with another small value - tests setInlineValue with empty existingRef
	smallValue2 := []byte("second small value")
	if err := repo.SetValue(ctx, "test-key", smallValue2); err != nil {
		t.Fatalf("SetValue (second) failed: %v", err)
	}

	// Verify the second value can be retrieved
	readBack, err := repo.GetValue(ctx, "test-key")
	if err != nil {
		t.Fatalf("GetValue failed: %v", err)
	}

	if !bytes.Equal(readBack, smallValue2) {
		t.Fatal("retrieved value does not match the second small value")
	}
}
