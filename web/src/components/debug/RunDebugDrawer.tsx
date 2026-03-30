import { useEffect, useMemo, useState } from "react";

import { inspectRun } from "../../lib/api";
import type { InspectRunResponse, TodoItem } from "../../lib/types";
import { JsonBlock } from "../JsonBlock";

type RunDebugDrawerProps = {
  runId: string | null;
  onClose: () => void;
  language: "en" | "zh";
};

type Tab = "overview" | "plan" | "llm" | "events" | "memories";
type MetricTone = "default" | "success" | "error" | "info";

const copy = {
  en: {
    title: "Run Debugger",
    loading: "Loading...",
    noData: "No data available",
    loadError: "Failed to load run details",
    overview: "Overview",
    plan: "Plan",
    llm: "LLM Calls",
    events: "Events",
    memories: "Memories",
    status: "Status",
    model: "Model",
    turns: "Turns",
    eventCount: "Events",
    taskId: "Task ID",
    planMode: "Plan mode",
    todoCount: "Todos",
    todoUpdates: "Todo updates",
    noPlan: "No plan generated",
    goal: "Goal",
    noTodos: "No todos captured",
    noModelCalls: "No model calls recorded",
    noEvents: "No events recorded",
    actor: "Actor",
    payloadDetails: "Payload Details",
    memoriesUnavailable: "Memories not available in this view",
    step: "Step",
    response: "Response",
    finishReason: "Finish Reason",
    timestamp: "Timestamp",
    error: "Error",
    noTool: "No Tool",
    notAvailable: "N/A",
  },
  zh: {
    title: "运行调试器",
    loading: "加载中...",
    noData: "暂无数据",
    loadError: "加载运行详情失败",
    overview: "概览",
    plan: "计划",
    llm: "模型请求",
    events: "事件流",
    memories: "记忆",
    status: "状态",
    model: "模型",
    turns: "轮数",
    eventCount: "事件数",
    taskId: "任务 ID",
    planMode: "规划模式",
    todoCount: "Todo 数量",
    todoUpdates: "Todo 更新",
    noPlan: "没有生成计划",
    goal: "目标",
    noTodos: "没有记录到 Todo",
    noModelCalls: "没有记录到模型调用",
    noEvents: "没有记录到事件",
    actor: "执行方",
    payloadDetails: "Payload 详情",
    memoriesUnavailable: "当前视图暂不展示记忆",
    step: "步骤",
    response: "响应",
    finishReason: "结束原因",
    timestamp: "时间",
    error: "错误",
    noTool: "无工具",
    notAvailable: "暂无",
  },
} as const;

