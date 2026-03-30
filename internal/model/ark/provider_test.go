package ark

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/huanglei214/agent-demo/internal/config"
	"github.com/huanglei214/agent-demo/internal/model"
)

func TestGenerateSuccess(t *testing.T) {
	t.Parallel()

	provider := New(config.ModelConfig{
		Ark: config.ArkConfig{
			APIKey:  "test-key",
			BaseURL: "https://ark.example.com",
			ModelID: "ark-test",
		},
	})
	provider.http = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/chat/completions" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("unexpected auth header: %s", got)
			}
			return jsonResponse(http.StatusOK, map[string]any{
				"id":      "resp_1",
				"model":   "ark-test",
				"created": 123,
				"choices": []map[string]any{
					{
						"finish_reason": "stop",
						"index":         0,
						"message": map[string]any{
							"role":    "assistant",
							"content": `{"action":"final","answer":"hello"}`,
						},
					},
				},
				"usage": map[string]any{
					"prompt_tokens":     10,
					"completion_tokens": 5,
				},
			})
		}),
	}

	resp, err := provider.Generate(context.Background(), model.Request{
		SystemPrompt: "system",
		Input:        "hello",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if resp.FinishReason != "stop" || resp.Text == "" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestGenerateStreamFallsBackToSingleDelta(t *testing.T) {
	t.Parallel()

	provider := New(config.ModelConfig{
		Ark: config.ArkConfig{
			APIKey:  "test-key",
			BaseURL: "https://ark.example.com",
			ModelID: "ark-test",
		},
	})
	provider.http = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":      "resp_1",
				"model":   "ark-test",
				"created": 123,
				"choices": []map[string]any{{
					"finish_reason": "stop",
					"index":         0,
					"message": map[string]any{
						"role":    "assistant",
						"content": `{"action":"final","answer":"hello"}`,
					},
				}},
			})
		}),
	}

	sink := &capturingStreamSink{}
	if err := provider.GenerateStream(context.Background(), model.Request{Input: "hello"}, sink); err != nil {
		t.Fatalf("generate stream: %v", err)
	}
	if sink.started != 1 || sink.completed != 1 || sink.failed != 0 {
		t.Fatalf("unexpected sink lifecycle: %#v", sink)
	}
	if got := sink.deltas; len(got) != 1 || got[0] != `{"action":"final","answer":"hello"}` {
		t.Fatalf("expected single fallback delta with final text, got %#v", got)
	}
}

type capturingStreamSink struct {
	started   int
	completed int
	failed    int
	deltas    []string
}

func (s *capturingStreamSink) Start() error {
	s.started++
	return nil
}

func (s *capturingStreamSink) Delta(text string) error {
	s.deltas = append(s.deltas, text)
	return nil
}

func (s *capturingStreamSink) Complete() error {
	s.completed++
	return nil
}

func (s *capturingStreamSink) Fail(err error) error {
	_ = err
	s.failed++
	return nil
}

func TestGenerateReturnsErrorOnBadStatus(t *testing.T) {
	t.Parallel()

	provider := New(config.ModelConfig{
		Ark: config.ArkConfig{
			APIKey:  "test-key",
			BaseURL: "https://ark.example.com",
			ModelID: "ark-test",
		},
	})
	provider.http = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusBadRequest, map[string]any{
				"error": map[string]any{
					"message": "bad request",
				},
			})
		}),
	}

	if _, err := provider.Generate(context.Background(), model.Request{
		SystemPrompt: "system",
		Input:        "hello",
	}); err == nil {
		t.Fatal("expected bad status error")
	} else {
		var arkErr *Error
		if !errors.As(err, &arkErr) {
			t.Fatalf("expected typed ark error, got %T", err)
		}
		if arkErr.Kind != ErrorKindHTTPStatus || arkErr.StatusCode != http.StatusBadRequest {
			t.Fatalf("unexpected ark error: %#v", arkErr)
		}
		if arkErr.Retryable() {
			t.Fatalf("expected 400 error to be non-retryable, got %#v", arkErr)
		}
	}
}

func TestGenerateReturnsErrorWhenChoicesMissing(t *testing.T) {
	t.Parallel()

	provider := New(config.ModelConfig{
		Ark: config.ArkConfig{
			APIKey:  "test-key",
			BaseURL: "https://ark.example.com",
			ModelID: "ark-test",
		},
	})
	provider.http = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":      "resp_1",
				"model":   "ark-test",
				"created": 123,
				"choices": []map[string]any{},
			})
		}),
	}

	if _, err := provider.Generate(context.Background(), model.Request{
		SystemPrompt: "system",
		Input:        "hello",
	}); err == nil {
		t.Fatal("expected missing choices error")
	} else {
		var arkErr *Error
		if !errors.As(err, &arkErr) {
			t.Fatalf("expected typed ark error, got %T", err)
		}
		if arkErr.Kind != ErrorKindEmptyChoices {
			t.Fatalf("unexpected ark error: %#v", arkErr)
		}
	}
}

