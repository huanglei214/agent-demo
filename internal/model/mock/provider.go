package mock

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/huanglei214/agent-demo/internal/model"
)

type Provider struct{}

func New() Provider {
	return Provider{}
}

func (p Provider) Generate(ctx context.Context, req model.Request) (model.Response, error) {
	_ = ctx

	lower := strings.ToLower(req.Input)
	role, _ := req.Metadata["role"].(string)
	if role == "subagent" {
		needsReplan := strings.Contains(lower, "replan") || strings.Contains(req.Input, "重规划") || strings.Contains(req.Input, "重新规划")
		return encodeAction(model.Action{
			Action: "final",
			Answer: delegationJSON(needsReplan),
		}), nil
	}
	if strings.Contains(req.Input, "Tool result:") || strings.Contains(req.Input, "New tool results:") {
		return encodeAction(model.Action{
			Action: "final",
			Answer: "I used the requested tool and summarized the result successfully.",
		}), nil
	}

	switch {
	case strings.Contains(lower, "delegate") || strings.Contains(lower, "委派") || strings.Contains(lower, "子任务"):
		return encodeAction(model.Action{
			Action:         "delegate",
			DelegationGoal: "Inspect the repository files needed for the requested task and return a concise structured summary.",
		}), nil
	case strings.Contains(lower, "readme"):
		return encodeAction(model.Action{
			Action: "tool",
			Calls: []model.ToolCall{{
				Tool: "fs.read_file",
				Input: map[string]any{
					"path": "README.md",
				},
			}},
		}), nil
	case strings.Contains(lower, "列出") || strings.Contains(lower, "list"):
		return encodeAction(model.Action{
			Action: "tool",
			Calls: []model.ToolCall{{
				Tool: "fs.list_dir",
				Input: map[string]any{
					"path": ".",
				},
			}},
		}), nil
	default:
		return encodeAction(model.Action{
			Action: "final",
			Answer: "mock response: " + req.Input,
		}), nil
	}
}

func (p Provider) GenerateStream(ctx context.Context, req model.Request, sink model.StreamSink) error {
	resp, err := p.Generate(ctx, req)
	if err != nil {
		if sink != nil {
			_ = sink.Fail(err)
		}
		return err
	}

	if sink == nil {
		return nil
	}
	if err := sink.Start(); err != nil {
		return err
	}

	text := finalAnswerText(resp.Text)
	for _, chunk := range splitStreamChunks(text) {
		if err := sink.Delta(chunk); err != nil {
			return err
		}
	}
	if err := sink.Complete(); err != nil {
		return err
	}
	return nil
}

func finalAnswerText(responseText string) string {
	var action model.Action
	if err := json.Unmarshal([]byte(responseText), &action); err != nil {
		return responseText
	}
	if action.Action != "final" || strings.TrimSpace(action.Answer) == "" {
		return responseText
	}
	return action.Answer
}

func splitStreamChunks(text string) []string {
	if text == "" {
		return nil
	}
	if strings.Contains(text, ", ") {
		parts := strings.SplitN(text, ", ", 2)
		if len(parts) == 2 {
			return []string{parts[0], ", ", parts[1]}
		}
	}
	if strings.Contains(text, " ") {
		parts := strings.Split(text, " ")
		chunks := make([]string, 0, len(parts))
		for i, part := range parts {
			if i > 0 {
				chunks = append(chunks, " ")
			}
			if part != "" {
				chunks = append(chunks, part)
			}
		}
		if len(chunks) > 0 {
			return chunks
		}
	}
	return []string{text}
}

func encodeAction(action model.Action) model.Response {
	data, _ := json.Marshal(action)
	return model.Response{
		Text:         string(data),
		FinishReason: "stop",
	}
}

func delegationJSON(needsReplan bool) string {
	payload := map[string]any{
		"summary":         "Delegated child analysis completed successfully.",
		"artifacts":       []string{},
		"findings":        []string{"Child run inspected the bounded task context."},
		"risks":           []string{},
		"recommendations": []string{},
		"needs_replan":    needsReplan,
	}
	data, _ := json.Marshal(payload)
	return string(data)
}
