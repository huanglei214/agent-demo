package ark

import (
	"context"
	"encoding/json"
	"fmt"
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
		return model.Response{}, fmt.Errorf("ark provider is not configured")
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
		return model.Response{}, err
	}

	url := strings.TrimRight(p.config.Ark.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(data)))
	if err != nil {
		return model.Response{}, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.config.Ark.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.http.Do(httpReq)
	if err != nil {
		return model.Response{}, err
	}
	defer resp.Body.Close()

	var decoded chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return model.Response{}, err
	}

	if resp.StatusCode >= 400 {
		return model.Response{}, fmt.Errorf("ark request failed: %s", decoded.Error.Message)
	}

	if len(decoded.Choices) == 0 {
		return model.Response{}, fmt.Errorf("ark request returned no choices")
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
