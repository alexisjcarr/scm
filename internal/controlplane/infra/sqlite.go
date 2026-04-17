package infra

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	cpapp "github.com/alexisjcarr/scm/internal/controlplane/app"
	applysvc "github.com/alexisjcarr/scm/internal/controlplane/apply"
	cpdomain "github.com/alexisjcarr/scm/internal/controlplane/domain"
	"github.com/alexisjcarr/scm/internal/controlplane/inventory"
	"github.com/alexisjcarr/scm/internal/controlplane/workqueue"
	_ "modernc.org/sqlite"
)

// SQLiteRepository stores control-plane state in a single SQLite database.
type SQLiteRepository struct {
	db *sql.DB
}

var _ cpapp.Repository = (*SQLiteRepository)(nil)
var _ inventory.Repository = (*SQLiteRepository)(nil)
var _ applysvc.Store = (*SQLiteRepository)(nil)
var _ workqueue.Store = (*SQLiteRepository)(nil)

func NewSQLiteRepository(dsn string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
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
	statements := []string{
		`CREATE TABLE IF NOT EXISTS agents (
			agent_id TEXT PRIMARY KEY,
			host_id TEXT NOT NULL,
			version TEXT NOT NULL,
			labels_json TEXT NOT NULL,
			capabilities_json TEXT NOT NULL,
			idle INTEGER NOT NULL,
			current_work_item_id TEXT NOT NULL,
			last_seen_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS applies (
			apply_id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			submitted_by TEXT NOT NULL,
			raw_manifest TEXT NOT NULL,
			manifest_json TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS work_items (
			work_item_id TEXT PRIMARY KEY,
			apply_id TEXT NOT NULL,
			host_id TEXT NOT NULL,
			state TEXT NOT NULL,
			summary TEXT NOT NULL,
			lease_token TEXT NOT NULL,
			assigned_agent_id TEXT NOT NULL,
			manifest_json TEXT NOT NULL,
			assigned_at TEXT,
			lease_expires_at TEXT,
			updated_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_work_items_claim ON work_items(state, host_id, updated_at);`,
		`CREATE TABLE IF NOT EXISTS apply_events (
			row_id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_id TEXT NOT NULL,
			apply_id TEXT NOT NULL,
			host_id TEXT NOT NULL,
			work_item_id TEXT NOT NULL,
			level TEXT NOT NULL,
			phase TEXT NOT NULL,
			message TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
	}

	for _, statement := range statements {
		if _, err := r.db.Exec(statement); err != nil {
			return fmt.Errorf("initialize sqlite schema: %w", err)
		}
	}

	return nil
}

func (r *SQLiteRepository) UpsertAgent(ctx context.Context, agent cpdomain.Agent) error {
	labels, err := json.Marshal(agent.Labels)
	if err != nil {
		return fmt.Errorf("marshal agent labels: %w", err)
	}
	caps, err := json.Marshal(agent.Capabilities)
	if err != nil {
		return fmt.Errorf("marshal agent capabilities: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO agents (agent_id, host_id, version, labels_json, capabilities_json, idle, current_work_item_id, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(agent_id) DO UPDATE SET
			host_id=excluded.host_id,
			version=excluded.version,
			labels_json=excluded.labels_json,
			capabilities_json=excluded.capabilities_json,
			idle=excluded.idle,
			current_work_item_id=excluded.current_work_item_id,
			last_seen_at=excluded.last_seen_at`,
		agent.AgentID,
		agent.HostID,
		agent.Version,
		string(labels),
		string(caps),
		boolToInt(agent.Idle),
		agent.CurrentWorkItemID,
		agent.LastSeenAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert agent: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) UpdateHeartbeat(ctx context.Context, agentID string, idle bool, currentWorkItemID string, now time.Time) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE agents
		SET idle = ?, current_work_item_id = ?, last_seen_at = ?
		WHERE agent_id = ?`,
		boolToInt(idle), currentWorkItemID, now.Format(time.RFC3339Nano), agentID,
	)
	if err != nil {
		return fmt.Errorf("update heartbeat: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("unknown agent")
	}
	return nil
}

func (r *SQLiteRepository) ListAgents(ctx context.Context) ([]cpdomain.Agent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT agent_id, host_id, version, labels_json, capabilities_json, idle, current_work_item_id, last_seen_at
		FROM agents
		ORDER BY host_id`)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var agents []cpdomain.Agent
	for rows.Next() {
		var (
			agent               cpdomain.Agent
			labelsJSON          string
			capabilitiesJSON    string
			idle                int
			lastSeenAtFormatted string
		)
		if err := rows.Scan(&agent.AgentID, &agent.HostID, &agent.Version, &labelsJSON, &capabilitiesJSON, &idle, &agent.CurrentWorkItemID, &lastSeenAtFormatted); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		if err := json.Unmarshal([]byte(labelsJSON), &agent.Labels); err != nil {
			return nil, fmt.Errorf("decode agent labels: %w", err)
		}
		if err := json.Unmarshal([]byte(capabilitiesJSON), &agent.Capabilities); err != nil {
			return nil, fmt.Errorf("decode agent capabilities: %w", err)
		}
		agent.Idle = idle == 1
		agent.LastSeenAt, err = time.Parse(time.RFC3339Nano, lastSeenAtFormatted)
		if err != nil {
			return nil, fmt.Errorf("parse agent heartbeat: %w", err)
		}
		agents = append(agents, agent)
	}
	return agents, rows.Err()
}

