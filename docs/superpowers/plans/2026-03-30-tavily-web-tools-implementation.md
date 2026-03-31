# Tavily Web Tools Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add automatic Tavily-first support to `web.search` and `web.fetch`, with transparent fallback to the current DuckDuckGo and direct-fetch implementations when Tavily is unconfigured, rate-limited, or unavailable.

**Architecture:** Keep the public tool surface unchanged and add the provider-selection logic inside `internal/tool/web`. Introduce a small Tavily client plus fallback helpers, preserve `web.fetch` public-URL validation ahead of any network call, and drive the change with focused failing tests before each production edit.

**Tech Stack:** Go, `net/http`, package-local test doubles in `internal/tool/web/web_test.go`, existing Cobra/service bootstrap, Markdown docs/specs.

---

## File map

- Modify: `internal/tool/web/search.go`
  - Keep `web.search` input/output stable.
  - Split current DuckDuckGo behavior into a reusable fallback path.
  - Add Tavily-first dispatch when `TAVILY_API_KEY` is present.
- Modify: `internal/tool/web/fetch.go`
  - Keep `web.fetch` input/output stable.
  - Preserve URL/public-address validation ahead of all network calls.
  - Add Tavily Extract first, direct-fetch fallback second.
- Modify: `internal/tool/web/common.go`
  - Add small shared helpers only if both tools need them, such as fallback classification or env lookup wrappers.
- Create: `internal/tool/web/tavily.go`
  - Isolate Tavily request/response structs and helper methods for `/search` and `/extract`.
- Modify: `internal/tool/web/web_test.go`
  - Add Tavily-first, fallback, and security-regression coverage.
- Modify: `README.md`
  - Document `TAVILY_API_KEY` and the fallback behavior.
- Modify: `.env.example`
  - Add `TAVILY_API_KEY=your_api_key`.
- Modify: `openspec/specs/web-retrieval-tools/spec.md`
  - Encode the Tavily-first auto-detection and fallback semantics.

## Implementation notes

- Do not add a new top-level config section or change CLI flags for this feature.
- Read `TAVILY_API_KEY` directly from the process environment inside the web tool package.
- Treat Tavily `429`, `5xx`, timeout, transport failure, and empty successful responses as fallback conditions.
- Do not fall back for invalid user input or restricted-address rejection.
- Keep `tool.Result` shapes unchanged so prompt logic, retrieval guard, and skills remain untouched.

### Task 1: Add failing tests for `web.search` Tavily-first behavior

**Files:**
- Modify: `internal/tool/web/web_test.go`
- Test: `internal/tool/web/web_test.go`

- [ ] **Step 1: Write the failing Tavily-first search test**

Add a test near the existing search tests that proves `web.search` prefers Tavily when `TAVILY_API_KEY` is set and maps Tavily content into the existing result format.

```go
func TestSearchToolUsesTavilyWhenAPIKeyIsPresent(t *testing.T) {
	t.Parallel()

	tool := NewSearchTool()
	tool.apiKey = "test-key"
	tool.tavilyEndpoint = "https://api.tavily.example"
	tool.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("expected bearer auth, got %q", got)
			}
			if r.URL.String() != "https://api.tavily.example/search" {
				t.Fatalf("unexpected Tavily URL: %s", r.URL.String())
			}
			return stringResponse(http.StatusOK, `{
				"results": [
					{
						"title": "Wuhan Weather",
						"url": "https://weather.example.com/wuhan",
						"content": "Cloudy and 22C."
					}
				]
			}`)
		}),
	}

	result, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"query": "wuhan weather",
		"limit": 3,
	}))
	if err != nil {
		t.Fatalf("search execute: %v", err)
	}

	results := result.Content["results"].([]map[string]any)
	if len(results) != 1 {
		t.Fatalf("unexpected search results: %#v", result.Content)
	}
	if results[0]["snippet"] != "Cloudy and 22C." {
		t.Fatalf("expected Tavily content as snippet, got %#v", results[0])
	}
}
```

- [ ] **Step 2: Run the targeted test and verify it fails**

Run: `go test ./internal/tool/web -run TestSearchToolUsesTavilyWhenAPIKeyIsPresent -count=1`

Expected: FAIL because `SearchTool` has no `apiKey` or `tavilyEndpoint` fields and no Tavily-first execution path yet.

- [ ] **Step 3: Write the failing Tavily fallback tests**

Add two more search tests before touching production code:

