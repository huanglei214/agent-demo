## Why

当前本地 Web UI 已经具备调试台能力，但对话体验仍然偏“执行结果查看”，而不是“实时协作聊天”。为了在不推翻现有调试 API 的前提下改善对话体验，需要新增一条 AG-UI 兼容的真流式聊天链路，让前端能够以标准事件流实时观察消息、步骤和工具调用。

## What Changes

- 新增一条并存的 AG-UI 聊天入口，使用 HTTP + SSE 返回 AG-UI 兼容事件流。
- 为现有运行时执行链增加实时 observer 旁路，使 runtime event 在持久化之外还能实时推送给 AG-UI adapter。
- 新增 runtime event 到 AG-UI event 的映射层，优先映射标准事件，无法直接映射的语义通过 `CUSTOM/RAW` 承接。
- 保留现有 `sessions / runs / inspect / replay / events` API 和调试页面，不把本次变更做成对现有 Web UI 的替换。
- 增加一个最小 chat-first 前端页面，用真实页面验证 AG-UI 流是否明显改善对话体验。

## Capabilities

### New Capabilities
- `agui-chat`: AG-UI 兼容的聊天入口与事件映射能力，包括真流式消息、步骤、工具调用和运行生命周期事件。

### Modified Capabilities
- `harness-runtime-core`: 运行时执行链需要支持实时 observer 旁路，在事件持久化的同时对外发出可订阅的运行时事件。
- `local-web-ui`: 本地 Web UI 能力需要增加 AG-UI 聊天链路，并允许现有调试台与新的 chat-first 入口并存。

## Impact

- 影响 `internal/app/run_service.go` 和相关应用层服务边界，增加可观察执行入口。
- 新增 `internal/agui` 目录，用于请求解析、事件映射和 SSE 写出。
- 扩展 `internal/httpapi`，新增 `POST /api/agui/chat` 入口。
- 会引入新的 AG-UI 兼容 API 合同，但不会移除现有本地调试 API。
