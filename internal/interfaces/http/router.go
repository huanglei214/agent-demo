package httpapi

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/huanglei214/agent-demo/internal/service"
)

type Server struct {
	services service.Services
}

func NewRouter(services service.Services) http.Handler {
	server := Server{services: services}

	router := chi.NewRouter()
	router.Use(requestLogger)
	router.Get("/healthz", server.handleHealth)

	router.Route("/api", func(r chi.Router) {
		r.Post("/agui/chat", server.handleAGUIChat)

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

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		log.Printf(
			"http request method=%s path=%s status=%d duration=%s remote=%s",
			r.Method,
			r.URL.Path,
			recorder.status,
			time.Since(start).Truncate(time.Millisecond),
			r.RemoteAddr,
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (s Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
	})
}
