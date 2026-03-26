## Why

当前仓库已经同时承载了两类入口：

- 面向终端交互的 CLI
- 面向本地 Web UI 的 HTTP server

但它们目前仍然混在同一个可执行入口和相邻目录里，例如 `cmd/harness` 同时承担 CLI 总入口，而 `internal/cli` 里又额外挂着 `serve` 命令。这让目录语义开始变得模糊：

- `cmd` 还没有明确区分不同启动方式
- `internal` 里混合了核心运行时、应用层和接口适配层
- 新加入的 Web / AG-UI 链路让“入口层”和“核心层”的边界更容易继续漂移

为了让项目结构更清晰，需要把 CLI 和 Web 的启动入口显式拆开，同时把接口适配代码从“看起来像核心”的位置收敛到更明确的层次里。

## What Changes

- 新增两个明确的可执行入口：
  - `cmd/cli`
  - `cmd/web`
- 将当前 `serve` 相关启动逻辑从 CLI 命令集中剥离，交给独立的 Web 入口负责。
- 收敛接口适配层命名，使 CLI / HTTP / AG-UI 不再和核心运行时包并列地散落在 `internal` 根层。
- 保持核心运行时、应用层和现有对外行为不变，不把这次结构调整做成功能变更。

## Capabilities

### Modified Capabilities
- `harness-runtime-core`: 运行时需要支持独立的 CLI 与 Web 入口，而不改变核心执行链和应用层契约。
- `local-web-ui`: 本地 Web UI 需要由独立 Web 入口启动，而不是作为 CLI 子命令的附属职责。

## Impact

- 影响 `cmd/` 目录结构和启动方式。
- 影响当前 `internal/cli`、`internal/httpapi`、`internal/agui` 的包组织。
- 会更新 README、Makefile 和开发脚本中的启动命令说明。
- 不应改变现有 CLI 能力、HTTP API 合同或前端行为。
