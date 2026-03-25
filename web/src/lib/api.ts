import type {
  ApiErrorResponse,
  CreateSessionResponse,
  HealthResponse,
  InspectRunResponse,
  ReplayEventsResponse,
  ReplaySummaryResponse,
  RunsResponse,
  SessionInspectResponse,
  SessionsResponse,
  StartRunRequest,
  StartRunResponse,
  ToolsResponse,
} from "./types";

async function request<T>(input: string, init?: RequestInit): Promise<T> {
  const response = await fetch(input, {
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
    ...init,
  });

  const payload = (await response.json()) as T | ApiErrorResponse;
  if (!response.ok) {
    if (hasApiError(payload)) {
      throw new Error(payload.error.message);
    }
    throw new Error("request failed");
  }
  return payload as T;
}

function hasApiError(value: unknown): value is ApiErrorResponse {
  return typeof value === "object" && value !== null && "error" in value;
}

export function getHealth() {
  return request<HealthResponse>("/healthz");
}

export function createSession() {
  return request<CreateSessionResponse>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({}),
  });
}

export function inspectSession(sessionId: string, recent = 20) {
  return request<SessionInspectResponse>(`/api/sessions/${sessionId}?recent=${recent}`);
}

export function listSessions(limit = 8) {
  return request<SessionsResponse>(`/api/sessions?limit=${limit}`);
}

export function startRun(body: StartRunRequest) {
  return request<StartRunResponse>("/api/runs", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function inspectRun(runId: string) {
  return request<InspectRunResponse>(`/api/runs/${runId}`);
}

export function listRuns(limit = 8) {
  return request<RunsResponse>(`/api/runs?limit=${limit}`);
}

export function replaySummary(runId: string) {
  return request<ReplaySummaryResponse>(`/api/runs/${runId}/replay`);
}

export function replayEvents(runId: string) {
  return request<ReplayEventsResponse>(`/api/runs/${runId}/events`);
}

export function listTools() {
  return request<ToolsResponse>("/api/tools");
}

type RunStreamHandlers = {
  afterSequence?: number;
  onOpen?: () => void;
  onEvent: (event: import("./types").ReplayEvent) => void;
  onDone?: (status: string) => void;
  onError?: () => void;
};

export function streamRun(runId: string, handlers: RunStreamHandlers) {
  const params = new URLSearchParams();
  if (handlers.afterSequence && handlers.afterSequence > 0) {
    params.set("after", String(handlers.afterSequence));
  }
  const url = `/api/runs/${runId}/stream${params.toString() ? `?${params.toString()}` : ""}`;
  const source = new EventSource(url);

  source.onopen = () => {
    handlers.onOpen?.();
  };
  source.addEventListener("runtime.event", (event) => {
    handlers.onEvent(JSON.parse((event as MessageEvent<string>).data));
  });
  source.addEventListener("done", (event) => {
    const payload = JSON.parse((event as MessageEvent<string>).data) as {
      status?: string;
    };
    handlers.onDone?.(payload.status ?? "completed");
    source.close();
  });
  source.onerror = () => {
    handlers.onError?.();
    source.close();
  };

  return source;
}
