package ark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	arksdkmodel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"

	"github.com/huanglei214/agent-demo/internal/config"
	internalmodel "github.com/huanglei214/agent-demo/internal/model"
)

type Provider struct {
	config       config.ModelConfig
	http         *http.Client
	concurrency  chan struct{}
	tokenLimiter *tokenLimiter
	wait         func(context.Context, time.Duration) error
}

type tokenLimiter struct {
	mu            sync.Mutex
	tokensPerSec  float64
	nextAllowedAt time.Time
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
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 90
	}

	provider := Provider{
		config: cfg,
		http: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		wait: sleepWithContext,
	}
	if cfg.Ark.MaxConcurrent > 0 {
		provider.concurrency = make(chan struct{}, cfg.Ark.MaxConcurrent)
	}
	if cfg.Ark.TPM > 0 {
		provider.tokenLimiter = &tokenLimiter{
			tokensPerSec: float64(cfg.Ark.TPM) / 60.0,
		}
	}
	return provider
}

func (p Provider) Generate(ctx context.Context, req internalmodel.Request) (internalmodel.Response, error) {
	if p.config.Ark.APIKey == "" || p.config.Ark.BaseURL == "" || p.config.Ark.ModelID == "" {
		return internalmodel.Response{}, &Error{
			Kind:    ErrorKindConfig,
			Message: "provider is not configured",
		}
	}

	if err := p.acquireConcurrency(ctx); err != nil {
		return internalmodel.Response{}, err
	}
	defer p.releaseConcurrency()

	if err := p.waitForTokenBudget(ctx, estimateRequestTokens(req)); err != nil {
		return internalmodel.Response{}, err
	}

	client := arkruntime.NewClientWithApiKey(
		p.config.Ark.APIKey,
		arkruntime.WithBaseUrl(strings.TrimRight(p.config.Ark.BaseURL, "/")),
		arkruntime.WithHTTPClient(p.http),
	)

	resp, err := client.CreateChatCompletion(ctx, arksdkmodel.CreateChatCompletionRequest{
		Model: p.config.Ark.ModelID,
		Messages: []*arksdkmodel.ChatCompletionMessage{
			{
				Role:    arksdkmodel.ChatMessageRoleSystem,
				Content: textContent(req.SystemPrompt),
			},
			{
				Role:    arksdkmodel.ChatMessageRoleUser,
				Content: textContent(req.Input),
			},
		},
	})
	if err != nil {
		return internalmodel.Response{}, classifySDKError(ctx, err)
	}

	if len(resp.Choices) == 0 {
		return internalmodel.Response{}, &Error{
			Kind:    ErrorKindEmptyChoices,
			Message: "request returned no choices",
		}
	}

	text, err := messageContentToText(resp.Choices[0].Message.Content)
	if err != nil {
		return internalmodel.Response{}, &Error{
			Kind:    ErrorKindDecodeResponse,
			Message: "decode success response",
			Err:     err,
		}
	}

	return internalmodel.Response{
		Text:         text,
		FinishReason: string(resp.Choices[0].FinishReason),
		Metadata: map[string]any{
			"id":      resp.ID,
			"model":   resp.Model,
			"created": resp.Created,
			"usage":   usageMetadata(resp.Usage),
		},
	}, nil
}