func TestGenerateClassifiesTimeoutErrors(t *testing.T) {
	t.Parallel()

	provider := New(config.ModelConfig{
		Ark: config.ArkConfig{
			APIKey:  "test-key",
			BaseURL: "https://ark.example.com",
			ModelID: "ark-test",
		},
	})
	provider.http = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		}),
	}

	if _, err := provider.Generate(context.Background(), model.Request{
		SystemPrompt: "system",
		Input:        "hello",
	}); err == nil {
		t.Fatal("expected timeout error")
	} else {
		var arkErr *Error
		if !errors.As(err, &arkErr) {
			t.Fatalf("expected typed ark error, got %T", err)
		}
		if arkErr.Kind != ErrorKindTimeout || !arkErr.Retryable() {
			t.Fatalf("unexpected ark error: %#v", arkErr)
		}
	}
}

func TestNewUsesConfiguredHTTPTimeout(t *testing.T) {
	t.Parallel()

	provider := New(config.ModelConfig{
		TimeoutSeconds: 135,
		Ark: config.ArkConfig{
			APIKey:  "test-key",
			BaseURL: "https://ark.example.com",
			ModelID: "ark-test",
		},
	})

	if provider.http.Timeout != 135*time.Second {
		t.Fatalf("expected configured timeout 135s, got %s", provider.http.Timeout)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(statusCode int, body map[string]any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(data)),
	}, nil
}

func TestGenerateWaitsForTokenBudgetBeforeSendingRequest(t *testing.T) {
	t.Parallel()

	provider := New(config.ModelConfig{
		Ark: config.ArkConfig{
			APIKey:        "test-key",
			BaseURL:       "https://ark.example.com",
			ModelID:       "ark-test",
			TPM:           120,
			MaxConcurrent: 1,
		},
	})
	provider.tokenLimiter.nextAllowedAt = time.Now().Add(10 * time.Second)

	var waited time.Duration
	provider.wait = func(ctx context.Context, delay time.Duration) error {
		waited = delay
		return nil
	}
	provider.http = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, map[string]any{
				"id":      "resp_1",
				"model":   "ark-test",
				"created": 123,
				"choices": []map[string]any{{
					"finish_reason": "stop",
					"index":         0,
					"message": map[string]any{
						"role":    "assistant",
						"content": `{"action":"final","answer":"hello"}`,
					},
				}},
			})
		}),
	}

	_, err := provider.Generate(context.Background(), model.Request{
		SystemPrompt: strings.Repeat("s", 12),
		Input:        strings.Repeat("i", 12),
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if waited < 9*time.Second {
		t.Fatalf("expected token limiter wait close to 10s, got %s", waited)
	}
}

func TestGenerateLimitsConcurrentRequests(t *testing.T) {
	t.Parallel()

	provider := New(config.ModelConfig{
		Ark: config.ArkConfig{
			APIKey:        "test-key",
			BaseURL:       "https://ark.example.com",
			ModelID:       "ark-test",
			MaxConcurrent: 1,
		},
	})

	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	provider.http = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			entered <- struct{}{}
			<-release
			return jsonResponse(http.StatusOK, map[string]any{
				"id":      "resp_1",
				"model":   "ark-test",
				"created": 123,
				"choices": []map[string]any{{
					"finish_reason": "stop",
					"index":         0,
					"message": map[string]any{
						"role":    "assistant",
						"content": `{"action":"final","answer":"hello"}`,
					},
				}},
			})
		}),
	}

	errCh := make(chan error, 2)
	request := model.Request{SystemPrompt: "system", Input: "hello"}
	for range 2 {
		go func() {
			_, err := provider.Generate(context.Background(), request)
			errCh <- err
		}()
	}

	<-entered
	select {
	case <-entered:
		t.Fatal("expected second request to wait for concurrency slot")
	case <-time.After(100 * time.Millisecond):
	}

	release <- struct{}{}
	<-entered
	release <- struct{}{}

	for range 2 {
		if err := <-errCh; err != nil {
			t.Fatalf("generate: %v", err)
		}
	}
}