```go
func TestSearchToolFallsBackToDuckDuckGoWhenTavilyRateLimited(t *testing.T) {
	t.Parallel()

	var seen []string
	tool := NewSearchTool()
	tool.apiKey = "test-key"
	tool.tavilyEndpoint = "https://api.tavily.example"
	tool.endpoint = "https://search.example.com/html/"
	tool.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			seen = append(seen, r.URL.String())
			switch r.URL.String() {
			case "https://api.tavily.example/search":
				return stringResponse(http.StatusTooManyRequests, `{"error":"rate limited"}`)
			case "https://search.example.com/html/?q=wuhan+weather":
				return stringResponse(http.StatusOK, `<a class="result__a" href="https://example.com/weather">Wuhan Weather</a>`)
			default:
				t.Fatalf("unexpected URL: %s", r.URL.String())
				return nil, nil
			}
		}),
	}

	result, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{"query": "wuhan weather"}))
	if err != nil {
		t.Fatalf("search execute: %v", err)
	}

	results := result.Content["results"].([]map[string]any)
	if len(results) != 1 || results[0]["url"] != "https://example.com/weather" {
		t.Fatalf("expected DuckDuckGo fallback result, got %#v", result.Content)
	}
}

func TestSearchToolFallsBackToDuckDuckGoWhenTavilyReturnsNoResults(t *testing.T) {
	t.Parallel()

	tool := NewSearchTool()
	tool.apiKey = "test-key"
	tool.tavilyEndpoint = "https://api.tavily.example"
	tool.endpoint = "https://search.example.com/html/"
	tool.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if strings.Contains(r.URL.String(), "/search") {
				return stringResponse(http.StatusOK, `{"results":[]}`)
			}
			return stringResponse(http.StatusOK, `<a class="result__a" href="https://example.com/weather">Wuhan Weather</a>`)
		}),
	}

	result, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{"query": "wuhan weather"}))
	if err != nil {
		t.Fatalf("search execute: %v", err)
	}

	results := result.Content["results"].([]map[string]any)
	if len(results) != 1 || results[0]["title"] != "Wuhan Weather" {
		t.Fatalf("expected fallback search result, got %#v", result.Content)
	}
}
```

- [ ] **Step 4: Run the fallback tests and verify they fail**

Run: `go test ./internal/tool/web -run 'TestSearchTool(FallsBackToDuckDuckGoWhenTavilyRateLimited|FallsBackToDuckDuckGoWhenTavilyReturnsNoResults)' -count=1`

Expected: FAIL because the current implementation has no Tavily branch and cannot fall back from it.

- [ ] **Step 5: Commit the red tests**

```bash
git add internal/tool/web/web_test.go
git commit -m "test: cover Tavily search fallback behavior"
```

### Task 2: Implement Tavily search client and `web.search` fallback flow

**Files:**
- Create: `internal/tool/web/tavily.go`
- Modify: `internal/tool/web/search.go`
- Modify: `internal/tool/web/common.go`
- Test: `internal/tool/web/web_test.go`

- [ ] **Step 1: Add the minimal Tavily client types**

Create `internal/tool/web/tavily.go` with package-local request/response structs and two small methods:

```go
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

const defaultTavilyEndpoint = "https://api.tavily.com"

type tavilyClient struct {
	httpClient *http.Client
	endpoint   string
	apiKey     string
}

type tavilySearchResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	} `json:"results"`
}

type tavilyExtractResponse struct {
	Results []struct {
		RawContent string `json:"raw_content"`
	} `json:"results"`
}

func (c tavilyClient) search(ctx context.Context, query string, limit int) (tavilySearchResponse, int, error) {
	body := map[string]any{
		"query":       query,
		"max_results": limit,
	}
	return doTavilyRequest[tavilySearchResponse](ctx, c, "/search", body)
}

func (c tavilyClient) extract(ctx context.Context, target string) (tavilyExtractResponse, int, error) {
	body := map[string]any{
		"urls": []string{target},
	}
	return doTavilyRequest[tavilyExtractResponse](ctx, c, "/extract", body)
}

func doTavilyRequest[T any](ctx context.Context, c tavilyClient, path string, payload map[string]any) (T, int, error) {
	var zero T
	data, err := json.Marshal(payload)
	if err != nil {
		return zero, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.endpoint, "/")+path, bytes.NewReader(data))
	if err != nil {
		return zero, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return zero, 0, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, resp.StatusCode, err
	}
	if resp.StatusCode >= 400 {
		return zero, resp.StatusCode, fmt.Errorf("tavily request failed with status %d", resp.StatusCode)
	}

	var decoded T
	if err := json.Unmarshal(bodyBytes, &decoded); err != nil {
		return zero, resp.StatusCode, err
	}
	return decoded, resp.StatusCode, nil
}
```

