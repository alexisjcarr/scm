package infra

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	agentdomain "github.com/alexisjcarr/scm/internal/agent/domain"
)

func TestCheckpointStoreSaveWorkWritesCurrentWork(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	store, err := NewCheckpointStore(stateDir)
	if err != nil {
		t.Fatalf("NewCheckpointStore returned error: %v", err)
	}

	work := agentdomain.LocalApply{
		WorkItemID:    "work-1",
		ApplyID:       "apply-1",
		ManifestJSON:  `{"kind":"Manifest"}`,
		State:         "persisted",
		Summary:       "manifest persisted locally",
		LeaseToken:    "lease-1",
		LastUpdatedAt: time.Unix(123, 0).UTC(),
	}
	if err := store.SaveWork(context.Background(), work); err != nil {
		t.Fatalf("SaveWork returned error: %v", err)
	}

	var got agentdomain.LocalApply
	readJSON(t, filepath.Join(stateDir, currentWorkFilename), &got)
	if got.WorkItemID != work.WorkItemID || got.ApplyID != work.ApplyID || got.LeaseToken != work.LeaseToken {
		t.Fatalf("unexpected current work checkpoint: %+v", got)
	}
}

func TestCheckpointStoreUpdateWorkWritesLastResultAndClearsCurrentWork(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	store, err := NewCheckpointStore(stateDir)
	if err != nil {
		t.Fatalf("NewCheckpointStore returned error: %v", err)
	}

	if err := store.SaveWork(context.Background(), agentdomain.LocalApply{
		WorkItemID:    "work-1",
		ApplyID:       "apply-1",
		ManifestJSON:  `{"kind":"Manifest"}`,
		State:         "persisted",
		Summary:       "manifest persisted locally",
		LeaseToken:    "lease-1",
		LastUpdatedAt: time.Unix(123, 0).UTC(),
	}); err != nil {
		t.Fatalf("SaveWork returned error: %v", err)
	}

	updatedAt := time.Unix(456, 0).UTC()
	if err := store.UpdateWork(context.Background(), "work-1", "completed", "apply finished", updatedAt); err != nil {
		t.Fatalf("UpdateWork returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(stateDir, currentWorkFilename)); !os.IsNotExist(err) {
		t.Fatalf("expected current work checkpoint to be removed, stat err=%v", err)
	}

	var got lastResult
	readJSON(t, filepath.Join(stateDir, lastResultFilename), &got)
	if got.WorkItemID != "work-1" || got.ApplyID != "apply-1" || got.State != "completed" || got.Summary != "apply finished" || !got.LastUpdatedAt.Equal(updatedAt) {
		t.Fatalf("unexpected last result checkpoint: %+v", got)
	}
}

func TestCheckpointStoreSaveWorkOverwritesExistingCheckpoint(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	store, err := NewCheckpointStore(stateDir)
	if err != nil {
		t.Fatalf("NewCheckpointStore returned error: %v", err)
	}

	first := agentdomain.LocalApply{
		WorkItemID:    "work-1",
		ApplyID:       "apply-1",
		ManifestJSON:  `{"version":1}`,
		State:         "persisted",
		Summary:       "first",
		LeaseToken:    "lease-1",
		LastUpdatedAt: time.Unix(100, 0).UTC(),
	}
	second := first
	second.ManifestJSON = `{"version":2}`
	second.Summary = "second"
	second.LastUpdatedAt = time.Unix(200, 0).UTC()

	if err := store.SaveWork(context.Background(), first); err != nil {
		t.Fatalf("first SaveWork returned error: %v", err)
	}
	if err := store.SaveWork(context.Background(), second); err != nil {
		t.Fatalf("second SaveWork returned error: %v", err)
	}

	var got agentdomain.LocalApply
	readJSON(t, filepath.Join(stateDir, currentWorkFilename), &got)
	if got.ManifestJSON != second.ManifestJSON || got.Summary != second.Summary || !got.LastUpdatedAt.Equal(second.LastUpdatedAt) {
		t.Fatalf("expected overwrite to preserve latest checkpoint, got %+v", got)
	}
}

func readJSON(t *testing.T, path string, out interface{}) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
