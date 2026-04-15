package scmv1

// The files in this package are maintained to mirror the contracts in
// proto/scm/v1/scm.proto. The project includes a generate script for teams that
// prefer protoc-driven regeneration, but the checked-in code keeps the MVP easy
// to build in constrained environments.

// HostSelector matches registered agents by exact label equality.
type HostSelector struct {
	MatchLabels map[string]string `json:"match_labels,omitempty"`
}

// TargetSpec describes the hosts that should receive a manifest.
type TargetSpec struct {
	Hosts    []string      `json:"hosts,omitempty"`
	Selector *HostSelector `json:"selector,omitempty"`
}

// ManifestResource is the transport model shared between the CLI, control
// plane, and agents.
type ManifestResource struct {
	ID       string   `json:"id,omitempty"`
	Type     string   `json:"type,omitempty"`
	Name     string   `json:"name,omitempty"`
	Path     string   `json:"path,omitempty"`
	Content  string   `json:"content,omitempty"`
	Mode     string   `json:"mode,omitempty"`
	Owner    string   `json:"owner,omitempty"`
	Group    string   `json:"group,omitempty"`
	State    string   `json:"state,omitempty"`
	Enabled  bool     `json:"enabled,omitempty"`
	Requires []string `json:"requires,omitempty"`
	Notifies []string `json:"notifies,omitempty"`
}

// Manifest is the transport-safe compiled representation of a YAML manifest.
type Manifest struct {
	APIVersion string              `json:"api_version,omitempty"`
	Kind       string              `json:"kind,omitempty"`
	Name       string              `json:"name,omitempty"`
	Labels     map[string]string   `json:"labels,omitempty"`
	Target     *TargetSpec         `json:"target,omitempty"`
	Resources  []*ManifestResource `json:"resources,omitempty"`
}

// ApplyEvent captures incremental output for an apply.
type ApplyEvent struct {
	ID         string `json:"id,omitempty"`
	ApplyID    string `json:"apply_id,omitempty"`
	HostID     string `json:"host_id,omitempty"`
	WorkItemID string `json:"work_item_id,omitempty"`
	Level      string `json:"level,omitempty"`
	Phase      string `json:"phase,omitempty"`
	Message    string `json:"message,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
}

type SubmitApplyRequest struct {
	Manifest    *Manifest `json:"manifest,omitempty"`
	RawManifest string    `json:"raw_manifest,omitempty"`
	SubmittedBy string    `json:"submitted_by,omitempty"`
}

type SubmitApplyResponse struct {
	ApplyID     string `json:"apply_id,omitempty"`
	Status      string `json:"status,omitempty"`
	TargetCount int32  `json:"target_count,omitempty"`
}

type GetApplyRequest struct {
	ApplyID string `json:"apply_id,omitempty"`
}

type ApplyTargetSummary struct {
	HostID     string `json:"host_id,omitempty"`
	WorkItemID string `json:"work_item_id,omitempty"`
	Status     string `json:"status,omitempty"`
	Summary    string `json:"summary,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

type ApplySummary struct {
	ApplyID     string                `json:"apply_id,omitempty"`
	Name        string                `json:"name,omitempty"`
	Status      string                `json:"status,omitempty"`
	SubmittedBy string                `json:"submitted_by,omitempty"`
	CreatedAt   string                `json:"created_at,omitempty"`
	Targets     []*ApplyTargetSummary `json:"targets,omitempty"`
}

type StreamApplyEventsRequest struct {
	ApplyID    string `json:"apply_id,omitempty"`
	FromOffset int64  `json:"from_offset,omitempty"`
}

type RegisterAgentRequest struct {
	AgentID      string            `json:"agent_id,omitempty"`
	HostID       string            `json:"host_id,omitempty"`
	Version      string            `json:"version,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
}

type RegisterAgentResponse struct {
	AgentID string `json:"agent_id,omitempty"`
}

type HeartbeatRequest struct {
	AgentID           string `json:"agent_id,omitempty"`
	Idle              bool   `json:"idle,omitempty"`
	CurrentWorkItemID string `json:"current_work_item_id,omitempty"`
}

type HeartbeatResponse struct {
	Status string `json:"status,omitempty"`
}

type FetchWorkRequest struct {
	AgentID string `json:"agent_id,omitempty"`
}

type WorkItem struct {
	WorkItemID     string `json:"work_item_id,omitempty"`
	ApplyID        string `json:"apply_id,omitempty"`
	HostID         string `json:"host_id,omitempty"`
	LeaseToken     string `json:"lease_token,omitempty"`
	State          string `json:"state,omitempty"`
	ManifestJSON   string `json:"manifest_json,omitempty"`
	AssignedAt     string `json:"assigned_at,omitempty"`
	LeaseExpiresAt string `json:"lease_expires_at,omitempty"`
}

type FetchWorkResponse struct {
	HasWork  bool      `json:"has_work,omitempty"`
	WorkItem *WorkItem `json:"work_item,omitempty"`
}

type ReportWorkStatusRequest struct {
	AgentID    string        `json:"agent_id,omitempty"`
	WorkItemID string        `json:"work_item_id,omitempty"`
	LeaseToken string        `json:"lease_token,omitempty"`
	State      string        `json:"state,omitempty"`
	Summary    string        `json:"summary,omitempty"`
	Events     []*ApplyEvent `json:"events,omitempty"`
}

type ReportWorkStatusResponse struct {
	Status string `json:"status,omitempty"`
}

type AgentSummary struct {
	AgentID           string            `json:"agent_id,omitempty"`
	HostID            string            `json:"host_id,omitempty"`
	Idle              bool              `json:"idle,omitempty"`
	CurrentWorkItemID string            `json:"current_work_item_id,omitempty"`
	Version           string            `json:"version,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	LastSeenAt        string            `json:"last_seen_at,omitempty"`
}

type ListAgentsRequest struct{}

type ListAgentsResponse struct {
	Agents []*AgentSummary `json:"agents,omitempty"`
}
