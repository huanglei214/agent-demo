package ark

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/huanglei214/agent-demo/internal/config"
	"github.com/huanglei214/agent-demo/internal/model"
)

type Provider struct {
	config config.ModelConfig
	http   *http.Client
}

type ErrorKind string

const (
	ErrorKindConfig         ErrorKind = "config"
	ErrorKindRequestBuild   ErrorKind = "request_build"
	ErrorKindTimeout        ErrorKind = "timeout"
	ErrorKindCanceled       ErrorKind = "canceled"
	ErrorKindNetwork        ErrorKind = "network"
	ErrorKindHTTPStatus     ErrorKind = "http_status"
	ErrorKindDecodeResponse ErrorKind = "decode_response"
	ErrorKindEmptyChoices   ErrorKind = "empty_choices"
)

type Error struct {
	Kind       ErrorKind
	StatusCode int
	Message    string
	Err        error
}

func (e *Error) Error() string {
	switch {
	case e == nil:
		return ""
	case e.Message == "" && e.Err != nil:
		return fmt.Sprintf("ark %s error: %v", e.Kind, e.Err)
	case e.StatusCode > 0:
		return fmt.Sprintf("ark %s error (status %d): %s", e.Kind, e.StatusCode, e.Message)
	default:
		return fmt.Sprintf("ark %s error: %s", e.Kind, e.Message)
	}
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *Error) FailureKind() string {
	if e == nil {
		return "ark_unknown"
	}
	return "ark_" + string(e.Kind)
}

func (e *Error) Retryable() bool {
	if e == nil {
		return false
	}
	switch e.Kind {
	case ErrorKindTimeout, ErrorKindCanceled, ErrorKindNetwork:
		return true
	case ErrorKindHTTPStatus:
		return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= http.StatusInternalServerError
	default:
		return false
	}
}

func New(cfg config.ModelConfig) Provider {
	return Provider{
		config: cfg,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (p Provider) Generate(ctx context.Context, req model.Request) (model.Response, error) {
	if p.config.Ark.APIKey == "" || p.config.Ark.BaseURL == "" || p.config.Ark.ModelID == "" {
		return model.Response{}, &Error{
			Kind:    ErrorKindConfig,
			Message: "provider is not configured",
		}
	}

	requestBody := chatCompletionRequest{
		Model: p.config.Ark.ModelID,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: req.SystemPrompt,
			},
			{
				Role:    "user",
				Content: req.Input,
			},
		},
		Stream: false,
	}

	data, err := json.Marshal(requestBody)
	if err != nil {
		return model.Response{}, &Error{
			Kind:    ErrorKindRequestBuild,
			Message: "marshal request body",
			Err:     err,
		}
	}

	url := strings.TrimRight(p.config.Ark.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return model.Response{}, &Error{
			Kind:    ErrorKindRequestBuild,
			Message: "create request",
			Err:     err,
		}
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.config.Ark.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.http.Do(httpReq)
	if err != nil {
		return model.Response{}, classifyTransportError(ctx, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.Response{}, &Error{
			Kind:    ErrorKindDecodeResponse,
			Message: "read response body",
			Err:     err,
		}
	}

	if resp.StatusCode >= 400 {
		return model.Response{}, &Error{
			Kind:       ErrorKindHTTPStatus,
			StatusCode: resp.StatusCode,
			Message:    extractErrorMessage(body, resp.Status),
		}
	}

	var decoded chatCompletionResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return model.Response{}, &Error{
			Kind:    ErrorKindDecodeResponse,
			Message: "decode success response",
			Err:     err,
		}
	}

	if len(decoded.Choices) == 0 {
		return model.Response{}, &Error{
			Kind:    ErrorKindEmptyChoices,
			Message: "request returned no choices",
		}
	}

	return model.Response{
		Text:         decoded.Choices[0].Message.Content,
		FinishReason: decoded.Choices[0].FinishReason,
		Metadata: map[string]any{
			"id":      decoded.ID,
			"model":   decoded.Model,
			"created": decoded.Created,
			"usage":   decoded.Usage,
		},
	}, nil
}

func classifyTransportError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(ctx.Err(), context.DeadlineExceeded):
		return &Error{
			Kind:    ErrorKindTimeout,
			Message: "request timed out",
			Err:     err,
		}
	case errors.Is(err, context.Canceled), errors.Is(ctx.Err(), context.Canceled):
		return &Error{
			Kind:    ErrorKindCanceled,
			Message: "request was canceled",
			Err:     err,
		}
	default:
		return &Error{
			Kind:    ErrorKindNetwork,
			Message: "request failed",
			Err:     err,
		}
	}
}

func extractErrorMessage(body []byte, fallback string) string {
	var decoded chatCompletionResponse
	if err := json.Unmarshal(body, &decoded); err == nil && strings.TrimSpace(decoded.Error.Message) != "" {
		return decoded.Error.Message
	}

	trimmed := strings.TrimSpace(string(body))
	if trimmed != "" {
		return trimmed
	}
	return fallback
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	ID      string                   `json:"id"`
	Model   string                   `json:"model"`
	Created int64                    `json:"created"`
	Choices []chatCompletionChoice   `json:"choices"`
	Usage   map[string]any           `json:"usage"`
	Error   chatCompletionErrorField `json:"error"`
}

type chatCompletionChoice struct {
	FinishReason string      `json:"finish_reason"`
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
}

type chatCompletionErrorField struct {
	Message string `json:"message"`
}
