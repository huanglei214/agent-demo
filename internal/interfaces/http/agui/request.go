package agui

type ChatRequest struct {
	ThreadID string         `json:"threadId"`
	RunID    string         `json:"runId,omitempty"`
	Messages []InputMessage `json:"messages"`
	State    RequestState   `json:"state"`
	Context  map[string]any `json:"context,omitempty"`
}

type InputMessage struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RequestState struct {
	Workspace string `json:"workspace"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	MaxTurns  int    `json:"maxTurns"`
}
