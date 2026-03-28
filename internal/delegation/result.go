package delegation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/huanglei214/agent-demo/internal/model"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func BuildChildInstruction(task harnessruntime.DelegationTask) string {
	return strings.TrimSpace(task.Goal)
}

func BuildResult(run harnessruntime.Run, runResult *harnessruntime.RunResult) (harnessruntime.DelegationResult, error) {
	summary := ""
	if runResult != nil {
		summary = strings.TrimSpace(runResult.Output)
	}
	if action := unwrapFinalAction(summary); action.Action == "final" && strings.TrimSpace(action.Answer) != "" {
		summary = strings.TrimSpace(action.Answer)
	}

	result := harnessruntime.DelegationResult{
		ChildRunID:      run.ID,
		Summary:         summary,
		Artifacts:       []harnessruntime.DelegationArtifact{},
		Findings:        []string{},
		Risks:           []string{},
		Recommendations: []string{},
		NeedsReplan:     false,
	}
	if summary == "" {
		return result, errors.New("child run did not return a structured result")
	}

	var decoded struct {
		Summary         string          `json:"summary"`
		Artifacts       json.RawMessage `json:"artifacts"`
		Findings        []string        `json:"findings"`
		Risks           []string        `json:"risks"`
		Recommendations []string        `json:"recommendations"`
		NeedsReplan     bool            `json:"needs_replan"`
	}
	if err := json.Unmarshal([]byte(summary), &decoded); err != nil {
		return result, fmt.Errorf("child run did not return valid structured result json: %w", err)
	}
	if strings.TrimSpace(decoded.Summary) == "" {
		return result, errors.New("child run structured result is missing summary")
	}

	artifacts, err := decodeArtifacts(decoded.Artifacts)
	if err != nil {
		return result, fmt.Errorf("child run returned invalid artifacts: %w", err)
	}
	result.Summary = strings.TrimSpace(decoded.Summary)
	result.Artifacts = artifacts
	result.Findings = ensureStringSlice(decoded.Findings)
	result.Risks = ensureStringSlice(decoded.Risks)
	result.Recommendations = ensureStringSlice(decoded.Recommendations)
	result.NeedsReplan = decoded.NeedsReplan
	return result, nil
}

func ResultContent(result harnessruntime.DelegationResult) map[string]any {
	return map[string]any{
		"summary":         result.Summary,
		"artifacts":       result.Artifacts,
		"findings":        result.Findings,
		"risks":           result.Risks,
		"recommendations": result.Recommendations,
		"needs_replan":    result.NeedsReplan,
		"child_run_id":    result.ChildRunID,
	}
}

func unwrapFinalAction(text string) model.Action {
	var action model.Action
	if err := json.Unmarshal([]byte(text), &action); err != nil {
		return model.Action{}
	}
	return action
}

func ensureStringSlice(value []string) []string {
	if value == nil {
		return []string{}
	}
	return value
}

func decodeArtifacts(raw json.RawMessage) ([]harnessruntime.DelegationArtifact, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []harnessruntime.DelegationArtifact{}, nil
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}

	artifacts := make([]harnessruntime.DelegationArtifact, 0, len(items))
	for _, item := range items {
		var text string
		if err := json.Unmarshal(item, &text); err == nil {
			artifacts = append(artifacts, harnessruntime.DelegationArtifact{Value: strings.TrimSpace(text)})
			continue
		}

		var object map[string]any
		if err := json.Unmarshal(item, &object); err != nil {
			return nil, err
		}
		artifact := harnessruntime.DelegationArtifact{
			Name: asString(object, "name"),
			Path: asString(object, "path"),
			URL:  asString(object, "url"),
		}
		delete(object, "name")
		delete(object, "path")
		delete(object, "url")
		if len(object) > 0 {
			artifact.Extra = object
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

func asString(values map[string]any, key string) string {
	raw, ok := values[key]
	if !ok {
		return ""
	}
	text, _ := raw.(string)
	return strings.TrimSpace(text)
}