- [ ] **Step 2: Refactor `SearchTool` to support Tavily-first dispatch**

Update `internal/tool/web/search.go` so the top-level tool can route to Tavily or DuckDuckGo without changing the public API:

```go
type SearchTool struct {
	client         *http.Client
	endpoint       string
	tavilyEndpoint string
	apiKey         string
}

func NewSearchTool() SearchTool {
	client := &http.Client{Timeout: 15 * time.Second}
	return SearchTool{
		client:         client,
		endpoint:       "https://html.duckduckgo.com/html/",
		tavilyEndpoint: defaultTavilyEndpoint,
		apiKey:         strings.TrimSpace(os.Getenv("TAVILY_API_KEY")),
	}
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

	if strings.TrimSpace(t.apiKey) != "" {
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
```

- [ ] **Step 3: Add explicit Tavily mapping and fallback classification helpers**

Complete the refactor with focused helper methods instead of leaving branching logic inline:

```go
func (t SearchTool) executeTavilySearch(ctx context.Context, query string, limit int) (tool.Result, bool, error) {
	resp, statusCode, err := tavilyClient{
		httpClient: t.client,
		endpoint:   t.tavilyEndpoint,
		apiKey:     t.apiKey,
	}.search(ctx, query, limit)
	if err != nil {
		return tool.Result{}, false, classifyTavilyError(statusCode, err)
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
	// move the current implementation here with no behavior change
}
```

Shared fallback helpers in `internal/tool/web/common.go`:

```go
var errTavilyNoResults = errors.New("tavily returned no usable results")

func shouldFallbackFromTavily(err error) bool {
	return errors.Is(err, errTavilyNoResults) ||
		errors.Is(err, errTavilyRateLimited) ||
		errors.Is(err, errTavilyUnavailable) ||
		errors.Is(err, context.DeadlineExceeded)
}
```

- [ ] **Step 4: Run the search-focused tests and verify green**

Run: `go test ./internal/tool/web -run 'TestSearchTool(ReturnsStructuredResults|NormalizesDuckDuckGoRedirectURL|UsesTavilyWhenAPIKeyIsPresent|FallsBackToDuckDuckGoWhenTavilyRateLimited|FallsBackToDuckDuckGoWhenTavilyReturnsNoResults|RejectsEmptyQuery)' -count=1`

Expected: PASS

- [ ] **Step 5: Commit the search implementation**

```bash
git add internal/tool/web/search.go internal/tool/web/common.go internal/tool/web/tavily.go internal/tool/web/web_test.go
git commit -m "feat: add Tavily-first web search"
```

### Task 3: Add failing tests for `web.fetch` Tavily-first and security behavior

**Files:**
- Modify: `internal/tool/web/web_test.go`
- Test: `internal/tool/web/web_test.go`

- [ ] **Step 1: Write the failing Tavily-first fetch test**

Add a fetch test that proves `web.fetch` uses Tavily Extract, preserves the tool output shape, and truncates extracted content.

```go
func TestFetchToolUsesTavilyWhenAPIKeyIsPresent(t *testing.T) {
	t.Parallel()

	tool := NewFetchTool()
	tool.apiKey = "test-key"
	tool.tavilyEndpoint = "https://api.tavily.example"
	tool.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("expected bearer auth, got %q", got)
			}
			return stringResponse(http.StatusOK, `{
				"results": [
					{"raw_content": "Today is cloudy and 22C."}
				]
			}`)
		}),
	}
	tool.resolver = staticResolver{addresses: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}}

	result, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"url": "https://example.com/weather",
	}))
	if err != nil {
		t.Fatalf("fetch execute: %v", err)
	}

	if result.Content["final_url"] != "https://example.com/weather" {
		t.Fatalf("expected final_url to stay at input URL, got %#v", result.Content)
	}
	if result.Content["content"] != "Today is cloudy and 22C." {
		t.Fatalf("expected Tavily extracted content, got %#v", result.Content)
	}
}
```

- [ ] **Step 2: Run the targeted fetch test and verify it fails**

Run: `go test ./internal/tool/web -run TestFetchToolUsesTavilyWhenAPIKeyIsPresent -count=1`

Expected: FAIL because `FetchTool` has no Tavily fields and no Extract branch yet.

- [ ] **Step 3: Write the failing Tavily fallback and security tests**

