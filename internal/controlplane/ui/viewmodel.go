package ui

import cpdomain "github.com/alexisjcarr/scm/internal/controlplane/domain"

// AgentRow is the presentation model for the control plane host table.
type AgentRow struct {
	HostID            string
	Labels            map[string]string
	LastSeenAtLabel   string
	StatusLabel       string
	StatusClass       string
	CurrentWorkItemID string
}

// AgentRows converts domain agents into operator-facing UI labels.
func AgentRows(agents []cpdomain.Agent) []AgentRow {
	rows := make([]AgentRow, 0, len(agents))
	for _, agent := range agents {
		statusLabel := "applying"
		statusClass := "working"
		if agent.Idle {
			statusLabel = "ready"
			statusClass = "ready"
		}

		rows = append(rows, AgentRow{
			HostID:            agent.HostID,
			Labels:            agent.Labels,
			LastSeenAtLabel:   agent.LastSeenAt.Format("15:04:05"),
			StatusLabel:       statusLabel,
			StatusClass:       statusClass,
			CurrentWorkItemID: agent.CurrentWorkItemID,
		})
	}
	return rows
}
