package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/huanglei214/agent-demo/internal/tool"
)

type SearchTool struct {
	client         *http.Client
	endpoint       string
	tavilyEndpoint string
	apiKey         string
}

var searchResultPattern = regexp.MustCompile(`(?s)<a[^>]*class="[^"]*result__a[^"]*"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)

func NewSearchTool() SearchTool {
	return SearchTool{
		client:         &http.Client{Timeout: 15 * time.Second},
		endpoint:       "https://html.duckduckgo.com/html/",
		tavilyEndpoint: "https://api.tavily.com",
		apiKey:         strings.TrimSpace(os.Getenv("TAVILY_API_KEY")),
	}
}

func (t SearchTool) Name() string {
	return "web.search"
}

func (t SearchTool) Description() string {
	return "Search the public web and return structured search results."
}

func (t SearchTool) AccessMode() tool.AccessMode {
	return tool.AccessReadOnly
}

func (t SearchTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return tool.Result{}, err
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		return tool.Result{}, errors.New("query is required")
	}
	limit := normalizeLimit(req.Limit)

	if t.apiKey != "" {
		result, ok, err := t.executeTavilySearch(ctx, query, limit)
		if err == nil && ok {
			return result, nil
		}
		if err != nil && !shouldFallbackFromTavily(err) {
			return tool.Result{}, err
		}
	}

	return t.executeDuckDuckGoSearch(ctx, query, limit)
}

func (t SearchTool) executeTavilySearch(ctx context.Context, query string, limit int) (tool.Result, bool, error) {
	var resp tavilySearchResponse
	err := doTavilyRequest(ctx, t.client, t.tavilyEndpoint, t.apiKey, "search", tavilySearchRequest{
		Query:      query,
		MaxResults: limit,
	}, &resp)
	if err != nil {
		return tool.Result{}, false, err
	}

	results := make([]map[string]any, 0, min(limit, len(resp.Results)))
	for _, item := range resp.Results {
		title := strings.TrimSpace(item.Title)
		href := strings.TrimSpace(item.URL)
		if title == "" || href == "" {
			continue
		}
		results = append(results, map[string]any{
			"title":   title,
			"url":     href,
			"snippet": strings.TrimSpace(item.Content),
		})
		if len(results) >= limit {
			break
		}
	}
	if len(results) == 0 {
		return tool.Result{}, false, errTavilyNoResults
	}

	return tool.Result{
		Content: map[string]any{
			"query":     query,
			"results":   results,
			"truncated": len(resp.Results) > limit,
		},
	}, true, nil
}

func (t SearchTool) executeDuckDuckGoSearch(ctx context.Context, query string, limit int) (tool.Result, error) {
	searchURL, err := url.Parse(t.endpoint)
	if err != nil {
		return tool.Result{}, err
	}
	params := searchURL.Query()
	params.Set("q", query)
	searchURL.RawQuery = params.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL.String(), nil)
	if err != nil {
		return tool.Result{}, err
	}
	httpReq.Header.Set("User-Agent", "agent-demo/1.0 (+https://localhost)")

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return tool.Result{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return tool.Result{}, fmt.Errorf("search request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return tool.Result{}, err
	}

	matches := searchResultPattern.FindAllStringSubmatch(string(body), limit+1)
	results := make([]map[string]any, 0, min(limit, len(matches)))
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		title := strings.TrimSpace(stripTags(match[2]))
		href := normalizeSearchURL(strings.TrimSpace(htmlUnescape(match[1])))
		if title == "" || href == "" {
			continue
		}
		results = append(results, map[string]any{
			"title":   title,
			"url":     href,
			"snippet": "",
		})
		if len(results) >= limit {
			break
		}
	}

	return tool.Result{
		Content: map[string]any{
			"query":     query,
			"results":   results,
			"truncated": len(matches) > limit,
		},
	}, nil
}

func htmlUnescape(value string) string {
	replacer := strings.NewReplacer("&amp;", "&", "&quot;", `"`, "&#x2F;", "/", "&#39;", "'")
	return replacer.Replace(value)
}

func normalizeSearchURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if strings.Contains(parsed.Host, "duckduckgo.com") {
		if target := strings.TrimSpace(parsed.Query().Get("uddg")); target != "" {
			return target
		}
	}
	return parsed.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
