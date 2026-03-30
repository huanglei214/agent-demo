package web

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestSearchToolReturnsStructuredResults(t *testing.T) {
	t.Parallel()

	tool := NewSearchTool()
	tool.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.URL.Query().Get("q"); got != "wuhan weather" {
				t.Fatalf("unexpected query: %s", got)
			}
			return stringResponse(http.StatusOK, `
				<html><body>
					<a class="result__a" href="https://example.com/weather">Wuhan Weather</a>
				</body></html>
			`)
		}),
	}
	tool.endpoint = "https://search.example.com/html/"

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
	if results[0]["title"] != "Wuhan Weather" {
		t.Fatalf("unexpected title: %#v", results[0])
	}
}

func TestSearchToolNormalizesDuckDuckGoRedirectURL(t *testing.T) {
	t.Parallel()

	tool := NewSearchTool()
	tool.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return stringResponse(http.StatusOK, `
				<html><body>
					<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fweather.example.com%2Fwuhan&amp;rut=abc">Wuhan Weather</a>
				</body></html>
			`)
		}),
	}
	tool.endpoint = "https://search.example.com/html/"

	result, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"query": "wuhan weather",
	}))
	if err != nil {
		t.Fatalf("search execute: %v", err)
	}

	results := result.Content["results"].([]map[string]any)
	if len(results) != 1 {
		t.Fatalf("unexpected search results: %#v", result.Content)
	}
	if results[0]["url"] != "https://weather.example.com/wuhan" {
		t.Fatalf("expected normalized URL, got %#v", results[0])
	}
}

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
	if len(seen) != 2 {
		t.Fatalf("expected Tavily then DuckDuckGo requests, got %#v", seen)
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

func TestSearchToolRejectsEmptyQuery(t *testing.T) {
	t.Parallel()

	_, err := NewSearchTool().Execute(context.Background(), mustJSON(t, map[string]any{}))
	if err == nil || !strings.Contains(err.Error(), "query is required") {
		t.Fatalf("expected query required error, got %v", err)
	}
}

func TestFetchToolExtractsContent(t *testing.T) {
	t.Parallel()

	tool := NewFetchTool()
	tool.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return stringResponse(http.StatusOK, `
				<html>
					<head><title>Wuhan Weather</title></head>
					<body><main>Today is cloudy and 22C.</main></body>
				</html>
			`)
		}),
	}

	result, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"url": "https://example.com/weather",
	}))
	if err != nil {
		t.Fatalf("fetch execute: %v", err)
	}

	if result.Content["title"] != "Wuhan Weather" {
		t.Fatalf("unexpected title: %#v", result.Content)
	}
	if !strings.Contains(result.Content["content"].(string), "Today is cloudy and 22C.") {
		t.Fatalf("unexpected content: %#v", result.Content)
	}
}

func TestFetchToolPrefersMainContentAndDropsNoise(t *testing.T) {
	t.Parallel()

	tool := NewFetchTool()
	tool.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return stringResponse(http.StatusOK, `
				<html>
					<head>
						<title>武汉天气</title>
						<style>.banner { color: red; }</style>
						<script>console.log("ignore me")</script>
					</head>
					<body>
						<nav>首页 天气 视频 财经</nav>
						<main>
							<h1>武汉天气</h1>
							<p>今天多云，22C，东北风 2 级。</p>
						</main>
						<footer>Copyright weather site</footer>
					</body>
				</html>
			`)
		}),
	}

	result, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"url": "https://example.com/weather",
	}))
	if err != nil {
		t.Fatalf("fetch execute: %v", err)
	}

	content := result.Content["content"].(string)
	if !strings.Contains(content, "今天多云，22C，东北风 2 级。") {
		t.Fatalf("expected main content, got %#v", result.Content)
	}
	for _, unwanted := range []string{"banner", "console.log", "首页 天气 视频 财经", "Copyright weather site"} {
		if strings.Contains(content, unwanted) {
			t.Fatalf("unexpected noise %q in content: %#v", unwanted, result.Content)
		}
	}
}

func TestFetchToolFallsBackToBodyWhenNoMainOrArticleExists(t *testing.T) {
	t.Parallel()

	tool := NewFetchTool()
	tool.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return stringResponse(http.StatusOK, `
				<html>
					<head><title>天气播报</title></head>
					<body>
						<section>实时天气：小雨，19C。</section>
					</body>
				</html>
			`)
		}),
	}

	result, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"url": "https://example.com/weather",
	}))
	if err != nil {
		t.Fatalf("fetch execute: %v", err)
	}

	if !strings.Contains(result.Content["content"].(string), "实时天气：小雨，19C。") {
		t.Fatalf("expected body fallback content, got %#v", result.Content)
	}
}

func TestFetchToolRejectsRelativeURL(t *testing.T) {
	t.Parallel()

	_, err := NewFetchTool().Execute(context.Background(), mustJSON(t, map[string]any{
		"url": "/weather",
	}))
	if err == nil || !strings.Contains(err.Error(), "absolute http(s) URL") {
		t.Fatalf("expected absolute URL error, got %v", err)
	}
}

func TestFetchToolRejectsLocalhostTargets(t *testing.T) {
	t.Parallel()

	_, err := NewFetchTool().Execute(context.Background(), mustJSON(t, map[string]any{
		"url": "http://127.0.0.1:8080/status",
	}))
	if err == nil || !strings.Contains(err.Error(), "restricted address") {
		t.Fatalf("expected restricted address error, got %v", err)
	}
}

func TestFetchToolRejectsResolvedPrivateAddresses(t *testing.T) {
	t.Parallel()

	tool := NewFetchTool()
	tool.resolver = staticResolver{addresses: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}}

	_, err := tool.Execute(context.Background(), mustJSON(t, map[string]any{
		"url": "https://weather.example.com/today",
	}))
	if err == nil || !strings.Contains(err.Error(), "restricted address") {
		t.Fatalf("expected resolved restricted address error, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

type staticResolver struct {
	addresses []net.IPAddr
}

func (r staticResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return r.addresses, nil
}

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func stringResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request: &http.Request{
			URL: mustURL("https://example.com/final"),
		},
	}, nil
}

func mustURL(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return parsed
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return data
}
