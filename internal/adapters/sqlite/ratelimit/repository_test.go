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

package ratelimit

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/retran/meowg1k/internal/adapters/sqlite/migrations"
	"github.com/retran/meowg1k/internal/domain/ratelimit"
)

func setupTestDB(t *testing.T) *sql.DB {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return db
}

func runMigrations(db *sql.DB) error {
	return migrations.RunMigrations(db, Migrations)
}

func TestNewRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	if repo == nil {
		t.Fatal("NewRepository returned nil")
	}
}

func TestNotEnoughTokensError_Error(t *testing.T) {
	err := &ratelimit.NotEnoughTokensError{
		BucketID: "test-bucket",
		Need:     10,
		Have:     5,
	}

	expectedMsg := `not enough tokens in bucket "test-bucket": need 10, have 5`
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestInitializeBuckets(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	configs := []ratelimit.BucketConfig{
		{
			ID:          "bucket1",
			Capacity:    100,
			RefillRate:  10,
			RefillEvery: time.Second,
		},
		{
			ID:          "bucket2",
			Capacity:    50,
			RefillRate:  5,
			RefillEvery: time.Minute,
		},
	}

	err := repo.InitializeBuckets(ctx, configs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify buckets were created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM rate_limit_buckets").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query buckets: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 buckets, got %d", count)
	}
}

func TestAcquireTokens_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	configs := []ratelimit.BucketConfig{
		{
			ID:          "test-bucket",
			Capacity:    100,
			RefillRate:  10,
			RefillEvery: time.Second,
		},
	}

	// Initialize bucket
	err := repo.InitializeBuckets(ctx, configs)
	if err != nil {
		t.Fatalf("failed to initialize buckets: %v", err)
	}

	// Acquire tokens
	requests := []ratelimit.AcquisitionRequest{
		{ID: "test-bucket", Count: 10},
	}

	err = repo.AcquireTokens(ctx, configs, requests)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify tokens were deducted
	var tokens int
	err = db.QueryRow("SELECT tokens FROM rate_limit_buckets WHERE id = ?", "test-bucket").Scan(&tokens)
	if err != nil {
		t.Fatalf("failed to query tokens: %v", err)
	}
	if tokens != 90 {
		t.Errorf("expected 90 tokens, got %d", tokens)
	}
}

func TestAcquireTokens_NotEnoughTokens(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	configs := []ratelimit.BucketConfig{
		{
			ID:          "test-bucket",
			Capacity:    100,
			RefillRate:  10,
			RefillEvery: time.Second,
		},
	}

	// Initialize bucket
	err := repo.InitializeBuckets(ctx, configs)
	if err != nil {
		t.Fatalf("failed to initialize buckets: %v", err)
	}

	// Try to acquire more tokens than available
	requests := []ratelimit.AcquisitionRequest{
		{ID: "test-bucket", Count: 150},
	}

	err = repo.AcquireTokens(ctx, configs, requests)
	if err == nil {
		t.Fatal("expected error for insufficient tokens, got nil")
	}

	var notEnoughErr *ratelimit.NotEnoughTokensError
	if !errors.As(err, &notEnoughErr) {
		t.Errorf("expected NotEnoughTokensError, got %T", err)
	}
}

func TestAcquireTokens_MultipleBuckets(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	configs := []ratelimit.BucketConfig{
		{
			ID:          "bucket1",
			Capacity:    100,
			RefillRate:  10,
			RefillEvery: time.Second,
		},
		{
			ID:          "bucket2",
			Capacity:    50,
			RefillRate:  5,
			RefillEvery: time.Second,
		},
	}

	// Initialize buckets
	err := repo.InitializeBuckets(ctx, configs)
	if err != nil {
		t.Fatalf("failed to initialize buckets: %v", err)
	}

	// Acquire from both buckets
	requests := []ratelimit.AcquisitionRequest{
		{ID: "bucket1", Count: 10},
		{ID: "bucket2", Count: 5},
	}

	err = repo.AcquireTokens(ctx, configs, requests)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify both buckets were updated
	var tokens1, tokens2 int
	db.QueryRow("SELECT tokens FROM rate_limit_buckets WHERE id = ?", "bucket1").Scan(&tokens1)
	db.QueryRow("SELECT tokens FROM rate_limit_buckets WHERE id = ?", "bucket2").Scan(&tokens2)

	if tokens1 != 90 {
		t.Errorf("bucket1: expected 90 tokens, got %d", tokens1)
	}
	if tokens2 != 45 {
		t.Errorf("bucket2: expected 45 tokens, got %d", tokens2)
	}
}

func TestResetBuckets(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	configs := []ratelimit.BucketConfig{
		{
			ID:          "test-bucket",
			Capacity:    100,
			RefillRate:  10,
			RefillEvery: time.Second,
		},
	}

	// Initialize and acquire some tokens
	repo.InitializeBuckets(ctx, configs)
	requests := []ratelimit.AcquisitionRequest{{ID: "test-bucket", Count: 50}}
	repo.AcquireTokens(ctx, configs, requests)

	// Reset bucket
	err := repo.ResetBuckets(ctx, configs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify tokens were reset to capacity
	var tokens int
	db.QueryRow("SELECT tokens FROM rate_limit_buckets WHERE id = ?", "test-bucket").Scan(&tokens)
	if tokens != 100 {
		t.Errorf("expected 100 tokens after reset, got %d", tokens)
	}
}

func TestResetBuckets_NonExistentBucket(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	configs := []ratelimit.BucketConfig{
		{
			ID:          "nonexistent",
			Capacity:    100,
			RefillRate:  10,
			RefillEvery: time.Second,
		},
	}

	// Reset should not fail for non-existent bucket
	err := repo.ResetBuckets(ctx, configs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAcquireTokens_Refill(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	ctx := context.Background()

	configs := []ratelimit.BucketConfig{
		{
			ID:          "test-bucket",
			Capacity:    100,
			RefillRate:  50,
			RefillEvery: 100 * time.Millisecond,
		},
	}

	// Initialize bucket
	repo.InitializeBuckets(ctx, configs)

	// Acquire tokens
	requests := []ratelimit.AcquisitionRequest{{ID: "test-bucket", Count: 60}}
	repo.AcquireTokens(ctx, configs, requests)

	// Wait for refill
	time.Sleep(150 * time.Millisecond)

	// Acquire more tokens (should succeed because of refill)
	requests = []ratelimit.AcquisitionRequest{{ID: "test-bucket", Count: 60}}
	err := repo.AcquireTokens(ctx, configs, requests)
	if err != nil {
		t.Fatalf("unexpected error after refill: %v", err)
	}
}
