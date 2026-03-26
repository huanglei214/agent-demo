package agui

type Event struct {
	Type       string         `json:"type"`
	ThreadID   string         `json:"threadId,omitempty"`
	RunID      string         `json:"runId,omitempty"`
	MessageID  string         `json:"messageId,omitempty"`
	Role       string         `json:"role,omitempty"`
	Delta      string         `json:"delta,omitempty"`
	StepID     string         `json:"stepId,omitempty"`
	StepName   string         `json:"stepName,omitempty"`
	ToolCallID string         `json:"toolCallId,omitempty"`
	ToolName   string         `json:"toolName,omitempty"`
	Args       any            `json:"args,omitempty"`
	Result     any            `json:"result,omitempty"`
	Messages   []Message      `json:"messages,omitempty"`
	Snapshot   map[string]any `json:"snapshot,omitempty"`
	Name       string         `json:"name,omitempty"`
	Value      any            `json:"value,omitempty"`
	Error      string         `json:"error,omitempty"`
}

type Message struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
}
