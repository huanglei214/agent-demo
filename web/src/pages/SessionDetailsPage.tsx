import { useEffect, useState } from "react";

import { JsonBlock } from "../components/JsonBlock";
import { inspectSession } from "../lib/api";
import { useI18n } from "../lib/i18n";
import type { SessionInspectResponse } from "../lib/types";

type SessionDetailsPageProps = {
  sessionId: string;
  onOpenRun: (runId: string) => void;
};

export function SessionDetailsPage({ sessionId, onOpenRun }: SessionDetailsPageProps) {
  const { copy, formatMessageRole, formatRunStatus } = useI18n();
  const [data, setData] = useState<SessionInspectResponse | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    setError("");
    inspectSession(sessionId)
      .then((response) => {
        if (active) {
          setData(response);
        }
      })
      .catch((err: Error) => {
        if (active) {
          setData(null);
          setError(err.message);
        }
      });
    return () => {
      active = false;
    };
  }, [sessionId]);

  return (
    <div className="stack">
      <div className="panel-card">
        <h2>
          {copy.session.title} {sessionId}
        </h2>
        <p>{copy.session.body}</p>
        {error ? <p className="error-text">{error}</p> : null}
      </div>

      <div className="two-up">
        <div className="panel-card">
          <h2>{copy.session.messages}</h2>
          <div className="message-list">
            {data?.messages.length ? (
              data.messages.map((message) => (
                <div key={message.id} className={`message message-${message.role}`}>
                  <span className="message-role">{formatMessageRole(message.role)}</span>
                  <p>{message.content}</p>
                </div>
              ))
            ) : (
              <p className="muted-text">{copy.session.noMessages}</p>
            )}
          </div>
        </div>

        <div className="panel-card">
          <h2>{copy.session.runs}</h2>
          <div className="run-list">
            {data?.runs.length ? (
              data.runs.map((run) => (
                <button key={run.run_id} className="run-card" onClick={() => onOpenRun(run.run_id)}>
                  <strong>{run.run_id}</strong>
                  <span className={`pill pill-${run.status}`}>{formatRunStatus(run.status)}</span>
                  <small>
                    {copy.session.currentStep}: {run.current_step_id || copy.common.notAvailable}
                  </small>
                </button>
              ))
            ) : (
              <p className="muted-text">{copy.session.noRuns}</p>
            )}
          </div>
        </div>
      </div>

      <div className="panel-card">
        <h2>{copy.session.rawPayload}</h2>
        <JsonBlock value={data ?? { hint: copy.session.payloadHint }} />
      </div>
    </div>
  );
}
