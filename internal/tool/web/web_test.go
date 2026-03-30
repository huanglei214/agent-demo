package web

import (
	"context"
	"encoding/json"
	"errors"
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
	t.Setenv("TAVILY_API_KEY", "test-key")
	tool := NewSearchTool()
	tool.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected Tavily POST request, got %s", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("expected bearer auth, got %q", got)
			}
			if r.URL.String() != "https://api.tavily.com/search" {
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
	t.Setenv("TAVILY_API_KEY", "test-key")
	var seen []string
	tool := NewSearchTool()
	tool.endpoint = "https://search.example.com/html/"
	tool.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			seen = append(seen, r.Method+" "+r.URL.String())
			switch r.URL.String() {
			case "https://api.tavily.com/search":
				if r.Method != http.MethodPost {
					t.Fatalf("expected Tavily POST request, got %s", r.Method)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
					t.Fatalf("expected bearer auth, got %q", got)
				}
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
	if len(seen) == 0 || seen[0] != "POST https://api.tavily.com/search" {
		t.Fatalf("expected Tavily request first, got %#v", seen)
	}
	fallbackSeen := false
	for _, got := range seen {
		if got == "GET https://search.example.com/html/?q=wuhan+weather" {
			fallbackSeen = true
			break
		}
	}
	if !fallbackSeen {
		t.Fatalf("expected fallback request against configured endpoint, got %#v", seen)
	}
}

func TestSearchToolFallsBackToDuckDuckGoWhenTavilyReturnsNoResults(t *testing.T) {
	t.Setenv("TAVILY_API_KEY", "test-key")
	var seen []string
	tool := NewSearchTool()
	tool.endpoint = "https://search.example.com/html/"
	tool.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			seen = append(seen, r.Method+" "+r.URL.String())
			if r.URL.String() == "https://api.tavily.com/search" {
				if r.Method != http.MethodPost {
					t.Fatalf("expected Tavily POST request, got %s", r.Method)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
					t.Fatalf("expected bearer auth, got %q", got)
				}
				return stringResponse(http.StatusOK, `{"results":[]}`)
			}
			if r.URL.String() == "https://search.example.com/html/?q=wuhan+weather" {
				return stringResponse(http.StatusOK, `<a class="result__a" href="https://example.com/weather">Wuhan Weather</a>`)
			}
			t.Fatalf("unexpected URL: %s", r.URL.String())
			return nil, nil
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
	if len(seen) == 0 || seen[0] != "POST https://api.tavily.com/search" {
		t.Fatalf("expected Tavily request first, got %#v", seen)
	}
	fallbackSeen := false
	for _, got := range seen {
		if got == "GET https://search.example.com/html/?q=wuhan+weather" {
			fallbackSeen = true
			break
		}
	}
	if !fallbackSeen {
		t.Fatalf("expected fallback request against configured endpoint, got %#v", seen)
	}
}

func TestSearchToolRejectsEmptyQuery(t *testing.T) {
	t.Parallel()

	_, err := NewSearchTool().Execute(context.Background(), mustJSON(t, map[string]any{}))
	if err == nil || !strings.Contains(err.Error(), "query is required") {
		t.Fatalf("expected query required error, got %v", err)
	}
}

func TestShouldFallbackFromTavilyDoesNotTreatEveryURLErrorAsFallback(t *testing.T) {
	t.Parallel()

	err := &url.Error{
		Op:  http.MethodPost,
		URL: defaultTavilyEndpoint + "/search",
		Err: errors.New("unsupported protocol scheme"),
	}

	if shouldFallbackFromTavily(err) {
		t.Fatalf("expected config-style url error to surface, but fallback was allowed")
	}
}

func TestShouldFallbackFromTavilyAllowsTransportFailures(t *testing.T) {
	t.Parallel()

	err := &url.Error{
		Op:  http.MethodPost,
		URL: defaultTavilyEndpoint + "/search",
		Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
	}

	if !shouldFallbackFromTavily(err) {
		t.Fatalf("expected transport failure to allow fallback")
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

func TestFetchToolUsesTavilyWhenAPIKeyIsPresent(t *testing.T) {
	t.Parallel()

	tool := NewFetchTool()
	tool.apiKey = "test-key"
	tool.tavilyEndpoint = "https://api.tavily.example"
	tool.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPost {
				t.Fatalf("expected Tavily POST request, got %s", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("expected bearer auth, got %q", got)
			}
			if r.URL.String() != "https://api.tavily.example/extract" {
				t.Fatalf("unexpected Tavily URL: %s", r.URL.String())
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

	if result.Content["url"] != "https://example.com/weather" {
		t.Fatalf("expected url to stay at input URL, got %#v", result.Content)
	}
	if result.Content["final_url"] != "https://example.com/weather" {
		t.Fatalf("expected final_url to stay at input URL, got %#v", result.Content)
	}
	if result.Content["status_code"] != http.StatusOK {
		t.Fatalf("expected 200 status for Tavily result, got %#v", result.Content)
	}
	if result.Content["content"] != "Today is cloudy and 22C." {
		t.Fatalf("expected Tavily extracted content, got %#v", result.Content)
	}
	if result.Content["truncated"] != false {
		t.Fatalf("expected untruncated content, got %#v", result.Content)
	}
}

func TestFetchToolFallsBackToDirectFetchWhenTavilyRateLimited(t *testing.T) {
	t.Parallel()

	var seen []string
	tool := NewFetchTool()
	tool.apiKey = "test-key"
	tool.tavilyEndpoint = "https://api.tavily.example"
	tool.resolver = staticResolver{addresses: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}}
	tool.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			seen = append(seen, r.Method+" "+r.URL.String())
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
	if len(seen) == 0 || seen[0] != "POST https://api.tavily.example/extract" {
		t.Fatalf("expected Tavily request first, got %#v", seen)
	}
}

func TestFetchToolFallsBackToDirectFetchWhenTavilyReturnsNoResults(t *testing.T) {
	t.Parallel()

	var seen []string
	tool := NewFetchTool()
	tool.apiKey = "test-key"
	tool.tavilyEndpoint = "https://api.tavily.example"
	tool.resolver = staticResolver{addresses: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}}
	tool.client = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			seen = append(seen, r.Method+" "+r.URL.String())
			switch r.URL.String() {
			case "https://api.tavily.example/extract":
				return stringResponse(http.StatusOK, `{"results":[]}`)
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
	if len(seen) == 0 || seen[0] != "POST https://api.tavily.example/extract" {
		t.Fatalf("expected Tavily request first, got %#v", seen)
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
		t.Fatalf("expected no outbound call for restricted target")
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
