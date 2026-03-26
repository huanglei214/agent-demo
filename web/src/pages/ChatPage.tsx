import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type FormEvent,
} from "react";

import {
  ComposerField,
  DrawerActionButton,
  HistorySessionButton,
  MessageBubble,
  MetaPill,
  SidebarCard,
  SuggestionChip,
} from "../components/chat";
import { JsonBlock } from "../components/JsonBlock";
import { inspectSession, listSessions, streamAGUIChat } from "../lib/api";
import { type Language, useI18n } from "../lib/i18n";
import type {
  AGUIChatRequest,
  AGUIEvent,
  AGUIMessage,
  SessionInspectResponse,
  SessionListItem,
} from "../lib/types";

type ChatPageProps = {
  workspace: string;
  healthLabel: string;
  healthState: string;
  language: Language;
  onLanguageChange: (language: Language) => void;
  onOpenSession: (sessionId: string) => void;
  onOpenRun: (runId: string) => void;
};

type ChatFormState = {
  sessionId: string;
  provider: string;
  model: string;
  maxTurns: number;
  prompt: string;
};

const initialForm: ChatFormState = {
  sessionId: "",
  provider: "mock",
  model: "mock-model",
  maxTurns: 8,
  prompt: "",
};

export function ChatPage({
  workspace,
  healthLabel,
  healthState,
  language,
  onLanguageChange,
  onOpenSession,
  onOpenRun,
}: ChatPageProps) {
  const { copy, formatMessageRole, formatRelativeTime } = useI18n();
  const [form, setForm] = useState<ChatFormState>(initialForm);
  const [messages, setMessages] = useState<AGUIMessage[]>([]);
  const [activity, setActivity] = useState<AGUIEvent[]>([]);
  const [snapshot, setSnapshot] = useState<Record<string, unknown> | null>(null);
  const [threadId, setThreadId] = useState("");
  const [runId, setRunId] = useState("");
  const [streamState, setStreamState] = useState<"idle" | "live" | "finished" | "failed">("idle");
  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);
  const [recentSessions, setRecentSessions] = useState<SessionListItem[]>([]);
  const [loadingSessions, setLoadingSessions] = useState(true);
  const [loadingConversation, setLoadingConversation] = useState(false);
  const [activityOpen, setActivityOpen] = useState(false);
  const [activeSession, setActiveSession] = useState<SessionInspectResponse | null>(null);
  const messageEndRef = useRef<HTMLDivElement | null>(null);

  const activeSessionId = form.sessionId.trim() || threadId;
  const activeSessionMessages = activeSession?.messages ?? [];

  const latestAssistant = useMemo(() => {
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      if (messages[index].role === "assistant") {
        return messages[index];
      }
    }
    return null;
  }, [messages]);

  const recentActivity = useMemo(() => activity.slice(-10).reverse(), [activity]);
  const conversationStarted = messages.length > 0;
  const groupedSessions = useMemo(
    () => groupSessionsByRecency(recentSessions, copy.chat),
    [copy.chat, recentSessions],
  );

  useEffect(() => {
    refreshSidebarSessions().catch(() => {
      setLoadingSessions(false);
    });
  }, []);

  useEffect(() => {
    if (!messageEndRef.current) {
      return;
    }
    messageEndRef.current.scrollIntoView({
      behavior: sending || streamState === "live" ? "auto" : "smooth",
      block: "end",
    });
  }, [messages, sending, streamState]);

  useEffect(() => {
    let active = true;

    if (!activeSessionId) {
      setActiveSession(null);
      return () => {
        active = false;
      };
    }

    setLoadingConversation(true);
    inspectSession(activeSessionId, 12)
      .then((response) => {
        if (!active) {
          return;
        }
        setActiveSession(response);
        if (!sending) {
          const safeMessages = response.messages ?? [];
          const safeRuns = response.runs ?? [];
          setMessages(
            safeMessages.map((message) => ({
              id: message.id,
              role: message.role,
              content: message.content,
            })),
          );
          setRunId(safeRuns[0]?.run_id ?? runId);
        }
      })
      .catch((err) => {
        if (active) {
          console.error("Failed to inspect active session", {
            sessionId: activeSessionId,
            error: err,
          });
          setActiveSession(null);
        }
      })
      .finally(() => {
        if (active) {
          setLoadingConversation(false);
        }
      });

    return () => {
      active = false;
    };
  }, [activeSessionId, sending]);

  async function refreshSidebarSessions() {
    setLoadingSessions(true);
    try {
      const response = await listSessions(14);
      setRecentSessions(response.sessions);
    } finally {
      setLoadingSessions(false);
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const prompt = form.prompt.trim();
    if (!prompt) {
      return;
    }

    const userMessage: AGUIMessage = {
      id: `msg_user_${Date.now()}`,
      role: "user",
      content: prompt,
    };

    setError("");
    setSending(true);
    setStreamState("live");
    setActivity([]);
    setActivityOpen(false);
    setMessages((current) => [...current, userMessage]);
    setForm((current) => ({ ...current, prompt: "" }));

    const body: AGUIChatRequest = {
      threadId: activeSessionId || undefined,
      messages: [userMessage],
      state: {
        workspace: workspace || undefined,
        provider: form.provider,
        model: form.model,
        maxTurns: form.maxTurns,
      },
      context: activeSessionId ? { sessionId: activeSessionId } : undefined,
    };

    try {
      await streamAGUIChat(body, {
        onEvent: (item) => {
          setActivity((current) => [...current, item]);

          if (item.threadId) {
            setThreadId(item.threadId);
            setForm((current) => ({ ...current, sessionId: item.threadId ?? current.sessionId }));
          }
          if (item.runId) {
            setRunId(item.runId);
          }
          if (item.type === "MESSAGES_SNAPSHOT" && item.messages) {
            setMessages(ensureMessages(item.messages));
          }
          if (item.type === "STATE_SNAPSHOT" && item.snapshot) {
            setSnapshot(item.snapshot);
          }
          if (item.type === "TEXT_MESSAGE_START" && item.messageId) {
            const messageId = item.messageId;
            setMessages((current) => {
              if (current.some((message) => message.id === messageId)) {
                return current;
              }
              return [
                ...current,
                {
                  id: messageId,
                  role: (item.role as "assistant" | "user") || "assistant",
                  content: "",
                },
              ];
            });
          }
          if (item.type === "TEXT_MESSAGE_CONTENT" && item.messageId) {
            const messageId = item.messageId;
            setMessages((current) =>
              current.map((message) =>
                message.id === messageId
                  ? { ...message, content: `${message.content}${item.delta ?? ""}` }
                  : message,
              ),
            );
          }
          if (item.type === "RUN_FINISHED") {
            setStreamState("finished");
          }
          if (item.type === "RUN_ERROR") {
            setStreamState("failed");
            setError(item.error ?? copy.chat.failed);
            setActivityOpen(true);
          }
        },
      });

      setStreamState((current) => (current === "failed" ? current : "finished"));
      await refreshSidebarSessions();
    } catch (err) {
      console.error("AG-UI chat request failed", {
        body,
        error: err,
      });
      setStreamState("failed");
      setError(err instanceof Error ? err.message : copy.chat.failed);
      setActivityOpen(true);
    } finally {
      setSending(false);
    }
  }

  function handlePickSession(sessionId: string) {
    setThreadId(sessionId);
    setRunId("");
    setSnapshot(null);
    setActivity([]);
    setError("");
    setStreamState("idle");
    setForm((current) => ({ ...current, sessionId }));
  }

  function handleNewChat() {
    setMessages([]);
    setActivity([]);
    setSnapshot(null);
    setThreadId("");
    setRunId("");
    setError("");
    setStreamState("idle");
    setActiveSession(null);
    setForm(initialForm);
  }

  return (
    <div
      className={`chat-shell bg-chat-bg text-chat-text selection:bg-white selection:text-black ${
        activityOpen ? "chat-shell-drawer-open" : ""
      }`}
    >
      <aside className="chat-sidebar bg-chat-panel">
        <div className="chat-sidebar-brand">
          <div className="chat-brand-badge">A</div>
          <div>
            <strong>{copy.chat.sidebarTitle}</strong>
            <p>{copy.app.eyebrow}</p>
          </div>
        </div>

        <a className="chat-nav-link chat-nav-link-active" href="/">
          {copy.app.navChat}
        </a>
        <a className="chat-nav-link" href="/launchpad">
          {copy.app.navLaunchpad}
        </a>

        <button
          className="chat-new-button transition hover:bg-white hover:shadow-[0_12px_36px_rgba(0,0,0,0.24)]"
          type="button"
          onClick={handleNewChat}
        >
          {copy.chat.newChat}
        </button>

        <SidebarCard>
          <span className="chat-sidebar-label">{copy.chat.currentSession}</span>
          <p>{activeSessionId || copy.chat.sessionPlaceholder}</p>
        </SidebarCard>

        <SidebarCard>
          <span className="chat-sidebar-label">{copy.chat.currentRun}</span>
          <p>{runId || copy.common.notAvailable}</p>
        </SidebarCard>

        <div className="chat-sidebar-history">
          <div className="chat-history-header">
            <span className="chat-sidebar-label">{copy.chat.historyTitle}</span>
            <button className="chat-history-refresh" type="button" onClick={() => void refreshSidebarSessions()}>
              {copy.chat.refreshHistory}
            </button>
          </div>
          <div className="chat-history-list">
            {loadingSessions ? (
              <span className="chat-activity-empty">{copy.common.loading}</span>
            ) : recentSessions.length ? (
              groupedSessions.map((group) => (
                group.sessions.length ? (
                  <div key={group.label} className="space-y-2">
                    <span className="chat-sidebar-label">{group.label}</span>
                    {group.sessions.map((session) => (
                      <HistorySessionButton
                        key={session.id}
                        active={session.id === activeSessionId}
                        title={session.id}
                        meta={formatRelativeTime(session.updated_at)}
                        detail={`${session.run_count} runs`}
                        onClick={() => handlePickSession(session.id)}
                      />
                    ))}
                  </div>
                ) : null
              ))
            ) : (
              <span className="chat-activity-empty">{copy.chat.noHistory}</span>
            )}
          </div>
        </div>

        <div className="chat-sidebar-footer">
          <div className="language-switch" aria-label={copy.app.language.label}>
            <span className="language-label">{copy.app.language.label}</span>
            <button
              className={language === "en" ? "language-button active" : "language-button"}
              type="button"
              onClick={() => onLanguageChange("en")}
            >
              {copy.app.language.english}
            </button>
            <button
              className={language === "zh" ? "language-button active" : "language-button"}
              type="button"
              onClick={() => onLanguageChange("zh")}
            >
              {copy.app.language.chinese}
            </button>
          </div>
          <div className={`health health-${healthState.replace(/\s+/g, "-")}`}>
            <span className="health-dot" />
            <span>{healthLabel}</span>
          </div>
        </div>
      </aside>

      <main className="chat-main">
        <header className="chat-header">
          <div>
            <h1>{copy.chat.headerTitle}</h1>
            <p>{copy.chat.body}</p>
          </div>
          <div className="chat-header-actions">
            <div className="chat-header-meta">
              <MetaPill>
                {copy.chat.provider}: {form.provider}
              </MetaPill>
              <MetaPill>
                {copy.chat.model}: {form.model}
              </MetaPill>
              <MetaPill>
                {copy.chat.streamStatus}: {formatChatStreamState(copy, streamState)}
              </MetaPill>
            </div>
            <button
              className={
                activityOpen
                  ? "chat-drawer-toggle active transition hover:opacity-90"
                  : "chat-drawer-toggle transition hover:bg-white/12"
              }
              type="button"
              onClick={() => setActivityOpen((current) => !current)}
            >
              {copy.chat.activityDrawerToggle}
            </button>
          </div>
        </header>

        <section className="chat-conversation">
          {!conversationStarted ? (
            <div className="chat-empty-state">
              <p className="chat-empty-kicker">{copy.chat.emptyKicker}</p>
              <h2>{copy.chat.emptyTitle}</h2>
              <p>{copy.chat.emptyBody}</p>
              <div className="chat-empty-actions">
                <SuggestionChip
                  onClick={() =>
                    setForm((current) => ({ ...current, prompt: copy.chat.quickPromptInspect }))
                  }
                >
                  {copy.chat.quickPromptInspect}
                </SuggestionChip>
                <SuggestionChip
                  onClick={() =>
                    setForm((current) => ({ ...current, prompt: copy.chat.quickPromptDebug }))
                  }
                >
                  {copy.chat.quickPromptDebug}
                </SuggestionChip>
                <SuggestionChip
                  onClick={() =>
                    setForm((current) => ({ ...current, prompt: copy.chat.quickPromptSummarize }))
                  }
                >
                  {copy.chat.quickPromptSummarize}
                </SuggestionChip>
              </div>
            </div>
          ) : (
            <div className="chat-message-flow">
              {messages.map((message) => (
                <MessageBubble
                  key={message.id}
                  role={message.role}
                  roleLabel={formatMessageRole(message.role)}
                  avatarLabel={copy.messages.role.assistant.slice(0, 1)}
                  content={
                    message.content ||
                    (message.id === latestAssistant?.id && sending ? copy.common.loading : "")
                  }
                />
              ))}
              <div ref={messageEndRef} />
            </div>
          )}
        </section>

        <form className="chat-composer-shell" onSubmit={handleSubmit}>
          <div className="chat-composer-meta">
            <ComposerField label={copy.chat.sessionId}>
              <input
                value={form.sessionId}
                onChange={(event) =>
                  setForm((current) => ({ ...current, sessionId: event.target.value }))
                }
                placeholder={copy.chat.sessionPlaceholder}
              />
            </ComposerField>
            <ComposerField label={copy.chat.provider} size="medium">
              <input
                value={form.provider}
                onChange={(event) =>
                  setForm((current) => ({ ...current, provider: event.target.value }))
                }
              />
            </ComposerField>
            <ComposerField label={copy.chat.model} size="medium">
              <input
                value={form.model}
                onChange={(event) =>
                  setForm((current) => ({ ...current, model: event.target.value }))
                }
              />
            </ComposerField>
            <ComposerField label={copy.chat.maxTurns} size="small">
              <input
                type="number"
                min={1}
                value={form.maxTurns}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    maxTurns: Number(event.target.value) || 1,
                  }))
                }
              />
            </ComposerField>
          </div>

          <div className="chat-composer">
            <button className="chat-composer-plus" type="button" aria-label={copy.chat.newChat} onClick={handleNewChat}>
              +
            </button>
            <textarea
              rows={1}
              value={form.prompt}
              onChange={(event) =>
                setForm((current) => ({ ...current, prompt: event.target.value }))
              }
              placeholder={copy.chat.promptPlaceholder}
            />
            <button
              className="chat-send-button transition hover:opacity-90 disabled:hover:opacity-68"
              type="submit"
              disabled={sending}
            >
              {sending ? copy.chat.sending : copy.chat.send}
            </button>
          </div>

          {error ? <p className="error-text chat-error-inline">{error}</p> : null}
          <div className="chat-footer-note">
            <span>{copy.chat.threadLabel}: {activeSessionId || copy.common.notAvailable}</span>
            <span>{copy.chat.runLabel}: {runId || copy.common.notAvailable}</span>
            <span>{copy.chat.eventCount}: {activity.length}</span>
          </div>
        </form>
      </main>

      <aside className={activityOpen ? "chat-drawer chat-drawer-open" : "chat-drawer"}>
        <div className="chat-drawer-header">
          <div>
            <span className="chat-sidebar-label">{copy.chat.activityTitle}</span>
            <h2>{copy.chat.drawerTitle}</h2>
          </div>
          <button className="chat-drawer-close" type="button" onClick={() => setActivityOpen(false)}>
            ×
          </button>
        </div>

        <div className="chat-drawer-section">
          {error ? (
            <>
              <span className="chat-sidebar-label">{copy.chat.failureTitle}</span>
              <p className="error-text chat-error-inline">{error}</p>
              <p className="muted-text">{copy.chat.failureHint}</p>
            </>
          ) : null}
        </div>

        <div className="chat-drawer-section">
          <div className="chat-drawer-actions">
            <DrawerActionButton
              type="button"
              disabled={!activeSessionId}
              onClick={() => activeSessionId && onOpenSession(activeSessionId)}
            >
              {copy.common.openSession}
            </DrawerActionButton>
            <DrawerActionButton
              type="button"
              disabled={!runId}
              onClick={() => runId && onOpenRun(runId)}
            >
              {copy.common.openRun}
            </DrawerActionButton>
          </div>
        </div>

        <div className="chat-drawer-section">
          <span className="chat-sidebar-label">{copy.chat.messagesTitle}</span>
          {loadingConversation ? (
            <p className="muted-text">{copy.common.loading}</p>
          ) : activeSessionMessages.length ? (
            <div className="chat-drawer-message-list">
              {activeSessionMessages.slice(-6).map((message) => (
                <div key={message.id} className="chat-drawer-message">
                  <strong>{formatMessageRole(message.role)}</strong>
                  <p>{message.content}</p>
                </div>
              ))}
            </div>
          ) : (
            <p className="muted-text">{copy.chat.emptyMessages}</p>
          )}
        </div>

        <div className="chat-drawer-section">
          <span className="chat-sidebar-label">{copy.chat.activityTitle}</span>
          {recentActivity.length ? (
            <div className="timeline">
              {recentActivity.map((event, index) => (
                <div key={`${event.type}-${index}`} className="timeline-item">
                  <strong>{event.name ? `${event.type} · ${event.name}` : event.type}</strong>
                  <p>{summarizeEvent(event)}</p>
                </div>
              ))}
            </div>
          ) : (
            <p className="muted-text">{copy.chat.emptyActivity}</p>
          )}
        </div>

        <div className="chat-drawer-section">
          <span className="chat-sidebar-label">{copy.chat.stateTitle}</span>
          <JsonBlock value={snapshot ?? { hint: copy.chat.emptyActivity }} />
        </div>
      </aside>
    </div>
  );
}

