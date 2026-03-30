package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type tavilySearchRequest struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
}

type tavilySearchResponse struct {
	Results []tavilySearchResult `json:"results"`
}

type tavilySearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

type tavilyExtractRequest struct {
	URLs []string `json:"urls"`
}

type tavilyExtractResponse struct {
	Results []tavilyExtractResult `json:"results"`
}

type tavilyExtractResult struct {
	URL        string `json:"url"`
	RawContent string `json:"raw_content"`
	Content    string `json:"content"`
}

type tavilyHTTPError struct {
	StatusCode int
	Body       string
}

func (e *tavilyHTTPError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Body) == "" {
		return fmt.Sprintf("tavily request failed with status %d", e.StatusCode)
	}
	return fmt.Sprintf("tavily request failed with status %d: %s", e.StatusCode, strings.TrimSpace(e.Body))
}

func doTavilyRequest(
	ctx context.Context,
	client *http.Client,
	endpoint string,
	apiKey string,
	method string,
	requestBody any,
	responseBody any,
) error {
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(endpoint, "/")+"/"+method, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "agent-demo/1.0 (+https://localhost)")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return &tavilyHTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}
	if responseBody == nil || len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, responseBody)
}
