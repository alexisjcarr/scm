package app

import (
	"sync"
	"time"
)

// StatusSnapshot is the public diagnostics payload served by the agent.
type StatusSnapshot struct {
	HostID                    string `json:"host_id"`
	AgentID                   string `json:"agent_id"`
	Version                   string `json:"version"`
	ConnectedToControlPlane   bool   `json:"connected_to_control_plane"`
	CurrentWorkItemID         string `json:"current_work_item_id,omitempty"`
	State                     string `json:"state"`
	LastSuccessfulHeartbeatAt string `json:"last_successful_heartbeat_at,omitempty"`
	LastWorkReportAt          string `json:"last_work_report_at,omitempty"`
}

type statusTracker struct {
	mu sync.RWMutex

	hostID                  string
	agentID                 string
	version                 string
	connectedToControlPlane bool
	currentWorkItemID       string
	state                   string
	lastHeartbeatAt         *time.Time
	lastWorkReportAt        *time.Time
}

func newStatusTracker(agentID, hostID, version string) *statusTracker {
	return &statusTracker{
		hostID:  hostID,
		agentID: agentID,
		version: version,
		state:   "starting",
	}
}

func (s *statusTracker) markRegisterSuccess() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connectedToControlPlane = true
	if s.state == "starting" {
		s.state = "ready"
	}
}

func (s *statusTracker) markConnectionFailure() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connectedToControlPlane = false
}

func (s *statusTracker) markHeartbeatSuccess(now time.Time, idle bool, currentWorkItemID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connectedToControlPlane = true
	s.lastHeartbeatAt = &now
	s.currentWorkItemID = currentWorkItemID
	if idle {
		s.state = "ready"
		if currentWorkItemID == "" {
			s.currentWorkItemID = ""
		}
		return
	}
	s.state = "applying"
}

func (s *statusTracker) markWorkReportSuccess(now time.Time, workItemID string, state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connectedToControlPlane = true
	s.lastWorkReportAt = &now
	switch state {
	case "completed", "failed":
		s.state = "ready"
		s.currentWorkItemID = ""
	default:
		s.state = "applying"
		s.currentWorkItemID = workItemID
	}
}

func (s *statusTracker) snapshot() StatusSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := StatusSnapshot{
		HostID:                  s.hostID,
		AgentID:                 s.agentID,
		Version:                 s.version,
		ConnectedToControlPlane: s.connectedToControlPlane,
		CurrentWorkItemID:       s.currentWorkItemID,
		State:                   s.state,
	}
	if s.lastHeartbeatAt != nil {
		snapshot.LastSuccessfulHeartbeatAt = s.lastHeartbeatAt.UTC().Format(time.RFC3339)
	}
	if s.lastWorkReportAt != nil {
		snapshot.LastWorkReportAt = s.lastWorkReportAt.UTC().Format(time.RFC3339)
	}
	return snapshot
}
