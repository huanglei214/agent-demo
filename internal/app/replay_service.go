package app

import (
	"fmt"
	"strings"
	"time"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

type ReplayEntry struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	Actor     string    `json:"actor"`
	Timestamp time.Time `json:"timestamp"`
	Summary   string    `json:"summary"`
}

func (s Services) ReplayRun(runID string) ([]harnessruntime.Event, error) {
	return s.EventStore.ReadAll(runID)
}

func (s Services) ReplayRunSummary(runID string) ([]ReplayEntry, error) {
	events, err := s.ReplayRun(runID)
	if err != nil {
		return nil, err
	}
	result := make([]ReplayEntry, 0, len(events))
	for _, event := range events {
		result = append(result, ReplayEntry{
			Sequence:  event.Sequence,
			Type:      event.Type,
			Actor:     event.Actor,
			Timestamp: event.Timestamp,
			Summary:   summarizeReplayEvent(event),
		})
	}
	return result, nil
}

func summarizeReplayEvent(event harnessruntime.Event) string {
	payload := event.Payload
	switch event.Type {
	case "task.created":
		return fmt.Sprintf("task %v created", payload["task_id"])
	case "session.created":
		return fmt.Sprintf("session %v created", payload["session_id"])
	case "run.created":
		return fmt.Sprintf("run created with status %v", payload["status"])
	case "run.role_assigned":
		return fmt.Sprintf("run role assigned as %v", payload["role"])
	case "run.status_changed":
		return fmt.Sprintf("run status %v -> %v", payload["from"], payload["to"])
	case "run.completed":
		return fmt.Sprintf("run %v completed", payload["run_id"])
	case "run.failed":
		return fmt.Sprintf("run failed: %v", payload["error"])
	case "plan.created":
		return fmt.Sprintf("plan %v created (v%v)", payload["plan_id"], payload["version"])
	case "plan.updated":
		return fmt.Sprintf("plan updated to v%v because %v", payload["version"], payload["reason"])
	case "plan.step.started":
		return fmt.Sprintf("step %v started", payload["step_id"])
	case "plan.step.completed":
		return fmt.Sprintf("step %v completed", payload["step_id"])
	case "user.message", "assistant.message":
		content, _ := payload["content"].(string)
		return truncateSummary(content, 80)
	case "prompt.built":
		return fmt.Sprintf("prompt built with %v layers", payload["layers"])
	case "context.built":
		return fmt.Sprintf("context built with %v messages, %v memories, %v summaries", payload["message_count"], payload["memory_count"], payload["summary_count"])
	case "memory.recalled":
		return fmt.Sprintf("recalled %v memories", payload["count"])
	case "memory.routed":
		return fmt.Sprintf("routed %v memory candidates", payload["count"])
	case "memory.candidate_extracted":
		return fmt.Sprintf("extracted %v memory candidates", payload["count"])
	case "memory.committed":
		return fmt.Sprintf("committed %v memories", payload["count"])
	case "model.called":
		if phase := fmt.Sprint(payload["phase"]); phase != "<nil>" {
			if role := fmt.Sprint(payload["role"]); role != "<nil>" && role != "" {
				return fmt.Sprintf("model called (%s, %s)", phase, role)
			}
			return fmt.Sprintf("model called (%s)", phase)
		}
		if role := fmt.Sprint(payload["role"]); role != "<nil>" && role != "" {
			return fmt.Sprintf("model called (%s)", role)
		}
		return "model called"
	case "model.responded":
		if phase := fmt.Sprint(payload["phase"]); phase != "<nil>" {
			return fmt.Sprintf("model responded (%s)", phase)
		}
		return fmt.Sprintf("model responded with finish_reason=%v", payload["finish_reason"])
	case "tool.called":
		return fmt.Sprintf("tool %v called", payload["tool"])
	case "tool.succeeded":
		return fmt.Sprintf("tool %v succeeded", payload["tool"])
	case "tool.failed":
		return fmt.Sprintf("tool %v failed: %v", payload["tool"], payload["error"])
	case "fs.file_created", "fs.file_updated":
		return fmt.Sprintf("%s %v", event.Type, payload["path"])
	case "subagent.spawned":
		return fmt.Sprintf("child run %v spawned for step %v", payload["child_run_id"], payload["step_id"])
	case "subagent.completed":
		return fmt.Sprintf("child run %v completed", payload["child_run_id"])
	case "subagent.rejected":
		return fmt.Sprintf("delegation rejected: %v", payload["reason"])
	case "result.generated":
		return fmt.Sprintf("generated %v bytes of output", payload["bytes"])
	default:
		return event.Type
	}
}

func truncateSummary(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit] + "..."
}
