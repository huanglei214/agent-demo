# AGENTS.md

本文件描述这个仓库的当前协作约定。目标是让进入项目的 agent 或开发者先快速建立正确心智，再开始改动。

## 项目定位

- 这是一个本地运行的 Agent Harness。
- 主要使用面有三类：
  - Web 聊天与调试页面
  - Cobra CLI
  - 本地 `.runtime/` 运行工件
- 当前默认 provider 是 `ark`，但做可重复本地验证时优先使用 `mock`。

## 主要入口

- Web 开发：
  - `make dev`
  - `make serve`
  - `make web-dev`
- CLI 主入口：
  - `harness chat`
  - `harness debug ...`
- 本地 API 默认监听：
  - `http://127.0.0.1:8088`

如果你只是想验证逻辑而不是联调真实模型，优先用：

```bash
make dev PROVIDER=mock
make run PROVIDER=mock ARGS='hello'
make chat PROVIDER=mock
```

## 当前架构

### 核心执行层

- `internal/agent/`
  - agent 执行引擎
  - action dispatch / parser
  - run loop
  - runtime policy 接入
- `internal/service/`
  - 对外服务编排层
  - run / session / inspect / replay / resume / tools
  - 通过 `agent.Runner` 调用执行层，不直接泄漏 executor 细节

### 领域模块

- `internal/planner/`
  - 规划与 replan 策略
- `internal/delegation/`
  - subagent delegation、child result 组装
- `internal/retrieval/`
  - 检索进展与 forced-final 收口保护
- `internal/context/`
  - context 构建与压缩
- `internal/memory/`
  - memory 存取与 recall
- `internal/prompt/`
  - prompt builder 与模板
- `internal/runtime/`
  - runtime 类型、错误、sequence、policy
- `internal/skill/`
  - skill registry

### 存储与接口适配

- `internal/store/`
  - store 接口与路径抽象
- `internal/store/filesystem/`
  - filesystem store 实现
- `internal/interfaces/cli/`
  - CLI 适配层
- `internal/interfaces/http/`
  - HTTP API 适配层
- `internal/interfaces/http/agui/`
  - AG-UI / SSE 映射层

### 前端与文档

- `web/`
  - React + TypeScript + Vite 本地 Web UI
- `skills/`
  - 项目级 skills
- `openspec/`
  - 主 specs 与 change archive
- `docs/superpowers/specs/`
  - 设计说明
- `docs/superpowers/plans/`
  - 实施计划

## 当前边界约定

- `service` 负责入口编排，不负责实现 agent 执行细节。
- `agent` 负责执行引擎，不反向依赖 `service`。
- `service` 与 `agent` 通过窄接口协作。
- `agent` / `service` 依赖 `store` 接口，不直接绑死 `filesystem` 具体类型。
- `Executor` 依赖按职责域分组管理，不要再把一长串依赖平铺回去。
- `internal/interfaces/` 保持为内部接口适配层，不要把它当成新的业务核心目录。

## 开发约定

- 优先做小步、可验证的改动，不要同时重写多个正交模块。
- 先遵循现有目录边界；如果要改边界，先确认会不会影响 CLI / HTTP / Web / `.runtime/` 工件。
- 用户可见行为变更时，至少同步这些面中的相关项：
  - `README.md`
  - `openspec/specs/`
  - 必要时补 `docs/superpowers/specs/` 或 `docs/superpowers/plans/`
- 运行时和 prompt 行为改动，优先保持：
  - chat-first 体验
  - lead / subagent 角色边界
  - `.runtime/` 工件兼容性

## 生成态与不应提交的内容

这些目录或文件属于生成态或本地产物，不应作为业务改动提交：

- `.runtime/`
- `bin/`
- `log/`
- `web/node_modules/`
- `web/dist/`
- `.gocache/`
- `.gomodcache/`
- `.tmp/`

另外：

- `.runtime/runs/<run-id>/children/` 是 child run 工件目录。
- 不要手工改运行工件来“修复”逻辑问题；优先修代码或测试。

## 工具与安全边界

当前默认工具面：

- `fs.list_dir`
- `fs.read_file`
- `fs.write_file`
- `fs.str_replace`
- `fs.search`
- `web.search`
- `web.fetch`
- `bash.exec`

当前安全边界：

- `bash.exec` 会拦截明显危险命令和命令链。
- `web.fetch` 会拒绝本地与内网地址。
- filesystem 工具会在解析 symlink 后继续校验 workspace 边界。

## 验证建议

按改动范围选择最小但足够的验证。

### 改 `internal/agent/`、`internal/service/`、delegation、planner、runtime policy

```bash
go test ./internal/agent ./internal/service
make verify-scenarios
```

### 改 memory、store、context、prompt

```bash
go test ./internal/memory ./internal/store/... ./internal/context ./internal/prompt
```

### 改工具实现

```bash
go test ./internal/tool/... ./internal/service
```

### 改 CLI / HTTP / Web

```bash
go test ./internal/interfaces/... ./cmd/...
make web-build
```

### 大范围改动或准备收口时

```bash
make build
go test ./...
```

## 工作方式建议

- 先读相关代码和测试，再改。
- 优先使用现有 helper、builder、store 接口和 service 构造路径，不要并行发明第二套装配方式。
- 如果一个改动会影响：
  - CLI 命令
  - HTTP 路由
  - SSE / AG-UI 事件
  - `.runtime/` 工件格式
  - OpenSpec 主 spec
  请显式检查这些面有没有同步。

## 一句话心智

这个仓库当前最重要的不是“继续加新层”，而是：

- 保持 `agent / service / store / interfaces` 的边界清楚
- 保持 Web、CLI、运行工件三条使用面一致
- 用 `mock` 做稳定验证，用 `ark` 做真实体验联调
