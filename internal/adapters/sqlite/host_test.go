// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

// mockDBPathService is a mock implementation of DBPathService for testing.
type mockDBPathService struct {
	getMainDBPathFunc    func() (string, error)
	getProjectDBPathFunc func() (string, error)
}

func (m *mockDBPathService) GetMainDBPath() (string, error) {
	if m.getMainDBPathFunc != nil {
		return m.getMainDBPathFunc()
	}
	return "", nil
}

func (m *mockDBPathService) GetProjectDBPath() (string, error) {
	if m.getProjectDBPathFunc != nil {
		return m.getProjectDBPathFunc()
	}
	return "", nil
}

func TestNewLocalHost_NilDBPathService(t *testing.T) {
	_, err := NewLocalHost(nil)
	if err == nil {
		t.Fatal("expected error for nil db path service, got nil")
	}
	expectedMsg := "db path service is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestNewLocalHost_DBPathServiceError(t *testing.T) {
	mock := &mockDBPathService{
		getMainDBPathFunc: func() (string, error) {
			return "", os.ErrNotExist
		},
	}

	_, err := NewLocalHost(mock)
	if err == nil {
		t.Fatal("expected error when db path service returns error, got nil")
	}
}

func TestNewLocalHost_InvalidDBPath(t *testing.T) {
	mock := &mockDBPathService{
		getMainDBPathFunc: func() (string, error) {
			return "/invalid/path/to/nonexistent/db.sqlite", nil
		},
	}

	_, err := NewLocalHost(mock)
	if err == nil {
		t.Fatal("expected error for invalid db path, got nil")
	}
}

func TestNewLocalHost_Success(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	mock := &mockDBPathService{
		getMainDBPathFunc: func() (string, error) {
			return dbPath, nil
		},
	}

	host, err := NewLocalHost(mock)
	if err != nil {
		t.Fatalf("unexpected error creating host: %v", err)
	}

	if host == nil {
		t.Fatal("expected non-nil host, got nil")
	}

	// Test GetDB
	db, err := host.GetMainDB()
	if err != nil {
		t.Fatalf("unexpected error getting db: %v", err)
	}
	if db == nil {
		t.Fatal("expected non-nil db, got nil")
	}

	// Test GetProjectDB
	projectDB, err := host.GetProjectDB()
	if err != nil {
		t.Fatalf("unexpected error getting project db: %v", err)
	}
	if projectDB == nil {
		t.Fatal("expected non-nil project db, got nil")
	}

	// Clean up
	if err := host.Close(); err != nil {
		t.Fatalf("unexpected error closing host: %v", err)
	}
}

func TestLocalHostImpl_GetDB_NilHost(t *testing.T) {
	var host *localHostImpl
	_, err := host.GetMainDB()
	if err == nil {
		t.Fatal("expected error for nil host, got nil")
	}
	expectedMsg := "host is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestLocalHostImpl_GetProjectDB_NilHost(t *testing.T) {
	var host *localHostImpl
	_, err := host.GetProjectDB()
	if err == nil {
		t.Fatal("expected error for nil host, got nil")
	}
	expectedMsg := "host is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestLocalHostImpl_GetMainDBMigrations_NilHost(t *testing.T) {
	var host *localHostImpl
	_, err := host.getMainDBMigrations()
	if err == nil {
		t.Fatal("expected error for nil host, got nil")
	}
	expectedMsg := "host is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestLocalHostImpl_GetMainDBMigrations_Success(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	mock := &mockDBPathService{
		getMainDBPathFunc: func() (string, error) {
			return dbPath, nil
		},
	}

	host, err := NewLocalHost(mock)
	if err != nil {
		t.Fatalf("unexpected error creating host: %v", err)
	}
	defer host.Close()

	impl, ok := host.(*localHostImpl)
	if !ok {
		t.Fatal("expected host to be *localHostImpl")
	}

	migrations, err := impl.getMainDBMigrations()
	if err != nil {
		t.Fatalf("unexpected error getting migrations: %v", err)
	}

	// Should have at least rate limiting migrations
	if len(migrations) == 0 {
		t.Error("expected at least one migration, got zero")
	}
}

func TestLocalHostImpl_MigrateDB_NilHost(t *testing.T) {
	var host *localHostImpl
	err := host.migrateMainDB()
	if err == nil {
		t.Fatal("expected error for nil host, got nil")
	}
	expectedMsg := "host is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestLocalHostImpl_MigrateProjectDB_NilHost(t *testing.T) {
	var host *localHostImpl
	err := host.migrateProjectDB()
	if err == nil {
		t.Fatal("expected error for nil host, got nil")
	}
	expectedMsg := "host is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestLocalHostImpl_Close_NilHost(t *testing.T) {
	var host *localHostImpl
	err := host.Close()
	if err == nil {
		t.Fatal("expected error for nil host, got nil")
	}
	expectedMsg := "host is nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestLocalHostImpl_Close_Success(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	mock := &mockDBPathService{
		getMainDBPathFunc: func() (string, error) {
			return dbPath, nil
		},
	}

	host, err := NewLocalHost(mock)
	if err != nil {
		t.Fatalf("unexpected error creating host: %v", err)
	}

	err = host.Close()
	if err != nil {
		t.Fatalf("unexpected error closing host: %v", err)
	}
}
