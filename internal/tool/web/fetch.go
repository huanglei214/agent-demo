package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/huanglei214/agent-demo/internal/tool"
)

type FetchTool struct {
	client         *http.Client
	resolver       ipResolver
	tavilyEndpoint string
	apiKey         string
}

var titlePattern = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

type ipResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

func NewFetchTool() FetchTool {
	return FetchTool{
		client:         &http.Client{Timeout: 15 * time.Second},
		resolver:       net.DefaultResolver,
		tavilyEndpoint: defaultTavilyEndpoint,
		apiKey:         strings.TrimSpace(os.Getenv("TAVILY_API_KEY")),
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
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return tool.Result{}, errors.New("url must be an absolute http(s) URL")
	}
	if err := t.validatePublicURL(ctx, parsed); err != nil {
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

	return t.executeDirectFetch(ctx, target)
}

func (t FetchTool) executeTavilyFetch(ctx context.Context, target string) (tool.Result, bool, error) {
	resp, err := tavilyClient{
		httpClient: t.client,
		endpoint:   t.tavilyEndpoint,
		apiKey:     t.apiKey,
	}.extract(ctx, tavilyExtractRequest{
		URLs: []string{target},
	})
	if err != nil {
		return tool.Result{}, false, err
	}
	if len(resp.Results) == 0 {
		return tool.Result{}, false, errTavilyNoResults
	}

	content, truncated := truncateString(strings.TrimSpace(resp.Results[0].RawContent), maxFetchedContentRunes)
	if content == "" {
		return tool.Result{}, false, errTavilyNoResults
	}

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

func (t FetchTool) executeDirectFetch(ctx context.Context, target string) (tool.Result, error) {
	client := *t.client
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return t.validatePublicURL(req.Context(), req.URL)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return tool.Result{}, err
	}
	httpReq.Header.Set("User-Agent", "agent-demo/1.0 (+https://localhost)")

	resp, err := client.Do(httpReq)
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

func (t FetchTool) validatePublicURL(ctx context.Context, parsed *url.URL) error {
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return errors.New("url must be an absolute http(s) URL")
	}
	if strings.EqualFold(host, "localhost") {
		return errors.New("web.fetch target resolves to a restricted address")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isRestrictedIP(ip) {
			return errors.New("web.fetch target resolves to a restricted address")
		}
		return nil
	}
	addresses, err := t.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return err
	}
	for _, address := range addresses {
		if isRestrictedIP(address.IP) {
			return errors.New("web.fetch target resolves to a restricted address")
		}
	}
	return nil
}

func isRestrictedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified()
}
