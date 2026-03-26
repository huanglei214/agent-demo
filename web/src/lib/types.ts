export type ApiErrorResponse = {
  error: {
    code: string;
    message: string;
  };
};

export type HealthResponse = {
  ok: boolean;
};

export type Session = {
  id: string;
  workspace: string;
  parent_id?: string;
  created_at: string;
  updated_at: string;
};

export type Task = {
  id: string;
  instruction: string;
  workspace: string;
  created_at: string;
};

export type Run = {
  id: string;
  task_id: string;
  session_id: string;
  parent_run_id?: string;
  status: string;
  current_step_id?: string;
  provider: string;
  model: string;
  max_turns: number;
  turn_count: number;
  created_at: string;
  updated_at: string;
  completed_at?: string;
};

export type RunResult = {
  run_id: string;
  status: string;
  output: string;
  completed_at?: string;
};

export type PlanStep = {
  id: string;
  title: string;
  description: string;
  status: string;
  delegatable: boolean;
};

export type Plan = {
  id: string;
  run_id: string;
  goal: string;
  version: number;
  steps: PlanStep[];
  created_at: string;
  updated_at: string;
};

export type SessionMessage = {
  id: string;
  session_id: string;
  run_id: string;
  role: "user" | "assistant";
  content: string;
  created_at: string;
};

export type SessionRunSummary = {
  run_id: string;
  status: string;
  current_step_id?: string;
  parent_run_id?: string;
  created_at: string;
  updated_at: string;
};

export type SessionInspectResponse = {
  session: Session;
  messages: SessionMessage[];
  runs: SessionRunSummary[];
};

export type SessionListItem = {
  id: string;
  workspace: string;
  run_count: number;
  created_at: string;
  updated_at: string;
};

export type StartRunRequest = {
  instruction: string;
  workspace?: string;
  provider?: string;
  model?: string;
  max_turns?: number;
  session_id?: string;
};

export type StartRunResponse = {
  task: Task;
  run: Run;
  result?: RunResult;
};

export type CreateSessionResponse = {
  session: Session;
};

export type InspectRunResponse = {
  run: Run;
  plan: Plan;
  state: {
    run_id: string;
    current_step_id?: string;
    turn_count: number;
    resume_phase?: string;
    pending_tool_name?: string;
    pending_tool_result?: Record<string, unknown>;
    updated_at: string;
  };
  result?: RunResult;
  current_step?: PlanStep;
  recent_failure?: ReplayEvent;
  child_runs?: Array<{
    run_id: string;
    status: string;
    summary: string;
    needs_replan: boolean;
    updated_at: string;
  }>;
  event_count: number;
};

export type ReplaySummaryEntry = {
  sequence: number;
  type: string;
  actor: string;
  timestamp: string;
  summary: string;
};

export type ReplaySummaryResponse = {
  entries: ReplaySummaryEntry[];
};

export type ReplayEvent = {
  id: string;
  run_id: string;
  session_id: string;
  task_id: string;
  sequence: number;
  type: string;
  timestamp: string;
  actor: string;
  payload?: Record<string, unknown>;
};

export type ReplayEventsResponse = {
  events: ReplayEvent[];
};

export type RunListItem = {
  id: string;
  session_id: string;
  task_id: string;
  status: string;
  current_step_id?: string;
  instruction?: string;
  provider: string;
  model: string;
  created_at: string;
  updated_at: string;
};

export type ToolDescriptor = {
  name: string;
  description: string;
  access: string;
};

export type ToolsResponse = {
  tools: ToolDescriptor[];
};

export type SessionsResponse = {
  sessions: SessionListItem[];
};

export type RunsResponse = {
  runs: RunListItem[];
};

export type AGUIChatRequest = {
  threadId?: string;
  runId?: string;
  messages: Array<{
    id: string;
    role: "user" | "assistant";
    content: string;
  }>;
  state: {
    workspace?: string;
    provider?: string;
    model?: string;
    maxTurns?: number;
  };
  context?: Record<string, unknown>;
};

export type AGUIMessage = {
  id: string;
  role: "user" | "assistant";
  content: string;
};

export type AGUIEvent = {
  type: string;
  threadId?: string;
  runId?: string;
  messageId?: string;
  role?: string;
  delta?: string;
  stepId?: string;
  stepName?: string;
  toolCallId?: string;
  toolName?: string;
  args?: unknown;
  result?: unknown;
  messages?: AGUIMessage[];
  snapshot?: Record<string, unknown>;
  name?: string;
  value?: unknown;
  error?: string;
};