Add one fallback test and one security regression test:

```go
func TestFetchToolFallsBackToDirectFetchWhenTavilyRateLimited(t *testing.T) {
	t.Parallel()

	tool := NewFetchTool()
	tool.apiKey = "test-key"
	tool.tavilyEndpoint = "https://api.tavily.example"
	tool.resolver = staticResolver{addresses: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}}
	tool.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.String() {
			case "https://api.tavily.example/extract":
				return stringResponse(http.StatusTooManyRequests, `{"error":"rate limited"}`)
			case "https://example.com/weather":
				return stringResponse(http.StatusOK, `
					<html>
						<head><title>Wuhan Weather</title></head>
						<body><main>Today is cloudy and 22C.</main></body>
					</html>
				`)
			default:
				t.Fatalf("unexpected URL: %s", r.URL.String())
				return nil, nil
			}
		}),
	}

	result, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"url": "https://example.com/weather",
	}))
	if err != nil {
		t.Fatalf("fetch execute: %v", err)
	}

	if result.Content["title"] != "Wuhan Weather" {
		t.Fatalf("expected direct-fetch fallback title, got %#v", result.Content)
	}
}

func TestFetchToolRejectsRestrictedTargetsBeforeCallingTavily(t *testing.T) {
	t.Parallel()

	called := false
	tool := NewFetchTool()
	tool.apiKey = "test-key"
	tool.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			called = true
			return stringResponse(http.StatusOK, `{}`)
		}),
	}

	_, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"url": "http://127.0.0.1:8080/status",
	}))
	if err == nil {
		t.Fatalf("expected restricted address error")
	}
	if called {
		t.Fatalf("expected no outbound Tavily call for restricted target")
	}
}
```

- [ ] **Step 4: Run the new fetch tests and verify they fail**

Run: `go test ./internal/tool/web -run 'TestFetchTool(UsesTavilyWhenAPIKeyIsPresent|FallsBackToDirectFetchWhenTavilyRateLimited|RejectsRestrictedTargetsBeforeCallingTavily)' -count=1`

Expected: FAIL because the fetch path still only supports direct HTTP fetching.

- [ ] **Step 5: Commit the red tests**

```bash
git add internal/tool/web/web_test.go
git commit -m "test: cover Tavily fetch fallback behavior"
```

### Task 4: Implement Tavily extract flow inside `web.fetch`

**Files:**
- Modify: `internal/tool/web/fetch.go`
- Modify: `internal/tool/web/common.go`
- Modify: `internal/tool/web/tavily.go`
- Test: `internal/tool/web/web_test.go`

- [ ] **Step 1: Extend `FetchTool` to carry Tavily settings**

Update the constructor and struct in `internal/tool/web/fetch.go`:

```go
type FetchTool struct {
	client         *http.Client
	resolver       ipResolver
	tavilyEndpoint string
	apiKey         string
}

func NewFetchTool() FetchTool {
	return FetchTool{
		client:         &http.Client{Timeout: 15 * time.Second},
		resolver:       net.DefaultResolver,
		tavilyEndpoint: defaultTavilyEndpoint,
		apiKey:         strings.TrimSpace(os.Getenv("TAVILY_API_KEY")),
	}
}
```

- [ ] **Step 2: Split direct-fetch logic into a helper and insert Tavily-first dispatch**

Keep validation at the top of `Execute`, then route:

```go
func (t FetchTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	target, parsed, err := t.parseAndValidateTarget(ctx, input)
	if err != nil {
		return tool.Result{}, err
	}

	if strings.TrimSpace(t.apiKey) != "" {
		result, ok, err := t.executeTavilyFetch(ctx, target)
		if err == nil && ok {
			return result, nil
		}
		if err != nil && !shouldFallbackFromTavily(err) {
			return tool.Result{}, err
		}
	}

	return t.executeDirectFetch(ctx, target, parsed)
}
```

- [ ] **Step 3: Implement Tavily mapping with truncation and empty-response fallback**

Add the Tavily fetch helper:

```go
func (t FetchTool) executeTavilyFetch(ctx context.Context, target string) (tool.Result, bool, error) {
	resp, statusCode, err := tavilyClient{
		httpClient: t.client,
		endpoint:   t.tavilyEndpoint,
		apiKey:     t.apiKey,
	}.extract(ctx, target)
	if err != nil {
		return tool.Result{}, false, classifyTavilyError(statusCode, err)
	}
	if len(resp.Results) == 0 || strings.TrimSpace(resp.Results[0].RawContent) == "" {
		return tool.Result{}, false, errTavilyNoResults
	}

	content, truncated := truncateString(strings.TrimSpace(resp.Results[0].RawContent), maxFetchedContentRunes)
	return tool.Result{
		Content: map[string]any{
			"url":         target,
			"final_url":   target,
			"status_code": http.StatusOK,
			"title":       "",
			"content":     content,
			"truncated":   truncated,
		},
	}, true, nil
}
```

