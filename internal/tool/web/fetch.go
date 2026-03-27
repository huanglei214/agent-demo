package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/huanglei214/agent-demo/internal/tool"
)

type FetchTool struct {
	client *http.Client
}

var titlePattern = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

func NewFetchTool() FetchTool {
	return FetchTool{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (t FetchTool) Name() string {
	return "web.fetch"
}

func (t FetchTool) Description() string {
	return "Fetch a public web page and return its title and extracted content."
}

func (t FetchTool) AccessMode() tool.AccessMode {
	return tool.AccessReadOnly
}

func (t FetchTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return tool.Result{}, err
	}

	target := strings.TrimSpace(req.URL)
	if target == "" {
		return tool.Result{}, errors.New("url is required")
	}
	parsed, err := url.Parse(target)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return tool.Result{}, errors.New("url must be an absolute http(s) URL")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return tool.Result{}, err
	}
	httpReq.Header.Set("User-Agent", "agent-demo/1.0 (+https://localhost)")

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return tool.Result{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return tool.Result{}, err
	}
	if resp.StatusCode >= 400 {
		return tool.Result{}, fmt.Errorf("fetch request failed with status %d", resp.StatusCode)
	}

	bodyText := string(body)
	title := ""
	if match := titlePattern.FindStringSubmatch(bodyText); len(match) >= 2 {
		title = strings.TrimSpace(stripTags(match[1]))
	}
	content, truncated := truncateString(extractMeaningfulContent(bodyText), maxFetchedContentRunes)

	finalURL := target
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}

	return tool.Result{
		Content: map[string]any{
			"url":         target,
			"final_url":   finalURL,
			"status_code": resp.StatusCode,
			"title":       title,
			"content":     content,
			"truncated":   truncated,
		},
	}, nil
}
