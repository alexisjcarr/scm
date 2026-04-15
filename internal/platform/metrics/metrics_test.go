package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewMetricsFamiliesRegisterAndServe(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	NewControlPlaneMetrics(reg)
	NewAgentMetrics(reg)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	Handler(reg).ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("expected metrics handler to return 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, metricName := range []string{
		"scm_inventory_registrations_total",
		"scm_agent_runtime_polls_total",
	} {
		if !strings.Contains(body, metricName) {
			t.Fatalf("expected metrics output to contain %s", metricName)
		}
	}
}
