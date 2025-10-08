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
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	"github.com/retran/meowg1k/pkg/migrations"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) (*sql.DB, Repository) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Run migrations
	if err := migrations.RunMigrations(db, Migrations); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := NewRepository(db)
	return db, repo
}

func TestNewBucket(t *testing.T) {
	db, repo := setupTestDB(t)
	defer db.Close()

	capacity := 10
	refillRate := 2
	refillEvery := time.Second

	bucket, err := NewBucket("test-bucket", capacity, refillRate, refillEvery, repo)
	if err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	if bucket.capacity != capacity {
		t.Errorf("Expected capacity %d, got %d", capacity, bucket.capacity)
	}
	if bucket.refillRate != refillRate {
		t.Errorf("Expected refillRate %d, got %d", refillRate, bucket.refillRate)
	}
	if bucket.refillEvery != refillEvery {
		t.Errorf("Expected refillEvery %v, got %v", refillEvery, bucket.refillEvery)
	}

	// Verify initial tokens in database
	if available := bucket.Available(); available != capacity {
		t.Errorf("Expected initial tokens %d, got %d", capacity, available)
	}
}

func TestBucketTryTake(t *testing.T) {
	db, repo := setupTestDB(t)
	defer db.Close()

	bucket, err := NewBucket("test-trytake", 5, 1, time.Minute, repo)
	if err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Should succeed initially
	if !bucket.TryTake(3) {
		t.Error("Expected TryTake(3) to succeed")
	}
	if bucket.Available() != 2 {
		t.Errorf("Expected 2 tokens available, got %d", bucket.Available())
	}

	// Should succeed for remaining tokens
	if !bucket.TryTake(2) {
		t.Error("Expected TryTake(2) to succeed")
	}
	if bucket.Available() != 0 {
		t.Errorf("Expected 0 tokens available, got %d", bucket.Available())
	}

	// Should fail when no tokens
	if bucket.TryTake(1) {
		t.Error("Expected TryTake(1) to fail when no tokens")
	}
}

func TestBucketTake(t *testing.T) {
	db, repo := setupTestDB(t)
	defer db.Close()

	bucket, err := NewBucket("test-take", 2, 2, 100*time.Millisecond, repo)
	if err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Take all tokens
	if err := bucket.Take(ctx, 2); err != nil {
		t.Errorf("Expected Take(2) to succeed, got error: %v", err)
	}

	// Try to take more, should wait for refill
	start := time.Now()
	err = bucket.Take(ctx, 1)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected Take(1) to succeed after refill, got error: %v", err)
	}
	if duration < 100*time.Millisecond {
		t.Errorf("Expected to wait at least 100ms for refill, waited %v", duration)
	}
}

func TestBucketTakeTimeout(t *testing.T) {
	db, repo := setupTestDB(t)
	defer db.Close()

	bucket, err := NewBucket("test-timeout", 1, 1, time.Minute, repo)
	if err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Take the only token
	if err := bucket.Take(ctx, 1); err != nil {
		t.Errorf("Expected Take(1) to succeed, got error: %v", err)
	}

	// Try to take another, should timeout
	err = bucket.Take(ctx, 1)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}
}

func TestBucketAvailable(t *testing.T) {
	db, repo := setupTestDB(t)
	defer db.Close()

	bucket, err := NewBucket("test-available", 10, 5, time.Minute, repo)
	if err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	if available := bucket.Available(); available != 10 {
		t.Errorf("Expected 10 available, got %d", available)
	}

	bucket.TryTake(3)
	if available := bucket.Available(); available != 7 {
		t.Errorf("Expected 7 available after taking 3, got %d", available)
	}
}

func TestBucketReset(t *testing.T) {
	db, repo := setupTestDB(t)
	defer db.Close()

	bucket, err := NewBucket("test-reset", 10, 5, time.Minute, repo)
	if err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	bucket.TryTake(7)
	if bucket.Available() != 3 {
		t.Errorf("Expected 3 available after taking 7, got %d", bucket.Available())
	}

	if err := bucket.Reset(); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}
	if bucket.Available() != 10 {
		t.Errorf("Expected 10 available after reset, got %d", bucket.Available())
	}
}

func TestBucketRefill(t *testing.T) {
	db, repo := setupTestDB(t)
	defer db.Close()

	bucket, err := NewBucket("test-refill", 10, 2, 50*time.Millisecond, repo)
	if err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Take all tokens
	bucket.TryTake(10)
	if bucket.Available() != 0 {
		t.Errorf("Expected 0 available after taking all, got %d", bucket.Available())
	}

	// Wait for refill
	time.Sleep(60 * time.Millisecond)

	available := bucket.Available()
	if available < 2 {
		t.Errorf("Expected at least 2 tokens after refill, got %d", available)
	}
}
