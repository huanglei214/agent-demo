package agent

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/huanglei214/agent-demo/internal/model"
	arkmodel "github.com/huanglei214/agent-demo/internal/model/ark"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/skill"
	toolruntime "github.com/huanglei214/agent-demo/internal/tool"
)

func (e *Executor) appendModelCall(run harnessruntime.Run, sequence int64, phase, toolName string, req model.Request, resp *model.Response, callErr error) error {
	call := harnessruntime.ModelCall{
		ID:       harnessruntime.NewID("modelcall"),
		RunID:    run.ID,
		Sequence: sequence,
		Phase:    phase,
		Tool:     toolName,
		Request: harnessruntime.ModelRequestSnapshot{
			SystemPrompt: req.SystemPrompt,
			Input:        req.Input,
			Provider:     run.Provider,
			Model:        run.Model,
			Messages: []harnessruntime.ModelMessage{
				{Role: "system", Content: req.SystemPrompt},
				{Role: "user", Content: req.Input},
			},
			Metadata: req.Metadata,
		},
		Timestamp: time.Now(),
	}
	if resp != nil {
		call.Response = &harnessruntime.ModelResponseSnapshot{
			Text:         resp.Text,
			FinishReason: resp.FinishReason,
			Metadata:     resp.Metadata,
		}
	}
	if callErr != nil {
		call.Error = callErr.Error()
	}
	return e.StateStore.AppendModelCall(call)
}

func (e *Executor) promptToolMetadataForSkill(activeSkill *skill.Definition) []map[string]string {
	descriptors := e.toolDescriptorsForNames(allowedToolSet(activeSkill))
	result := make([]map[string]string, 0, len(descriptors))
	for _, item := range descriptors {
		result = append(result, map[string]string{
			"name":        item.Name,
			"description": item.Description,
			"access":      string(item.Access),
		})
	}
	return result
}

func (e *Executor) activeSkillForTask(task harnessruntime.Task) (*skill.Definition, error) {
	if task.Metadata == nil {
		return nil, nil
	}
	name := strings.TrimSpace(task.Metadata["skill"])
	if name == "" {
		return nil, nil
	}
	definition, ok, err := e.SkillRegistry.Resolve(name)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("skill referenced by task is missing: %s", name)
	}
	if err := definition.Metadata.ValidateAllowedTools(e.availableToolSet()); err != nil {
		return nil, err
	}
	return &definition, nil
}

func allowedToolSet(activeSkill *skill.Definition) map[string]struct{} {
	if activeSkill == nil || len(activeSkill.AllowedTools) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(activeSkill.AllowedTools))
	for _, name := range activeSkill.AllowedTools {
		result[name] = struct{}{}
	}
	return result
}

func ensureSkillAllowsTool(activeSkill *skill.Definition, toolName string) error {
	if activeSkill == nil || len(activeSkill.AllowedTools) == 0 {
		return nil
	}
	for _, allowed := range activeSkill.AllowedTools {
		if allowed == toolName {
			return nil
		}
	}
	return fmt.Errorf("tool %s is not allowed by active skill %s", toolName, activeSkill.Name)
}

func (e *Executor) appendEvent(event harnessruntime.Event, observer RunObserver) error {
	if err := e.EventStore.Append(event); err != nil {
		return err
	}
	ensureRunObserver(observer).OnRuntimeEvent(event)
	return nil
}

func (e *Executor) appendEvents(events []harnessruntime.Event, observer RunObserver) error {
	for _, event := range events {
		if err := e.appendEvent(event, observer); err != nil {
			return err
		}
	}
	return nil
}

func (e *Executor) newEvent(run harnessruntime.Run, taskID, sessionID string, sequence int64, eventType, actor string, payload map[string]any) harnessruntime.Event {
	return harnessruntime.Event{
		ID:        harnessruntime.NewID("evt"),
		RunID:     run.ID,
		SessionID: sessionID,
		TaskID:    taskID,
		Sequence:  sequence,
		Type:      eventType,
		Timestamp: time.Now(),
		Actor:     actor,
		Payload:   payload,
	}
}