func (p Provider) GenerateStream(ctx context.Context, req internalmodel.Request, sink internalmodel.StreamSink) error {
	if p.config.Ark.APIKey == "" || p.config.Ark.BaseURL == "" || p.config.Ark.ModelID == "" {
		return &Error{
			Kind:    ErrorKindConfig,
			Message: "provider is not configured",
		}
	}

	if err := p.acquireConcurrency(ctx); err != nil {
		return err
	}
	defer p.releaseConcurrency()

	if err := p.waitForTokenBudget(ctx, estimateRequestTokens(req)); err != nil {
		return err
	}

	client := arkruntime.NewClientWithApiKey(
		p.config.Ark.APIKey,
		arkruntime.WithBaseUrl(strings.TrimRight(p.config.Ark.BaseURL, "/")),
		arkruntime.WithHTTPClient(p.http),
	)

	stream, err := client.CreateChatCompletionStream(ctx, arksdkmodel.CreateChatCompletionRequest{
		Model: p.config.Ark.ModelID,
		Messages: []*arksdkmodel.ChatCompletionMessage{
			{
				Role:    arksdkmodel.ChatMessageRoleSystem,
				Content: textContent(req.SystemPrompt),
			},
			{
				Role:    arksdkmodel.ChatMessageRoleUser,
				Content: textContent(req.Input),
			},
		},
	})
	if err != nil {
		return classifySDKError(ctx, err)
	}
	defer stream.Close()

	if sink == nil {
		sink = noopStreamSink{}
	}

	var (
		started       bool
		modeKnown     bool
		structured    bool
		emittedAnswer string
		fullResponse  strings.Builder
	)
	for {
		chunk, recvErr := stream.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				break
			}
			if started {
				_ = sink.Fail(recvErr)
			}
			return classifySDKError(ctx, recvErr)
		}
		for _, choice := range chunk.Choices {
			if choice == nil {
				continue
			}
			delta := choice.Delta.Content
			if delta == "" {
				continue
			}
			fullResponse.WriteString(delta)
			if !modeKnown {
				trimmed := strings.TrimSpace(fullResponse.String())
				if trimmed == "" {
					continue
				}
				modeKnown = true
				structured = strings.HasPrefix(trimmed, "{")
				if !structured {
					if err := sink.Start(); err != nil {
						return err
					}
					started = true
				}
			}
			if !structured {
				if err := sink.Delta(delta); err != nil {
					return err
				}
				continue
			}

			action, actionFound, actionComplete := extractJSONStringFieldProgress(fullResponse.String(), "action")
			if !actionFound || !actionComplete || action != "final" {
				continue
			}
			answer, answerFound, _ := extractJSONStringFieldProgress(fullResponse.String(), "answer")
			if !answerFound {
				continue
			}
			if !started {
				if err := sink.Start(); err != nil {
					return err
				}
				started = true
			}
			if len(answer) > len(emittedAnswer) {
				suffix := answer[len(emittedAnswer):]
				if suffix != "" {
					if err := sink.Delta(suffix); err != nil {
						return err
					}
				}
				emittedAnswer = answer
			}
		}
	}

	responseText := fullResponse.String()
	if responseText == "" {
		resp, err := p.Generate(ctx, req)
		if err != nil {
			return err
		}
		responseText = resp.Text
	}

	if !modeKnown || structured {
		text, final := finalAnswerText(responseText)
		if !final {
			return &internalmodel.NonFinalStreamResponseError{Response: internalmodel.Response{Text: responseText, FinishReason: "stop"}}
		}
		if !started {
			if err := sink.Start(); err != nil {
				return err
			}
			started = true
		}
		if len(text) > len(emittedAnswer) {
			suffix := text[len(emittedAnswer):]
			if suffix != "" {
				if err := sink.Delta(suffix); err != nil {
					return err
				}
			}
			emittedAnswer = text
		}
	}
	return sink.Complete()
}

type noopStreamSink struct{}

func (noopStreamSink) Start() error       { return nil }
func (noopStreamSink) Delta(string) error { return nil }
func (noopStreamSink) Complete() error    { return nil }
func (noopStreamSink) Fail(error) error   { return nil }

func extractJSONStringFieldProgress(input, field string) (value string, found bool, complete bool) {
	key := `"` + field + `"`
	idx := strings.Index(input, key)
	if idx < 0 {
		return "", false, false
	}
	i := idx + len(key)
	for i < len(input) {
		switch input[i] {
		case ' ', '\n', '\r', '\t':
			i++
		default:
			goto colon
		}
	}
	return "", true, false

colon:
	if input[i] != ':' {
		return "", true, false
	}
	i++
	for i < len(input) {
		switch input[i] {
		case ' ', '\n', '\r', '\t':
			i++
		default:
			goto value
		}
	}
	return "", true, false

value:
	if i >= len(input) || input[i] != '"' {
		return "", true, false
	}
	i++

	var builder strings.Builder
	for i < len(input) {
		ch := input[i]
		if ch == '"' {
			return builder.String(), true, true
		}
		if ch != '\\' {
			builder.WriteByte(ch)
			i++
			continue
		}
		if i+1 >= len(input) {
			return builder.String(), true, false
		}
		esc := input[i+1]
		switch esc {
		case '"', '\\', '/':
			builder.WriteByte(esc)
			i += 2
		case 'b':
			builder.WriteByte('\b')
			i += 2
		case 'f':
			builder.WriteByte('\f')
			i += 2
		case 'n':
			builder.WriteByte('\n')
			i += 2
		case 'r':
			builder.WriteByte('\r')
			i += 2
		case 't':
			builder.WriteByte('\t')
			i += 2
		case 'u':
			if i+6 > len(input) {
				return builder.String(), true, false
			}
			codepoint, err := strconv.ParseUint(input[i+2:i+6], 16, 64)
			if err != nil {
				return builder.String(), true, false
			}
			builder.WriteRune(rune(codepoint))
			i += 6
		default:
			return builder.String(), true, false
		}
	}
	return builder.String(), true, false
}

func finalAnswerText(responseText string) (string, bool) {
	var action internalmodel.Action
	if err := json.Unmarshal([]byte(responseText), &action); err != nil {
		return responseText, true
	}
	if action.Action != "final" || strings.TrimSpace(action.Answer) == "" {
		return responseText, false
	}
	return action.Answer, true
}