Keep the current HTTP GET code in a new `executeDirectFetch` helper with no behavior change.

- [ ] **Step 4: Run the full web tool test package and verify green**

Run: `go test ./internal/tool/web -count=1`

Expected: PASS

- [ ] **Step 5: Commit the fetch implementation**

```bash
git add internal/tool/web/fetch.go internal/tool/web/common.go internal/tool/web/tavily.go internal/tool/web/web_test.go
git commit -m "feat: add Tavily-first web fetch"
```

### Task 5: Update docs and specs, then run cross-package verification

**Files:**
- Modify: `README.md`
- Modify: `.env.example`
- Modify: `openspec/specs/web-retrieval-tools/spec.md`
- Test: `internal/tool/web/web_test.go`
- Test: `internal/service/services.go`

- [ ] **Step 1: Add `TAVILY_API_KEY` to `.env.example`**

Update `.env.example` to:

```bash
# Secrets only
ARK_API_KEY=your_api_key
TAVILY_API_KEY=your_api_key
```

- [ ] **Step 2: Update the README configuration section**

Extend the existing configuration docs with a short Tavily note near the `.env` example and tool list:

```md
`.env` 示例：

```bash
ARK_API_KEY=your_api_key
TAVILY_API_KEY=your_api_key
```

如果配置了 `TAVILY_API_KEY`，`web.search` 和 `web.fetch` 会自动优先使用 Tavily。
当 Tavily 返回限流、5xx、超时或空结果时，系统会自动回退到当前 DuckDuckGo 搜索和直接页面抓取实现。
```

- [ ] **Step 3: Update the OpenSpec for Tavily-first semantics**

Add a new scenario block to `openspec/specs/web-retrieval-tools/spec.md` covering auto-detection and fallback:

```md
#### Scenario: 搜索工具自动优先 Tavily
- **WHEN** Agent 调用 `web.search` 且运行环境中配置了 `TAVILY_API_KEY`
- **THEN** 系统 MUST 优先调用 Tavily Search
- **THEN** 在 Tavily 成功时 MUST 返回与现有结构兼容的搜索结果

#### Scenario: Tavily 搜索失败后回退
- **WHEN** Tavily Search 因限流、5xx、超时或空结果不可用
- **THEN** 系统 MUST 自动回退到默认搜索实现

#### Scenario: 抓取工具自动优先 Tavily
- **WHEN** Agent 调用 `web.fetch`、目标 URL 通过安全校验且运行环境中配置了 `TAVILY_API_KEY`
- **THEN** 系统 MUST 优先调用 Tavily Extract

#### Scenario: Tavily 抓取失败后回退
- **WHEN** Tavily Extract 因限流、5xx、超时或空结果不可用
- **THEN** 系统 MUST 自动回退到默认页面抓取实现
```

- [ ] **Step 4: Run package verification**

Run: `go test ./internal/tool/web ./internal/service -count=1`

Expected: PASS

- [ ] **Step 5: Run repo-wide verification if package tests passed**

Run: `go test ./...`

Expected: PASS

- [ ] **Step 6: Commit the docs and verification pass**

```bash
git add README.md .env.example openspec/specs/web-retrieval-tools/spec.md
git commit -m "docs: describe Tavily web tool fallback behavior"
```

## Self-review

- Spec coverage:
  - Tavily-first `web.search`: Tasks 1-2.
  - Tavily-first `web.fetch`: Tasks 3-4.
  - Fallback on `429`, `5xx`, timeout, and empty results: Tasks 1-4.
  - Restricted-address preservation for `web.fetch`: Task 3 and Task 4.
  - README, env example, and OpenSpec updates: Task 5.
- Placeholder scan:
  - No placeholder tokens or deferred “write tests later” language remain.
  - Every code-changing step includes concrete code or exact command targets.
- Type consistency:
  - `SearchTool` and `FetchTool` both use `apiKey` and `tavilyEndpoint`.
  - Shared Tavily transport lives in `tavilyClient`.
  - Fallback classification uses `shouldFallbackFromTavily` and `classifyTavilyError` consistently.
