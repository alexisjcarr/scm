package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRegistry returns a registry with Go and process collectors installed.
func NewRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		prometheus.NewGoCollector(),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
	)
	return reg
}

// Handler returns a metrics HTTP handler bound to the provided registry.
func Handler(reg *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}

// ControlPlaneMetrics collects phase-oriented daemon counters.
type ControlPlaneMetrics struct {
	InventoryRegistrations prometheus.Counter
	InventoryHeartbeats    prometheus.Counter
	ApplySubmissions       prometheus.Counter
	WorkClaims             prometheus.Counter
	WorkReports            *prometheus.CounterVec
}

// AgentMetrics collects agent runtime counters.
type AgentMetrics struct {
	Polls        prometheus.Counter
	WorkStarted  prometheus.Counter
	WorkFinished *prometheus.CounterVec
}

// NewControlPlaneMetrics creates and registers control-plane metrics.
func NewControlPlaneMetrics(reg prometheus.Registerer) *ControlPlaneMetrics {
	metrics := &ControlPlaneMetrics{
		InventoryRegistrations: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "scm_inventory_registrations_total",
			Help: "Total successful agent registrations.",
		}),
		InventoryHeartbeats: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "scm_inventory_heartbeats_total",
			Help: "Total successful agent heartbeats.",
		}),
		ApplySubmissions: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "scm_apply_submissions_total",
			Help: "Total submitted applies.",
		}),
		WorkClaims: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "scm_work_claims_total",
			Help: "Total work items claimed by agents.",
		}),
		WorkReports: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "scm_work_reports_total",
			Help: "Total reported work transitions grouped by resulting state.",
		}, []string{"state"}),
	}
	registerAll(reg, metrics.InventoryRegistrations, metrics.InventoryHeartbeats, metrics.ApplySubmissions, metrics.WorkClaims, metrics.WorkReports)
	return metrics
}

// NewAgentMetrics creates and registers agent runtime metrics.
func NewAgentMetrics(reg prometheus.Registerer) *AgentMetrics {
	metrics := &AgentMetrics{
		Polls: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "scm_agent_runtime_polls_total",
			Help: "Total agent poll loop iterations.",
		}),
		WorkStarted: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "scm_agent_runtime_work_started_total",
			Help: "Total work items started by the agent runtime.",
		}),
		WorkFinished: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "scm_agent_runtime_work_finished_total",
			Help: "Total work items finished by the agent runtime grouped by result.",
		}, []string{"result"}),
	}
	registerAll(reg, metrics.Polls, metrics.WorkStarted, metrics.WorkFinished)
	return metrics
}

func registerAll(reg prometheus.Registerer, collectors ...prometheus.Collector) {
	if reg == nil {
		return
	}
	for _, collector := range collectors {
		reg.MustRegister(collector)
	}
}
