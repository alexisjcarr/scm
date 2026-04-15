package domain

import "time"

const (
	StateIdle = "idle"
	StateBusy = "busy"
)

const (
	PhaseAccepted  = "accepted"
	PhasePersisted = "persisted"
	PhasePlanning  = "planning"
	PhaseApplying  = "applying"
	PhaseNotifying = "notifying"
	PhaseReporting = "reporting"
	PhaseTerminal  = "terminal"
)

// LocalApply tracks work persisted on the host.
type LocalApply struct {
	WorkItemID    string
	ApplyID       string
	ManifestJSON  string
	State         string
	Summary       string
	LeaseToken    string
	LastUpdatedAt time.Time
}

// ResourceResult captures the outcome of a single resource reconciliation.
type ResourceResult struct {
	ResourceID   string
	ResourceType string
	Changed      bool
	Message      string
}

// ExecutionPlan records the ordered resources and notify follow-ups selected for execution.
type ExecutionPlan struct {
	OrderedIDs []string
	NotifyIDs  []string
}