func (p Provider) acquireConcurrency(ctx context.Context) error {
	if p.concurrency == nil {
		return nil
	}
	select {
	case p.concurrency <- struct{}{}:
		return nil
	case <-ctx.Done():
		return classifyTransportError(ctx, ctx.Err())
	}
}

func (p Provider) releaseConcurrency() {
	if p.concurrency == nil {
		return
	}
	select {
	case <-p.concurrency:
	default:
	}
}

func (p Provider) waitForTokenBudget(ctx context.Context, tokens int) error {
	if p.tokenLimiter == nil || tokens <= 0 {
		return nil
	}
	delay := p.tokenLimiter.reserve(tokens)
	if delay <= 0 {
		return nil
	}
	return p.wait(ctx, delay)
}

func (l *tokenLimiter) reserve(tokens int) time.Duration {
	if l == nil || tokens <= 0 || l.tokensPerSec <= 0 {
		return 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if l.nextAllowedAt.Before(now) {
		l.nextAllowedAt = now
	}
	start := l.nextAllowedAt
	seconds := float64(tokens) / l.tokensPerSec
	l.nextAllowedAt = l.nextAllowedAt.Add(time.Duration(seconds * float64(time.Second)))
	if start.After(now) {
		return start.Sub(now)
	}
	return 0
}

func estimateRequestTokens(req internalmodel.Request) int {
	chars := len(req.SystemPrompt) + len(req.Input)
	if chars <= 0 {
		return 1
	}
	tokens := chars / 4
	if chars%4 != 0 {
		tokens += 1
	}
	if tokens < 1 {
		return 1
	}
	return tokens
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return classifyTransportError(ctx, ctx.Err())
	}
}

func classifySDKError(ctx context.Context, err error) error {
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
	}

	var apiErr *arksdkmodel.APIError
	if errors.As(err, &apiErr) {
		return &Error{
			Kind:       ErrorKindHTTPStatus,
			StatusCode: apiErr.HTTPStatusCode,
			Message:    strings.TrimSpace(apiErr.Message),
			Err:        err,
		}
	}

	var requestErr *arksdkmodel.RequestError
	if errors.As(err, &requestErr) {
		switch {
		case errors.Is(requestErr.Err, context.DeadlineExceeded):
			return &Error{
				Kind:    ErrorKindTimeout,
				Message: "request timed out",
				Err:     err,
			}
		case errors.Is(requestErr.Err, context.Canceled):
			return &Error{
				Kind:    ErrorKindCanceled,
				Message: "request was canceled",
				Err:     err,
			}
		case requestErr.HTTPStatusCode >= 400:
			return &Error{
				Kind:       ErrorKindHTTPStatus,
				StatusCode: requestErr.HTTPStatusCode,
				Message:    requestErrorMessage(requestErr),
				Err:        err,
			}
		}
	}

	return classifyTransportError(ctx, err)
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

func requestErrorMessage(err *arksdkmodel.RequestError) string {
	if err == nil {
		return ""
	}

	if err.Err != nil {
		return strings.TrimSpace(err.Err.Error())
	}
	return "request failed"
}

func textContent(text string) *arksdkmodel.ChatCompletionMessageContent {
	return &arksdkmodel.ChatCompletionMessageContent{
		StringValue: &text,
	}
}

func messageContentToText(content *arksdkmodel.ChatCompletionMessageContent) (string, error) {
	if content == nil {
		return "", errors.New("response choice content is empty")
	}
	if content.StringValue != nil {
		return *content.StringValue, nil
	}
	if len(content.ListValue) == 0 {
		return "", errors.New("response choice content is empty")
	}

	var parts []string
	for _, part := range content.ListValue {
		if part == nil {
			continue
		}
		if strings.TrimSpace(part.Text) == "" {
			continue
		}
		parts = append(parts, part.Text)
	}
	if len(parts) == 0 {
		return "", errors.New("response choice content is empty")
	}
	return strings.Join(parts, "\n"), nil
}

func usageMetadata(usage arksdkmodel.Usage) map[string]any {
	return map[string]any{
		"prompt_tokens":     usage.PromptTokens,
		"completion_tokens": usage.CompletionTokens,
		"total_tokens":      usage.TotalTokens,
		"prompt_tokens_details": map[string]any{
			"cached_tokens":      usage.PromptTokensDetails.CachedTokens,
			"provisioned_tokens": usage.PromptTokensDetails.ProvisionedTokens,
		},
		"completion_tokens_details": map[string]any{
			"reasoning_tokens":   usage.CompletionTokensDetails.ReasoningTokens,
			"provisioned_tokens": usage.CompletionTokensDetails.ProvisionedTokens,
		},
	}
}
