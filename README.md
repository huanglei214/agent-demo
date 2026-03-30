# agent-demo

本项目是一个本地运行的 Agent Harness。当前提供三类主要使用面：

- Web 聊天与调试页面
- Cobra CLI
- 本地 `.runtime/` 运行工件

当前核心能力包括：

- `chat` 主交互入口
- `debug` 调试命令集
- 本地 HTTP API 与 Web UI
- `mock` / `ark` 两种模型 provider
- plan-driven agent loop
- session 多轮对话
- 受控 subagent delegation
- 本地 skills

## 快速开始

推荐先从 Web 开始：

```bash
make dev
```

默认行为：

- API 监听 `http://127.0.0.1:8088`
- 前端通过 Vite 本地开发服务启动
- Web 页面会连接本地 API

如果你想用可重复的本地验证流，改用 `mock`：

```bash
make dev PROVIDER=mock
```

只启动后端：

```bash
make serve
```

只启动前端：

```bash
make web-dev
```

构建前端：

```bash
make web-build
```

## CLI

CLI 现在收敛成两层：

- `harness chat`：日常对话入口
- `harness debug ...`：调试和工件查看入口

查看帮助：

```bash
make help
```

启动多轮聊天：

```bash
make chat
```

做一次单次运行：

```bash
make run ARGS='请读取 README.md 并总结当前项目状态'
```

在已有 session 上继续一轮：

```bash
make run SESSION=<session-id> ARGS='继续刚才的话题'
```

常用调试命令：

```bash
make inspect RUN=<run-id>
make replay RUN=<run-id>
make debug-events RUN=<run-id>
make session-inspect SESSION=<session-id>
make resume RUN=<run-id>
make tools
```

如果你直接用原始 CLI，对应命令形态是：

```bash
harness chat
harness debug run <instruction>
harness debug inspect <run-id>
harness debug replay <run-id>
harness debug events <run-id>
harness debug session <session-id>
harness debug resume <run-id>
harness debug tools
```

## Provider 与配置

默认 `make` 命令会走 `ark`。如果你只想做本地验证，显式传：

```bash
make run PROVIDER=mock ARGS='hello'
make chat PROVIDER=mock
make verify-scenarios
```

工作区配置文件：

- `config.json`：非敏感配置
- `.env`：敏感配置，当前主要包括 `ARK_API_KEY` 和可选的 `TAVILY_API_KEY`

推荐做法：

```bash
cp config.example.json config.json
cp .env.example .env
```

`config.json` 示例：

```json
{
  "runtime": {
    "root": ".runtime"
  },
  "model": {
    "provider": "ark",
    "model": "ep-xxxxxxxx",
    "timeout_seconds": 90,
    "ark": {
      "base_url": "https://ark.cn-beijing.volces.com/api/v3",
      "api_key": "${ARK_API_KEY}",
      "model_id": "ep-xxxxxxxx",
      "tpm": 120000,
      "max_concurrent": 2
    }
  }
}
```

`.env` 示例：

```bash
ARK_API_KEY=your_api_key
TAVILY_API_KEY=your_api_key
```

如果配置了 `TAVILY_API_KEY`，`web.search` 和 `web.fetch` 会自动优先使用 Tavily。
当 Tavily 返回限流、5xx、超时、传输错误或空结果时，系统会自动回退到当前 DuckDuckGo 搜索和直接页面抓取实现。

配置覆盖优先级：

1. `<workspace>/config.json`
2. `<workspace>/.env`
3. 进程环境变量
4. 显式 CLI / Web 启动参数

Ark provider 额外支持两个限流配置：

- `model.ark.tpm`：每分钟 token 预算
- `model.ark.max_concurrent`：同时最多几个 Ark 请求

如果你在 Web 验证时经常碰到 429，优先配置这两个值，而不是靠重复点击重试。

## Web UI

本地 Web UI 位于 [web/](/Users/huanglei/repos/src/github.com/huanglei214/agent-demo/web)。

主要页面：

- `/`：chat-first 首页
- `/chat`：聊天页别名
- `/launchpad`：启动台
- `/sessions/<session-id>`：session 详情
- `/runs/<run-id>`：run 详情

当前 Web 主要用于：

- 发起聊天和运行
- 查看 recent sessions / runs
- 查看 session 消息与关联 runs
- 查看 run inspect、replay、raw events
- 通过 SSE 接收运行状态更新

前端默认会把 `/api` 和 `/healthz` 代理到 `http://127.0.0.1:8088`。

## Skills

项目支持文件系统原生 skills。

skill 目录优先级：

- 项目级：`skills/`
- 用户级：`~/.agent-demo/skills/`

每个 skill 目录入口为 `SKILL.md`，可附带：

- `references/`
- `scripts/`
- `assets/`

显式启用 skill：

```bash
make run SKILL=weather-lookup ARGS='武汉天气怎么样' PROVIDER=mock
make chat SKILL=weather-lookup PROVIDER=mock
```

## 工具面

当前默认工具：

- `fs.list_dir`
- `fs.read_file`
- `fs.write_file`
- `fs.str_replace`
- `fs.search`
- `web.search`
- `web.fetch`
- `bash.exec`

访问级别：

- `read_only`
- `write`
- `exec`

当前安全边界：

- `bash.exec` 会拦截明显危险命令和命令链
- `web.fetch` 会先拒绝本地和内网地址，再决定是否调用 Tavily 或直接抓取
- filesystem 工具会在解析 symlink 后继续校验 workspace 边界

查看工具清单：

```bash
make tools
curl -s http://127.0.0.1:8088/api/tools
```

## `.runtime` 工件

run 级目录：

```bash
.runtime/runs/<run-id>/
```

常见文件：

- `run.json`
- `state.json`
- `plan.json`
- `events.jsonl`
- `result.json`
- `summaries.json`
- `memories.json`
- `children/`

session 级目录：

```bash
.runtime/sessions/<session-id>/
```

常见文件：

- `session.json`
- `messages.jsonl`
- `input.history`

## AG-UI

项目提供一条 chat-first 的 AG-UI 兼容入口：

```bash
POST /api/agui/chat
```

这条入口更适合聊天流式体验；排障仍建议优先用：

- `inspect`
- `replay`
- `debug-events`
- Web 的 run / session 详情页

## 验证

常用验证命令：

```bash
make build
make verify-scenarios
go test ./...
```

当前固定回归场景包括：

- 基础规划
- 文件系统工具
- 多轮 chat
- delegation
