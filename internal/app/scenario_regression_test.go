package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/huanglei214/agent-demo/internal/config"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

type scenarioDefinition struct {
	Name                     string         `json:"name"`
	Kind                     string         `json:"kind"`
	Description              string         `json:"description"`
	Provider                 string         `json:"provider"`
	Instruction              string         `json:"instruction,omitempty"`
	Turns                    []string       `json:"turns,omitempty"`
	SetupFiles               []scenarioFile `json:"setup_files,omitempty"`
	ExpectedEvents           []string       `json:"expected_events,omitempty"`
	ExpectedArtifacts        []string       `json:"expected_artifacts,omitempty"`
	ExpectedSessionArtifacts []string       `json:"expected_session_artifacts,omitempty"`
	ExpectedMessageCount     int            `json:"expected_message_count,omitempty"`
	ExpectedRunCount         int            `json:"expected_run_count,omitempty"`
	ExpectedChildren         int            `json:"expected_children,omitempty"`
	ExpectedResultContains   string         `json:"expected_result_contains,omitempty"`
}

type scenarioFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func TestScenarioRegression(t *testing.T) {
	t.Setenv("HARNESS_PROVIDER", "mock")

	scenarioFiles, err := filepath.Glob(filepath.Join(scenarioDataDir(t), "*.json"))
	if err != nil {
		t.Fatalf("glob scenario definitions: %v", err)
	}
	sort.Strings(scenarioFiles)
	if len(scenarioFiles) == 0 {
		t.Fatal("expected scenario definitions under testdata/scenarios")
	}

	for _, scenarioPath := range scenarioFiles {
		scenario := loadScenarioDefinition(t, scenarioPath)
		t.Run(scenario.Name, func(t *testing.T) {
			workspace := t.TempDir()
			writeScenarioSetupFiles(t, workspace, scenario.SetupFiles)

			services := NewServices(config.Load(workspace))
			var (
				sessionID string
				runs      []RunResponse
			)

			switch scenario.Kind {
			case "single_run":
				run := executeScenarioRun(t, services, workspace, sessionID, scenario.Instruction, scenario.Provider)
				runs = append(runs, run)
				sessionID = run.Run.SessionID
			case "multi_turn_session":
				session, err := services.CreateSession(workspace)
				if err != nil {
					t.Fatalf("create scenario session for %s: %v", scenario.Name, err)
				}
				sessionID = session.ID
				for _, turn := range scenario.Turns {
					run := executeScenarioRun(t, services, workspace, sessionID, turn, scenario.Provider)
					runs = append(runs, run)
				}
			default:
				t.Fatalf("unsupported scenario kind %q in %s", scenario.Kind, scenarioPath)
			}

			if scenario.ExpectedRunCount > 0 && len(runs) != scenario.ExpectedRunCount {
				t.Fatalf("scenario %s expected %d runs, got %d", scenario.Name, scenario.ExpectedRunCount, len(runs))
			}

			assertScenarioArtifacts(t, workspace, runs, scenario)
			assertScenarioEvents(t, services, runs, scenario)
			assertScenarioSessionState(t, services, sessionID, scenario)
			assertScenarioDelegationState(t, services, runs, scenario)
			assertScenarioResult(t, runs, scenario)
		})
	}
}

func scenarioDataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller for scenario data dir")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "scenarios")
}

func loadScenarioDefinition(t *testing.T, path string) scenarioDefinition {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read scenario definition %s: %v", path, err)
	}
	var scenario scenarioDefinition
	if err := json.Unmarshal(data, &scenario); err != nil {
		t.Fatalf("decode scenario definition %s: %v", path, err)
	}
	if scenario.Name == "" {
		t.Fatalf("scenario definition %s missing name", path)
	}
	if scenario.Provider == "" {
		scenario.Provider = "mock"
	}
	return scenario
}

func writeScenarioSetupFiles(t *testing.T, workspace string, files []scenarioFile) {
	t.Helper()
	for _, file := range files {
		path := filepath.Join(workspace, file.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir scenario setup dir for %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(file.Content), 0o644); err != nil {
			t.Fatalf("write scenario setup file %s: %v", path, err)
		}
	}
}

