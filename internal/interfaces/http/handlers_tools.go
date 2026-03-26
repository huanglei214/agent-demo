package httpapi

import "net/http"

func (s Server) handleListTools(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"tools": s.services.ListTools(),
	})
}
