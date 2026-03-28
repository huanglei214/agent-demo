package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

type errorResponse struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if value == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{
		Error: apiError{
			Code:    code,
			Message: message,
		},
	})
}

func decodeJSON(r *http.Request, out any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
}

func writeServiceError(w http.ResponseWriter, err error) {
	status, code := statusForError(err)
	writeError(w, status, code, err.Error())
}

func statusForError(err error) (int, string) {
	switch {
	case err == nil:
		return http.StatusOK, ""
	case errors.Is(err, harnessruntime.ErrRunNotFound), errors.Is(err, harnessruntime.ErrSessionNotFound), errors.Is(err, os.ErrNotExist), os.IsNotExist(err):
		return http.StatusNotFound, "not_found"
	case errors.Is(err, harnessruntime.ErrUnsupportedProvider):
		return http.StatusBadRequest, "invalid_request"
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "unsupported model provider"),
		strings.Contains(message, "workspace does not match"),
		strings.Contains(message, "empty plan"),
		strings.Contains(message, "cannot be resumed"),
		strings.Contains(message, "requires manual intervention"),
		strings.Contains(message, "not resumable"),
		strings.Contains(message, "not found"),
		strings.Contains(message, "invalid"):
		return http.StatusBadRequest, "invalid_request"
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}
