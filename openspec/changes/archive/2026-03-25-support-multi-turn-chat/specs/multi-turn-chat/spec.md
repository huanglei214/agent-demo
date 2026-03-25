## ADDED Requirements

### Requirement: 本地多轮会话
系统 MUST 支持在同一个 `Session` 中连续接收多轮用户输入，并为每轮输入创建独立的 `Run`。

#### Scenario: 在已有 session 中追加一轮输入
- **WHEN** 用户基于已有 `session_id` 发起一轮新的输入
- **THEN** 系统 MUST 复用该 `Session`
- **THEN** 系统 MUST 为该轮输入创建新的 `Task`
- **THEN** 系统 MUST 为该轮输入创建新的 `Run`

### Requirement: 会话级消息持久化
系统 MUST 将多轮对话中的用户消息与助手消息持久化到 session 范围的消息存储中。

#### Scenario: 保存用户消息和助手消息
- **WHEN** 一轮对话被执行
- **THEN** 系统 MUST 持久化该轮的 `user` 消息
- **THEN** 系统 MUST 持久化该轮的 `assistant` 消息
- **THEN** 每条消息 MUST 关联 `session_id` 和 `run_id`

### Requirement: 交互式 chat 命令
系统 MUST 提供交互式 CLI 命令用于本地多轮对话。

#### Scenario: 启动新的交互式会话
- **WHEN** 用户执行 `harness chat`
- **THEN** 系统 MUST 创建新的 `Session`
- **THEN** 系统 MUST 进入交互式输入循环
- **THEN** 系统 MUST 在每轮回复后继续等待下一轮输入，直到用户显式退出

#### Scenario: 继续已有会话
- **WHEN** 用户执行 `harness chat --session <session-id>`
- **THEN** 系统 MUST 加载已有 `Session`
- **THEN** 系统 MUST 继续在该 `Session` 中进行新的多轮输入

### Requirement: 基于 run 命令的单轮续聊
系统 MUST 支持通过非交互方式向已有会话追加单轮输入。

#### Scenario: 使用 run 追加一轮输入
- **WHEN** 用户执行 `harness run --session <session-id> <instruction>`
- **THEN** 系统 MUST 在目标 `Session` 中追加一轮新的用户输入
- **THEN** 系统 MUST 返回该轮新建 `Run` 的结果
