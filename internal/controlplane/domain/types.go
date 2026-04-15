package domain

import "time"

const (
	ApplyStatusPending   = "pending"
	ApplyStatusRunning   = "running"
	ApplyStatusCompleted = "completed"
	ApplyStatusFailed    = "failed"
)

const (
	WorkStatePending   = "pending"
	WorkStateAssigned  = "assigned"
	WorkStateRunning   = "running"
	WorkStateCompleted = "completed"
	WorkStateFailed    = "failed"
)

// Agent represents a registered host agent.
type Agent struct {
	AgentID           string
	HostID            string
	Version           string
	Labels            map[string]string
	Capabilities      []string
	Idle              bool
	CurrentWorkItemID string
	LastSeenAt        time.Time
}

// Apply tracks a user-submitted manifest rollout.
type Apply struct {
	ApplyID      string
	Name         string
	Status       string
	SubmittedBy  string
	RawManifest  string
	ManifestJSON string
	CreatedAt    time.Time
}

// WorkItem is the per-host executable unit created from an apply.
type WorkItem struct {
	WorkItemID      string
	ApplyID         string
	HostID          string
	State           string
	Summary         string
	LeaseToken      string
	AssignedAgentID string
	ManifestJSON    string
	AssignedAt      *time.Time
	LeaseExpiresAt  *time.Time
	UpdatedAt       time.Time
}

// ApplyEvent records operational progress for an apply.
type ApplyEvent struct {
	ID         string
	ApplyID    string
	HostID     string
	WorkItemID string
	Level      string
	Phase      string
	Message    string
	CreatedAt  time.Time
}
