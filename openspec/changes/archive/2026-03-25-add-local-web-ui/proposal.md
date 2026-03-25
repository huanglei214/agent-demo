## Why

当前项目已经具备稳定的本地 Agent Harness 运行时，但所有能力仍主要通过 CLI 暴露。为了让 `run / session / inspect / replay / events` 这些能力更容易观察和交互，需要增加一个面向本机开发的 Web UI，以及一层足够薄的本地 HTTP API 来承接前端访问。

## What Changes

- 新增一个本地 Web UI，用于展示最近会话、运行详情、摘要时间线和原始事件。
- 新增一个基于 `chi` 的本地 HTTP server，复用现有 `app.Services` 暴露 JSON API。
- 增加 `serve` 入口，用于本地启动 HTTP API，并为后续 SSE/AG-UI-compatible 适配预留边界。
- 增加 `make dev` 一键联调入口，便于同时启动后端 API 和前端开发服务器。
- 第一版 API 聚焦 `sessions`、`runs`、`replay/events` 和 `tools`，不引入鉴权、多用户或远程部署能力。
- 前端补充首页 continuation 面板、错误边界、中英文切换，以及 run 详情页的 SSE 自动重连能力。

## Capabilities

### New Capabilities
- `local-web-ui`: 本地 Web 界面与页面交互能力，包括会话查看、运行查看、时间线与事件面板，以及基础 run 发起能力。

### Modified Capabilities
- `harness-runtime-core`: 增加本地 HTTP server 入口，并允许通过 HTTP API 复用现有运行时与调试能力。

## Impact

- 影响 `cmd/harness`、`internal/app` 和新增的 `internal/httpapi` 路由/handler 层。
- 引入 `chi` 作为本地 HTTP 路由框架。
- 新增前端工程目录，例如 `web/`，用于 React + Vite + TypeScript 页面。
- 会新增本地 API 合同，但不会改变现有 `.runtime` 文件型工件和 CLI 运行模型。
