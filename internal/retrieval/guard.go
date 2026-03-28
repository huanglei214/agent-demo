package retrieval

import (
	"strings"

	"github.com/huanglei214/agent-demo/internal/model"
	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
)

type RetrievalEvidence struct {
	URL            string `json:"url,omitempty"`
	Title          string `json:"title,omitempty"`
	ContentExcerpt string `json:"content_excerpt,omitempty"`
}

type RetrievalProgress struct {
	SearchQueries     []string            `json:"search_queries,omitempty"`
	FetchedURLs       []string            `json:"fetched_urls,omitempty"`
	Evidence          []RetrievalEvidence `json:"evidence,omitempty"`
	SuccessfulFetches int                 `json:"successful_fetches,omitempty"`
	EmptyFetches      int                 `json:"empty_fetches,omitempty"`
	DuplicateFetches  int                 `json:"duplicate_fetches,omitempty"`
	DistinctEvidence  int                 `json:"distinct_evidence,omitempty"`
}

type RetrievalDecision struct {
	ShouldForceFinal bool
	Reason           string
}

func BuildRetrievalProgress(events []harnessruntime.Event) RetrievalProgress {
	progress := RetrievalProgress{}
	for _, event := range events {
		if event.Type != "tool.succeeded" {
			continue
		}
		toolName, _ := event.Payload["tool"].(string)
		result, _ := event.Payload["result"].(map[string]any)
		if toolName == "" || len(result) == 0 {
			continue
		}
		UpdateProgress(&progress, toolName, result)
	}
	return progress
}

func UpdateProgress(progress *RetrievalProgress, toolName string, result map[string]any) {
	switch toolName {
	case "web.search":
		if query := firstNonEmptyString(result["query"], result["search_query"]); query != "" {
			progress.SearchQueries = append(progress.SearchQueries, query)
		}
	case "web.fetch":
		url := firstNonEmptyString(result["final_url"], result["url"])
		if url != "" {
			if containsString(progress.FetchedURLs, url) {
				progress.DuplicateFetches++
			} else {
				progress.FetchedURLs = append(progress.FetchedURLs, url)
			}
		}

		if !isUsefulFetch(result) {
			progress.EmptyFetches++
			return
		}

		progress.SuccessfulFetches++
		progress.Evidence = append(progress.Evidence, RetrievalEvidence{
			URL:            url,
			Title:          firstNonEmptyString(result["title"]),
			ContentExcerpt: excerptForEvidence(firstNonEmptyString(result["content"], result["content_excerpt"])),
		})
		progress.DistinctEvidence = len(progress.Evidence)
	}
}

func DecideProgress(progress RetrievalProgress, nextAction model.Action) RetrievalDecision {
	if nextAction.Action != "tool" {
		return RetrievalDecision{}
	}
	if len(nextAction.Calls) == 0 {
		return RetrievalDecision{}
	}

	webCallCount := 0
	for _, call := range nextAction.Calls {
		if call.Tool != "web.search" && call.Tool != "web.fetch" {
			continue
		}
		webCallCount++
		if call.Tool == "web.fetch" {
			nextURL := firstNonEmptyString(call.Input["url"], call.Input["final_url"])
			if nextURL != "" && containsString(progress.FetchedURLs, nextURL) && progress.SuccessfulFetches > 0 {
				return RetrievalDecision{
					ShouldForceFinal: true,
					Reason:           "the next web.fetch repeats a URL that already produced evidence",
				}
			}
		}
	}
	if webCallCount == 0 {
		return RetrievalDecision{}
	}

	if progress.SuccessfulFetches >= 2 {
		return RetrievalDecision{
			ShouldForceFinal: true,
			Reason:           "retrieved evidence is already sufficient to answer without more web calls",
		}
	}

	if progress.DuplicateFetches > 0 && progress.SuccessfulFetches > 0 {
		return RetrievalDecision{
			ShouldForceFinal: true,
			Reason:           "web retrieval is starting to loop without adding new evidence",
		}
	}

	if progress.EmptyFetches >= 2 && progress.SuccessfulFetches > 0 {
		return RetrievalDecision{
			ShouldForceFinal: true,
			Reason:           "recent web.fetch calls were empty while earlier evidence is already available",
		}
	}

	return RetrievalDecision{}
}

func isUsefulFetch(result map[string]any) bool {
	title := strings.TrimSpace(firstNonEmptyString(result["title"]))
	content := strings.TrimSpace(firstNonEmptyString(result["content"], result["content_excerpt"]))
	return title != "" || content != ""
}

func excerptForEvidence(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	runes := []rune(content)
	if len(runes) <= 400 {
		return content
	}
	return string(runes[:400]) + "..."
}

func firstNonEmptyString(values ...any) string {
	for _, value := range values {
		if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func BuildEvidencePayload(progress RetrievalProgress) map[string]any {
	return map[string]any{
		"search_queries":     progress.SearchQueries,
		"fetched_urls":       progress.FetchedURLs,
		"successful_fetches": progress.SuccessfulFetches,
		"empty_fetches":      progress.EmptyFetches,
		"duplicate_fetches":  progress.DuplicateFetches,
		"distinct_evidence":  progress.DistinctEvidence,
		"retrieved_evidence": progress.Evidence,
	}
}
