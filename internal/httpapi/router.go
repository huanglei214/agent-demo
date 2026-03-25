package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/huanglei214/agent-demo/internal/app"
)

type Server struct {
	services app.Services
}

func NewRouter(services app.Services) http.Handler {
	server := Server{services: services}

	router := chi.NewRouter()
	router.Get("/healthz", server.handleHealth)

	router.Route("/api", func(r chi.Router) {
		r.Get("/sessions", server.handleListSessions)
		r.Post("/sessions", server.handleCreateSession)
		r.Get("/sessions/{sessionID}", server.handleInspectSession)

		r.Get("/runs", server.handleListRuns)
		r.Post("/runs", server.handleStartRun)
		r.Post("/runs/{runID}/resume", server.handleResumeRun)
		r.Get("/runs/{runID}", server.handleInspectRun)
		r.Get("/runs/{runID}/replay", server.handleReplaySummary)
		r.Get("/runs/{runID}/events", server.handleReplayEvents)
		r.Get("/runs/{runID}/stream", server.handleRunStream)

		r.Get("/tools", server.handleListTools)
	})

	return router
}

func (s Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
	})
}