func (e *Executor) failRun(run harnessruntime.Run, plan harnessruntime.Plan, taskID, sessionID string, state harnessruntime.RunState, cause error, sequence int64, observer RunObserver) (ExecutionResponse, error) {
	now := time.Now()
	previousStatus := run.Status
	run.Status = harnessruntime.RunFailed
	run.UpdatedAt = now
	run.CompletedAt = now
	state.UpdatedAt = now
	planUpdated := false
	for i := range plan.Steps {
		if plan.Steps[i].ID == state.CurrentStepID || (state.CurrentStepID == "" && plan.Steps[i].ID == run.CurrentStepID) {
			plan.Steps[i].Status = harnessruntime.StepFailed
			planUpdated = true
			break
		}
	}
	if planUpdated {
		plan.UpdatedAt = now
	}

	if err := e.StateStore.SaveRun(run); err != nil {
		return ExecutionResponse{}, err
	}
	if err := e.StateStore.SaveState(state); err != nil {
		return ExecutionResponse{}, err
	}
	if planUpdated {
		if err := e.StateStore.SavePlan(plan); err != nil {
			return ExecutionResponse{}, err
		}
	}

	failureKind, retryable := classifyRunFailure(cause)
	events := []harnessruntime.Event{
		e.newEvent(run, taskID, sessionID, sequence, "run.status_changed", "runtime", map[string]any{
			"from": previousStatus,
			"to":   harnessruntime.RunFailed,
		}),
		e.newEvent(run, taskID, sessionID, sequence+1, "run.failed", "runtime", map[string]any{
			"error":        cause.Error(),
			"failure_kind": failureKind,
			"retryable":    retryable,
			"step_id":      state.CurrentStepID,
		}),
	}
	if err := e.appendEvents(events, observer); err != nil {
		return ExecutionResponse{}, err
	}

	return ExecutionResponse{}, cause
}

func classifyRunFailure(cause error) (string, bool) {
	var arkErr *arkmodel.Error
	if errors.As(cause, &arkErr) {
		return arkErr.FailureKind(), arkErr.Retryable()
	}
	switch {
	case errors.Is(cause, context.DeadlineExceeded):
		return "timeout", true
	case errors.Is(cause, context.Canceled):
		return "canceled", true
	case errors.Is(cause, harnessruntime.ErrUnsupportedProvider):
		return "unsupported_provider", false
	default:
		return "runtime_error", false
	}
}

func toolBytes(content map[string]any) int {
	switch value := content["bytes"].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return len(harnessruntime.MustJSON(content))
	}
}

func firstNonEmptyString(values ...any) string {
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text != "" {
			return text
		}
	}
	return ""
}

func (e *Executor) generateWithModelTimeout(parent context.Context, provider model.Model, req model.Request) (model.Response, error) {
	timeoutSeconds := e.Config.Model.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 90
	}
	callCtx, cancel := context.WithTimeout(parent, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	return provider.Generate(callCtx, req)
}

func (e *Executor) GenerateWithModelTimeout(parent context.Context, provider model.Model, req model.Request) (model.Response, error) {
	return e.generateWithModelTimeout(parent, provider, req)
}

func responsePtr(resp model.Response, err error) *model.Response {
	if err != nil {
		return nil
	}
	return &resp
}

func (e *Executor) toolDescriptorsForNames(allowed map[string]struct{}) []toolDescriptor {
	descriptors := e.ToolRegistry.Descriptors()
	sort.Slice(descriptors, func(i, j int) bool {
		return descriptors[i].Name < descriptors[j].Name
	})

	result := make([]toolDescriptor, 0, len(descriptors))
	for _, item := range descriptors {
		if len(allowed) > 0 {
			if _, ok := allowed[item.Name]; !ok {
				continue
			}
		}
		result = append(result, toolDescriptor{
			Name:        item.Name,
			Description: item.Description,
			Access:      item.Access,
		})
	}
	return result
}

func (e *Executor) availableToolSet() map[string]struct{} {
	descriptors := e.ToolRegistry.Descriptors()
	result := make(map[string]struct{}, len(descriptors))
	for _, item := range descriptors {
		result[item.Name] = struct{}{}
	}
	return result
}

type toolDescriptor struct {
	Name        string
	Description string
	Access      toolruntime.AccessMode
}
