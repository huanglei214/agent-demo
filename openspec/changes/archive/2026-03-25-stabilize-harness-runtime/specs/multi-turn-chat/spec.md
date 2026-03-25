## ADDED Requirements

### Requirement: 会话级运行查看
系统 MUST 提供会话级查看能力，用于检查最近消息和关联运行。

#### Scenario: 查看 session 最近消息与关联运行
- **WHEN** 用户执行 `harness session inspect <session-id>`
- **THEN** 系统 MUST 返回该 `Session` 的最近消息
- **THEN** 系统 MUST 返回与该 `Session` 关联的 `Run` 列表

#### Scenario: 限制返回的最近消息条数
- **WHEN** 用户为 `harness session inspect <session-id>` 指定 recent 限制
- **THEN** 系统 MUST 仅返回最近的 N 条消息
- **THEN** 系统 MUST 保持这些消息的时间顺序
