package ui

import (
	"testing"
	"time"

	cpdomain "github.com/alexisjcarr/scm/internal/controlplane/domain"
)

func TestAgentRowsUsesOperatorFacingLabels(t *testing.T) {
	t.Parallel()

	rows := AgentRows([]cpdomain.Agent{
		{
			HostID:            "web-1",
			Labels:            map[string]string{"role": "web"},
			Idle:              true,
			LastSeenAt:        time.Date(2026, 4, 15, 12, 34, 56, 0, time.UTC),
			CurrentWorkItemID: "",
		},
		{
			HostID:            "web-2",
			Idle:              false,
			LastSeenAt:        time.Date(2026, 4, 15, 12, 35, 1, 0, time.UTC),
			CurrentWorkItemID: "work-1",
		},
	})

	if rows[0].StatusLabel != "ready" || rows[0].StatusClass != "ready" {
		t.Fatalf("expected ready row, got %+v", rows[0])
	}
	if rows[1].StatusLabel != "applying" || rows[1].StatusClass != "working" {
		t.Fatalf("expected applying row, got %+v", rows[1])
	}
}
