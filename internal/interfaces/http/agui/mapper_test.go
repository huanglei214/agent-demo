package agui

import (
	"testing"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func TestMapRuntimeEventAssistantMessage(t *testing.T) {
	events := MapRuntimeEvent(harnessruntime.Event{
		RunID:     "run_123",
		SessionID: "session_123",
		Type:      "assistant.message",
		Payload: map[string]any{
			"message_id": "msg_1",
			"content":    "hello",
		},
	})

	if len(events) != 3 {
		t.Fatalf("expected 3 mapped events, got %d", len(events))
	}
	if events[0].Type != "TEXT_MESSAGE_START" {
		t.Fatalf("expected TEXT_MESSAGE_START, got %#v", events[0])
	}
	if events[1].Type != "TEXT_MESSAGE_CONTENT" || events[1].Delta != "hello" {
		t.Fatalf("expected content event with hello, got %#v", events[1])
	}
	if events[2].Type != "TEXT_MESSAGE_END" {
		t.Fatalf("expected TEXT_MESSAGE_END, got %#v", events[2])
	}
}

func TestMapRuntimeEventToolSucceededUsesStableToolCallID(t *testing.T) {
	events := MapRuntimeEvent(harnessruntime.Event{
		ID:        "evt_123",
		RunID:     "run_123",
		SessionID: "session_123",
		Type:      "tool.succeeded",
		Payload: map[string]any{
			"tool_call_id": "toolcall_123",
			"tool":         "fs.read_file",
			"result": map[string]any{
				"path": "README.md",
			},
		},
	})

	if len(events) != 2 {
		t.Fatalf("expected 2 mapped events, got %d", len(events))
	}
	if events[0].ToolCallID != "toolcall_123" || events[1].ToolCallID != "toolcall_123" {
		t.Fatalf("expected stable tool call id, got %#v", events)
	}
	if events[1].Type != "TOOL_CALL_RESULT" {
		t.Fatalf("expected TOOL_CALL_RESULT, got %#v", events[1])
	}
}
