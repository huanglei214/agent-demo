import { useEffect, useState } from "react";
import { inspectRun } from "../../lib/api";
import { InspectRunResponse } from "../../lib/types";
import { JsonBlock } from "../JsonBlock";

type RunDebugDrawerProps = {
  runId: string | null;
  onClose: () => void;
  language: "en" | "zh";
};

type Tab = "overview" | "plan" | "llm" | "events" | "memories";

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
          setError(err instanceof Error ? err.message : "Failed to load run details");
          setLoading(false);
        }
      });

    return () => {
      active = false;
    };
  }, [runId]);

  if (!runId) return null;

  const tabs: { id: Tab; label: string }[] = [
    { id: "overview", label: language === "zh" ? "概览" : "Overview" },
    { id: "plan", label: language === "zh" ? "计划" : "Plan" },
    { id: "llm", label: language === "zh" ? "模型请求" : "LLM Calls" },
    { id: "events", label: language === "zh" ? "事件流" : "Events" },
    { id: "memories", label: language === "zh" ? "记忆" : "Memories" },
  ];

  return (
    <>
      {/* Backdrop */}
      <div 
        className="fixed inset-0 bg-black/50 z-40 backdrop-blur-sm transition-opacity" 
        onClick={onClose}
      />
      
      {/* Drawer */}
      <div className="fixed inset-y-0 right-0 w-[600px] max-w-[90vw] bg-[#1a1a1a] shadow-2xl z-50 flex flex-col border-l border-white/10 animate-in slide-in-from-right duration-300">
        <div className="flex items-center justify-between p-4 border-b border-white/10 flex-shrink-0">
          <div>
            <h2 className="text-lg font-medium text-white/90">Run Debugger</h2>
            <p className="text-xs text-white/40 font-mono mt-1">{runId}</p>
          </div>
          <button 
            className="p-2 text-white/60 hover:text-white hover:bg-white/10 rounded-lg transition-colors"
            onClick={onClose}
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
          </button>
        </div>

        {/* Tabs */}
        <div className="flex overflow-x-auto border-b border-white/10 px-4 flex-shrink-0 hide-scrollbar">
          {tabs.map(tab => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`px-4 py-3 text-sm font-medium whitespace-nowrap border-b-2 transition-colors ${
                activeTab === tab.id 
                  ? "border-blue-500 text-blue-400" 
                  : "border-transparent text-white/60 hover:text-white hover:bg-white/5"
              }`}
            >
              {tab.label}
              {tab.id === 'llm' && data?.model_calls && (
                <span className="ml-2 text-xs bg-white/10 px-1.5 py-0.5 rounded-full">{data.model_calls.length}</span>
              )}
            </button>
          ))}
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-4">
          {loading ? (
            <div className="flex items-center justify-center h-full text-white/40">
              <svg className="animate-spin -ml-1 mr-3 h-5 w-5 text-white/40" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              Loading...
            </div>
          ) : error ? (
            <div className="text-red-400 p-4 bg-red-400/10 rounded-xl">{error}</div>
          ) : !data ? (
            <div className="flex items-center justify-center h-full text-white/40">No data available</div>
          ) : (
            <div className="h-full">
              {activeTab === "overview" && (
                <div className="flex flex-col gap-4">
                  <div className="grid grid-cols-2 gap-4">
                    <div className="bg-white/5 rounded-xl p-4 border border-white/5">
                      <span className="text-xs text-white/40 block mb-1">Status</span>
                      <span className={`inline-flex px-2 py-1 rounded text-xs ${
                        data.run.status === 'completed' ? 'bg-green-500/20 text-green-400' :
                        data.run.status === 'failed' ? 'bg-red-500/20 text-red-400' :
                        data.run.status === 'running' ? 'bg-blue-500/20 text-blue-400' :
                        'bg-white/10 text-white/80'
                      }`}>{data.run.status}</span>
                    </div>
                    <div className="bg-white/5 rounded-xl p-4 border border-white/5">
                      <span className="text-xs text-white/40 block mb-1">Model</span>
                      <span className="text-sm text-white/90">{data.run.provider} / {data.run.model}</span>
                    </div>
                    <div className="bg-white/5 rounded-xl p-4 border border-white/5">
                      <span className="text-xs text-white/40 block mb-1">Turns</span>
                      <span className="text-sm text-white/90">{data.run.turn_count} / {data.run.max_turns}</span>
                    </div>
                    <div className="bg-white/5 rounded-xl p-4 border border-white/5">
                      <span className="text-xs text-white/40 block mb-1">Events</span>
                      <span className="text-sm text-white/90">{data.event_count}</span>
                    </div>
                  </div>
                  <div className="bg-white/5 rounded-xl p-4 border border-white/5">
                    <span className="text-xs text-white/40 block mb-2">Task ID</span>
                    <p className="text-sm text-white/80 whitespace-pre-wrap">{data.run.task_id || "N/A"}</p>
                  </div>
                </div>
              )}

              {activeTab === "plan" && (
                <div className="flex flex-col gap-4">
                  {!data.plan ? (
                    <p className="text-white/40 text-sm text-center py-8">No plan generated</p>
                  ) : (
                    <div className="flex flex-col gap-4">
                      <div className="bg-white/5 p-4 rounded-xl border border-white/5">
                        <strong className="text-sm text-white/80 block mb-2">Goal</strong>
                        <p className="text-sm text-white/60">{data.plan.goal}</p>
                      </div>
                      <div className="relative pl-6">
                        <div className="absolute left-[11px] top-4 bottom-4 w-px bg-white/10"></div>
                        {data.plan.steps.map((step, idx) => (
                          <div key={step.id} className="relative mb-6 last:mb-0">
                            <div className={`absolute -left-[29px] top-1.5 w-3 h-3 rounded-full ring-4 ring-[#1a1a1a] ${
                              step.status === 'completed' ? 'bg-green-400' :
                              step.status === 'running' ? 'bg-blue-400' :
                              step.status === 'failed' ? 'bg-red-400' :
                              'bg-white/20'
                            }`}></div>
                            <div className="bg-white/5 border border-white/5 rounded-xl p-4">
                              <div className="flex justify-between items-start mb-2">
                                <strong className="text-sm text-white/90">Step {idx + 1}: {step.title}</strong>
                                <span className={`text-[10px] px-1.5 py-0.5 rounded ${
                                  step.status === 'completed' ? 'bg-green-500/10 text-green-400' :
                                  step.status === 'running' ? 'bg-blue-500/10 text-blue-400' :
                                  step.status === 'failed' ? 'bg-red-500/10 text-red-400' :
                                  'bg-white/10 text-white/60'
                                }`}>{step.status}</span>
                              </div>
                              <p className="text-sm text-white/60">{step.description}</p>
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              )}

              {activeTab === "llm" && (
                <div className="flex flex-col gap-4">
                  {!data.model_calls?.length ? (
                    <p className="text-white/40 text-sm text-center py-8">No model calls recorded</p>
                  ) : (
                    data.model_calls.map((call, idx) => (
                      <details key={idx} className="group bg-white/5 border border-white/5 rounded-xl overflow-hidden">
                        <summary className="p-4 cursor-pointer flex items-center justify-between hover:bg-white/5 transition-colors">
                          <div className="flex items-center gap-3">
                            <span className="text-white/40 font-mono text-xs">#{call.sequence}</span>
                            <span className="text-sm font-medium text-white/80">{call.tool || 'No Tool'}</span>
                          </div>
                          <div className="flex items-center gap-2">
                            <span className="text-[10px] text-white/40 bg-black/20 px-2 py-1 rounded">{call.phase}</span>
                            <svg className="w-4 h-4 text-white/40 transition-transform group-open:rotate-180" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                            </svg>
                          </div>
                        </summary>
                        <div className="p-4 border-t border-white/5 bg-black/20 flex flex-col gap-4">
                          <div className="flex items-center justify-between">
                            <div className="flex flex-col gap-1">
                               <span className="text-xs text-white/40 uppercase">Finish Reason</span>
                               <span className="text-sm text-white/80">{call.response?.finish_reason || 'N/A'}</span>
                            </div>
                            <div className="flex flex-col gap-1 text-right">
                               <span className="text-xs text-white/40 uppercase">Timestamp</span>
                               <span className="text-xs text-white/80">{new Date(call.timestamp).toLocaleString()}</span>
                            </div>
                          </div>
                          
                          {call.error && (
                            <div className="flex flex-col gap-1 text-red-400">
                               <span className="text-xs uppercase">Error</span>
                               <span className="text-sm">{call.error}</span>
                            </div>
                          )}

                          {call.request?.messages?.map((msg, i) => (
                            <div key={i} className="flex flex-col gap-1">
                              <span className="text-xs text-white/40 uppercase">{msg.role}</span>
                              <div className="text-xs text-white/80 font-mono whitespace-pre-wrap bg-black/40 p-3 rounded-lg overflow-x-auto">
                                {msg.content}
                              </div>
                            </div>
                          ))}

                          {call.response && (
                            <div className="flex flex-col gap-1 mt-2">
                              <span className="text-xs text-green-400 uppercase">Response</span>
                              <div className="text-xs text-white/80 font-mono whitespace-pre-wrap bg-green-500/5 border border-green-500/10 p-3 rounded-lg overflow-x-auto">
                                {call.response.text}
                              </div>
                            </div>
                          )}
                        </div>
                      </details>
                    ))
                  )}
                </div>
              )}

              {activeTab === "events" && (
                <div className="flex flex-col gap-4">
                  {!data.events?.length ? (
                    <p className="text-white/40 text-sm text-center py-8">No events recorded</p>
                  ) : (
                    <div className="relative pl-6">
                      <div className="absolute left-[11px] top-4 bottom-4 w-px bg-white/10"></div>
                      {data.events.map((event, idx) => (
                        <div key={event.id || idx} className="relative mb-6 last:mb-0">
                          <div className="absolute -left-[29px] top-1.5 w-3 h-3 rounded-full ring-4 ring-[#1a1a1a] bg-white/20"></div>
                          <div className="bg-white/5 border border-white/5 rounded-xl p-4">
                            <div className="flex justify-between items-start mb-2">
                              <strong className="text-sm text-white/90">{event.type}</strong>
                              <span className="text-[10px] text-white/40">{new Date(event.timestamp).toLocaleTimeString()}</span>
                            </div>
                            <div className="text-xs text-white/60 mb-2">
                              Actor: {event.actor}
                            </div>
                            {event.payload && Object.keys(event.payload).length > 0 && (
                              <details className="group mt-2">
                                <summary className="text-xs text-white/40 cursor-pointer hover:text-white/60">Payload Details</summary>
                                <div className="mt-2 bg-black/40 rounded-lg p-2 overflow-x-auto">
                                  <pre className="text-[10px] text-white/80 font-mono m-0">{JSON.stringify(event.payload, null, 2)}</pre>
                                </div>
                              </details>
                            )}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {activeTab === "memories" && (
                <div className="rounded-xl border border-white/5 overflow-hidden">
                  <JsonBlock value={{ info: "Memories not available in this view" }} />
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </>
  );
}