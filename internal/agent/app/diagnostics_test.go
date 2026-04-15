package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	platformmetrics "github.com/alexisjcarr/scm/internal/platform/metrics"
)

type trackerStatusProvider struct {
	tracker *statusTracker
}

func (p trackerStatusProvider) StatusSnapshot() StatusSnapshot {
	return p.tracker.snapshot()
}

func TestDiagnosticsHandlerExposesHealthReadinessAndStatus(t *testing.T) {
	t.Parallel()

	tracker := newStatusTracker("agent-1", "host-1", "dev")
	handler := NewDiagnosticsHandler(platformmetrics.NewRegistry(), trackerStatusProvider{tracker: tracker})

	healthReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthRec := httptest.NewRecorder()
	handler.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("expected /healthz 200, got %d", healthRec.Code)
	}

	readyReq := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	readyRec := httptest.NewRecorder()
	handler.ServeHTTP(readyRec, readyReq)
	if readyRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected /readyz 503 before control-plane connectivity, got %d", readyRec.Code)
	}

	now := time.Date(2026, 4, 15, 18, 30, 0, 0, time.UTC)
	tracker.markRegisterSuccess()
	tracker.markHeartbeatSuccess(now, false, "work-42")
	tracker.markWorkReportSuccess(now.Add(10*time.Second), "work-42", "running")

	readyRec = httptest.NewRecorder()
	handler.ServeHTTP(readyRec, readyReq)
	if readyRec.Code != http.StatusOK {
		t.Fatalf("expected /readyz 200 after control-plane connectivity, got %d", readyRec.Code)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/status", nil)
	statusRec := httptest.NewRecorder()
	handler.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected /status 200, got %d", statusRec.Code)
	}

	var snapshot StatusSnapshot
	if err := json.Unmarshal(statusRec.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("decode /status response: %v", err)
	}
	if snapshot.AgentID != "agent-1" || snapshot.HostID != "host-1" {
		t.Fatalf("unexpected identity payload: %+v", snapshot)
	}
	if !snapshot.ConnectedToControlPlane {
		t.Fatalf("expected connected status in %+v", snapshot)
	}
	if snapshot.State != "applying" {
		t.Fatalf("expected operator-facing applying state, got %q", snapshot.State)
	}
	if snapshot.CurrentWorkItemID != "work-42" {
		t.Fatalf("expected current work item id, got %q", snapshot.CurrentWorkItemID)
	}
	if snapshot.LastSuccessfulHeartbeatAt != now.Format(time.RFC3339) {
		t.Fatalf("unexpected heartbeat timestamp: %+v", snapshot)
	}
	if snapshot.LastWorkReportAt != now.Add(10*time.Second).Format(time.RFC3339) {
		t.Fatalf("unexpected work report timestamp: %+v", snapshot)
	}
}
