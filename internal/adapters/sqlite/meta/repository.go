// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

// Package meta provides a SQLite-based repository for storing and retrieving metadata key-value pairs.
package meta

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/retran/meowg1k/internal/ports"
)

const (
	fileRefPrefix        = "__meowg1k_file__:"
	maxInlineValueSize   = 8 * 1024 * 1024 // 8 MiB cap for in-DB storage
	tempBlobFilePattern  = "meta-blob-*"
	defaultBlobFilePerms = 0o600
)

var fileRefPrefixBytes = []byte(fileRefPrefix)

// Repository implements metadata storage using SQLite.
type Repository struct {
	host        ports.Host
	blobDirErr  error
	blobDir     string
	blobDirOnce sync.Once
}

// Compile-time interface compliance check.
var _ ports.MetaRepository = (*Repository)(nil)

// NewRepository creates a new metadata repository.
func NewRepository(host ports.Host) *Repository {
	return &Repository{host: host}
}

// SetValue stores a metadata value with the given key.
// If the key already exists, the value is updated.
func (r *Repository) SetValue(ctx context.Context, key string, value []byte) error {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	existingRef, err := r.getExistingFileReference(ctx, db, key)
	if err != nil {
		return fmt.Errorf("failed to inspect existing value for key '%s': %w", key, err)
	}

	if len(value) > maxInlineValueSize {
		return r.setBlobValue(ctx, db, key, value, existingRef)
	}

	return r.setInlineValue(ctx, db, key, value, existingRef)
}

func (r *Repository) setBlobValue(ctx context.Context, db *sql.DB, key string, value []byte, existingRef string) error {
	blobDir, err := r.ensureBlobDir(db)
	if err != nil {
		return fmt.Errorf("failed to prepare blob storage for key '%s': %w", key, err)
	}

	fileName := r.buildBlobFileName(key, value)
	if err := r.writeBlob(blobDir, fileName, value); err != nil {
		return fmt.Errorf("failed to persist blob for key '%s': %w", key, err)
	}

	sentinel := append([]byte{}, fileRefPrefixBytes...)
	sentinel = append(sentinel, fileName...)

	if _, err := db.ExecContext(ctx,
		`INSERT INTO meta_kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, sentinel,
	); err != nil {
		_ = r.removeBlob(blobDir, fileName) //nolint:errcheck // Best effort cleanup on failure
		return fmt.Errorf("failed to set meta value for key '%s': %w", key, err)
	}

	if existingRef != "" && existingRef != fileName {
		if err := r.removeBlob(blobDir, existingRef); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to remove old blob for key '%s': %w", key, err)
		}
	}

	return nil
}

func (r *Repository) setInlineValue(ctx context.Context, db *sql.DB, key string, value []byte, existingRef string) error {
	if _, err := db.ExecContext(ctx,
		`INSERT INTO meta_kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	); err != nil {
		return fmt.Errorf("failed to set meta value for key '%s': %w", key, err)
	}

	if existingRef != "" {
		if blobDir, derr := r.ensureBlobDir(db); derr == nil {
			if err := r.removeBlob(blobDir, existingRef); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("failed to remove obsolete blob for key '%s': %w", key, err)
			}
		} else {
			return fmt.Errorf("failed to clean up blob for key '%s': %w", key, derr)
		}
	}

	return nil
}

// GetValue retrieves a metadata value by key.
// Returns nil if the key does not exist.
func (r *Repository) GetValue(ctx context.Context, key string) ([]byte, error) {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %w", err)
	}

	var value []byte
	err = db.QueryRowContext(ctx, "SELECT value FROM meta_kv WHERE key = ?", key).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get meta value for key '%s': %w", key, err)
	}

	if fileName, ok := parseFileReference(value); ok {
		blobDir, err := r.ensureBlobDir(db)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve blob storage for key '%s': %w", key, err)
		}

		data, err := os.ReadFile(filepath.Join(blobDir, fileName))
		if err != nil {
			return nil, fmt.Errorf("failed to read blob for key '%s': %w", key, err)
		}
		return data, nil
	}

	return value, nil
}

// DeleteValue deletes a metadata value by key.
// Does not return an error if the key does not exist.
func (r *Repository) DeleteValue(ctx context.Context, key string) error {
	db, err := r.host.GetProjectDB()
	if err != nil {
		return fmt.Errorf("failed to get database: %w", err)
	}

	existingRef, err := r.getExistingFileReference(ctx, db, key)
	if err != nil {
		return fmt.Errorf("failed to inspect existing value for key '%s': %w", key, err)
	}

	_, err = db.ExecContext(ctx, "DELETE FROM meta_kv WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("failed to delete meta value for key '%s': %w", key, err)
	}

	if existingRef != "" {
		blobDir, derr := r.ensureBlobDir(db)
		if derr != nil {
			return fmt.Errorf("failed to resolve blob storage for key '%s': %w", key, derr)
		}
		if err := r.removeBlob(blobDir, existingRef); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to remove blob for key '%s': %w", key, err)
		}
	}

	return nil
}