func executeScenarioRun(t *testing.T, services Services, workspace, sessionID, instruction, provider string) RunResponse {
	t.Helper()
	response, err := services.StartRun(RunRequest{
		Instruction: instruction,
		Workspace:   workspace,
		Provider:    provider,
		Model:       provider + "-model",
		MaxTurns:    5,
		SessionID:   sessionID,
	})
	if err != nil {
		t.Fatalf("start scenario run for instruction %q: %v", instruction, err)
	}
	if response.Run.Status != harnessruntime.RunCompleted {
		t.Fatalf("expected completed scenario run for %q, got %#v", instruction, response.Run)
	}
	return response
}

func assertScenarioArtifacts(t *testing.T, workspace string, runs []RunResponse, scenario scenarioDefinition) {
	t.Helper()
	for _, run := range runs {
		runDir := filepath.Join(workspace, ".runtime", "runs", run.Run.ID)
		for _, artifact := range scenario.ExpectedArtifacts {
			path := filepath.Join(runDir, artifact)
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("scenario %s missing artifact %s for run %s: %v", scenario.Name, artifact, run.Run.ID, err)
			}
		}
	}
}

func assertScenarioEvents(t *testing.T, services Services, runs []RunResponse, scenario scenarioDefinition) {
	t.Helper()
	seen := map[string]bool{}
	for _, run := range runs {
		events, err := services.ReplayRun(run.Run.ID)
		if err != nil {
			t.Fatalf("replay scenario run %s: %v", run.Run.ID, err)
		}
		for _, event := range events {
			seen[event.Type] = true
		}
	}
	for _, eventType := range scenario.ExpectedEvents {
		if !seen[eventType] {
			t.Fatalf("scenario %s missing expected event %q", scenario.Name, eventType)
		}
	}
}

func assertScenarioSessionState(t *testing.T, services Services, sessionID string, scenario scenarioDefinition) {
	t.Helper()
	if sessionID == "" {
		return
	}
	sessionDir := filepath.Join(services.Paths.Root(), "sessions", sessionID)
	for _, artifact := range scenario.ExpectedSessionArtifacts {
		path := filepath.Join(sessionDir, artifact)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("scenario %s missing session artifact %s: %v", scenario.Name, artifact, err)
		}
	}
	if scenario.ExpectedMessageCount > 0 {
		messages, err := services.StateStore.LoadSessionMessages(sessionID)
		if err != nil {
			t.Fatalf("load scenario session messages for %s: %v", scenario.Name, err)
		}
		if len(messages) != scenario.ExpectedMessageCount {
			t.Fatalf("scenario %s expected %d session messages, got %d", scenario.Name, scenario.ExpectedMessageCount, len(messages))
		}
	}
}

func assertScenarioDelegationState(t *testing.T, services Services, runs []RunResponse, scenario scenarioDefinition) {
	t.Helper()
	if scenario.ExpectedChildren == 0 {
		return
	}
	if len(runs) == 0 {
		t.Fatalf("scenario %s expected child runs but no parent runs were recorded", scenario.Name)
	}
	children, err := services.DelegationManager.ListChildren(runs[0].Run.ID)
	if err != nil {
		t.Fatalf("list child runs for scenario %s: %v", scenario.Name, err)
	}
	if len(children) != scenario.ExpectedChildren {
		t.Fatalf("scenario %s expected %d child runs, got %d", scenario.Name, scenario.ExpectedChildren, len(children))
	}
}

func assertScenarioResult(t *testing.T, runs []RunResponse, scenario scenarioDefinition) {
	t.Helper()
	if scenario.ExpectedResultContains == "" || len(runs) == 0 {
		return
	}
	last := runs[len(runs)-1]
	if last.Result == nil || !strings.Contains(last.Result.Output, scenario.ExpectedResultContains) {
		t.Fatalf("scenario %s expected result containing %q, got %#v", scenario.Name, scenario.ExpectedResultContains, last.Result)
	}
}
