package httpapi

import (
	"errors"
	"net/http"
	"testing"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func TestStatusForErrorRecognizesRunNotFound(t *testing.T) {
	t.Parallel()

	status, code := statusForError(harnessruntime.NewRunNotFoundError("run_1", errors.New("missing")))
	if status != http.StatusNotFound || code != "not_found" {
		t.Fatalf("expected 404/not_found, got %d/%s", status, code)
	}
}

func TestStatusForErrorRecognizesUnsupportedProvider(t *testing.T) {
	t.Parallel()

	status, code := statusForError(harnessruntime.NewUnsupportedProviderError("bad"))
	if status != http.StatusBadRequest || code != "invalid_request" {
		t.Fatalf("expected 400/invalid_request, got %d/%s", status, code)
	}
}
