// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package finalizeindex

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/retran/meowg1k/internal/activities/scanworktree"
	"github.com/retran/meowg1k/internal/core/index"
	domainindex "github.com/retran/meowg1k/internal/domain/index"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

type stubIndexRepo struct {
	ports.IndexRepository
}

type stubSnapshotRepo struct {
	cleared []string
	linked  []string
}

func (s *stubSnapshotRepo) LinkVersionToSnapshot(ctx context.Context, snapshotName string, versionID int64) error {
	_ = ctx
	s.linked = append(s.linked, snapshotName)
	return nil
}

func (s *stubSnapshotRepo) UnlinkVersionFromSnapshot(ctx context.Context, commitHash string, versionID int64) error {
	_ = ctx
	_ = commitHash
	_ = versionID
	return nil
}

func (s *stubSnapshotRepo) GetVersionIDsForSnapshot(ctx context.Context, commitHash string) ([]int64, error) {
	_ = ctx
	_ = commitHash
	return nil, nil
}

func (s *stubSnapshotRepo) ClearSnapshotLinks(ctx context.Context, commitHash string) error {
	_ = ctx
	s.cleared = append(s.cleared, commitHash)
	return nil
}

func TestNewFactoryNil(t *testing.T) {
	_, err := NewFactory(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "indexService cannot be nil")
}

func TestFinalizeIndexActivitySuccess(t *testing.T) {
	indexRepo := &stubIndexRepo{}
	snapshotRepo := &stubSnapshotRepo{}
	indexSvc, err := index.NewService(indexRepo, snapshotRepo)
	require.NoError(t, err)

	factory, err := NewFactory(indexSvc)
	require.NoError(t, err)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	scanResult := &scanworktree.Output{
		HeadState: map[string]domainindex.FileState{
			"a.txt": {ContentHash: "hash1", Content: []byte("a")},
		},
		StageState: map[string]domainindex.FileState{
			"b.txt": {ContentHash: "hash1", Content: []byte("a")},
		},
		WorkdirState: map[string]domainindex.FileState{
			"c.txt": {ContentHash: "hash1", Content: []byte("a")},
		},
	}

	_, err = activity(context.Background(), flowCtx, &Input{
		ScanResult:       scanResult,
		ExistingVersions: map[string]int64{"hash1": 1},
		NewVersions:      map[string]int64{},
	})
	require.NoError(t, err)
	assert.Len(t, snapshotRepo.cleared, 3)
	assert.Len(t, snapshotRepo.linked, 3)
}

func TestFinalizeIndexActivityError(t *testing.T) {
	indexRepo := &stubIndexRepo{}
	snapshotRepo := &stubSnapshotRepo{}
	indexSvc, err := index.NewService(indexRepo, snapshotRepo)
	require.NoError(t, err)

	factory, err := NewFactory(indexSvc)
	require.NoError(t, err)
	activity := factory.NewActivity()

	exec := executor.NewExecutor(0)
	flowCtx := executor.NewContext("test", executor.NoOpFeedbackHandler, exec)

	_, err = activity(context.Background(), flowCtx, &Input{
		ScanResult:       nil,
		ExistingVersions: map[string]int64{},
		NewVersions:      map[string]int64{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to finalize snapshots")
}
