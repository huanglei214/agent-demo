# agent-demo

本项目当前是一个单机本地运行的 Agent Harness MVP，已经具备：

- Cobra CLI 入口
- 本地 `.runtime/` 工件落盘
- `run / chat / inspect / session inspect / replay / resume / tools list / debug events` 基础命令
- 本地 `serve` 命令和首批 Web API
- Ark / Mock provider
- 最小 plan-driven loop：`planning -> memory recall -> context build -> prompt build -> model -> tool -> result`
- 基于 `Session` 的本地多轮对话
- 最小受控 delegation
- 固定 4 条回归场景，可通过 `make verify-scenarios` 统一验证
- `web/` 下的本地 React + Vite 调试界面骨架

## 快速开始

先查看 CLI：

```bash
make help
```

创建一次本地 run：

```bash
make run ARGS='verify scaffold runtime'
```

在已有 session 上追加一轮输入：

```bash
make run SESSION=<session-id> ARGS='继续刚才的话题'
```

启动本地交互式多轮对话：

```bash
make chat PROVIDER=mock
```

如果你想强制使用 mock provider：

```bash
make run ARGS='say hello from mock provider' PROVIDER=mock
```

查看工具列表：

```bash
make tools
```

启动本地 HTTP API：

```bash
make serve PROVIDER=mock
```

一条命令同时启动后端和前端：

```bash
make dev PROVIDER=mock
```

启动本地 Web UI：

```bash
cd web
npm install
npm run dev
```

默认情况下，Vite 会把 `/api` 和 `/healthz` 代理到 `http://127.0.0.1:8080`。

查看某个 run：

```bash
make inspect RUN=<run-id>
```

查看某个 session：

```bash
make session-inspect SESSION=<session-id>
```

回放某个 run 的事件：

```bash
make replay RUN=<run-id>
```

查看某个 run 的原始事件：

```bash
make debug-events RUN=<run-id>
```

## 环境变量

真实调用 Ark 时需要提供：

```bash
export ARK_API_KEY=your_api_key
export ARK_BASE_URL=https://ark.cn-beijing.volces.com/api/v3
export ARK_MODEL_ID=your_model_id
```

可选变量：

```bash
export HARNESS_PROVIDER=ark
export HARNESS_MODEL=$ARK_MODEL_ID
```

如果只是本地验证流程，可以直接切到 mock：

```bash
make run ARGS='请读取 README.md 并总结当前项目状态' PROVIDER=mock
make verify-scenarios
```

## 最小闭环示例

1. 发起一次运行：

```bash
make run ARGS='请读取 README.md 并总结当前项目状态' PROVIDER=mock
```

2. 记下输出里的 `run.id`，然后查看运行工件：

```bash
make inspect RUN=<run-id>
make replay RUN=<run-id>
make debug-events RUN=<run-id>
make resume RUN=<run-id>
```

这里有两个查看视角：

- `make replay`：输出更适合人读的摘要时间线
- `make debug-events`：输出原始事件 JSON，适合排障

3. 查看本地工件目录：

```bash
ls .runtime/runs/<run-id>
```

你会看到 `run.json`、`state.json`、`plan.json`、`events.jsonl`、`result.json`、`summaries.json`、`memories.json`。
如果这次运行触发了子代理委派，还会看到 `children/` 目录，里面保存每个 child run 的结构化记录。

4. 启动一个多轮会话：

```bash
make chat PROVIDER=mock
```

你会先看到一行 `session_id: ...`，然后可以连续输入多轮内容。输入 `/exit` 或 `/quit` 结束会话。
在真实 terminal 中，`chat` 现在支持：

