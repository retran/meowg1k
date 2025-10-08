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

// setupTestDBForLimiter creates an in-memory SQLite database for testing
func setupTestDBForLimiter(t *testing.T) (*sql.DB, Repository) {
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

func TestNewLimiter(t *testing.T) {
	db, repo := setupTestDBForLimiter(t)
	defer db.Close()

	config := Config{
		RequestsPerMinute: 10,
		TokensPerMinute:   100,
		RequestsPerDay:    1000,
	}

	limiter, err := NewLimiter("test-limiter", config, repo)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	if limiter.rpm == nil {
		t.Error("Expected rpm bucket to be initialized")
	}
	if limiter.tpm == nil {
		t.Error("Expected tpm bucket to be initialized")
	}
	if limiter.rpd == nil {
		t.Error("Expected rpd bucket to be initialized")
	}
}

func TestNewLimiterUnlimited(t *testing.T) {
	db, repo := setupTestDBForLimiter(t)
	defer db.Close()

	limiter, err := NewLimiter("test-unlimited", Unlimited, repo)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	if limiter.rpm != nil {
		t.Error("Expected rpm bucket to be nil for unlimited")
	}
	if limiter.tpm != nil {
		t.Error("Expected tpm bucket to be nil for unlimited")
	}
	if limiter.rpd != nil {
		t.Error("Expected rpd bucket to be nil for unlimited")
	}
}

func TestLimiterWait(t *testing.T) {
	db, repo := setupTestDBForLimiter(t)
	defer db.Close()

	// Create limiter with fast refill for testing
	rpm, err := NewBucket("test-wait:rpm", 2, 2, 100*time.Millisecond, repo)
	if err != nil {
		t.Fatalf("Failed to create rpm bucket: %v", err)
	}

	tpm, err := NewBucket("test-wait:tpm", 20, 20, 100*time.Millisecond, repo)
	if err != nil {
		t.Fatalf("Failed to create tpm bucket: %v", err)
	}

	rpd, err := NewBucket("test-wait:rpd", 100, 100, 24*time.Hour, repo)
	if err != nil {
		t.Fatalf("Failed to create rpd bucket: %v", err)
	}

	limiter := &Limiter{
		rpm: rpm,
		tpm: tpm,
		rpd: rpd,
		id:  "test-wait",
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

	// Third request should wait for refill
	ctxTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	start := time.Now()
	err = limiter.Wait(ctxTimeout, 5)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected Wait to succeed after refill, got error: %v", err)
	}
	if duration < 100*time.Millisecond {
		t.Errorf("Expected to wait at least 100ms for refill, waited %v", duration)
	}
}

func TestLimiterTryAcquire(t *testing.T) {
	db, repo := setupTestDBForLimiter(t)
	defer db.Close()

	config := Config{
		RequestsPerMinute: 5,
		TokensPerMinute:   50,
		RequestsPerDay:    100,
	}

	limiter, err := NewLimiter("test-tryacquire", config, repo)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	// Should succeed initially
	if !limiter.TryAcquire(5) {
		t.Error("Expected TryAcquire to succeed")
	}

	// Should succeed for remaining capacity
	if !limiter.TryAcquire(5) {
		t.Error("Expected TryAcquire to succeed")
	}

	// Should fail when exceeding limits
	if limiter.TryAcquire(50) {
		t.Error("Expected TryAcquire to fail when exceeding token limit")
	}
}

func TestLimiterReset(t *testing.T) {
	db, repo := setupTestDBForLimiter(t)
	defer db.Close()

	config := Config{
		RequestsPerMinute: 10,
		TokensPerMinute:   100,
		RequestsPerDay:    50,
	}

	limiter, err := NewLimiter("test-reset", config, repo)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	// Consume some resources
	limiter.TryAcquire(5)

	rpm, tpm, rpd := limiter.Stats()
	if rpm >= 10 || tpm >= 100 || rpd >= 50 {
		t.Errorf("Expected some resources consumed, got rpm=%d, tpm=%d, rpd=%d", rpm, tpm, rpd)
	}

	limiter.Reset()

	rpm, tpm, rpd = limiter.Stats()
	if rpm != 10 || tpm != 100 || rpd != 50 {
		t.Errorf("Expected full capacity after reset, got rpm=%d, tpm=%d, rpd=%d", rpm, tpm, rpd)
	}
}

func TestLimiterStats(t *testing.T) {
	db, repo := setupTestDBForLimiter(t)
	defer db.Close()

	config := Config{
		RequestsPerMinute: 10,
		TokensPerMinute:   100,
		RequestsPerDay:    50,
	}

	limiter, err := NewLimiter("test-stats", config, repo)
	if err != nil {
		t.Fatalf("Failed to create limiter: %v", err)
	}

	rpm, tpm, rpd := limiter.Stats()
	if rpm != 10 || tpm != 100 || rpd != 50 {
		t.Errorf("Expected full capacity, got rpm=%d, tpm=%d, rpd=%d", rpm, tpm, rpd)
	}

	limiter.TryAcquire(5)

	rpm, tpm, rpd = limiter.Stats()
	if rpm != 9 || tpm != 95 || rpd != 49 {
		t.Errorf("Expected reduced capacity, got rpm=%d, tpm=%d, rpd=%d", rpm, tpm, rpd)
	}
}