func (r *SQLiteRepository) ListApplies(ctx context.Context) ([]cpdomain.Apply, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT apply_id, name, status, submitted_by, raw_manifest, manifest_json, created_at
		FROM applies
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list applies: %w", err)
	}
	defer rows.Close()

	var applies []cpdomain.Apply
	for rows.Next() {
		var apply cpdomain.Apply
		var createdAt string
		if err := rows.Scan(&apply.ApplyID, &apply.Name, &apply.Status, &apply.SubmittedBy, &apply.RawManifest, &apply.ManifestJSON, &createdAt); err != nil {
			return nil, fmt.Errorf("scan apply: %w", err)
		}
		apply.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse apply created_at: %w", err)
		}
		applies = append(applies, apply)
	}
	return applies, rows.Err()
}

func (r *SQLiteRepository) CreateApply(ctx context.Context, apply cpdomain.Apply, workItems []cpdomain.WorkItem, events []cpdomain.ApplyEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create apply tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO applies (apply_id, name, status, submitted_by, raw_manifest, manifest_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		apply.ApplyID,
		apply.Name,
		apply.Status,
		apply.SubmittedBy,
		apply.RawManifest,
		apply.ManifestJSON,
		apply.CreatedAt.Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("insert apply: %w", err)
	}

	for _, work := range workItems {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_items (work_item_id, apply_id, host_id, state, summary, lease_token, assigned_agent_id, manifest_json, assigned_at, lease_expires_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			work.WorkItemID,
			work.ApplyID,
			work.HostID,
			work.State,
			work.Summary,
			work.LeaseToken,
			work.AssignedAgentID,
			work.ManifestJSON,
			timePtrString(work.AssignedAt),
			timePtrString(work.LeaseExpiresAt),
			work.UpdatedAt.Format(time.RFC3339Nano),
		); err != nil {
			return fmt.Errorf("insert work item: %w", err)
		}
	}

	if err := insertEvents(ctx, tx, events); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *SQLiteRepository) GetApply(ctx context.Context, applyID string) (cpdomain.Apply, []cpdomain.WorkItem, error) {
	var apply cpdomain.Apply
	var createdAt string
	if err := r.db.QueryRowContext(ctx, `
		SELECT apply_id, name, status, submitted_by, raw_manifest, manifest_json, created_at
		FROM applies
		WHERE apply_id = ?`,
		applyID,
	).Scan(&apply.ApplyID, &apply.Name, &apply.Status, &apply.SubmittedBy, &apply.RawManifest, &apply.ManifestJSON, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return cpdomain.Apply{}, nil, errors.New("apply not found")
		}
		return cpdomain.Apply{}, nil, fmt.Errorf("load apply: %w", err)
	}
	var err error
	apply.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return cpdomain.Apply{}, nil, fmt.Errorf("parse apply timestamp: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT work_item_id, apply_id, host_id, state, summary, lease_token, assigned_agent_id, manifest_json, assigned_at, lease_expires_at, updated_at
		FROM work_items
		WHERE apply_id = ?
		ORDER BY host_id`, applyID)
	if err != nil {
		return cpdomain.Apply{}, nil, fmt.Errorf("load apply work items: %w", err)
	}
	defer rows.Close()

	var workItems []cpdomain.WorkItem
	for rows.Next() {
		work, err := scanWorkItem(rows)
		if err != nil {
			return cpdomain.Apply{}, nil, err
		}
		workItems = append(workItems, work)
	}
	return apply, workItems, rows.Err()
}

func (r *SQLiteRepository) ListEvents(ctx context.Context, applyID string, fromOffset int64) ([]cpdomain.ApplyEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT row_id, event_id, apply_id, host_id, work_item_id, level, phase, message, created_at
		FROM apply_events
		WHERE apply_id = ? AND row_id > ?
		ORDER BY row_id ASC`, applyID, fromOffset)
	if err != nil {
		return nil, fmt.Errorf("list apply events: %w", err)
	}
	defer rows.Close()

	var events []cpdomain.ApplyEvent
	for rows.Next() {
		var (
			rowID int64
			event cpdomain.ApplyEvent
			ts    string
		)
		if err := rows.Scan(&rowID, &event.ID, &event.ApplyID, &event.HostID, &event.WorkItemID, &event.Level, &event.Phase, &event.Message, &ts); err != nil {
			return nil, fmt.Errorf("scan apply event: %w", err)
		}
		event.CreatedAt, err = time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return nil, fmt.Errorf("parse apply event timestamp: %w", err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *SQLiteRepository) ClaimNextWork(ctx context.Context, agentID string, leaseDuration time.Duration, now time.Time) (*cpdomain.WorkItem, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin work claim tx: %w", err)
	}
	defer tx.Rollback()

	var hostID string
	if err := tx.QueryRowContext(ctx, `
		SELECT host_id
		FROM agents
		WHERE agent_id = ?`, agentID,
	).Scan(&hostID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("agent %q is not registered", agentID)
		}
		return nil, fmt.Errorf("load agent host for claim: %w", err)
	}

	var workID string
	err = tx.QueryRowContext(ctx, `
		SELECT work_item_id
		FROM work_items
		WHERE state = ? AND host_id = ?
		ORDER BY updated_at ASC
		LIMIT 1`, cpdomain.WorkStatePending, hostID,
	).Scan(&workID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select pending work item: %w", err)
	}

	leaseToken := fmt.Sprintf("%s-%d", agentID, now.UnixNano())
	leaseExpires := now.Add(leaseDuration)
	result, err := tx.ExecContext(ctx, `
		UPDATE work_items
		SET state = ?, assigned_agent_id = ?, lease_token = ?, assigned_at = ?, lease_expires_at = ?, updated_at = ?
		WHERE work_item_id = ? AND state = ?`,
		cpdomain.WorkStateAssigned,
		agentID,
		leaseToken,
		now.Format(time.RFC3339Nano),
		leaseExpires.Format(time.RFC3339Nano),
		now.Format(time.RFC3339Nano),
		workID,
		cpdomain.WorkStatePending,
	)
	if err != nil {
		return nil, fmt.Errorf("claim work item: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil, nil
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE agents
		SET idle = 0, current_work_item_id = ?, last_seen_at = ?
		WHERE agent_id = ?`, workID, now.Format(time.RFC3339Nano), agentID); err != nil {
		return nil, fmt.Errorf("mark agent busy: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT work_item_id, apply_id, host_id, state, summary, lease_token, assigned_agent_id, manifest_json, assigned_at, lease_expires_at, updated_at
		FROM work_items
		WHERE work_item_id = ?`, workID)
	if err != nil {
		return nil, fmt.Errorf("load claimed work item: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, errors.New("claimed work item disappeared")
	}
	work, err := scanWorkItem(rows)
	if err != nil {
		return nil, err
	}

	if err := insertEvents(ctx, tx, []cpdomain.ApplyEvent{{
		ID:         fmt.Sprintf("%s-claimed", work.WorkItemID),
		ApplyID:    work.ApplyID,
		HostID:     work.HostID,
		WorkItemID: work.WorkItemID,
		Level:      "info",
		Phase:      "assigned",
		Message:    fmt.Sprintf("work claimed by agent %s", agentID),
		CreatedAt:  now,
	}}); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit work claim: %w", err)
	}
	return &work, nil
}

