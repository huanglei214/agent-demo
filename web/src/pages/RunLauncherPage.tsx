import { useEffect, useState, type FormEvent } from "react";

import { JsonBlock } from "../components/JsonBlock";
import {
  createSession,
  inspectSession,
  listRuns,
  listSessions,
  listTools,
  startRun,
} from "../lib/api";
import { useI18n } from "../lib/i18n";
import type {
  RunListItem,
  SessionListItem,
  SessionInspectResponse,
  StartRunRequest,
  ToolDescriptor,
} from "../lib/types";

type RunLauncherPageProps = {
  onOpenRun: (runId: string) => void;
  onOpenSession: (sessionId: string) => void;
};

const initialForm: StartRunRequest = {
  instruction: "",
  workspace: "",
  provider: "mock",
  model: "mock-model",
  max_turns: 8,
  session_id: "",
};

export function RunLauncherPage({ onOpenRun, onOpenSession }: RunLauncherPageProps) {
  const { copy, formatMessageRole, formatRelativeTime, formatRunStatus, formatToolAccess } =
    useI18n();
  const [form, setForm] = useState<StartRunRequest>(initialForm);
  const [creating, setCreating] = useState(false);
  const [recentSessions, setRecentSessions] = useState<SessionListItem[]>([]);
  const [recentRuns, setRecentRuns] = useState<RunListItem[]>([]);
  const [selectedSession, setSelectedSession] = useState<SessionInspectResponse | null>(null);
  const [selectedSessionError, setSelectedSessionError] = useState("");
  const [tools, setTools] = useState<ToolDescriptor[]>([]);
  const [result, setResult] = useState<unknown>(null);
  const [error, setError] = useState<string>("");
  const selectedSessionID = form.session_id ?? "";

  async function refreshHomeData() {
    const [toolsResponse, sessionsResponse, runsResponse] = await Promise.all([
      listTools(),
      listSessions(),
      listRuns(),
    ]);
    setTools(toolsResponse.tools);
    setRecentSessions(sessionsResponse.sessions);
    setRecentRuns(runsResponse.runs);
  }

  useEffect(() => {
    let active = true;
    refreshHomeData()
      .catch((err: Error) => {
        if (active) {
          setError(err.message);
        }
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    let active = true;
    if (!selectedSessionID.trim()) {
      setSelectedSession(null);
      setSelectedSessionError("");
      return () => {
        active = false;
      };
    }

    setSelectedSession(null);
    setSelectedSessionError("");
    inspectSession(selectedSessionID.trim(), 6)
      .then((response) => {
        if (active) {
          setSelectedSession(response);
        }
      })
      .catch((err: Error) => {
        if (active) {
          setSelectedSession(null);
          setSelectedSessionError(err.message);
        }
      });

    return () => {
      active = false;
    };
  }, [selectedSessionID]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setCreating(true);
    setError("");
    try {
      const response = await startRun(form);
      setResult(response);
      await refreshHomeData();
      onOpenRun(response.run.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : copy.launcher.createRunFailed);
    } finally {
      setCreating(false);
    }
  }

  async function handleCreateSession() {
    setCreating(true);
    setError("");
    try {
      const response = await createSession();
      await refreshHomeData();
      setForm((current) => ({ ...current, session_id: response.session.id }));
      onOpenSession(response.session.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : copy.launcher.createSessionFailed);
    } finally {
      setCreating(false);
    }
  }

  function handleContinueSession(sessionId: string) {
    setForm((current) => ({ ...current, session_id: sessionId }));
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  const groupedRuns = groupRunsByStatus(recentRuns);

  return (
    <div className="stack">
      <div className="panel-card accent-card">
        <h2>{copy.launcher.title}</h2>
        <p>{copy.launcher.body}</p>
        {selectedSessionID ? (
          <div className="selection-banner">
            <div>
              <strong>{copy.launcher.continuingSession}</strong>
              <p>{selectedSessionID}</p>
            </div>
            <button
              className="secondary-button"
              type="button"
              onClick={() => setForm((current) => ({ ...current, session_id: "" }))}
            >
              {copy.common.clear}
            </button>
          </div>
        ) : null}
        <form className="form-grid" onSubmit={handleSubmit}>
          <label className="field field-wide">
            <span>{copy.launcher.instruction}</span>
            <textarea
              rows={5}
              value={form.instruction}
              onChange={(event) =>
                setForm((current) => ({ ...current, instruction: event.target.value }))
              }
              placeholder={copy.launcher.instructionPlaceholder}
            />
          </label>

          <label className="field">
            <span>{copy.launcher.provider}</span>
            <input
              value={form.provider}
              onChange={(event) =>
                setForm((current) => ({ ...current, provider: event.target.value }))
              }
            />
          </label>

          <label className="field">
            <span>{copy.launcher.model}</span>
            <input
              value={form.model}
              onChange={(event) =>
                setForm((current) => ({ ...current, model: event.target.value }))
              }
            />
          </label>

          <label className="field">
            <span>{copy.launcher.maxTurns}</span>
            <input
              type="number"
              min={1}
              value={form.max_turns}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  max_turns: Number(event.target.value) || 1,
                }))
              }
            />
          </label>

          <label className="field field-wide">
            <span>{copy.launcher.sessionId}</span>
            <input
              value={form.session_id}
              onChange={(event) =>
                setForm((current) => ({ ...current, session_id: event.target.value }))
              }
              placeholder={copy.launcher.sessionPlaceholder}
            />
          </label>

          <div className="button-row field-wide">
            <button className="primary-button" type="submit" disabled={creating}>
              {creating ? copy.launcher.launching : copy.launcher.startRun}
            </button>
            <button className="secondary-button" type="button" onClick={handleCreateSession}>
              {copy.common.createSessionOnly}
            </button>
          </div>
        </form>
        {error ? <p className="error-text">{error}</p> : null}
      </div>

      <div className="two-up">
        <div className="stack">
          <div className="panel-card">
            <h2>{copy.launcher.recentSessions}</h2>
            <div className="run-list">
              {recentSessions.length ? (
                recentSessions.map((session) => (
                  <div key={session.id} className="run-card run-card-static">
                    <div className="tool-row">
                      <strong>{session.id}</strong>
                      <span className="meta-chip">{formatRelativeTime(session.updated_at)}</span>
                    </div>
                    <p>{session.workspace}</p>
                    <small>{copy.launcher.sessionRunsCount(session.run_count)}</small>
                    <div className="button-row compact-row">
                      <button
                        className="secondary-button"
                        type="button"
                        onClick={() => handleContinueSession(session.id)}
                      >
                        {copy.launcher.continueHere}
                      </button>
                      <button
                        className="secondary-button"
                        type="button"
                        onClick={() => onOpenSession(session.id)}
                      >
                        {copy.common.openDetails}
                      </button>
                    </div>
                  </div>
                ))
              ) : (
                <p className="muted-text">{copy.launcher.noSessions}</p>
              )}
            </div>
          </div>

          {selectedSessionID ? (
            <div className="panel-card">
              <div className="heading-row">
                <div>
                  <h2>{copy.launcher.selectedSessionTitle}</h2>
                  <p>
                    {copy.launcher.selectedSessionBody} <strong>{selectedSessionID}</strong>
                  </p>
                </div>
                <button
                  className="secondary-button"
                  type="button"
                  onClick={() => onOpenSession(selectedSessionID)}
                >
                  {copy.common.openSession}
                </button>
              </div>

              {selectedSessionError ? <p className="error-text">{selectedSessionError}</p> : null}

              {selectedSession ? (
                <div className="stack compact-stack">
                  <div>
                    <h3 className="subheading">{copy.launcher.recentMessages}</h3>
                    <div className="message-list">
                      {selectedSession.messages.length ? (
                        selectedSession.messages.map((message) => (
                          <div key={message.id} className={`message message-${message.role}`}>
                            <span className="message-role">{formatMessageRole(message.role)}</span>
                            <p>{message.content}</p>
                          </div>
                        ))
                      ) : (
                        <p className="muted-text">{copy.launcher.noMessages}</p>
                      )}
                    </div>
                  </div>

                  <div>
                    <h3 className="subheading">{copy.launcher.sessionRuns}</h3>
                    <div className="run-list">
                      {selectedSession.runs.length ? (
                        selectedSession.runs.map((run) => (
                          <button
                            key={run.run_id}
                            className="run-card"
                            onClick={() => onOpenRun(run.run_id)}
                          >
                            <div className="tool-row">
                              <strong>{run.run_id}</strong>
                              <span className={`pill pill-${run.status}`}>
                                {formatRunStatus(run.status)}
                              </span>
                            </div>
                            <small>
                              {copy.launcher.currentStep}: {run.current_step_id || copy.common.notAvailable} ·{" "}
                              {copy.launcher.updatedLabel} {formatRelativeTime(run.updated_at)}
                            </small>
                          </button>
                        ))
                      ) : (
                        <p className="muted-text">{copy.launcher.noSessionRuns}</p>
                      )}
                    </div>
                  </div>
                </div>
              ) : (
                <p className="muted-text">{copy.launcher.loadingSessionContext}</p>
              )}
            </div>
          ) : null}

          <div className="panel-card">
            <h2>{copy.launcher.recentRuns}</h2>
            {recentRuns.length ? (
              <div className="stack compact-stack">
                {groupedRuns.map(([status, runs]) => (
                  <div key={status} className="status-group">
                    <div className="status-header">
                      <span className={`pill pill-${status}`}>{formatRunStatus(status)}</span>
                      <small>{copy.launcher.runsCount(runs.length)}</small>
                    </div>
                    <div className="run-list">
                      {runs.map((run) => (
                        <button key={run.id} className="run-card" onClick={() => onOpenRun(run.id)}>
                          <div className="tool-row">
                            <strong>{run.id}</strong>
                            <span className="meta-chip">{formatRelativeTime(run.updated_at)}</span>
                          </div>
                          <p>{run.instruction || copy.launcher.noInstruction}</p>
                          <small>
                            {copy.launcher.sessionLabel}: {run.session_id} · {run.provider}/{run.model}
                          </small>
                        </button>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <p className="muted-text">{copy.launcher.noRuns}</p>
            )}
          </div>
        </div>

        <div className="stack">
          <div className="panel-card">
            <h2>{copy.launcher.availableTools}</h2>
            <div className="tool-list">
              {tools.map((tool) => (
                <div key={tool.name} className="tool-row">
                  <div>
                    <strong>{tool.name}</strong>
                    <p>{tool.description}</p>
                  </div>
                  <span className={`pill pill-${tool.access}`}>{formatToolAccess(tool.access)}</span>
                </div>
              ))}
            </div>
          </div>

          <div className="panel-card">
            <h2>{copy.launcher.latestPayload}</h2>
            <JsonBlock value={result ?? { hint: copy.launcher.payloadHint }} />
          </div>
        </div>
      </div>
    </div>
  );
}

function groupRunsByStatus(runs: RunListItem[]) {
  const order = ["running", "pending", "blocked", "failed", "completed", "cancelled"];
  const groups = new Map<string, RunListItem[]>();
  for (const run of runs) {
    const current = groups.get(run.status) ?? [];
    current.push(run);
    groups.set(run.status, current);
  }

  return [...groups.entries()].sort((a, b) => {
    const left = order.indexOf(a[0]);
    const right = order.indexOf(b[0]);
    const leftIndex = left === -1 ? order.length : left;
    const rightIndex = right === -1 ? order.length : right;
    return leftIndex - rightIndex;
  });
}
