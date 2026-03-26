## 1. Runtime Observer Path

- [x] 1.1 为应用层增加可观察执行入口，使 run 执行过程可以实时向 observer 广播事件
- [x] 1.2 抽取统一事件写入 helper，保证事件在持久化时可选地同步推送给 observer
- [x] 1.3 保持现有 `StartRun` 同步路径兼容，避免影响 CLI 和现有 HTTP 调试 API

## 2. AG-UI Adapter

- [x] 2.1 新增 `internal/agui` 目录，定义 chat 请求体和最小 AG-UI 事件结构
- [x] 2.2 实现 runtime event 到 AG-UI event 的 mapper，覆盖 run、step、tool、message 和 `CUSTOM/RAW`
- [x] 2.3 实现 AG-UI SSE writer，支持按顺序输出事件并正确收口 `RUN_FINISHED / RUN_ERROR`
- [x] 2.4 实现 AG-UI service，协调 run 启动、observer 消费和 SSE 输出

## 3. HTTP API

- [x] 3.1 在 `internal/httpapi` 中新增 `POST /api/agui/chat` handler
- [x] 3.2 保持现有 `sessions / runs / replay / events` API 不变，确保调试台能力不回退
- [x] 3.3 为 AG-UI handler 补充测试，覆盖成功路径、错误路径和事件顺序的基本断言

## 4. Verification

- [x] 4.1 使用 mock provider 增加一条 AG-UI chat 联调验证路径
- [x] 4.2 更新 README 或相关开发文档，说明 AG-UI 聊天入口与现有调试 API 的分工
- [x] 4.3 增加一个最小 chat-first 前端页面，直接消费 `/api/agui/chat` 验证真实对话体验
