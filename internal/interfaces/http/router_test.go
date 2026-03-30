package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/huanglei214/agent-demo/internal/config"
	httpapi "github.com/huanglei214/agent-demo/internal/interfaces/http"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/service"
)

func TestCreateAndInspectSession(t *testing.T) {
	t.Parallel()

	handler, services := newTestHandler(t)

	status, sessionBody := doJSONRequest(t, handler, http.MethodPost, "/api/sessions", map[string]any{
		"workspace": services.Config.Workspace,
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%#v", status, sessionBody)
	}

	session := sessionBody["session"].(map[string]any)
	sessionID := session["id"].(string)

	status, body := doJSONRequest(t, handler, http.MethodGet, "/api/sessions/"+sessionID+"?recent=5", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%#v", status, body)
	}
	if body["session"] == nil {
		t.Fatalf("expected session inspect response, got %#v", body)
	}
	if messages, ok := body["messages"].([]any); !ok || messages == nil {
		t.Fatalf("expected messages array, got %#v", body["messages"])
	}
	if runs, ok := body["runs"].([]any); !ok || runs == nil {
		t.Fatalf("expected runs array, got %#v", body["runs"])
	}
}

func TestListEndpoints(t *testing.T) {
	t.Parallel()

	handler, services := newTestHandler(t)

	firstSession, err := services.CreateSession(services.Config.Workspace)
	if err != nil {
		t.Fatal(err)
	}
	secondSession, err := services.CreateSession(services.Config.Workspace)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := services.StartRun(context.Background(), service.RunRequest{
		Instruction: "Summarize the repository",
		Workspace:   services.Config.Workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
		SessionID:   firstSession.ID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := services.StartRun(context.Background(), service.RunRequest{
		Instruction: "Check runtime files",
		Workspace:   services.Config.Workspace,
		Provider:    "mock",
		Model:       "mock-model",
		MaxTurns:    5,
		SessionID:   secondSession.ID,
	}); err != nil {
		t.Fatal(err)
	}

	status, sessionsBody := doJSONRequest(t, handler, http.MethodGet, "/api/sessions?limit=5", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%#v", status, sessionsBody)
	}
	sessions := sessionsBody["sessions"].([]any)
	if len(sessions) < 2 {
		t.Fatalf("expected at least 2 sessions, got %#v", sessionsBody)
	}

	status, runsBody := doJSONRequest(t, handler, http.MethodGet, "/api/runs?limit=5", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%#v", status, runsBody)
	}
	runs := runsBody["runs"].([]any)
	if len(runs) < 2 {
		t.Fatalf("expected at least 2 runs, got %#v", runsBody)
	}
}

func TestListSessionsReturnsEmptyArrayWhenNoSessionsExist(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	status, body := doJSONRequest(t, handler, http.MethodGet, "/api/sessions?limit=5", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%#v", status, body)
	}
	sessions, ok := body["sessions"].([]any)
	if !ok || sessions == nil {
		t.Fatalf("expected sessions array, got %#v", body["sessions"])
	}
	if len(sessions) != 0 {
		t.Fatalf("expected empty sessions list, got %#v", sessions)
	}
}

func TestRunEndpoints(t *testing.T) {
	t.Parallel()

	handler, services := newTestHandler(t)

	status, body := doJSONRequest(t, handler, http.MethodPost, "/api/runs", map[string]any{
		"instruction": "Read the repository summary and respond briefly",
		"workspace":   services.Config.Workspace,
		"provider":    "mock",
		"model":       "mock-model",
		"max_turns":   5,
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%#v", status, body)
	}

	run := body["run"].(map[string]any)
	runID := run["id"].(string)

	status, inspectBody := doJSONRequest(t, handler, http.MethodGet, "/api/runs/"+runID, nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%#v", status, inspectBody)
	}
	if inspectBody["run"] == nil || inspectBody["plan"] == nil {
		t.Fatalf("expected inspect payload, got %#v", inspectBody)
	}

	status, replayBody := doJSONRequest(t, handler, http.MethodGet, "/api/runs/"+runID+"/replay", nil)
	if status != http.StatusOK {
		t.Fatalf("expected replay 200, got %d body=%#v", status, replayBody)
	}
	entries := replayBody["entries"].([]any)
	if len(entries) == 0 {
		t.Fatalf("expected replay entries, got %#v", replayBody)
	}

	status, eventsBody := doJSONRequest(t, handler, http.MethodGet, "/api/runs/"+runID+"/events", nil)
	if status != http.StatusOK {
		t.Fatalf("expected events 200, got %d body=%#v", status, eventsBody)
	}
	events := eventsBody["events"].([]any)
	if len(events) == 0 {
		t.Fatalf("expected events, got %#v", eventsBody)
	}

	status, toolsBody := doJSONRequest(t, handler, http.MethodGet, "/api/tools", nil)
	if status != http.StatusOK {
		t.Fatalf("expected tools 200, got %d body=%#v", status, toolsBody)
	}
	tools := toolsBody["tools"].([]any)
	if len(tools) == 0 {
		t.Fatalf("expected tools, got %#v", toolsBody)
	}
}

func TestResumeRunEndpoint(t *testing.T) {
	t.Parallel()

	handler, services := newTestHandler(t)
	runID := seedPendingRun(t, services)

	status, body := doJSONRequest(t, handler, http.MethodPost, "/api/runs/"+runID+"/resume", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%#v", status, body)
	}
	if body["run"] == nil {
		t.Fatalf("expected run response, got %#v", body)
	}

	status, body = doJSONRequest(t, handler, http.MethodPost, "/api/runs/run_missing/resume", nil)
	if status != http.StatusNotFound {
		t.Fatalf("expected 404 for missing run, got %d body=%#v", status, body)
	}
}

func TestValidationErrors(t *testing.T) {
	t.Parallel()

	handler, services := newTestHandler(t)

	status, body := doJSONRequest(t, handler, http.MethodPost, "/api/runs", map[string]any{
		"workspace": services.Config.Workspace,
	})
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%#v", status, body)
	}

	status, body = doJSONRequest(t, handler, http.MethodGet, "/api/sessions/session_missing?recent=oops", nil)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%#v", status, body)
	}

	status, body = doJSONRequest(t, handler, http.MethodGet, "/api/sessions?limit=oops", nil)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid sessions limit, got %d body=%#v", status, body)
	}

	status, body = doJSONRequest(t, handler, http.MethodGet, "/api/runs?limit=oops", nil)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid runs limit, got %d body=%#v", status, body)
	}

	status, body = doJSONRequest(t, handler, http.MethodPost, "/api/sessions", map[string]any{
		"workspace": services.Config.Workspace + "/elsewhere",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 for mismatched workspace, got %d body=%#v", status, body)
	}
}

func TestRunStreamEndpoint(t *testing.T) {
	t.Parallel()

	handler, services := newTestHandler(t)
	status, body := doJSONRequest(t, handler, http.MethodPost, "/api/runs", map[string]any{
		"instruction": "Read the repository summary and respond briefly",
		"workspace":   services.Config.Workspace,
		"provider":    "mock",
		"model":       "mock-model",
		"max_turns":   5,
	})
	if status != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%#v", status, body)
	}

	run := body["run"].(map[string]any)
	runID := run["id"].(string)
	events, err := services.ReplayRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) < 2 {
		t.Fatalf("expected replay events, got %d", len(events))
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/runs/"+runID+"/stream?after="+strconv.FormatInt(events[len(events)-2].Sequence, 10), nil)
	handler.ServeHTTP(recorder, req)

	bodyText := recorder.Body.String()
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, bodyText)
	}
	if !strings.Contains(bodyText, "event: runtime.event") {
		t.Fatalf("expected runtime event in stream body, got %s", bodyText)
	}
	if !strings.Contains(bodyText, "event: done") {
		t.Fatalf("expected done event in stream body, got %s", bodyText)
	}
}

