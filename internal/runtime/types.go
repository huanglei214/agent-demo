package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"
)

type RunStatus string

type RunRole string

type PlanMode string

type TodoStatus string

const (
	RunPending   RunStatus = "pending"
	RunRunning   RunStatus = "running"
	RunBlocked   RunStatus = "blocked"
	RunCompleted RunStatus = "completed"
	RunFailed    RunStatus = "failed"
	RunCancelled RunStatus = "cancelled"
)

const (
	RunRoleLead     RunRole = "lead-agent"
	RunRoleSubagent RunRole = "subagent"
)

const (
	PlanModeNone PlanMode = "none"
	PlanModeTodo PlanMode = "todo"
)

const (
	TodoPending    TodoStatus = "pending"
	TodoInProgress TodoStatus = "in_progress"
	TodoDone       TodoStatus = "done"
)

type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepCompleted StepStatus = "completed"
	StepBlocked   StepStatus = "blocked"
	StepFailed    StepStatus = "failed"
	StepCancelled StepStatus = "cancelled"
)

type Task struct {
	ID          string            `json:"id"`
	Instruction string            `json:"instruction"`
	Workspace   string            `json:"workspace"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

type Session struct {
	ID        string    `json:"id"`
	Workspace string    `json:"workspace"`
	ParentID  string    `json:"parent_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
)

type SessionMessage struct {
	ID        string      `json:"id"`
	SessionID string      `json:"session_id"`
	RunID     string      `json:"run_id"`
	Role      MessageRole `json:"role"`
	Content   string      `json:"content"`
	CreatedAt time.Time   `json:"created_at"`
}

type Run struct {
	ID            string    `json:"id"`
	TaskID        string    `json:"task_id"`
	SessionID     string    `json:"session_id"`
	ParentRunID   string    `json:"parent_run_id,omitempty"`
	Role          RunRole   `json:"role,omitempty"`
	PlanMode      PlanMode  `json:"plan_mode"`
	Status        RunStatus `json:"status"`
	CurrentStepID string    `json:"current_step_id,omitempty"`
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	MaxTurns      int       `json:"max_turns"`
	TurnCount     int       `json:"turn_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	CompletedAt   time.Time `json:"completed_at,omitempty"`
}

func (r *Run) UnmarshalJSON(data []byte) error {
	type runAlias Run

	aux := runAlias{
		PlanMode: PlanModeNone,
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*r = Run(aux)
	if r.PlanMode == "" {
		r.PlanMode = PlanModeNone
	}
	return nil
}

type Event struct {
	ID        string         `json:"id"`
	RunID     string         `json:"run_id"`
	SessionID string         `json:"session_id"`
	TaskID    string         `json:"task_id"`
	Sequence  int64          `json:"sequence"`
	Type      string         `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Actor     string         `json:"actor"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type Plan struct {
	ID        string     `json:"id"`
	RunID     string     `json:"run_id"`
	Goal      string     `json:"goal"`
	Steps     []PlanStep `json:"steps"`
	Version   int        `json:"version"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type PlanStep struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	Status          StepStatus `json:"status"`
	Delegatable     bool       `json:"delegatable"`
	EstimatedCost   string     `json:"estimated_cost,omitempty"`
	EstimatedEffort string     `json:"estimated_effort,omitempty"`
	Dependencies    []string   `json:"dependencies,omitempty"`
	OutputSchema    string     `json:"output_schema_hint,omitempty"`
}

type TodoItem struct {
	ID        string     `json:"id"`
	Content   string     `json:"content"`
	Status    TodoStatus `json:"status"`
	Priority  int        `json:"priority,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type Summary struct {
	ID        string    `json:"id"`
	RunID     string    `json:"run_id"`
	Scope     string    `json:"scope"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type MemoryEntry struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	Scope       string    `json:"scope"`
	Kind        string    `json:"kind"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags,omitempty"`
	SourceRunID string    `json:"source_run_id"`
	CreatedAt   time.Time `json:"created_at"`
}

