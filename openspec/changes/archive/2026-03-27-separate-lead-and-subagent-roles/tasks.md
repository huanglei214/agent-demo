## 1. 角色模板与 prompt 分层

- [x] 1.1 在 `internal/prompt/` 中拆分 `lead-agent` 与 `subagent` 角色模板，并保持现有 prompt builder 主流程可复用
- [x] 1.2 让主 `Run` 首次模型调用和 follow-up 调用固定继承 `lead-agent` 角色语义
- [x] 1.3 让 delegated child `Run` 首次模型调用和 follow-up 调用固定继承 `subagent` 角色语义

## 2. delegation 契约收紧

- [x] 2.1 在 child run 构建路径中显式注入 `subagent` 角色约束，而不是仅依赖零散 constraint 文本
- [x] 2.2 收紧 child result 约束，确保 `summary`、`needs_replan`、`artifacts`、`findings`、`risks`、`recommendations` 的结构稳定
- [x] 2.3 确保主 `Run` 在消费 child result 时保持 `lead-agent` 的最终答案责任，不将 child 文本直接视为用户答复

## 3. 运行时可观测性与 CLI 验证

- [x] 3.1 为主 run 和 child run 增加可在 inspect / replay / debug-events 中识别的角色标识
- [x] 3.2 为 lead/subagent 分层补 targeted tests，覆盖主 run、child run 和 post-tool follow-up 的角色继承行为
- [x] 3.3 通过 CLI 手工验证至少一条 delegation 链路，确认主 run 由 lead 收口、child run 以结构化 subagent 结果返回

## 4. delegation 单层约束

- [x] 4.1 在 prompt 层明确禁止 subagent 继续创建或请求新的 subagent
- [x] 4.2 在运行时增加硬校验，确保只有 lead-agent 可以发起 delegation
- [x] 4.3 为 subagent 递归 delegation 拒绝路径补 targeted tests

## 5. task-scoped subagent 输入

- [x] 5.1 让 child run 的输入以 DelegationTask 为主，而不是主 run 的会话历史或父目标
- [x] 5.2 默认移除 child run 的 Conversation History 与 parent goal 注入
- [x] 5.3 为 child prompt 的 task-scoped 输入补 targeted tests
