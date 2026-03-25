import { useEffect, useState } from "react";

import { JsonBlock } from "../components/JsonBlock";
import { inspectRun, replayEvents, replaySummary, streamRun } from "../lib/api";
import { useI18n } from "../lib/i18n";
import type {
  InspectRunResponse,
  ReplayEvent,
  ReplaySummaryResponse,
} from "../lib/types";

type RunDetailsPageProps = {
  runId: string;
  onOpenSession: (sessionId: string) => void;
};

export function RunDetailsPage({ runId, onOpenSession }: RunDetailsPageProps) {
  const { copy, formatRunStatus, formatStreamState } = useI18n();
  const [inspectData, setInspectData] = useState<InspectRunResponse | null>(null);
  const [summary, setSummary] = useState<ReplaySummaryResponse | null>(null);
  const [events, setEvents] = useState<ReplayEvent[]>([]);
  const [error, setError] = useState("");
  const [streamState, setStreamState] = useState("connecting");

  useEffect(() => {
    let active = true;
    let refreshTimer: number | undefined;
    let reconnectTimer: number | undefined;
    let eventSource: EventSource | null = null;
    let lastSequence = 0;
    let terminal = false;
    setError("");
    setStreamState("connecting");

    const refreshSnapshot = () => {
      if (refreshTimer !== undefined) {
        window.clearTimeout(refreshTimer);
      }
      refreshTimer = window.setTimeout(() => {
        Promise.all([inspectRun(runId), replaySummary(runId)])
          .then(([inspectResponse, replayResponse]) => {
            if (!active) {
              return;
            }
            setInspectData(inspectResponse);
            setSummary(replayResponse);
          })
          .catch((err: Error) => {
            if (active) {
              setError(err.message);
            }
          });
      }, 120);
    };

    const connectStream = (afterSequence: number) => {
      if (!active || terminal) {
        return;
      }
      eventSource?.close();
      setStreamState(afterSequence > 0 ? "reconnecting" : "connecting");
      eventSource = streamRun(runId, {
        afterSequence,
        onOpen: () => {
          if (!active || terminal) {
            return;
          }
          setStreamState("live");
        },
        onEvent: (event) => {
          if (!active) {
            return;
          }
          lastSequence = Math.max(lastSequence, event.sequence);
          setEvents((current) => {
            if (current.some((item) => item.id === event.id)) {
              return current;
            }
            return [...current, event];
          });
          refreshSnapshot();
        },
        onDone: (status) => {
          if (!active) {
            return;
          }
          terminal = true;
          setStreamState(`done:${status}`);
          refreshSnapshot();
        },
        onError: () => {
          if (!active || terminal) {
            return;
          }
          setStreamState("disconnected");
          if (reconnectTimer !== undefined) {
            window.clearTimeout(reconnectTimer);
          }
          reconnectTimer = window.setTimeout(() => {
            connectStream(lastSequence);
          }, 1200);
        },
      });
    };

    Promise.all([inspectRun(runId), replaySummary(runId), replayEvents(runId)])
      .then(([inspectResponse, replayResponse, rawEvents]) => {
        if (!active) {
          return;
        }
        setInspectData(inspectResponse);
        setSummary(replayResponse);
        setEvents(rawEvents.events);
        lastSequence = rawEvents.events.length
          ? rawEvents.events[rawEvents.events.length - 1].sequence
          : 0;
        connectStream(lastSequence);
      })
      .catch((err: Error) => {
        if (active) {
          setInspectData(null);
          setSummary(null);
          setEvents([]);
          setStreamState("error");
          setError(err.message);
        }
      });

    return () => {
      active = false;
      if (refreshTimer !== undefined) {
        window.clearTimeout(refreshTimer);
      }
      if (reconnectTimer !== undefined) {
        window.clearTimeout(reconnectTimer);
      }
      eventSource?.close();
    };
  }, [runId]);

  return (
    <div className="stack">
      <div className="panel-card">
        <div className="heading-row">
          <div>
            <h2>
              {copy.run.title} {runId}
            </h2>
            <p>{copy.run.body}</p>
          </div>
          {inspectData ? (
            <button
              className="secondary-button"
              type="button"
              onClick={() => onOpenSession(inspectData.run.session_id)}
            >
              {copy.common.openSession}
            </button>
          ) : null}
        </div>
        {error ? <p className="error-text">{error}</p> : null}
      </div>

      <div className="metrics-grid">
        <MetricCard
          label={copy.run.runStatus}
          value={formatRunStatus(inspectData?.run.status ?? "unknown")}
        />
        <MetricCard
          label={copy.run.currentStep}
          value={inspectData?.current_step?.title ?? copy.common.notAvailable}
        />
        <MetricCard label={copy.run.events} value={String(events.length || inspectData?.event_count || 0)} />
        <MetricCard label={copy.run.childRuns} value={String(inspectData?.child_runs?.length ?? 0)} />
        <MetricCard label={copy.run.stream} value={formatStreamState(streamState)} />
      </div>

      <div className="two-up">
        <div className="panel-card">
          <h2>{copy.run.replayTimeline}</h2>
          <div className="timeline">
            {summary?.entries.length ? (
              summary.entries.map((entry) => (
                <div key={`${entry.sequence}-${entry.type}`} className="timeline-item">
                  <strong>{entry.type}</strong>
                  <small>#{entry.sequence}</small>
                  <p>{entry.summary}</p>
                </div>
              ))
            ) : (
              <p className="muted-text">{copy.run.noReplayEntries}</p>
            )}
          </div>
        </div>

        <div className="panel-card">
          <h2>{copy.run.inspectPayload}</h2>
          <JsonBlock value={inspectData ?? { hint: copy.run.inspectHint }} />
        </div>
      </div>

      <div className="panel-card">
        <h2>{copy.run.rawEvents}</h2>
        <JsonBlock value={events} />
      </div>
    </div>
  );
}

type MetricCardProps = {
  label: string;
  value: string;
};

function MetricCard({ label, value }: MetricCardProps) {
  return (
    <div className="metric-card">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}
