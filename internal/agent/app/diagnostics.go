package app

import (
	"encoding/json"
	"net/http"

	platformmetrics "github.com/alexisjcarr/scm/internal/platform/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// NewDiagnosticsHandler exposes the agent diagnostics surface on one HTTP listener.
func NewDiagnosticsHandler(reg *prometheus.Registry, statusProvider interface{ StatusSnapshot() StatusSnapshot }) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", platformmetrics.Handler(reg))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !statusProvider.StatusSnapshot().ConnectedToControlPlane {
			http.Error(w, "control plane not connected", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready\n"))
	})
	mux.HandleFunc("/status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(statusProvider.StatusSnapshot()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	return mux
}