function summarizeEvent(event: AGUIEvent) {
  if (event.delta) {
    return event.delta;
  }
  if (event.toolName) {
    return event.toolName;
  }
  if (event.stepName) {
    return event.stepName;
  }
  if (event.error) {
    return event.error;
  }
  if (event.snapshot) {
    return JSON.stringify(event.snapshot);
  }
  if (event.name) {
    return JSON.stringify(event.value ?? {});
  }
  return "";
}

function ensureMessages(value: AGUIMessage[] | null | undefined): AGUIMessage[] {
  return Array.isArray(value) ? value : [];
}

function formatChatStreamState(
  copy: ReturnType<typeof useI18n>["copy"],
  value: "idle" | "live" | "finished" | "failed",
) {
  if (value === "live") {
    return copy.chat.live;
  }
  if (value === "finished") {
    return copy.chat.finished;
  }
  if (value === "failed") {
    return copy.chat.failed;
  }
  return copy.chat.idle;
}

function groupSessionsByRecency(
  sessions: SessionListItem[],
  copy: Pick<
    ReturnType<typeof useI18n>["copy"]["chat"],
    "historyToday" | "historyThisWeek" | "historyOlder"
  >,
) {
  const now = Date.now();
  const day = 24 * 60 * 60 * 1000;

  const groups = [
    { label: copy.historyToday, sessions: [] as SessionListItem[] },
    { label: copy.historyThisWeek, sessions: [] as SessionListItem[] },
    { label: copy.historyOlder, sessions: [] as SessionListItem[] },
  ];

  for (const session of sessions) {
    const diff = now - new Date(session.updated_at).getTime();
    if (diff < day) {
      groups[0].sessions.push(session);
      continue;
    }
    if (diff < day * 7) {
      groups[1].sessions.push(session);
      continue;
    }
    groups[2].sessions.push(session);
  }

  return groups;
}