func (r *SQLiteRepository) UpdateWork(ctx context.Context, agentID string, workItemID string, leaseToken string, state string, summary string, events []cpdomain.ApplyEvent, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update work tx: %w", err)
	}
	defer tx.Rollback()

	var (
		applyID         string
		currentLease    string
		assignedAgentID string
	)
	if err := tx.QueryRowContext(ctx, `
		SELECT apply_id, lease_token, assigned_agent_id
		FROM work_items
		WHERE work_item_id = ?`, workItemID,
	).Scan(&applyID, &currentLease, &assignedAgentID); err != nil {
		return fmt.Errorf("load work item lease: %w", err)
	}

	if currentLease != leaseToken || assignedAgentID != agentID {
		return errors.New("work item lease token does not match claiming agent")
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE work_items
		SET state = ?, summary = ?, updated_at = ?
		WHERE work_item_id = ?`,
		state, summary, now.Format(time.RFC3339Nano), workItemID,
	); err != nil {
		return fmt.Errorf("update work item: %w", err)
	}

	if state == cpdomain.WorkStateCompleted || state == cpdomain.WorkStateFailed {
		if _, err := tx.ExecContext(ctx, `
			UPDATE agents
			SET idle = 1, current_work_item_id = '', last_seen_at = ?
			WHERE agent_id = ?`,
			now.Format(time.RFC3339Nano), agentID,
		); err != nil {
			return fmt.Errorf("mark agent idle: %w", err)
		}
	}

	if err := insertEvents(ctx, tx, events); err != nil {
		return err
	}

	status, err := aggregateApplyStatus(ctx, tx, applyID)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE applies SET status = ? WHERE apply_id = ?`, status, applyID); err != nil {
		return fmt.Errorf("update apply status: %w", err)
	}

	return tx.Commit()
}

