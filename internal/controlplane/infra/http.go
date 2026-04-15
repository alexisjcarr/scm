package infra

import (
	"html/template"
	"net/http"
	"time"

	cpapp "github.com/alexisjcarr/scm/internal/controlplane/app"
	cpdomain "github.com/alexisjcarr/scm/internal/controlplane/domain"
	"github.com/alexisjcarr/scm/internal/controlplane/ui"
)

// NewHTTPHandler serves the read-only control plane UI.
func NewHTTPHandler(service *cpapp.Service) (http.Handler, error) {
	tmpl, err := template.ParseFS(
		ui.TemplatesFS,
		"templates/layout.html.tmpl",
		"templates/index.html.tmpl",
		"templates/apply.html.tmpl",
	)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		agents, err := service.GetAgents(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		applies, err := service.ListApplies(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data := struct {
			Now     time.Time
			Agents  []ui.AgentRow
			Applies []cpdomain.Apply
		}{
			Now:     time.Now().UTC(),
			Agents:  ui.AgentRows(agents),
			Applies: applies,
		}
		if err := tmpl.ExecuteTemplate(w, "index", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/applies/", func(w http.ResponseWriter, r *http.Request) {
		applyID := r.URL.Path[len("/applies/"):]
		apply, workItems, err := service.GetApply(r.Context(), applyID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		events, err := service.ListEvents(r.Context(), applyID, 0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data := struct {
			Apply   cpdomain.Apply
			Targets []cpdomain.WorkItem
			Events  []cpdomain.ApplyEvent
		}{
			Apply:   apply,
			Targets: workItems,
			Events:  events,
		}
		if err := tmpl.ExecuteTemplate(w, "apply", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	return mux, nil
}
