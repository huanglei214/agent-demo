import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

export type Language = "en" | "zh";

type Dictionary = {
  common: {
    loading: string;
    unknown: string;
    notAvailable: string;
    openDetails: string;
    openSession: string;
    openRun: string;
    clear: string;
    createSessionOnly: string;
  };
  app: {
    eyebrow: string;
    title: string;
    lede: string;
    navLaunchpad: string;
    navChat: string;
    navSession: string;
    navRun: string;
    routingNotesTitle: string;
    routingNotesBody: string;
    usefulPathsTitle: string;
    health: {
      checking: string;
      connected: string;
      serverOffline: string;
    };
    paths: {
      home: string;
      chat: string;
      session: string;
      run: string;
      api: string;
    };
    language: {
      label: string;
      english: string;
      chinese: string;
    };
  };
  errorBoundary: {
    eyebrow: string;
    title: string;
    body: string;
    back: string;
    reload: string;
    unexpected: string;
  };
  launcher: {
    title: string;
    body: string;
    continuingSession: string;
    instruction: string;
    instructionPlaceholder: string;
    provider: string;
    model: string;
    maxTurns: string;
    sessionId: string;
    sessionPlaceholder: string;
    launching: string;
    startRun: string;
    createSessionFailed: string;
    createRunFailed: string;
    recentSessions: string;
    noSessions: string;
    sessionRunsCount: (count: number) => string;
    continueHere: string;
    selectedSessionTitle: string;
    selectedSessionBody: string;
    recentMessages: string;
    noMessages: string;
    sessionRuns: string;
    noSessionRuns: string;
    loadingSessionContext: string;
    recentRuns: string;
    noRuns: string;
    runsCount: (count: number) => string;
    noInstruction: string;
    sessionLabel: string;
    updatedLabel: string;
    currentStep: string;
    availableTools: string;
    latestPayload: string;
    payloadHint: string;
    toolAccess: {
      read_only: string;
      write: string;
    };
  };
  session: {
    title: string;
    body: string;
    messages: string;
    runs: string;
    noMessages: string;
    noRuns: string;
    rawPayload: string;
    payloadHint: string;
    currentStep: string;
  };
  run: {
    title: string;
    body: string;
    runStatus: string;
    currentStep: string;
    events: string;
    childRuns: string;
    stream: string;
    replayTimeline: string;
    noReplayEntries: string;
    inspectPayload: string;
    inspectHint: string;
    rawEvents: string;
    streamStates: {
      connecting: string;
      reconnecting: string;
      live: string;
      disconnected: string;
      error: string;
      done: string;
    };
    status: Record<string, string>;
  };
  chat: {
    title: string;
    body: string;
    headerTitle: string;
    sidebarTitle: string;
    newChat: string;
    currentSession: string;
    currentRun: string;
    quickPromptTitle: string;
    quickPromptInspect: string;
    quickPromptDebug: string;
    quickPromptSummarize: string;
    historyTitle: string;
    historyToday: string;
    historyThisWeek: string;
    historyOlder: string;
    refreshHistory: string;
    noHistory: string;
    drawerTitle: string;
    activityDrawerToggle: string;
    emptyKicker: string;
    emptyTitle: string;
    emptyBody: string;
    lastPromptLabel: string;
    sessionId: string;
    sessionPlaceholder: string;
    provider: string;
    model: string;
    maxTurns: string;
    prompt: string;
    promptPlaceholder: string;
    send: string;
    sending: string;
    newSessionHint: string;
    messagesTitle: string;
    activityTitle: string;
    stateTitle: string;
    emptyMessages: string;
    emptyActivity: string;
    failureTitle: string;
    failureHint: string;
    eventCount: string;
    threadLabel: string;
    runLabel: string;
    streamStatus: string;
    live: string;
    idle: string;
    finished: string;
    failed: string;
  };
  messages: {
    role: Record<"user" | "assistant", string>;
  };
};

type I18nContextValue = {
  language: Language;
  setLanguage: (language: Language) => void;
  copy: Dictionary;
  formatRelativeTime: (value: string) => string;
  formatRunStatus: (value: string) => string;
  formatToolAccess: (value: string) => string;
  formatMessageRole: (value: "user" | "assistant") => string;
  formatStreamState: (value: string) => string;
};

