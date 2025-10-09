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
	"errors"
	"time"
)

// Limiter определяет интерфейс для rate limiting операций.
type Limiter interface {
	Wait(ctx context.Context, tokenCount int) error
	TryAcquire(ctx context.Context, tokenCount int) bool
}

// dbLimiter предоставляет мульти-измерный rate limiting.
type dbLimiter struct {
	repo    Repository
	configs []BucketConfig
}

// Config описывает конфигурацию для всех бакетов лимитера.
type Config struct {
	ID                string
	RequestsPerMinute int
	TokensPerMinute   int
	RequestsPerDay    int
}

// Unlimited is a predefined config with no limits
var Unlimited = Config{
	ID:                "unlimited",
	RequestsPerMinute: 0,
	TokensPerMinute:   0,
	RequestsPerDay:    0,
}

// NewLimiter создает новый мульти-измерный rate limiter.
func NewLimiter(ctx context.Context, config Config, repo Repository) (Limiter, error) {
	if config.ID == "" {
		return nil, errors.New("config ID is empty")
	}
	if repo == nil {
		return nil, errors.New("repository is nil")
	}

	var configs []BucketConfig
	if config.RequestsPerMinute > 0 {
		configs = append(configs, BucketConfig{
			ID:          config.ID + ":rpm",
			Capacity:    config.RequestsPerMinute,
			RefillRate:  config.RequestsPerMinute,
			RefillEvery: time.Minute,
		})
	}
	if config.TokensPerMinute > 0 {
		configs = append(configs, BucketConfig{
			ID:          config.ID + ":tpm",
			Capacity:    config.TokensPerMinute,
			RefillRate:  config.TokensPerMinute,
			RefillEvery: time.Minute,
		})
	}
	if config.RequestsPerDay > 0 {
		configs = append(configs, BucketConfig{
			ID:          config.ID + ":rpd",
			Capacity:    config.RequestsPerDay,
			RefillRate:  config.RequestsPerDay,
			RefillEvery: 24 * time.Hour,
		})
	}

	if len(configs) == 0 {
		return NewNoOpLimiter(), nil // Если лимиты не заданы, возвращаем "пустышку"
	}

	// Инициализируем бакеты в базе данных при старте
	if err := repo.InitializeBuckets(ctx, configs); err != nil {
		return nil, err
	}

	return &dbLimiter{
		repo:    repo,
		configs: configs,
	}, nil
}

// TryAcquire пытается атомарно захватить все необходимые токены.
func (l *dbLimiter) TryAcquire(ctx context.Context, tokenCount int) bool {
	requests := l.buildRequests(tokenCount)
	if len(requests) == 0 {
		return true
	}

	err := l.repo.AcquireTokens(ctx, l.configs, requests)
	return err == nil
}

// Wait блокирует выполнение, пока не удастся захватить все необходимые токены.
func (l *dbLimiter) Wait(ctx context.Context, tokenCount int) error {
	requests := l.buildRequests(tokenCount)
	if len(requests) == 0 {
		return nil
	}

	// Интервал опроса, чтобы не "долбить" БД слишком часто.
	const pollInterval = 100 * time.Millisecond

	for {
		err := l.repo.AcquireTokens(ctx, l.configs, requests)
		if err == nil {
			return nil // Успех
		}

		// Если не хватает токенов, ждем и пробуем снова.
		// При других ошибках (например, отмена контекста) выходим.
		if !errors.Is(err, ErrNotEnoughTokens) {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
			// Повторяем попытку
		}
	}
}

// buildRequests создает список запросов на списание на основе конфига.
func (l *dbLimiter) buildRequests(tokenCount int) []AcquisitionRequest {
	var requests []AcquisitionRequest
	for _, config := range l.configs {
		switch {
		case config.RefillEvery == time.Minute && config.ID[len(config.ID)-4:] == ":rpm":
			requests = append(requests, AcquisitionRequest{ID: config.ID, Count: 1})
		case config.RefillEvery == time.Minute && config.ID[len(config.ID)-4:] == ":tpm":
			if tokenCount > 0 {
				requests = append(requests, AcquisitionRequest{ID: config.ID, Count: tokenCount})
			}
		case config.RefillEvery == 24*time.Hour:
			requests = append(requests, AcquisitionRequest{ID: config.ID, Count: 1})
		}
	}
	return requests
}

// noOpLimiter — реализация, которая ничего не делает.
type noOpLimiter struct{}

func NewNoOpLimiter() Limiter {
	return &noOpLimiter{}
}

func (n *noOpLimiter) Wait(ctx context.Context, tokenCount int) error {
	return nil
}

func (n *noOpLimiter) TryAcquire(ctx context.Context, tokenCount int) bool {
	return true
}