func TestAGUIChatEndpoint(t *testing.T) {
	t.Parallel()

	handler, services := newTestHandler(t)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/agui/chat", bytes.NewBufferString(`{
		"messages": [{"id":"msg_user_1","role":"user","content":"Summarize this repository"}],
		"state": {"workspace":"`+services.Config.Workspace+`","provider":"mock","model":"mock-model","maxTurns":4}
	}`))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(recorder, req)

	bodyText := recorder.Body.String()
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, bodyText)
	}
	for _, needle := range []string{
		`"type":"RUN_STARTED"`,
		`"type":"MESSAGES_SNAPSHOT"`,
		`"type":"STATE_SNAPSHOT"`,
		`"type":"TEXT_MESSAGE_START"`,
		`"type":"RUN_FINISHED"`,
	} {
		if !strings.Contains(bodyText, needle) {
			t.Fatalf("expected stream body to contain %s, got %s", needle, bodyText)
		}
	}
	if strings.Index(bodyText, `"type":"RUN_STARTED"`) > strings.Index(bodyText, `"type":"RUN_FINISHED"`) {
		t.Fatalf("expected RUN_STARTED before RUN_FINISHED, got %s", bodyText)
	}
}

func TestAGUIChatEndpointRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/agui/chat", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func newTestHandler(t *testing.T) (http.Handler, service.Services) {
	t.Helper()

	workspace := t.TempDir()
	cfg := config.Load(workspace)
	cfg.Workspace = workspace
	cfg.Runtime.Root = filepath.Join(workspace, ".runtime")
	cfg.Model.Provider = "mock"
	cfg.Model.Model = "mock-model"

	services := service.NewServices(cfg)
	return httpapi.NewRouter(services), services
}

func seedPendingRun(t *testing.T, services service.Services) string {
	t.Helper()

	now := time.Now()
	task := harnessruntime.Task{
		ID:          harnessruntime.NewID("task"),
		Instruction: "Resume this run",
		Workspace:   services.Config.Workspace,
		CreatedAt:   now,
	}
	session := harnessruntime.Session{
		ID:        harnessruntime.NewID("session"),
		Workspace: services.Config.Workspace,
		CreatedAt: now,
		UpdatedAt: now,
	}
	run := harnessruntime.Run{
		ID:        harnessruntime.NewID("run"),
		TaskID:    task.ID,
		SessionID: session.ID,
		Status:    harnessruntime.RunPending,
		Provider:  "mock",
		Model:     "mock-model",
		MaxTurns:  5,
		CreatedAt: now,
		UpdatedAt: now,
	}
	plan := harnessruntime.Plan{
		ID:        harnessruntime.NewID("plan"),
		RunID:     run.ID,
		Goal:      task.Instruction,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
		Steps: []harnessruntime.PlanStep{
			{
				ID:          harnessruntime.NewID("step"),
				Title:       "Test step",
				Description: "Resume should continue this step",
				Status:      harnessruntime.StepPending,
			},
		},
	}
	state := harnessruntime.RunState{
		RunID:     run.ID,
		TurnCount: 0,
		UpdatedAt: now,
	}

	if err := services.StateStore.SaveTask(task); err != nil {
		t.Fatal(err)
	}
	if err := services.StateStore.SaveSession(session); err != nil {
		t.Fatal(err)
	}
	if err := services.StateStore.SaveRun(run); err != nil {
		t.Fatal(err)
	}
	if err := services.StateStore.SavePlan(plan); err != nil {
		t.Fatal(err)
	}
	if err := services.StateStore.SaveState(state); err != nil {
		t.Fatal(err)
	}
	return run.ID
}

func doJSONRequest(t *testing.T, handler http.Handler, method, path string, body any) (int, map[string]any) {
	t.Helper()

	var requestBody []byte
	if body != nil {
		var err error
		requestBody, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		requestBody = []byte("{}")
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return recorder.Code, payload
}