const dictionaries: Record<Language, Dictionary> = {
  en: {
    common: {
      loading: "Loading…",
      unknown: "unknown",
      notAvailable: "n/a",
      openDetails: "Open details",
      openSession: "Open session",
      openRun: "Open run",
      clear: "Clear",
      createSessionOnly: "Create session only",
    },
    app: {
      eyebrow: "Local Agent Harness",
      title: "Runtime cockpit for sessions, runs, and event traces.",
      lede: "Start runs, inspect session history, and drill into replay timelines without leaving the browser.",
      navLaunchpad: "Launchpad",
      navChat: "Chat",
      navSession: "Session view",
      navRun: "Run view",
      routingNotesTitle: "Routing notes",
      routingNotesBody:
        "This first pass keeps routing lightweight on purpose. We only need direct navigation to the launcher, one session page, and one run page.",
      usefulPathsTitle: "Useful paths",
      health: {
        checking: "checking",
        connected: "connected",
        serverOffline: "server offline",
      },
      paths: {
        home: "/",
        chat: "/chat",
        session: "/sessions/<session-id>",
        run: "/runs/<run-id>",
        api: "/api/* via Vite proxy -> 127.0.0.1:8080",
      },
      language: {
        label: "Language",
        english: "EN",
        chinese: "中文",
      },
    },
    errorBoundary: {
      eyebrow: "UI Recovery",
      title: "Something broke in the page layer.",
      body: "The runtime is still there, but this screen hit a rendering error. You can jump back to the launcher or reload the browser and keep going.",
      back: "Back to launcher",
      reload: "Reload page",
      unexpected: "Unexpected UI error",
    },
    launcher: {
      title: "Launch a run",
      body: "Use this page as a local control desk. You can start a standalone run or attach it to a session you already created.",
      continuingSession: "Continuing session",
      instruction: "Instruction",
      instructionPlaceholder: "Ask the harness to inspect code, call tools, or delegate a subtask.",
      provider: "Provider",
      model: "Model",
      maxTurns: "Max turns",
      sessionId: "Session ID",
      sessionPlaceholder: "Leave empty to create a fresh session automatically.",
      launching: "Launching…",
      startRun: "Start run",
      createSessionFailed: "failed to create session",
      createRunFailed: "failed to create run",
      recentSessions: "Recent sessions",
      noSessions: "No persisted sessions yet.",
      sessionRunsCount: (count) => `${count} runs in this session`,
      continueHere: "Continue here",
      selectedSessionTitle: "Selected session context",
      selectedSessionBody: "Recent messages and runs for",
      recentMessages: "Recent messages",
      noMessages: "No messages yet for this session.",
      sessionRuns: "Session runs",
      noSessionRuns: "No runs yet for this session.",
      loadingSessionContext: "Loading session context…",
      recentRuns: "Recent runs",
      noRuns: "No runs yet.",
      runsCount: (count) => `${count} runs`,
      noInstruction: "No instruction summary available.",
      sessionLabel: "session",
      updatedLabel: "updated",
      currentStep: "current step",
      availableTools: "Available tools",
      latestPayload: "Latest API payload",
      payloadHint: "Submit a run to inspect the response payload.",
      toolAccess: {
        read_only: "read only",
        write: "write",
      },
    },
    session: {
      title: "Session",
      body: "This view focuses on conversational continuity: recent messages and the run chain connected to the same session.",
      messages: "Messages",
      runs: "Runs",
      noMessages: "No messages persisted for this session yet.",
      noRuns: "No runs linked to this session.",
      rawPayload: "Raw payload",
      payloadHint: "Load a real session id to inspect the payload.",
      currentStep: "current step",
    },
    run: {
      title: "Run",
      body: "Inspect live state, child runs, summary replay, and raw events side by side.",
      runStatus: "Run status",
      currentStep: "Current step",
      events: "Events",
      childRuns: "Child runs",
      stream: "Stream",
      replayTimeline: "Replay timeline",
      noReplayEntries: "No replay entries loaded.",
      inspectPayload: "Inspect payload",
      inspectHint: "Load a real run id to inspect state.",
      rawEvents: "Raw events",
      streamStates: {
        connecting: "connecting",
        reconnecting: "reconnecting",
        live: "live",
        disconnected: "disconnected",
        error: "error",
        done: "done",
      },
      status: {
        unknown: "unknown",
        pending: "pending",
        running: "running",
        blocked: "blocked",
        failed: "failed",
        completed: "completed",
        cancelled: "cancelled",
      },
    },
    chat: {
      title: "Chat-first playground",
      body: "Use the AG-UI adapter to stream one conversation turn at a time while keeping the existing debug APIs for deeper inspection.",
      headerTitle: "Agent Chat",
      sidebarTitle: "Workspace Chat",
      newChat: "New chat",
      currentSession: "Current session",
      currentRun: "Current run",
      quickPromptTitle: "Quick prompts",
      quickPromptInspect: "Inspect the current repository and highlight risky areas.",
      quickPromptDebug: "Help me debug the latest regression in this project.",
      quickPromptSummarize: "Summarize what has been completed in this repository.",
      historyTitle: "Recent sessions",
      historyToday: "Today",
      historyThisWeek: "Last 7 days",
      historyOlder: "Older",
      refreshHistory: "Refresh",
      noHistory: "No recent sessions yet.",
      drawerTitle: "Context and activity",
      activityDrawerToggle: "Activity",
      emptyKicker: "AG-UI connected",
      emptyTitle: "What should we work on next?",
      emptyBody: "Use the chat surface as the primary way to talk to the harness. Steps and tool activity stay visible without taking over the screen.",
      lastPromptLabel: "Last prompt",
      sessionId: "Session ID",
      sessionPlaceholder: "Leave empty to let the server create a session.",
      provider: "Provider",
      model: "Model",
      maxTurns: "Max turns",
      prompt: "Message",
      promptPlaceholder: "Ask the agent to inspect code, explain a bug, or summarize the repo.",
      send: "Send message",
      sending: "Streaming…",
      newSessionHint: "This page talks to /api/agui/chat and will keep the latest thread / run ids once the stream starts.",
      messagesTitle: "Messages",
      activityTitle: "Live activity",
      stateTitle: "Latest state",
      emptyMessages: "No messages yet. Send one to start the stream.",
      emptyActivity: "No activity events yet.",
      failureTitle: "Latest failure",
      failureHint: "Check the terminal output for [api] logs if you need the backend trace.",
      eventCount: "events",
      threadLabel: "thread",
      runLabel: "run",
      streamStatus: "stream",
      live: "live",
      idle: "idle",
      finished: "finished",
      failed: "failed",
    },
    messages: {
      role: {
        user: "user",
        assistant: "assistant",
      },
    },
  },
  zh: {
    common: {
      loading: "加载中…",
      unknown: "未知",
      notAvailable: "暂无",
      openDetails: "查看详情",
      openSession: "打开会话",
      openRun: "打开运行",
      clear: "清空",
      createSessionOnly: "仅创建会话",
    },
    app: {
      eyebrow: "本地 Agent Harness",
      title: "在浏览器里查看会话、运行和事件轨迹的控制台。",
      lede: "直接在浏览器里发起运行、查看会话历史，并沿着 replay 时间线排查执行过程。",
      navLaunchpad: "启动台",
      navChat: "聊天页",
      navSession: "会话页",
      navRun: "运行页",
      routingNotesTitle: "路由说明",
      routingNotesBody: "这一版刻意保持轻量路由，只保留启动台、单个会话页和单个运行页的直接导航。",
      usefulPathsTitle: "常用路径",
      health: {
        checking: "检查中",
        connected: "已连接",
        serverOffline: "服务未启动",
      },
      paths: {
        home: "/",
        chat: "/chat",
        session: "/sessions/<session-id>",
        run: "/runs/<run-id>",
        api: "/api/* 通过 Vite 代理到 127.0.0.1:8080",
      },
      language: {
        label: "语言",
        english: "EN",
        chinese: "中文",
      },
    },
    errorBoundary: {
      eyebrow: "界面恢复",
      title: "页面渲染出了点问题。",
      body: "运行时本身还在，只是当前界面遇到了渲染错误。你可以先回到启动台，或者刷新页面后继续。",
      back: "回到启动台",
      reload: "刷新页面",
      unexpected: "界面出现未预期错误",
    },
    launcher: {
      title: "发起一次运行",
      body: "这里可以当作本地控制台来用。你可以直接启动一次独立运行，也可以把它挂到已有会话里继续。",
      continuingSession: "正在续写会话",
      instruction: "指令",
      instructionPlaceholder: "让 harness 检查代码、调用工具，或者委派一个子任务。",
      provider: "Provider",
      model: "模型",
      maxTurns: "最大轮数",
      sessionId: "会话 ID",
      sessionPlaceholder: "留空会自动创建一个新的会话。",
      launching: "启动中…",
      startRun: "开始运行",
      createSessionFailed: "创建会话失败",
      createRunFailed: "创建运行失败",
      recentSessions: "最近会话",
      noSessions: "还没有持久化会话。",
      sessionRunsCount: (count) => `这个会话里有 ${count} 次运行`,
      continueHere: "继续在这里聊",
      selectedSessionTitle: "当前会话上下文",
      selectedSessionBody: "下面是最近消息和运行记录：",
      recentMessages: "最近消息",
      noMessages: "这个会话里还没有消息。",
      sessionRuns: "会话运行记录",
      noSessionRuns: "这个会话里还没有运行记录。",
      loadingSessionContext: "正在加载会话上下文…",
      recentRuns: "最近运行",
      noRuns: "还没有运行记录。",
      runsCount: (count) => `${count} 次运行`,
      noInstruction: "暂无可用的指令摘要。",
      sessionLabel: "会话",
      updatedLabel: "更新于",
      currentStep: "当前步骤",
      availableTools: "可用工具",
      latestPayload: "最近一次 API 响应",
      payloadHint: "提交一次运行后，这里会显示响应 payload。",
      toolAccess: {
        read_only: "只读",
        write: "可写",
      },
    },
    session: {
      title: "会话",
      body: "这里聚焦在会话连续性：最近消息，以及挂在同一会话上的运行链路。",
      messages: "消息",
      runs: "运行",
      noMessages: "这个会话里还没有持久化消息。",
      noRuns: "这个会话下还没有关联运行。",
      rawPayload: "原始 payload",
      payloadHint: "输入一个真实的 session id 后，这里会展示完整 payload。",
      currentStep: "当前步骤",
    },
    run: {
      title: "运行",
      body: "并排查看实时状态、子运行、摘要时间线和原始事件。",
      runStatus: "运行状态",
      currentStep: "当前步骤",
      events: "事件数",
      childRuns: "子运行",
      stream: "流状态",
      replayTimeline: "回放时间线",
      noReplayEntries: "还没有加载到 replay 条目。",
      inspectPayload: "检查 payload",
      inspectHint: "输入真实的 run id 后，这里会展示状态快照。",
      rawEvents: "原始事件",
      streamStates: {
        connecting: "连接中",
        reconnecting: "重连中",
        live: "实时中",
        disconnected: "已断开",
        error: "出错",
        done: "已结束",
      },
      status: {
        unknown: "未知",
        pending: "等待中",
        running: "运行中",
        blocked: "被阻塞",
        failed: "失败",
        completed: "已完成",
        cancelled: "已取消",
      },
    },
    chat: {
      title: "聊天实验页",
      body: "这里直接走 AG-UI adapter，以聊天视角实时展示消息、步骤和工具调用；更深的排障仍然交给现有调试页。",
      headerTitle: "Agent 对话",
      sidebarTitle: "工作区聊天",
      newChat: "新对话",
      currentSession: "当前会话",
      currentRun: "当前运行",
      quickPromptTitle: "快捷问题",
      quickPromptInspect: "检查当前仓库，并指出最值得关注的风险点。",
      quickPromptDebug: "帮我排查这个项目里最新出现的回归问题。",
      quickPromptSummarize: "总结这个仓库目前已经完成了哪些工作。",
      historyTitle: "最近会话",
      historyToday: "今天",
      historyThisWeek: "近 7 天",
      historyOlder: "更早",
      refreshHistory: "刷新",
      noHistory: "还没有最近会话。",
      drawerTitle: "上下文与活动",
      activityDrawerToggle: "活动抽屉",
      emptyKicker: "AG-UI 已连接",
      emptyTitle: "接下来我们做什么？",
      emptyBody: "把这里当成主聊天界面来用。步骤变化和工具活动会保留，但不会再把整屏都挤满。",
      lastPromptLabel: "上一条问题",
      sessionId: "会话 ID",
      sessionPlaceholder: "留空会由服务端自动创建一个会话。",
      provider: "Provider",
      model: "模型",
      maxTurns: "最大轮数",
      prompt: "消息",
      promptPlaceholder: "让 agent 检查代码、解释 bug，或者总结仓库状态。",
      send: "发送消息",
      sending: "流式返回中…",
      newSessionHint: "这个页面会直接请求 /api/agui/chat，并在流开始后记住最新的 thread / run 标识。",
      messagesTitle: "消息流",
      activityTitle: "实时活动",
      stateTitle: "最新状态",
      emptyMessages: "还没有消息，先发一条开始。",
      emptyActivity: "还没有活动事件。",
      failureTitle: "最近错误",
      failureHint: "如果需要后端堆栈或请求细节，请查看终端里的 [api] 日志。",
      eventCount: "个事件",
      threadLabel: "线程",
      runLabel: "运行",
      streamStatus: "流状态",
      live: "实时中",
      idle: "空闲",
      finished: "已完成",
      failed: "失败",
    },
    messages: {
      role: {
        user: "用户",
        assistant: "助手",
      },
    },
  },
};

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [language, setLanguage] = useState<Language>(() => getInitialLanguage());

  useEffect(() => {
    window.localStorage.setItem("harness-ui-language", language);
  }, [language]);

  const copy = dictionaries[language];

  const value = useMemo<I18nContextValue>(
    () => ({
      language,
      setLanguage,
      copy,
      formatRelativeTime: (value) => formatRelativeTime(value, language),
      formatRunStatus: (value) => copy.run.status[value] ?? value,
      formatToolAccess: (value) => copy.launcher.toolAccess[value as "read_only" | "write"] ?? value,
      formatMessageRole: (value) => copy.messages.role[value] ?? value,
      formatStreamState: (value) => formatStreamState(value, copy),
    }),
    [copy, language],
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const context = useContext(I18nContext);
  if (!context) {
    throw new Error("useI18n must be used inside I18nProvider");
  }
  return context;
}

