## MODIFIED Requirements

### Requirement: 本地 Web UI 入口
系统 MUST 提供一个面向本机开发的 Web UI，用于展示当前 Harness 的核心运行时信息。

#### Scenario: 调试台与聊天入口并存
- **WHEN** 系统新增 AG-UI 聊天入口
- **THEN** 现有基于 `sessions / runs / replay / events` 的调试台能力 MUST 继续可用
- **THEN** 系统 MUST 不要求现有本地 Web UI 页面全部迁移到 AG-UI 才能工作

### Requirement: 会话与运行详情查看
系统 MUST 在 Web UI 中展示 `Session` 和 `Run` 的核心信息，便于调试和观察执行过程。

#### Scenario: chat-first 页面接入 AG-UI 链路
- **WHEN** 后续聊天页面通过 AG-UI 入口发起一次对话
- **THEN** 系统 MUST 提供一个最小 chat-first 页面来消费该事件流
- **THEN** 系统 MUST 允许前端同时获得聊天事件流与现有 run/session 标识
- **THEN** 用户 MUST 仍然可以跳转到现有 run/session 调试页查看 inspect、replay 和原始事件