func (r *Repository) ensureBlobDir(db *sql.DB) (string, error) {
	if db == nil {
		return "", fmt.Errorf("project database is nil")
	}

	r.blobDirOnce.Do(func() {
		dir, err := resolveBlobDir(db)
		if err != nil {
			r.blobDirErr = err
			return
		}
		r.blobDir = dir
	})

	if r.blobDirErr != nil {
		return "", r.blobDirErr
	}

	return r.blobDir, nil
}

func resolveBlobDir(db *sql.DB) (string, error) {
	rows, err := db.QueryContext(context.Background(), "PRAGMA database_list")
	if err != nil {
		return "", fmt.Errorf("failed to query database list: %w", err)
	}
	defer func() { _ = rows.Close() }() //nolint:errcheck // Defer close errors are not critical

	for rows.Next() {
		var seq int64
		var name string
		var file sql.NullString

		if err := rows.Scan(&seq, &name, &file); err != nil {
			return "", fmt.Errorf("failed to scan database list: %w", err)
		}

		if name != "main" {
			continue
		}

		if !file.Valid || file.String == "" {
			return "", fmt.Errorf("main database has no associated file path")
		}

		dbPath, err := normalizeDBPath(file.String)
		if err != nil {
			return "", fmt.Errorf("failed to normalize database path: %w", err)
		}

		blobDir := filepath.Join(filepath.Dir(dbPath), "meta_blobs")
		if err := os.MkdirAll(blobDir, 0o750); err != nil {
			return "", fmt.Errorf("failed to create blob directory: %w", err)
		}

		return blobDir, nil
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to iterate database list: %w", err)
	}

	return "", fmt.Errorf("could not locate main database entry")
}

func normalizeDBPath(raw string) (string, error) {
	path := strings.TrimSpace(raw)

	if path == "" {
		return "", fmt.Errorf("database path is empty")
	}

	path = strings.TrimPrefix(path, "file:")

	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}

	path = filepath.Clean(path)

	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("failed to resolve absolute path: %w", err)
		}
		path = abs
	}

	return path, nil
}

func parseFileReference(value []byte) (string, bool) {
	if len(value) <= len(fileRefPrefixBytes) {
		return "", false
	}
	if !bytes.HasPrefix(value, fileRefPrefixBytes) {
		return "", false
	}
	return string(value[len(fileRefPrefixBytes):]), true
}

func (r *Repository) getExistingFileReference(ctx context.Context, db *sql.DB, key string) (string, error) {
	query := `
		SELECT CASE
			WHEN substr(value, 1, ?) = ? THEN substr(value, ? + 1)
			ELSE NULL
		END
		FROM meta_kv
		WHERE key = ?
	`

	var tail []byte
	err := db.QueryRowContext(ctx, query,
		len(fileRefPrefixBytes),
		fileRefPrefixBytes,
		len(fileRefPrefixBytes),
		key,
	).Scan(&tail)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("failed to scan tail: %w", err)
	}

	if len(tail) == 0 {
		return "", nil
	}

	return string(tail), nil
}

func (r *Repository) buildBlobFileName(key string, value []byte) string {
	hasher := sha256.New()
	hasher.Write([]byte(key))
	hasher.Write([]byte{0})
	hasher.Write(value)
	sum := hasher.Sum(nil)
	return hex.EncodeToString(sum) + ".blob"
}

func (r *Repository) writeBlob(dir, name string, data []byte) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to ensure blob directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, tempBlobFilePattern)
	if err != nil {
		return fmt.Errorf("failed to create temp blob file: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name()) //nolint:errcheck // Defer cleanup errors are not critical
	}()

	if err := writeAndSync(tmpFile, data); err != nil {
		return err
	}

	if err := os.Chmod(tmpFile.Name(), defaultBlobFilePerms); err != nil {
		return fmt.Errorf("failed to set blob file permissions: %w", err)
	}

	destPath := filepath.Join(dir, name)
	return replaceFile(tmpFile.Name(), destPath)
}

func writeAndSync(file *os.File, data []byte) error {
	if _, err := file.Write(data); err != nil {
		_ = file.Close() //nolint:errcheck // Error on close after write error is not critical
		return fmt.Errorf("failed to write blob data: %w", err)
	}

	if err := file.Sync(); err != nil {
		_ = file.Close() //nolint:errcheck // Error on close after sync error is not critical
		return fmt.Errorf("failed to sync blob data: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close temp blob file: %w", err)
	}

	return nil
}

func replaceFile(sourcePath, destPath string) error {
	if err := os.Rename(sourcePath, destPath); err != nil {
		if errors.Is(err, os.ErrExist) {
			if remErr := os.Remove(destPath); remErr != nil && !errors.Is(remErr, os.ErrNotExist) {
				return fmt.Errorf("failed to replace existing blob file: %w", remErr)
			}
			if err := os.Rename(sourcePath, destPath); err != nil {
				return fmt.Errorf("failed to finalize blob file: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to finalize blob file: %w", err)
	}

	return nil
}

func (r *Repository) removeBlob(dir, name string) error {
	if dir == "" || name == "" {
		return nil
	}

	if err := os.Remove(filepath.Join(dir, name)); err != nil {
		return fmt.Errorf("failed to remove blob file: %w", err)
	}
	return nil
}