export function RunDebugDrawer({ runId, onClose, language }: RunDebugDrawerProps) {
  const [activeTab, setActiveTab] = useState<Tab>("overview");
  const [data, setData] = useState<InspectRunResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!runId) {
      setData(null);
      return;
    }

    let active = true;
    setLoading(true);
    setError(null);

    inspectRun(runId)
      .then((res) => {
        if (active) {
          setData(res);
          setLoading(false);
        }
      })
      .catch((err: unknown) => {
        if (active) {
          setError(err instanceof Error ? err.message : copy[language].loadError);
          setLoading(false);
        }
      });

    return () => {
      active = false;
    };
  }, [language, runId]);

  const labels = copy[language];
  const todos = data?.state.todos ?? [];
  const todoUpdates = useMemo(
    () => data?.events?.filter((event) => event.type === "todo.updated") ?? [],
    [data?.events],
  );

  if (!runId) {
    return null;
  }

  const tabs: { id: Tab; label: string }[] = [
    { id: "overview", label: labels.overview },
    { id: "plan", label: labels.plan },
    { id: "llm", label: labels.llm },
    { id: "events", label: labels.events },
    { id: "memories", label: labels.memories },
  ];

  return (
    <>
      <div
        className="fixed inset-0 z-40 bg-black/50 backdrop-blur-sm transition-opacity"
        onClick={onClose}
      />

      <div className="fixed inset-y-0 right-0 z-50 flex w-[600px] max-w-[90vw] flex-col border-l border-white/10 bg-[#1a1a1a] shadow-2xl animate-in slide-in-from-right duration-300">
        <div className="flex shrink-0 items-center justify-between border-b border-white/10 p-4">
          <div>
            <h2 className="text-lg font-medium text-white/90">{labels.title}</h2>
            <p className="mt-1 font-mono text-xs text-white/40">{runId}</p>
          </div>
          <button
            className="rounded-lg p-2 text-white/60 transition-colors hover:bg-white/10 hover:text-white"
            onClick={onClose}
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </button>
        </div>

        <div className="hide-scrollbar flex shrink-0 overflow-x-auto border-b border-white/10 px-4">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`whitespace-nowrap border-b-2 px-4 py-3 text-sm font-medium transition-colors ${
                activeTab === tab.id
                  ? "border-blue-500 text-blue-400"
                  : "border-transparent text-white/60 hover:bg-white/5 hover:text-white"
              }`}
            >
              {tab.label}
              {tab.id === "llm" && data?.model_calls ? (
                <span className="ml-2 rounded-full bg-white/10 px-1.5 py-0.5 text-xs">
                  {data.model_calls.length}
                </span>
              ) : null}
            </button>
          ))}
        </div>

        <div className="flex-1 overflow-y-auto p-4">
          {loading ? (
            <div className="flex h-full items-center justify-center text-white/40">
              <svg className="-ml-1 mr-3 h-5 w-5 animate-spin text-white/40" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              {labels.loading}
            </div>
          ) : error ? (
            <div className="rounded-xl bg-red-400/10 p-4 text-red-400">{error}</div>
          ) : !data ? (
            <div className="flex h-full items-center justify-center text-white/40">{labels.noData}</div>
          ) : (
            <div className="h-full">
              {activeTab === "overview" ? (
                <div className="flex flex-col gap-4">
                  <div className="grid grid-cols-2 gap-4">
                    <MetricCard
                      label={labels.status}
                      value={data.run.status}
                      tone={statusTone(data.run.status)}
                    />
                    <MetricCard label={labels.model} value={`${data.run.provider} / ${data.run.model}`} />
                    <MetricCard label={labels.turns} value={`${data.run.turn_count} / ${data.run.max_turns}`} />
                    <MetricCard label={labels.eventCount} value={String(data.event_count)} />
                    <MetricCard label={labels.planMode} value={data.run.plan_mode ?? "none"} />
                    <MetricCard label={labels.todoCount} value={String(todos.length)} />
                  </div>
                  <div className="rounded-xl border border-white/5 bg-white/5 p-4">
                    <span className="mb-2 block text-xs text-white/40">{labels.taskId}</span>
                    <p className="whitespace-pre-wrap text-sm text-white/80">
                      {data.run.task_id || labels.notAvailable}
                    </p>
                  </div>
                </div>
              ) : null}

              {activeTab === "plan" ? (
                <div className="flex flex-col gap-4">
                  <div className="rounded-xl border border-white/5 bg-white/5 p-4">
                    <div className="flex flex-wrap items-center justify-between gap-3">
                      <div>
                        <strong className="mb-2 block text-sm text-white/80">{labels.planMode}</strong>
                        <p className="text-sm text-white/60">{data.run.plan_mode ?? "none"}</p>
                      </div>
                      <div>
                        <strong className="mb-2 block text-sm text-white/80">{labels.todoUpdates}</strong>
                        <p className="text-sm text-white/60">{todoUpdates.length}</p>
                      </div>
                    </div>
                  </div>

                  {data.plan ? (
                    <div className="flex flex-col gap-4">
                      <div className="rounded-xl border border-white/5 bg-white/5 p-4">
                        <strong className="mb-2 block text-sm text-white/80">{labels.goal}</strong>
                        <p className="text-sm text-white/60">{data.plan.goal}</p>
                      </div>
                      <div className="relative pl-6">
                        <div className="absolute bottom-4 left-[11px] top-4 w-px bg-white/10"></div>
                        {data.plan.steps.map((step, idx) => (
                          <div key={step.id} className="relative mb-6 last:mb-0">
                            <div
                              className={`absolute -left-[29px] top-1.5 h-3 w-3 rounded-full ring-4 ring-[#1a1a1a] ${stepDotClass(step.status)}`}
                            ></div>
                            <div className="rounded-xl border border-white/5 bg-white/5 p-4">
                              <div className="mb-2 flex items-start justify-between gap-3">
                                <strong className="text-sm text-white/90">
                                  {labels.step} {idx + 1}: {step.title}
                                </strong>
                                <span className={`rounded px-1.5 py-0.5 text-[10px] ${stepBadgeClass(step.status)}`}>
                                  {step.status}
                                </span>
                              </div>
                              <p className="text-sm text-white/60">{step.description}</p>
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  ) : (
                    <p className="py-8 text-center text-sm text-white/40">{labels.noPlan}</p>
                  )}

                  <div className="rounded-xl border border-white/5 bg-white/5 p-4">
                    <strong className="mb-3 block text-sm text-white/80">{labels.todoCount}</strong>
                    {todos.length ? (
                      <div className="flex flex-col gap-3">
                        {todos.map((todo) => (
                          <TodoRow key={todo.id} todo={todo} />
                        ))}
                      </div>
                    ) : (
                      <p className="text-sm text-white/40">{labels.noTodos}</p>
                    )}
                  </div>
                </div>
              ) : null}

              {activeTab === "llm" ? (
                <div className="flex flex-col gap-4">
                  {!data.model_calls?.length ? (
                    <p className="py-8 text-center text-sm text-white/40">{labels.noModelCalls}</p>
                  ) : (
                    data.model_calls.map((call, idx) => (
                      <details key={idx} className="group overflow-hidden rounded-xl border border-white/5 bg-white/5">
                        <summary className="flex cursor-pointer items-center justify-between p-4 transition-colors hover:bg-white/5">
                          <div className="flex items-center gap-3">
                            <span className="font-mono text-xs text-white/40">#{call.sequence}</span>
                            <span className="text-sm font-medium text-white/80">{call.tool || labels.noTool}</span>
                          </div>
                          <div className="flex items-center gap-2">
                            <span className="rounded bg-black/20 px-2 py-1 text-[10px] text-white/40">{call.phase}</span>
                            <svg className="h-4 w-4 text-white/40 transition-transform group-open:rotate-180" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                            </svg>
                          </div>
                        </summary>
                        <div className="flex flex-col gap-4 border-t border-white/5 bg-black/20 p-4">
                          <div className="flex items-center justify-between gap-3">
                            <div className="flex flex-col gap-1">
                              <span className="text-xs uppercase text-white/40">{labels.finishReason}</span>
                              <span className="text-sm text-white/80">{call.response?.finish_reason || labels.notAvailable}</span>
                            </div>
                            <div className="flex flex-col gap-1 text-right">
                              <span className="text-xs uppercase text-white/40">{labels.timestamp}</span>
                              <span className="text-xs text-white/80">{new Date(call.timestamp).toLocaleString()}</span>
                            </div>
                          </div>

                          {call.error ? (
                            <div className="flex flex-col gap-1 text-red-400">
                              <span className="text-xs uppercase">{labels.error}</span>
                              <span className="text-sm">{call.error}</span>
                            </div>
                          ) : null}

                          {call.request?.messages?.map((msg, i) => (
                            <div key={i} className="flex flex-col gap-1">
                              <span className="text-xs uppercase text-white/40">{msg.role}</span>
                              <div className="overflow-x-auto rounded-lg bg-black/40 p-3 font-mono text-xs whitespace-pre-wrap text-white/80">
                                {msg.content}
                              </div>
                            </div>
                          ))}

                          {call.response ? (
                            <div className="mt-2 flex flex-col gap-1">
                              <span className="text-xs uppercase text-green-400">{labels.response}</span>
                              <div className="overflow-x-auto rounded-lg border border-green-500/10 bg-green-500/5 p-3 font-mono text-xs whitespace-pre-wrap text-white/80">
                                {call.response.text}
                              </div>
                            </div>
                          ) : null}
                        </div>
                      </details>
                    ))
                  )}
                </div>
              ) : null}

              {activeTab === "events" ? (
                <div className="flex flex-col gap-4">
                  {!data.events?.length ? (
                    <p className="py-8 text-center text-sm text-white/40">{labels.noEvents}</p>
                  ) : (
                    <div className="relative pl-6">
                      <div className="absolute bottom-4 left-[11px] top-4 w-px bg-white/10"></div>
                      {data.events.map((event, idx) => (
                        <div key={event.id || idx} className="relative mb-6 last:mb-0">
                          <div className="absolute -left-[29px] top-1.5 h-3 w-3 rounded-full bg-white/20 ring-4 ring-[#1a1a1a]"></div>
                          <div className="rounded-xl border border-white/5 bg-white/5 p-4">
                            <div className="mb-2 flex items-start justify-between gap-3">
                              <strong className="text-sm text-white/90">{event.type}</strong>
                              <span className="text-[10px] text-white/40">{new Date(event.timestamp).toLocaleTimeString()}</span>
                            </div>
                            <div className="mb-2 text-xs text-white/60">
                              {labels.actor}: {event.actor}
                            </div>
                            {event.payload && Object.keys(event.payload).length > 0 ? (
                              <details className="group mt-2">
                                <summary className="cursor-pointer text-xs text-white/40 hover:text-white/60">
                                  {labels.payloadDetails}
                                </summary>
                                <div className="mt-2 overflow-x-auto rounded-lg bg-black/40 p-2">
                                  <pre className="m-0 font-mono text-[10px] text-white/80">
                                    {JSON.stringify(event.payload, null, 2)}
                                  </pre>
                                </div>
                              </details>
                            ) : null}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              ) : null}

              {activeTab === "memories" ? (
                <div className="overflow-hidden rounded-xl border border-white/5">
                  <JsonBlock value={{ info: labels.memoriesUnavailable }} />
                </div>
              ) : null}
            </div>
          )}
        </div>
      </div>
    </>
  );
}

function MetricCard({
  label,
  value,
  tone = "default",
}: {
  label: string;
  value: string;
  tone?: MetricTone;
}) {
  const classes =
    tone === "success"
      ? "bg-green-500/20 text-green-400"
      : tone === "error"
        ? "bg-red-500/20 text-red-400"
        : tone === "info"
          ? "bg-blue-500/20 text-blue-400"
          : "bg-white/10 text-white/80";

  return (
    <div className="rounded-xl border border-white/5 bg-white/5 p-4">
      <span className="mb-1 block text-xs text-white/40">{label}</span>
      <span className={`inline-flex rounded px-2 py-1 text-xs ${classes}`}>{value}</span>
    </div>
  );
}

function TodoRow({ todo }: { todo: TodoItem }) {
  return (
    <div className="rounded-xl border border-white/5 bg-black/20 p-3">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="m-0 text-sm text-white/90">{todo.content}</p>
          <p className="mt-2 mb-0 font-mono text-xs text-white/40">{todo.id}</p>
        </div>
        <span className={`inline-flex rounded px-2 py-1 text-[10px] ${todoStatusClass(todo.status)}`}>
          {todo.status}
        </span>
      </div>
    </div>
  );
}

function statusTone(status: string): MetricTone {
  if (status === "completed") {
    return "success";
  }
  if (status === "failed") {
    return "error";
  }
  if (status === "running") {
    return "info";
  }
  return "default";
}

function stepDotClass(status: string) {
  if (status === "completed") {
    return "bg-green-400";
  }
  if (status === "running") {
    return "bg-blue-400";
  }
  if (status === "failed") {
    return "bg-red-400";
  }
  return "bg-white/20";
}

function stepBadgeClass(status: string) {
  if (status === "completed") {
    return "bg-green-500/10 text-green-400";
  }
  if (status === "running") {
    return "bg-blue-500/10 text-blue-400";
  }
  if (status === "failed") {
    return "bg-red-500/10 text-red-400";
  }
  return "bg-white/10 text-white/60";
}

function todoStatusClass(status: string) {
  if (status === "done") {
    return "bg-green-500/10 text-green-400";
  }
  if (status === "in_progress") {
    return "bg-blue-500/10 text-blue-400";
  }
  return "bg-white/10 text-white/70";
}
