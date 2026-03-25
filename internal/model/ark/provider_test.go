package ark

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

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
