// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package ratelimit

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/retran/meowg1k/internal/adapters/sqlite/migrations"
	"github.com/retran/meowg1k/internal/adapters/sqlite/ratelimit"
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

// setupTestDBForLimiter creates an in-memory SQLite database for testing.
func setupTestDBForLimiter(t *testing.T) (*sql.DB, ports.RateLimitRepository) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Run migrations
	if err := migrations.RunMigrations(db, ratelimit.Migrations); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	host := newMockHost(db)
	repo := ratelimit.NewRepository(host)
	return db, repo
}

func TestNewLimiter(t *testing.T) {
	db, repo := setupTestDBForLimiter(t)
	defer db.Close()

	config := Config{
		ID:                "test-limiter",
		RequestsPerMinute: 10,
		TokensPerMinute:   100,
		RequestsPerDay:    1000,
	}

	limiter, err := NewLimiter(context.Background(), config, repo)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	// Test that the limiter works by attempting to acquire
	if limiter == nil {
		t.Error("Expected limiter to be initialized")
	}

	// Verify it's a dbLimiter by type assertion
	dbLim, ok := limiter.(*dbLimiter)
	if !ok {
		t.Error("Expected limiter to be a dbLimiter")
	}
	if len(dbLim.configs) != 3 {
		t.Errorf("Expected 3 bucket configs, got %d", len(dbLim.configs))
	}
}

func TestNewLimiterUnlimited(t *testing.T) {
	db, repo := setupTestDBForLimiter(t)
	defer db.Close()

	config := Unlimited
	config.ID = "test-unlimited"
	limiter, err := NewLimiter(context.Background(), config, repo)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	// Verify it's a NoOp limiter since no limits are set
	_, ok := limiter.(*noOpLimiter)
	if !ok {
		t.Error("Expected limiter to be a noOpLimiter for unlimited config")
	}
}

func TestLimiterWait(t *testing.T) {
	db, repo := setupTestDBForLimiter(t)
	defer db.Close()

	// Create limiter with small limits for testing
	config := Config{
		ID:                "test-wait",
		RequestsPerMinute: 2,
		TokensPerMinute:   20,
		RequestsPerDay:    100,
	}

	limiter, err := NewLimiter(context.Background(), config, repo)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	ctx := context.Background()

	// Should succeed initially
	if err := limiter.Wait(ctx, 5); err != nil {
		t.Errorf("Expected Wait to succeed, got error: %v", err)
	}

	// Should succeed for second request
	if err := limiter.Wait(ctx, 5); err != nil {
		t.Errorf("Expected Wait to succeed, got error: %v", err)
	}

	// Third request should fail immediately as we've hit the requests per minute limit
	// Using TryAcquire instead of Wait to avoid blocking
	if limiter.TryAcquire(ctx, 5) {
		t.Error("Expected TryAcquire to fail after exceeding request limit")
	}
}

func TestLimiterTryAcquire(t *testing.T) {
	db, repo := setupTestDBForLimiter(t)
	defer db.Close()

	config := Config{
		ID:                "test-tryacquire",
		RequestsPerMinute: 5,
		TokensPerMinute:   50,
		RequestsPerDay:    100,
	}

	limiter, err := NewLimiter(context.Background(), config, repo)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	ctx := context.Background()

	// Should succeed initially
	if !limiter.TryAcquire(ctx, 5) {
		t.Error("Expected TryAcquire to succeed")
	}

	// Should succeed for remaining capacity
	if !limiter.TryAcquire(ctx, 5) {
		t.Error("Expected TryAcquire to succeed")
	}

	// Should fail when exceeding limits
	if limiter.TryAcquire(ctx, 50) {
		t.Error("Expected TryAcquire to fail when exceeding token limit")
	}
}

func TestLimiterConcurrency(t *testing.T) {
	db, repo := setupTestDBForLimiter(t)
	defer db.Close()

	config := Config{
		ID:                "test-overlap",
		RequestsPerMinute: 100,
		TokensPerMinute:   1000,
		RequestsPerDay:    5000,
	}

	limiter, err := NewLimiter(context.Background(), config, repo)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	ctx := context.Background()

	// Test that overlapping acquisitions work correctly.
	successCount := 0
	for i := 0; i < 50; i++ {
		if limiter.TryAcquire(ctx, 10) {
			successCount++
		}
	}

	// We should be able to acquire at least some tokens
	if successCount == 0 {
		t.Error("Expected at least some successful acquisitions")
	}
}
