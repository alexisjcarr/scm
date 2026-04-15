package infra

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	agentdomain "github.com/alexisjcarr/scm/internal/agent/domain"
	agentruntime "github.com/alexisjcarr/scm/internal/agent/runtime"
	_ "modernc.org/sqlite"
)

// SQLiteRepository stores host-local work metadata.
type SQLiteRepository struct {
	db *sql.DB
}

var _ agentruntime.Repository = (*SQLiteRepository)(nil)

func NewSQLiteRepository(dsn string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open agent sqlite database: %w", err)
	}
	repo := &SQLiteRepository{db: db}
	if err := repo.initSchema(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}

func (r *SQLiteRepository) initSchema() error {
	_, err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS local_work_items (
			work_item_id TEXT PRIMARY KEY,
			apply_id TEXT NOT NULL,
			manifest_json TEXT NOT NULL,
			state TEXT NOT NULL,
			summary TEXT NOT NULL,
			lease_token TEXT NOT NULL,
			last_updated_at TEXT NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("initialize agent schema: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) SaveWork(ctx context.Context, work agentdomain.LocalApply) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO local_work_items (work_item_id, apply_id, manifest_json, state, summary, lease_token, last_updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(work_item_id) DO UPDATE SET
			manifest_json=excluded.manifest_json,
			state=excluded.state,
			summary=excluded.summary,
			lease_token=excluded.lease_token,
			last_updated_at=excluded.last_updated_at`,
		work.WorkItemID,
		work.ApplyID,
		work.ManifestJSON,
		work.State,
		work.Summary,
		work.LeaseToken,
		work.LastUpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("save local work item: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) UpdateWork(ctx context.Context, workItemID string, state string, summary string, updatedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE local_work_items
		SET state = ?, summary = ?, last_updated_at = ?
		WHERE work_item_id = ?`,
		state, summary, updatedAt.Format(time.RFC3339Nano), workItemID,
	)
	if err != nil {
		return fmt.Errorf("update local work item: %w", err)
	}
	return nil
}
