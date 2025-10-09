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
	"fmt"
	"time"

	"github.com/retran/meowg1k/pkg/migrations"
)

var (
	ErrNotEnoughTokens = errors.New("not enough tokens in one of the buckets")
	ErrDatabaseIsNil   = errors.New("database connection is nil")
	ErrBucketNotFound  = errors.New("bucket not found")
)

// BucketConfig описывает параметры для пополнения бакета.
type BucketConfig struct {
	ID          string
	Capacity    int
	RefillRate  int
	RefillEvery time.Duration
}

// AcquisitionRequest описывает, сколько токенов нужно взять из одного бакета.
type AcquisitionRequest struct {
	ID    string
	Count int
}

// Repository определяет интерфейс для атомарной работы с бакетами.
type Repository interface {
	// AcquireTokens атомарно списывает токены из нескольких бакетов.
	// Если хотя бы в одном бакете не хватает токенов, вся операция откатывается.
	AcquireTokens(ctx context.Context, configs []BucketConfig, requests []AcquisitionRequest) error

	// InitializeBuckets создает и инициализирует бакеты, если их нет.
	InitializeBuckets(ctx context.Context, configs []BucketConfig) error

	// ResetBuckets сбрасывает токены до максимальной емкости.
	ResetBuckets(ctx context.Context, configs []BucketConfig) error
}

type repositoryImpl struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &repositoryImpl{db: db}
}

// refill — это внутренняя функция-помощник для расчета пополнения.
func refill(tokens, capacity, refillRate int, lastRefill time.Time, refillEvery time.Duration) (int, time.Time) {
	now := time.Now()
	elapsed := now.Sub(lastRefill)
	if elapsed < refillEvery {
		return tokens, lastRefill
	}

	intervals := int(elapsed / refillEvery)
	if intervals == 0 {
		return tokens, lastRefill
	}

	tokens += intervals * refillRate
	if tokens > capacity {
		tokens = capacity
	}

	newLastRefill := lastRefill.Add(time.Duration(intervals) * refillEvery)
	return tokens, newLastRefill
}

// AcquireTokens — сердце нового лимитера. Все в одной транзакции.
func (r *repositoryImpl) AcquireTokens(ctx context.Context, configs []BucketConfig, requests []AcquisitionRequest) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Безопасный откат, если коммит не удался

	configMap := make(map[string]BucketConfig)
	for _, config := range configs {
		configMap[config.ID] = config
	}

	type bucketState struct {
		id        string
		newTokens int
		newRefill time.Time
	}
	var statesToUpdate []bucketState

	// 1. Фаза проверки (Read Phase)
	for _, req := range requests {
		if req.Count <= 0 {
			continue // Пропускаем запросы на 0 токенов
		}

		config, ok := configMap[req.ID]
		if !ok {
			return fmt.Errorf("config for bucket %s not found", req.ID)
		}

		var currentTokens int
		var lastRefillNano int64
		err := tx.QueryRowContext(ctx, "SELECT tokens, last_refill FROM rate_limit_buckets WHERE id = ?", req.ID).Scan(&currentTokens, &lastRefillNano)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("bucket %s not initialized: %w", req.ID, ErrBucketNotFound)
			}
			return fmt.Errorf("failed to read bucket %s: %w", req.ID, err)
		}

		refilledTokens, newLastRefill := refill(currentTokens, config.Capacity, config.RefillRate, time.Unix(0, lastRefillNano), config.RefillEvery)

		if refilledTokens < req.Count {
			return ErrNotEnoughTokens // Немедленный выход без изменений
		}

		statesToUpdate = append(statesToUpdate, bucketState{
			id:        req.ID,
			newTokens: refilledTokens - req.Count,
			newRefill: newLastRefill,
		})
	}

	// 2. Фаза записи (Write Phase)
	stmt, err := tx.PrepareContext(ctx, "UPDATE rate_limit_buckets SET tokens = ?, last_refill = ? WHERE id = ?")
	if err != nil {
		return fmt.Errorf("failed to prepare update statement: %w", err)
	}
	defer stmt.Close()

	for _, state := range statesToUpdate {
		_, err := stmt.ExecContext(ctx, state.newTokens, state.newRefill.UnixNano(), state.id)
		if err != nil {
			return fmt.Errorf("failed to update bucket %s: %w", state.id, err)
		}
	}

	// 3. Коммит
	return tx.Commit()
}

func (r *repositoryImpl) InitializeBuckets(ctx context.Context, configs []BucketConfig) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction for init: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO rate_limit_buckets (id, tokens, last_refill) VALUES (?, ?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement for init: %w", err)
	}
	defer stmt.Close()

	for _, config := range configs {
		_, err := stmt.ExecContext(ctx, config.ID, config.Capacity, time.Now().UnixNano())
		if err != nil {
			return fmt.Errorf("failed to initialize bucket %s: %w", config.ID, err)
		}
	}

	return tx.Commit()
}

func (r *repositoryImpl) ResetBuckets(ctx context.Context, configs []BucketConfig) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction for reset: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "UPDATE rate_limit_buckets SET tokens = ?, last_refill = ? WHERE id = ?")
	if err != nil {
		return fmt.Errorf("failed to prepare update statement for reset: %w", err)
	}
	defer stmt.Close()

	for _, config := range configs {
		_, err := stmt.ExecContext(ctx, config.Capacity, time.Now().UnixNano(), config.ID)
		if err != nil {
			return fmt.Errorf("failed to reset bucket %s: %w", config.ID, err)
		}
	}
	return tx.Commit()
}

var Migrations = []migrations.Migration{
	{
		Version: 1,
		Up: func(tx *sql.Tx) error {
			_, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS rate_limit_buckets (
					id TEXT PRIMARY KEY,
					tokens INTEGER NOT NULL,
					last_refill INTEGER NOT NULL
				)
			`)
			if err != nil {
				return fmt.Errorf("failed to create rate_limit_buckets table: %w", err)
			}
			return nil
		},
	},
}
