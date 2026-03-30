import type {
  AGUIChatRequest,
  AGUIEvent,
  AGUIStreamResult,
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
  return request<SessionInspectResponse>(`/api/sessions/${sessionId}?recent=${recent}`).then(
    (response) => ({
      ...response,
      messages: Array.isArray(response.messages) ? response.messages : [],
      runs: Array.isArray(response.runs) ? response.runs : [],
    }),
  );
}

export function listSessions(limit = 8) {
  return request<SessionsResponse>(`/api/sessions?limit=${limit}`).then((response) => ({
    ...response,
    sessions: Array.isArray(response.sessions) ? response.sessions : [],
  }));
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
  return request<RunsResponse>(`/api/runs?limit=${limit}`).then((response) => ({
    ...response,
    runs: Array.isArray(response.runs) ? response.runs : [],
  }));
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

export async function streamAGUIChat(
  body: AGUIChatRequest,
  handlers: {
    onEvent: (event: AGUIEvent) => void;
  },
): Promise<AGUIStreamResult> {
  const response = await fetch("/api/agui/chat", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "text/event-stream",
    },
    body: JSON.stringify(body),
  });

  if (!response.ok) {
    let message = "request failed";
    try {
      const payload = (await response.json()) as ApiErrorResponse;
      if (hasApiError(payload)) {
        message = payload.error.message;
      }
    } catch {
      // ignore fallback parse failures
    }
    throw new Error(message);
  }

  if (!response.body) {
    throw new Error("stream response body is missing");
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let sawContent = false;
  let sawFailure = false;
  let sawFinished = false;
  let interrupted = false;

  try {
    while (true) {
      const { value, done } = await reader.read();
      if (done) {
        break;
      }
      buffer += decoder.decode(value, { stream: true });
      const chunks = buffer.split("\n\n");
      buffer = chunks.pop() ?? "";

      for (const chunk of chunks) {
        const lines = chunk
          .split("\n")
          .filter((line) => line.startsWith("data: "))
          .map((line) => line.slice("data: ".length));
        if (!lines.length) {
          continue;
        }
        try {
          const event = JSON.parse(lines.join("\n")) as AGUIEvent;
          if (event.type === "TEXT_MESSAGE_CONTENT" && event.delta) {
            sawContent = true;
          }
          if (event.type === "RUN_ERROR") {
            sawFailure = true;
          }
          if (event.type === "RUN_FINISHED") {
            sawFinished = true;
          }
          handlers.onEvent(event);
        } catch (error) {
          console.error("Failed to parse AG-UI SSE payload", {
            chunk,
            error,
          });
        }
      }
    }
  } catch (error) {
    if (sawFinished) {
      return {
        completed: true,
        interrupted: false,
      };
    }
    if (sawFailure || !sawContent) {
      throw error;
    }
    interrupted = true;
  }

  if (!sawFinished && !sawFailure && !interrupted) {
    throw new Error("stream ended before terminal event");
  }

  return {
    completed: sawFinished,
    interrupted,
  };
}
