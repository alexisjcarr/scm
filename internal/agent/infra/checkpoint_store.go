package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	agentdomain "github.com/alexisjcarr/scm/internal/agent/domain"
	agentruntime "github.com/alexisjcarr/scm/internal/agent/runtime"
)

const (
	currentWorkFilename = "current-work.json"
	lastResultFilename  = "last-result.json"
)

// CheckpointStore persists the current in-flight work item plus the most recent
// terminal result. The control plane remains the canonical audit log.
type CheckpointStore struct {
	stateDir string
}

var _ agentruntime.Repository = (*CheckpointStore)(nil)

type lastResult struct {
	WorkItemID    string    `json:"work_item_id"`
	ApplyID       string    `json:"apply_id"`
	State         string    `json:"state"`
	Summary       string    `json:"summary"`
	LastUpdatedAt time.Time `json:"last_updated_at"`
}

func NewCheckpointStore(stateDir string) (*CheckpointStore, error) {
	if stateDir == "" {
		return nil, fmt.Errorf("state dir is required")
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("create checkpoint state dir: %w", err)
	}
	return &CheckpointStore{stateDir: stateDir}, nil
}

func (s *CheckpointStore) SaveWork(ctx context.Context, work agentdomain.LocalApply) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.writeJSONAtomic(s.currentWorkPath(), work); err != nil {
		return fmt.Errorf("save current work checkpoint: %w", err)
	}
	return nil
}

func (s *CheckpointStore) UpdateWork(ctx context.Context, workItemID string, state string, summary string, updatedAt time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	work, err := s.loadCurrentWork()
	if err != nil {
		return fmt.Errorf("load current work checkpoint: %w", err)
	}
	if work.WorkItemID != "" && work.WorkItemID != workItemID {
		return fmt.Errorf("current checkpoint work item mismatch: have %s want %s", work.WorkItemID, workItemID)
	}

	result := lastResult{
		WorkItemID:    workItemID,
		ApplyID:       work.ApplyID,
		State:         state,
		Summary:       summary,
		LastUpdatedAt: updatedAt.UTC(),
	}
	if err := s.writeJSONAtomic(s.lastResultPath(), result); err != nil {
		return fmt.Errorf("save last result checkpoint: %w", err)
	}
	if err := os.Remove(s.currentWorkPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove current work checkpoint: %w", err)
	}
	return nil
}

func (s *CheckpointStore) loadCurrentWork() (agentdomain.LocalApply, error) {
	var work agentdomain.LocalApply
	data, err := os.ReadFile(s.currentWorkPath())
	if err != nil {
		if os.IsNotExist(err) {
			return work, nil
		}
		return work, err
	}
	if err := json.Unmarshal(data, &work); err != nil {
		return work, fmt.Errorf("decode current work checkpoint: %w", err)
	}
	return work, nil
}

func (s *CheckpointStore) currentWorkPath() string {
	return filepath.Join(s.stateDir, currentWorkFilename)
}

func (s *CheckpointStore) lastResultPath() string {
	return filepath.Join(s.stateDir, lastResultFilename)
}

func (s *CheckpointStore) writeJSONAtomic(path string, value interface{}) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal checkpoint JSON: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(s.stateDir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp checkpoint file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp checkpoint file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp checkpoint file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp checkpoint file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename checkpoint file into place: %w", err)
	}
	return nil
}