func (r *SQLiteRepository) ReconcileStalled(ctx context.Context, now time.Time, stalledAfter time.Duration) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin stalled reconcile tx: %w", err)
	}
	defer tx.Rollback()

	type agentSnapshot struct {
		lastSeenAt time.Time
	}
	agents := make(map[string]agentSnapshot)
	agentRows, err := tx.QueryContext(ctx, `SELECT host_id, last_seen_at FROM agents`)
	if err != nil {
		return fmt.Errorf("load agents for stalled reconcile: %w", err)
	}
	for agentRows.Next() {
		var hostID, ts string
		if err := agentRows.Scan(&hostID, &ts); err != nil {
			agentRows.Close()
			return fmt.Errorf("scan agent for stalled reconcile: %w", err)
		}
		lastSeenAt, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			agentRows.Close()
			return fmt.Errorf("parse agent last_seen_at: %w", err)
		}
		agents[hostID] = agentSnapshot{lastSeenAt: lastSeenAt}
	}
	if err := agentRows.Err(); err != nil {
		agentRows.Close()
		return fmt.Errorf("iterate agents for stalled reconcile: %w", err)
	}
	agentRows.Close()

	rows, err := tx.QueryContext(ctx, `
		SELECT work_item_id, apply_id, host_id, state, lease_expires_at, updated_at
		FROM work_items
		WHERE state IN (?, ?, ?)`,
		cpdomain.WorkStatePending, cpdomain.WorkStateAssigned, cpdomain.WorkStateRunning,
	)
	if err != nil {
		return fmt.Errorf("load active work items for stalled reconcile: %w", err)
	}
	type candidate struct {
		workItemID     string
		applyID        string
		hostID         string
		state          string
		leaseExpiresAt *time.Time
		updatedAt      time.Time
	}
	var candidates []candidate
	for rows.Next() {
		var (
			candidate candidate
			leaseTS   sql.NullString
			updatedTS string
		)
		if err := rows.Scan(&candidate.workItemID, &candidate.applyID, &candidate.hostID, &candidate.state, &leaseTS, &updatedTS); err != nil {
			rows.Close()
			return fmt.Errorf("scan work item for stalled reconcile: %w", err)
		}
		candidate.updatedAt, err = time.Parse(time.RFC3339Nano, updatedTS)
		if err != nil {
			rows.Close()
			return fmt.Errorf("parse work updated_at: %w", err)
		}
		if leaseTS.Valid && leaseTS.String != "" {
			leaseExpiresAt, err := time.Parse(time.RFC3339Nano, leaseTS.String)
			if err != nil {
				rows.Close()
				return fmt.Errorf("parse lease_expires_at: %w", err)
			}
			candidate.leaseExpiresAt = &leaseExpiresAt
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate work items for stalled reconcile: %w", err)
	}
	rows.Close()

	stallDeadline := now.Add(-stalledAfter)
	affectedApplies := make(map[string]struct{})
	for _, item := range candidates {
		agent, ok := agents[item.hostID]
		agentHealthy := ok && !agent.lastSeenAt.Before(stallDeadline)

		shouldStall := false
		summary := ""
		switch item.state {
		case cpdomain.WorkStatePending:
			if !agentHealthy {
				shouldStall = true
				summary = "no healthy agent heartbeat for target host"
			}
		case cpdomain.WorkStateAssigned, cpdomain.WorkStateRunning:
			if item.leaseExpiresAt != nil && item.leaseExpiresAt.Before(now) && !agentHealthy {
				shouldStall = true
				summary = "work lease expired without agent progress"
			}
		}
		if !shouldStall {
			continue
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE work_items
			SET state = ?, summary = ?, updated_at = ?
			WHERE work_item_id = ?`,
			cpdomain.WorkStateStalled, summary, now.Format(time.RFC3339Nano), item.workItemID,
		); err != nil {
			return fmt.Errorf("mark work item stalled: %w", err)
		}

		if err := insertEvents(ctx, tx, []cpdomain.ApplyEvent{{
			ID:         fmt.Sprintf("%s-stalled", item.workItemID),
			ApplyID:    item.applyID,
			HostID:     item.hostID,
			WorkItemID: item.workItemID,
			Level:      "warn",
			Phase:      "stalled",
			Message:    summary,
			CreatedAt:  now,
		}}); err != nil {
			return err
		}
		affectedApplies[item.applyID] = struct{}{}
	}

	for applyID := range affectedApplies {
		status, err := aggregateApplyStatus(ctx, tx, applyID)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE applies SET status = ? WHERE apply_id = ?`, status, applyID); err != nil {
			return fmt.Errorf("update stalled apply status: %w", err)
		}
	}

	return tx.Commit()
}