- 上下箭头回看当前 REPL 会话里的历史输入
- 同一个 `session` 重开后，继续使用上下箭头找回之前的输入历史
- 行尾输入 `\` 继续下一行，形成多行消息
- `/session` 查看当前 session id
- `/history` 查看最近 10 条会话消息，或用 `/history 20` 查看更多
- `/clear` 清空当前终端显示

例如：

```text
you> 请帮我整理下面这段需求\
...> 并给出一个计划
```

5. 如果要继续之前的会话：

```bash
make chat PROVIDER=mock SESSION=<session-id>
```

或者：

```bash
make run PROVIDER=mock SESSION=<session-id> ARGS='补充一个追问'
```

6. 运行固定回归场景：

```bash
make verify-scenarios
```

当前会跑 4 条固定场景：

- 基础规划
- 文件系统工具
- 多轮 chat
- delegation

## 本地 Web UI

本地 Web UI 位于 [web/package.json](/Users/huanglei/repos/src/github.com/huanglei214/agent-demo/web/package.json)，当前是一个面向本机开发的薄页面层，重点用于：

- 发起 run
- 在首页直接查看 recent sessions / recent runs，并点进详情页
- 在首页选择某个 session 后直接查看最近消息和关联 runs，并继续追加一轮输入
- 查看 session 消息和关联 run
- 查看 run inspect、replay timeline、raw events
- 在 run 详情页通过 SSE 订阅增量事件，实时刷新时间线和状态指标

推荐启动方式：

1. 一条命令同时启动后端 API 和前端：

```bash
make dev PROVIDER=mock
```

2. 如果你想分开调试，也可以继续分别启动：

```bash
make serve PROVIDER=mock
make web-dev
```

3. 打开 Vite 输出的本地地址，默认会进入：

- `/`：Launchpad + recent sessions / recent runs
- `/sessions/<session-id>`：Session 详情页
- `/runs/<run-id>`：Run 详情页

`Run` 详情页在首屏会先加载一次 `inspect / replay / events`，随后自动连接 `/api/runs/<run-id>/stream?after=<sequence>`，用 SSE 追增量事件；当 run 进入 `completed / failed / cancelled / blocked` 时，这条连接会自动结束。

这个 server 当前是单 workspace 绑定的：启动 `serve` 时绑定哪个 `--workspace`，HTTP API 就只接受该 workspace 下的 session/run 请求，不支持跨 workspace 复用。

## `.runtime` 工件说明

`run` 级目录位于 `.runtime/runs/<run-id>/`，常见文件包括：

- `run.json`：本次运行的元信息、状态、provider、current step
- `state.json`：恢复执行所需的中间状态，例如 `turn_count`、`resume_phase`
- `plan.json`：当前计划和步骤状态
- `events.jsonl`：按顺序追加的完整事件流
- `result.json`：最终输出结果
- `summaries.json`：compaction 生成的摘要
- `memories.json`：本次运行的 recall、candidate、committed memory
- `children/`：child run 的结构化结果记录

`session` 级目录位于 `.runtime/sessions/<session-id>/`：

- `session.json`：session 元信息
- `messages.jsonl`：多轮 user / assistant 消息历史
- `input.history`：TTY chat 输入历史

## 说明

- 当前版本会创建 `Task / Session / Run / Plan / State / Events` 工件。
- 当前版本已经支持本地多轮对话：同一个 `Session` 下可以连续创建多个 `Run`。
- 当前版本已经接上最小 plan-driven agent loop：会先生成结构化计划，再进行 memory recall、context build、prompt build，并支持单次文件系统工具调用后生成最终答案。
- 当前版本已经有结构化 memory recall、compaction 和 memory candidate write-back。
- 当前版本已经支持最小受控 delegation：父 run 可以生成 child run、记录 `subagent.spawned / subagent.completed / subagent.rejected` 事件，并在 `children/` 目录中持久化 child 结果；但它仍然不是开放式多代理系统。
- `resume` 现在已经可以基于持久化的 `run.json`、`state.json`、`plan.json` 继续执行未完成运行，并支持恢复到 `post_tool` 之后的续跑阶段；terminal 状态和 `blocked` 状态不会自动恢复。
- 所有运行工件默认写入仓库根目录下的 `.runtime/`。
- session 级消息历史会保存在 `.runtime/sessions/<session-id>/messages.jsonl`，并以最近消息的形式注入后续轮次的上下文。
- `inspect` 现在会返回 current step、最近失败事件和 child run 摘要；`session inspect` 会返回最近消息和关联 run 列表。
- `verify-scenarios` 会固定验证基础规划、文件系统工具、多轮 chat、delegation 四条场景。