type MemoryCandidate struct {
	Kind        string    `json:"kind"`
	Scope       string    `json:"scope"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags,omitempty"`
	SourceRunID string    `json:"source_run_id"`
	CreatedAt   time.Time `json:"created_at"`
}

type RunMemories struct {
	RunID      string            `json:"run_id"`
	Recalled   []MemoryEntry     `json:"recalled,omitempty"`
	Candidates []MemoryCandidate `json:"candidates,omitempty"`
	Committed  []MemoryEntry     `json:"committed,omitempty"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

type DelegationTask struct {
	ID                 string    `json:"id"`
	ParentRunID        string    `json:"parent_run_id"`
	SessionID          string    `json:"session_id"`
	PlanStepID         string    `json:"plan_step_id"`
	Role               RunRole   `json:"role,omitempty"`
	Goal               string    `json:"goal"`
	AllowedTools       []string  `json:"allowed_tools,omitempty"`
	StepTitle          string    `json:"step_title,omitempty"`
	StepDesc           string    `json:"step_description,omitempty"`
	Constraints        []string  `json:"constraints,omitempty"`
	CompletionCriteria []string  `json:"completion_criteria,omitempty"`
	TaskLocalContext   []string  `json:"task_local_context,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

type DelegationResult struct {
	ChildRunID      string               `json:"child_run_id"`
	Summary         string               `json:"summary"`
	Artifacts       []DelegationArtifact `json:"artifacts"`
	Findings        []string             `json:"findings"`
	Risks           []string             `json:"risks"`
	Recommendations []string             `json:"recommendations"`
	NeedsReplan     bool                 `json:"needs_replan"`
}

type DelegationArtifact struct {
	Value string         `json:"value,omitempty"`
	Name  string         `json:"name,omitempty"`
	Path  string         `json:"path,omitempty"`
	URL   string         `json:"url,omitempty"`
	Extra map[string]any `json:"extra,omitempty"`
}

type RunState struct {
	RunID              string            `json:"run_id"`
	CurrentStepID      string            `json:"current_step_id,omitempty"`
	TurnCount          int               `json:"turn_count"`
	StepResults        map[string]string `json:"step_results,omitempty"`
	LastEventID        string            `json:"last_event_id,omitempty"`
	ResumePhase        string            `json:"resume_phase,omitempty"`
	PendingToolName    string            `json:"pending_tool_name,omitempty"`
	PendingToolResult  map[string]any    `json:"pending_tool_result,omitempty"`
	PendingToolResults []ToolCallResult  `json:"pending_tool_results,omitempty"`
	Todos              []TodoItem        `json:"todos,omitempty"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

type RunResult struct {
	RunID       string    `json:"run_id"`
	Status      RunStatus `json:"status"`
	Output      string    `json:"output"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

type ToolCallResult struct {
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Tool       string         `json:"tool"`
	Input      map[string]any `json:"input,omitempty"`
	Result     map[string]any `json:"result,omitempty"`
}

type ModelCall struct {
	ID        string                 `json:"id"`
	RunID     string                 `json:"run_id"`
	Sequence  int64                  `json:"sequence"`
	Phase     string                 `json:"phase,omitempty"`
	Tool      string                 `json:"tool,omitempty"`
	Request   ModelRequestSnapshot   `json:"request"`
	Response  *ModelResponseSnapshot `json:"response,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

type ModelRequestSnapshot struct {
	SystemPrompt string         `json:"system_prompt"`
	Input        string         `json:"input"`
	Provider     string         `json:"provider,omitempty"`
	Model        string         `json:"model,omitempty"`
	Messages     []ModelMessage `json:"messages,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type ModelMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ModelResponseSnapshot struct {
	Text         string         `json:"text"`
	FinishReason string         `json:"finish_reason"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

func NewID(prefix string) string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_" + time.Now().Format("20060102150405")
	}

	return prefix + "_" + time.Now().Format("20060102150405") + "_" + hex.EncodeToString(buf)
}

func MustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}
