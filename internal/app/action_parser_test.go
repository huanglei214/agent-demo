package app

import "testing"

func TestParseActionUnwrapsNestedFinalAnswer(t *testing.T) {
	action := parseAction(`{"action":"final","answer":"{\"action\":\"final\",\"answer\":\"hello\"}"}`)
	if action.Action != "final" {
		t.Fatalf("expected final action, got %#v", action)
	}
	if action.Answer != "hello" {
		t.Fatalf("expected nested final answer to unwrap, got %#v", action)
	}
}

func TestParseActionKeepsStructuredJSONAnswer(t *testing.T) {
	action := parseAction(`{"action":"final","answer":"{\"summary\":\"ok\",\"needs_replan\":false}"}`)
	if action.Action != "final" {
		t.Fatalf("expected final action, got %#v", action)
	}
	if action.Answer != "{\"summary\":\"ok\",\"needs_replan\":false}" {
		t.Fatalf("expected structured JSON answer to remain intact, got %#v", action)
	}
}

func TestParseActionUsesSubtaskGoalForDelegation(t *testing.T) {
	action := parseAction(`{"action":"delegate","subtask":{"goal":"查询武汉实时天气","skill":"weather-lookup"}}`)
	if action.Action != "delegate" {
		t.Fatalf("expected delegate action, got %#v", action)
	}
	if action.DelegationGoal != "查询武汉实时天气" {
		t.Fatalf("expected delegation goal to be derived from subtask goal, got %#v", action)
	}
	if action.Subtask == nil || action.Subtask.Skill != "weather-lookup" {
		t.Fatalf("expected subtask metadata to remain available, got %#v", action)
	}
}

func TestParseActionSanitizesLiteralNewlinesInsideFinalAnswer(t *testing.T) {
	action := parseAction("{\"action\":\"final\",\"answer\":\"第一行\n第二行\"}")
	if action.Action != "final" {
		t.Fatalf("expected final action, got %#v", action)
	}
	if action.Answer != "第一行\n第二行" {
		t.Fatalf("expected literal newlines to survive answer parsing, got %#v", action)
	}
}
