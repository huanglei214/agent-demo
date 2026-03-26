package agui

import (
	"fmt"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func MapRuntimeEvent(event harnessruntime.Event) []Event {
	switch event.Type {
	case "run.started":
		return []Event{{
			Type:     "RUN_STARTED",
			ThreadID: event.SessionID,
			RunID:    event.RunID,
		}}
	case "run.completed":
		return []Event{{
			Type:     "RUN_FINISHED",
			ThreadID: event.SessionID,
			RunID:    event.RunID,
		}}
	case "run.failed":
		return []Event{{
			Type:     "RUN_ERROR",
			ThreadID: event.SessionID,
			RunID:    event.RunID,
			Error:    stringValue(event.Payload, "error"),
		}}
	case "plan.step.started":
		return []Event{{
			Type:   "STEP_STARTED",
			RunID:  event.RunID,
			StepID: stringValue(event.Payload, "step_id"),
		}}
	case "plan.step.completed":
		return []Event{{
			Type:   "STEP_FINISHED",
			RunID:  event.RunID,
			StepID: stringValue(event.Payload, "step_id"),
		}}
	case "assistant.message":
		messageID := stringValue(event.Payload, "message_id")
		content := stringValue(event.Payload, "content")
		return []Event{
			{
				Type:      "TEXT_MESSAGE_START",
				ThreadID:  event.SessionID,
				RunID:     event.RunID,
				MessageID: messageID,
				Role:      "assistant",
			},
			{
				Type:      "TEXT_MESSAGE_CONTENT",
				ThreadID:  event.SessionID,
				RunID:     event.RunID,
				MessageID: messageID,
				Delta:     content,
			},
			{
				Type:      "TEXT_MESSAGE_END",
				ThreadID:  event.SessionID,
				RunID:     event.RunID,
				MessageID: messageID,
			},
		}
	case "tool.called":
		toolCallID := toolCallIDFromPayload(event)
		return []Event{
			{
				Type:       "TOOL_CALL_START",
				ThreadID:   event.SessionID,
				RunID:      event.RunID,
				ToolCallID: toolCallID,
				ToolName:   stringValue(event.Payload, "tool"),
			},
			{
				Type:       "TOOL_CALL_ARGS",
				ThreadID:   event.SessionID,
				RunID:      event.RunID,
				ToolCallID: toolCallID,
				Args:       event.Payload["input"],
			},
		}
	case "tool.succeeded":
		toolCallID := toolCallIDFromPayload(event)
		return []Event{
			{
				Type:       "TOOL_CALL_END",
				ThreadID:   event.SessionID,
				RunID:      event.RunID,
				ToolCallID: toolCallID,
				ToolName:   stringValue(event.Payload, "tool"),
			},
			{
				Type:       "TOOL_CALL_RESULT",
				ThreadID:   event.SessionID,
				RunID:      event.RunID,
				ToolCallID: toolCallID,
				Result:     event.Payload["result"],
			},
		}
	case "tool.failed":
		toolCallID := toolCallIDFromPayload(event)
		return []Event{
			{
				Type:       "TOOL_CALL_END",
				ThreadID:   event.SessionID,
				RunID:      event.RunID,
				ToolCallID: toolCallID,
				ToolName:   stringValue(event.Payload, "tool"),
			},
			{
				Type:     "CUSTOM",
				ThreadID: event.SessionID,
				RunID:    event.RunID,
				Name:     "tool.failed",
				Value:    event.Payload,
			},
		}
	case "plan.updated", "subagent.spawned", "subagent.completed", "subagent.rejected", "memory.recalled", "memory.routed", "memory.candidate_extracted", "memory.committed":
		return []Event{{
			Type:     "CUSTOM",
			ThreadID: event.SessionID,
			RunID:    event.RunID,
			Name:     event.Type,
			Value:    event.Payload,
		}}
	default:
		return []Event{{
			Type:     "RAW",
			ThreadID: event.SessionID,
			RunID:    event.RunID,
			Name:     event.Type,
			Value: map[string]any{
				"id":        event.ID,
				"sequence":  event.Sequence,
				"actor":     event.Actor,
				"timestamp": event.Timestamp,
				"payload":   event.Payload,
			},
		}}
	}
}

func stringValue(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	raw, ok := payload[key]
	if !ok || raw == nil {
		return ""
	}
	return fmt.Sprint(raw)
}

func toolCallIDFromPayload(event harnessruntime.Event) string {
	if value := stringValue(event.Payload, "tool_call_id"); value != "" {
		return value
	}
	return event.ID
}