function getInitialLanguage(): Language {
  const saved = window.localStorage.getItem("harness-ui-language");
  if (saved === "en" || saved === "zh") {
    return saved;
  }
  return window.navigator.language.toLowerCase().startsWith("zh") ? "zh" : "en";
}

function formatRelativeTime(value: string, language: Language) {
  const date = new Date(value);
  const diffMs = Date.now() - date.getTime();
  const minute = 60_000;
  const hour = 60 * minute;
  const day = 24 * hour;

  if (diffMs < minute) {
    return language === "zh" ? "刚刚" : "just now";
  }
  if (diffMs < hour) {
    const amount = Math.max(1, Math.floor(diffMs / minute));
    return language === "zh" ? `${amount} 分钟前` : `${amount}m ago`;
  }
  if (diffMs < day) {
    const amount = Math.max(1, Math.floor(diffMs / hour));
    return language === "zh" ? `${amount} 小时前` : `${amount}h ago`;
  }
  const amount = Math.max(1, Math.floor(diffMs / day));
  return language === "zh" ? `${amount} 天前` : `${amount}d ago`;
}

function formatStreamState(value: string, copy: Dictionary) {
  if (value.startsWith("done:")) {
    const status = value.slice("done:".length);
    return `${copy.run.streamStates.done} · ${copy.run.status[status] ?? status}`;
  }
  return copy.run.streamStates[value as keyof Dictionary["run"]["streamStates"]] ?? value;
}