func aggregateApplyStatus(ctx context.Context, tx *sql.Tx, applyID string) (string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT state FROM work_items WHERE apply_id = ?`, applyID)
	if err != nil {
		return "", fmt.Errorf("load work item states: %w", err)
	}
	defer rows.Close()

	states := make([]string, 0, 8)
	for rows.Next() {
		var state string
		if err := rows.Scan(&state); err != nil {
			return "", fmt.Errorf("scan work item state: %w", err)
		}
		states = append(states, state)
	}
	return applysvc.AggregateStatus(states), nil
}

func insertEvents(ctx context.Context, tx *sql.Tx, events []cpdomain.ApplyEvent) error {
	for _, event := range events {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO apply_events (event_id, apply_id, host_id, work_item_id, level, phase, message, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			event.ID,
			event.ApplyID,
			event.HostID,
			event.WorkItemID,
			event.Level,
			event.Phase,
			event.Message,
			event.CreatedAt.Format(time.RFC3339Nano),
		); err != nil {
			return fmt.Errorf("insert apply event: %w", err)
		}
	}
	return nil
}

func scanWorkItem(rows interface{ Scan(...interface{}) error }) (cpdomain.WorkItem, error) {
	var (
		work           cpdomain.WorkItem
		assignedAt     sql.NullString
		leaseExpiresAt sql.NullString
		updatedAt      string
	)
	if err := rows.Scan(
		&work.WorkItemID,
		&work.ApplyID,
		&work.HostID,
		&work.State,
		&work.Summary,
		&work.LeaseToken,
		&work.AssignedAgentID,
		&work.ManifestJSON,
		&assignedAt,
		&leaseExpiresAt,
		&updatedAt,
	); err != nil {
		return cpdomain.WorkItem{}, fmt.Errorf("scan work item: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return cpdomain.WorkItem{}, fmt.Errorf("parse work item update time: %w", err)
	}
	work.UpdatedAt = parsedUpdatedAt

	if assignedAt.Valid {
		ts, err := time.Parse(time.RFC3339Nano, assignedAt.String)
		if err != nil {
			return cpdomain.WorkItem{}, fmt.Errorf("parse assigned_at: %w", err)
		}
		work.AssignedAt = &ts
	}
	if leaseExpiresAt.Valid {
		ts, err := time.Parse(time.RFC3339Nano, leaseExpiresAt.String)
		if err != nil {
			return cpdomain.WorkItem{}, fmt.Errorf("parse lease_expires_at: %w", err)
		}
		work.LeaseExpiresAt = &ts
	}

	return work, nil
}

func timePtrString(ts *time.Time) interface{} {
	if ts == nil {
		return nil
	}
	return ts.Format(time.RFC3339Nano)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
