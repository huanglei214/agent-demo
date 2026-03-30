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
import { RunDebugDrawer } from "../components/debug/RunDebugDrawer";
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
  onOpenSession?: (id: string) => void;
  onOpenRun?: (id: string) => void;
};

type ViewState = 'chat' | 'launchpad';

type ChatFormState = {
  sessionId: string;
  provider: string;
  model: string;
  maxTurns: number;
  planMode: "" | "none" | "todo";
  prompt: string;
};

const initialForm: ChatFormState = {
  sessionId: "",
  provider: "",
  model: "",
  maxTurns: 8,
  planMode: "",
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
  const [view, setView] = useState<ViewState>('chat');
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
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [debugRunId, setDebugRunId] = useState<string | null>(null);
  const [activeSession, setActiveSession] = useState<SessionInspectResponse | null>(null);
  const messageEndRef = useRef<HTMLDivElement | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);

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
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
      textareaRef.current.style.height = `${Math.min(textareaRef.current.scrollHeight, 200)}px`;
    }
  }, [form.prompt]);

  useEffect(() => {
    refreshSidebarSessions().catch(() => {
      setLoadingSessions(false);
    });
  }, []);

  useEffect(() => {
    if (!messageEndRef.current) {
      return;
    }
    
    const scrollToBottom = () => {
      // Get the scrollable container (the parent section element)
      const container = messageEndRef.current?.closest('.chat-conversation');
      if (container) {
        container.scrollTo({
          top: container.scrollHeight,
          behavior: sending || streamState === "live" ? "auto" : "smooth",
        });
      } else {
        // Fallback
        messageEndRef.current?.scrollIntoView({
          behavior: sending || streamState === "live" ? "auto" : "smooth",
          block: "end",
        });
      }
    };
    
    // Use a small timeout to ensure DOM has updated
    setTimeout(scrollToBottom, 50);
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
              runId: message.run_id,
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
        planMode: form.planMode,
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
      className={`chat-shell bg-[#212121] text-chat-text selection:bg-white selection:text-black h-screen overflow-hidden ${
        activityOpen ? "chat-shell-drawer-open" : ""
      } flex group/app`}
    >
      <aside className={`chat-sidebar bg-[#171717] border-r border-white/10 flex flex-col p-0 h-screen overflow-hidden transition-all duration-300 ${sidebarOpen ? 'w-[260px]' : 'w-[60px]'}`}>
        <div className={`flex items-center p-3 flex-shrink-0 ${sidebarOpen ? 'justify-between' : 'justify-center relative group/sidebar-header'}`}>
          {sidebarOpen ? (
            <button
              className="flex items-center gap-2 text-white/80 hover:bg-white/10 px-3 py-2 rounded-lg transition-colors flex-1 text-left"
              type="button"
              onClick={handleNewChat}
            >
              <div className="chat-brand-badge w-6 h-6 rounded-md text-xs flex items-center justify-center">A</div>
              <span className="font-medium text-sm">新聊天</span>
            </button>
          ) : (
            <>
              <button
                className="p-2 text-white/80 hover:text-white hover:bg-white/10 rounded-lg transition-colors group-hover/sidebar-header:opacity-0"
                type="button"
                onClick={handleNewChat}
              >
                <div className="chat-brand-badge w-6 h-6 rounded-md text-xs flex items-center justify-center">A</div>
              </button>
              
              <div className="absolute inset-0 flex items-center justify-center opacity-0 group-hover/sidebar-header:opacity-100 transition-opacity">
                <div className="relative group/toggle">
                  <button className="p-2 text-white/60 hover:text-white hover:bg-white/10 rounded-lg transition-colors" onClick={() => setSidebarOpen(true)}>
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                      <path d="M19 3H5C3.89543 3 3 3.89543 3 5V19C3 20.1046 3.89543 21 5 21H19C20.1046 21 21 20.1046 21 19V5C21 3.89543 20.1046 3 19 3Z" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                      <path d="M9 3V21" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                    </svg>
                  </button>
                  {/* Tooltip */}
                  <div className="absolute left-full ml-2 top-1/2 -translate-y-1/2 bg-black text-white text-xs py-1.5 px-3 rounded-md whitespace-nowrap opacity-0 pointer-events-none group-hover/toggle:opacity-100 transition-opacity z-50">
                    打开边栏
                  </div>
                </div>
              </div>
            </>
          )}
          {sidebarOpen && (
            <div className="relative group/toggle-close">
              <button className="p-2 text-white/60 hover:text-white hover:bg-white/10 rounded-lg transition-colors" onClick={() => setSidebarOpen(false)}>
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                  <path d="M19 3H5C3.89543 3 3 3.89543 3 5V19C3 20.1046 3.89543 21 5 21H19C20.1046 21 21 20.1046 21 19V5C21 3.89543 20.1046 3 19 3Z" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                  <path d="M9 3V21" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
              </button>
              {/* Tooltip */}
              <div className="absolute left-full ml-2 top-1/2 -translate-y-1/2 bg-black text-white text-xs py-1.5 px-3 rounded-md whitespace-nowrap opacity-0 pointer-events-none group-hover/toggle-close:opacity-100 transition-opacity z-50">
                关闭边栏
              </div>
            </div>
          )}
        </div>

        <div className={`px-3 pb-2 space-y-1 flex-shrink-0 ${sidebarOpen ? '' : 'flex flex-col items-center px-0'}`}>
          {/* Removed Launchpad/App tab */}
        </div>

        {sidebarOpen ? (
          <div className="chat-sidebar-history flex-1 overflow-y-auto px-3 py-2">
            {loadingSessions ? (
              <div className="px-3 py-2 text-sm text-white/40">{copy.common.loading}</div>
            ) : recentSessions.length ? (
              groupedSessions.map((group) => (
                group.sessions.length ? (
                  <div key={group.label} className="mb-6">
                    <div className="px-3 mb-2 text-xs font-medium text-white/40">{group.label}</div>
                    <div className="space-y-0.5">
                      {group.sessions.map((session) => (
                        <button
                          key={session.id}
                          onClick={() => handlePickSession(session.id)}
                          className={`w-full text-left px-3 py-2 rounded-lg text-sm truncate transition-colors ${
                            session.id === activeSessionId
                              ? 'bg-white/10 text-white'
                              : 'text-white/70 hover:bg-white/5'
                          }`}
                        >
                          {session.id}
                        </button>
                      ))}
                    </div>
                  </div>
                ) : null
              ))
            ) : (
              <div className="px-3 py-2 text-sm text-white/40">{copy.chat.noHistory}</div>
            )}
          </div>
        ) : (
          <div className="flex-1"></div>
        )}

        <div className={`chat-sidebar-footer p-3 border-t border-white/10 flex-shrink-0 ${sidebarOpen ? '' : 'flex justify-center px-0'}`}>
          <div className={`relative group/user-item flex items-center ${sidebarOpen ? 'gap-3 px-3 py-2 w-full' : 'justify-center p-2 w-10 h-10'} text-sm text-white/80 hover:bg-white/10 rounded-lg cursor-pointer transition-colors`}>
            <div className="w-6 h-6 rounded-full bg-white/20 flex items-center justify-center text-xs flex-shrink-0">H</div>
            {sidebarOpen && <span className="flex-1 truncate">huang lei</span>}
            {!sidebarOpen && (
              <div className="absolute left-full ml-2 top-1/2 -translate-y-1/2 bg-black text-white text-xs py-1.5 px-3 rounded-md whitespace-nowrap opacity-0 pointer-events-none group-hover/user-item:opacity-100 transition-opacity z-50">
                huang lei
              </div>
            )}
          </div>
        </div>
      </aside>

      <main className="chat-main flex-1 flex flex-col relative h-screen overflow-hidden transition-all duration-300">
        <header className="chat-header items-center px-4 py-3 sticky top-0 z-20 bg-[#212121]">
          <div className="flex items-center gap-3">
            <h1 className="text-xl font-medium m-0 text-white/90 cursor-pointer hover:bg-white/5 px-3 py-1.5 rounded-lg transition-colors flex items-center gap-2">
              {copy.chat.headerTitle}
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" className="opacity-60">
                <path d="M6 9L12 15L18 9" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
            </h1>
          </div>
        </header>

        <section className={`chat-conversation flex-1 overflow-y-auto ${!conversationStarted ? 'flex flex-col justify-center' : ''}`}>
            {!conversationStarted ? (
              <div className="w-full flex flex-col items-center justify-center p-8 max-w-3xl mx-auto mt-[-20vh]">
                <h1 className="text-5xl font-bold text-white mb-12">准备好了，随时开始</h1>

                <div className="flex flex-wrap justify-center gap-3 mt-8">
                  <button
                    type="button"
                    className="px-4 py-2 rounded-full border border-white/10 bg-white/5 text-sm text-gray-300 hover:bg-white/10 transition-colors pointer-events-auto"
                    onClick={() =>
                      setForm((current) => ({ ...current, prompt: copy.chat.quickPromptInspect }))
                    }
                  >
                    {copy.chat.quickPromptInspect}
                  </button>
                  <button
                    type="button"
                    className="px-4 py-2 rounded-full border border-white/10 bg-white/5 text-sm text-gray-300 hover:bg-white/10 transition-colors pointer-events-auto"
                    onClick={() =>
                      setForm((current) => ({ ...current, prompt: copy.chat.quickPromptDebug }))
                    }
                  >
                    {copy.chat.quickPromptDebug}
                  </button>
                </div>
              </div>
            ) : (
              <div className="chat-message-flow pb-[280px] max-w-3xl mx-auto w-full px-4 pt-4">
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
                    onDebug={message.runId ? () => setDebugRunId(message.runId!) : undefined}
                  />
                ))}
                <div ref={messageEndRef} />
              </div>
            )}
          </section>

        {/* Common Composer for both empty state and conversation */}
        <div className={`absolute bottom-0 left-0 right-0 bg-gradient-to-t from-[#212121] via-[#212121] to-transparent pt-10 pb-6 px-4 z-10 pointer-events-none transition-all duration-300 ${!conversationStarted ? 'translate-y-[-20vh]' : ''}`}>
          <form className="chat-composer-shell max-w-3xl mx-auto w-full relative pointer-events-auto" onSubmit={handleSubmit}>
            <div className="chat-composer-meta">
              <ComposerField label={copy.chat.planMode} size="small">
                <select
                  aria-label={copy.chat.planMode}
                  value={form.planMode}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      planMode: event.target.value as ChatFormState["planMode"],
                    }))
                  }
                >
                  <option value="">{copy.chat.planModeAuto}</option>
                  <option value="none">{copy.chat.planModeNone}</option>
                  <option value="todo">{copy.chat.planModeTodo}</option>
                </select>
              </ComposerField>
            </div>
            <div className="chat-composer rounded-3xl bg-[#303030] shadow-lg flex items-end px-4 py-3 gap-3">
              <button className="text-gray-400 hover:text-white transition-colors flex-shrink-0 self-center h-[32px] w-[32px] flex items-center justify-center rounded-full hover:bg-white/10" type="button" aria-label={copy.chat.newChat} onClick={handleNewChat}>
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" className="block m-auto">
                  <path d="M12 5V19M5 12H19" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
              </button>
              <div className="flex-1 flex flex-col justify-end min-h-[32px]">
                <textarea
                  ref={textareaRef}
                  className="w-full bg-transparent border-0 outline-none text-white max-h-[200px] py-[4px] placeholder-gray-400 overflow-y-auto leading-[24px]"
                  style={{ resize: 'none' }}
                  rows={1}
                  aria-label={copy.chat.prompt}
                  value={form.prompt}
                  onChange={(event) =>
                    setForm((current) => ({ ...current, prompt: event.target.value }))
                  }
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && !e.shiftKey) {
                      e.preventDefault();
                      if (form.prompt.trim() && !sending) {
                        const event = new Event('submit', { bubbles: true, cancelable: true });
                        e.currentTarget.form?.dispatchEvent(event);
                      }
                    }
                  }}
                  placeholder={copy.chat.promptPlaceholder}
                />
              </div>
              <button
                className="w-8 h-8 rounded-full bg-white/10 flex items-center justify-center text-white transition hover:bg-white/20 disabled:opacity-50 disabled:cursor-not-allowed flex-shrink-0"
                type="submit"
                aria-label={copy.chat.send}
                disabled={sending || !form.prompt.trim()}
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                  <path d="M12 19V5M12 5L5 12M12 5L19 12" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
              </button>
            </div>

            {error ? <p className="error-text chat-error-inline">{error}</p> : null}
          </form>
        </div>
      </main>

      <aside className={activityOpen ? "chat-drawer chat-drawer-open" : "chat-drawer"}>
        <div className="chat-drawer-header">
          <div>
            <span className="chat-sidebar-label">{copy.chat.activityTitle}</span>
            <h2>{copy.chat.drawerTitle}</h2>
          </div>
          <button className="chat-drawer-close" type="button" aria-label="Close" onClick={() => setActivityOpen(false)}>
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
              onClick={() => activeSessionId && onOpenSession?.(activeSessionId)}
            >
              {copy.common.openSession}
            </DrawerActionButton>
            <DrawerActionButton
              type="button"
              disabled={!runId}
              onClick={() => runId && onOpenRun?.(runId)}
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

      <RunDebugDrawer
        runId={debugRunId}
        onClose={() => setDebugRunId(null)}
        language={language}
      />
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
