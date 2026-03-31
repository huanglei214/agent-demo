package retrieval

import (
	"strings"
	"testing"

	"github.com/huanglei214/agent-demo/internal/model"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

func TestBuildRetrievalProgressFromEvents(t *testing.T) {
	t.Parallel()

	events := []harnessruntime.Event{
		{Type: "tool.succeeded", Payload: map[string]any{"tool": "web.search", "result": map[string]any{"query": "go testing"}}},
		{Type: "tool.succeeded", Payload: map[string]any{"tool": "web.fetch", "result": map[string]any{"url": "https://example.com", "title": "Example", "content": "Hello"}}},
		{Type: "tool.called", Payload: map[string]any{"tool": "web.search"}}, // should be ignored
	}

	progress := BuildRetrievalProgress(events)
	if len(progress.SearchQueries) != 1 || progress.SearchQueries[0] != "go testing" {
		t.Fatalf("expected search query 'go testing', got %#v", progress.SearchQueries)
	}
	if progress.SuccessfulFetches != 1 {
		t.Fatalf("expected 1 successful fetch, got %d", progress.SuccessfulFetches)
	}
	if len(progress.Evidence) != 1 || progress.Evidence[0].URL != "https://example.com" {
		t.Fatalf("expected evidence from example.com, got %#v", progress.Evidence)
	}
}

func TestUpdateProgressTracksSearchQueries(t *testing.T) {
	t.Parallel()

	var progress RetrievalProgress
	UpdateProgress(&progress, "web.search", map[string]any{"query": "first"})
	UpdateProgress(&progress, "web.search", map[string]any{"search_query": "second"})

	if len(progress.SearchQueries) != 2 {
		t.Fatalf("expected 2 search queries, got %d", len(progress.SearchQueries))
	}
}

func TestUpdateProgressTracksFetchesAndDeduplicates(t *testing.T) {
	t.Parallel()

	var progress RetrievalProgress
	UpdateProgress(&progress, "web.fetch", map[string]any{"url": "https://a.com", "title": "A", "content": "text"})
	UpdateProgress(&progress, "web.fetch", map[string]any{"url": "https://a.com", "title": "A", "content": "text"})

	if progress.DuplicateFetches != 1 {
		t.Fatalf("expected 1 duplicate fetch, got %d", progress.DuplicateFetches)
	}
	// Duplicate URLs still count as successful if content is useful.
	if progress.SuccessfulFetches != 2 {
		t.Fatalf("expected 2 successful fetches (duplicate still counted), got %d", progress.SuccessfulFetches)
	}
	if len(progress.FetchedURLs) != 1 {
		t.Fatalf("expected 1 unique URL in FetchedURLs, got %d", len(progress.FetchedURLs))
	}
}

func TestUpdateProgressCountsEmptyFetches(t *testing.T) {
	t.Parallel()

	var progress RetrievalProgress
	UpdateProgress(&progress, "web.fetch", map[string]any{"url": "https://empty.com", "title": "", "content": ""})

	if progress.EmptyFetches != 1 {
		t.Fatalf("expected 1 empty fetch, got %d", progress.EmptyFetches)
	}
	if progress.SuccessfulFetches != 0 {
		t.Fatalf("expected 0 successful fetches, got %d", progress.SuccessfulFetches)
	}
}

func TestDecideProgressIgnoresNonToolActions(t *testing.T) {
	t.Parallel()

	progress := RetrievalProgress{SuccessfulFetches: 5}
	decision := DecideProgress(progress, model.Action{Action: "final", Answer: "done"})
	if decision.ShouldForceFinal {
		t.Fatal("expected no force-final for non-tool action")
	}

	decision = DecideProgress(progress, model.Action{Action: "delegate", DelegationGoal: "research"})
	if decision.ShouldForceFinal {
		t.Fatal("expected no force-final for delegate action")
	}
}

func TestDecideProgressForceFinalOnDuplicateURL(t *testing.T) {
	t.Parallel()

	progress := RetrievalProgress{
		FetchedURLs:       []string{"https://a.com"},
		SuccessfulFetches: 1,
	}
	action := model.Action{
		Action: "tool",
		Calls:  []model.ToolCall{{Tool: "web.fetch", Input: map[string]any{"url": "https://a.com"}}},
	}

	decision := DecideProgress(progress, action)
	if !decision.ShouldForceFinal {
		t.Fatal("expected force-final when fetching duplicate URL with existing evidence")
	}
}

func TestDecideProgressForceFinalOnSufficientFetches(t *testing.T) {
	t.Parallel()

	progress := RetrievalProgress{SuccessfulFetches: 2}
	action := model.Action{
		Action: "tool",
		Calls:  []model.ToolCall{{Tool: "web.search", Input: map[string]any{"query": "more"}}},
	}

	decision := DecideProgress(progress, action)
	if !decision.ShouldForceFinal {
		t.Fatal("expected force-final when >=2 successful fetches")
	}
}

func TestDecideProgressForceFinalOnLooping(t *testing.T) {
	t.Parallel()

	progress := RetrievalProgress{
		DuplicateFetches:  1,
		SuccessfulFetches: 1,
	}
	action := model.Action{
		Action: "tool",
		Calls:  []model.ToolCall{{Tool: "web.fetch", Input: map[string]any{"url": "https://new.com"}}},
	}

	decision := DecideProgress(progress, action)
	if !decision.ShouldForceFinal {
		t.Fatal("expected force-final when looping with duplicates + evidence")
	}
}

func TestDecideProgressForceFinalOnEmptyFetches(t *testing.T) {
	t.Parallel()

	progress := RetrievalProgress{
		EmptyFetches:      2,
		SuccessfulFetches: 1,
	}
	action := model.Action{
		Action: "tool",
		Calls:  []model.ToolCall{{Tool: "web.fetch", Input: map[string]any{"url": "https://c.com"}}},
	}

	decision := DecideProgress(progress, action)
	if !decision.ShouldForceFinal {
		t.Fatal("expected force-final when >=2 empty fetches + evidence")
	}
}

func TestDecideProgressContinuesWithoutEvidence(t *testing.T) {
	t.Parallel()

	progress := RetrievalProgress{
		EmptyFetches:      3,
		SuccessfulFetches: 0,
	}
	action := model.Action{
		Action: "tool",
		Calls:  []model.ToolCall{{Tool: "web.fetch", Input: map[string]any{"url": "https://d.com"}}},
	}

	decision := DecideProgress(progress, action)
	if decision.ShouldForceFinal {
		t.Fatal("expected continue when no successful evidence exists")
	}
}

func TestExcerptForEvidenceTruncatesLongContent(t *testing.T) {
	t.Parallel()

	longContent := strings.Repeat("字", 500)
	result := excerptForEvidence(longContent)
	runes := []rune(result)
	// 400 runes + "..."
	if len(runes) != 403 {
		t.Fatalf("expected 403 runes (400 + ...), got %d", len(runes))
	}
	if !strings.HasSuffix(result, "...") {
		t.Fatal("expected truncated content to end with ...")
	}

	shortContent := "short"
	if got := excerptForEvidence(shortContent); got != "short" {
		t.Fatalf("expected short content unchanged, got %q", got)
	}
}

func TestBuildEvidencePayload(t *testing.T) {
	t.Parallel()

	progress := RetrievalProgress{
		SearchQueries:     []string{"test query"},
		FetchedURLs:       []string{"https://a.com"},
		SuccessfulFetches: 1,
		EmptyFetches:      0,
		DuplicateFetches:  0,
		DistinctEvidence:  1,
		Evidence:          []RetrievalEvidence{{URL: "https://a.com", Title: "A"}},
	}

	payload := BuildEvidencePayload(progress)
	if payload["successful_fetches"] != 1 {
		t.Fatalf("expected successful_fetches=1, got %v", payload["successful_fetches"])
	}
	evidence, ok := payload["retrieved_evidence"].([]RetrievalEvidence)
	if !ok || len(evidence) != 1 {
		t.Fatalf("expected 1 evidence entry, got %v", payload["retrieved_evidence"])
	}
}
