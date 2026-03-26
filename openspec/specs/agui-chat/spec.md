# agui-chat

## Purpose
定义面向 chat-first 体验的 AG-UI 兼容聊天入口、事件映射契约，以及本地前端如何通过 HTTP + SSE 实时消费该事件流。

## Requirements

### Requirement: AG-UI 兼容聊天入口
系统 MUST 提供一条面向聊天体验的 AG-UI 兼容入口，并通过 HTTP + SSE 输出标准化事件流。

#### Scenario: 发起一次 AG-UI 聊天请求
- **WHEN** 客户端向 AG-UI 聊天入口提交包含用户消息和运行配置的请求
- **THEN** 系统 MUST 创建或复用对应的 `Session`
- **THEN** 系统 MUST 启动一次新的 `Run`
- **THEN** 系统 MUST 以 SSE 的形式返回 AG-UI 兼容事件流

### Requirement: 生命周期与消息事件映射
系统 MUST 将运行时生命周期和消息事件映射为 AG-UI 兼容事件，以支持 chat-first 前端体验。

#### Scenario: run 开始执行
- **WHEN** 系统开始执行一次由 AG-UI 入口触发的 `Run`
- **THEN** 系统 MUST 发出 `RUN_STARTED` 事件
- **THEN** 系统 MUST 发出初始 `MESSAGES_SNAPSHOT`
- **THEN** 系统 MUST 发出初始 `STATE_SNAPSHOT`

#### Scenario: assistant 输出最终消息
- **WHEN** 一次 `Run` 产出助手消息
- **THEN** 系统 MUST 发出 `TEXT_MESSAGE_START`
- **THEN** 系统 MUST 发出至少一条 `TEXT_MESSAGE_CONTENT`
- **THEN** 系统 MUST 发出 `TEXT_MESSAGE_END`

#### Scenario: run 完成或失败
- **WHEN** 一次 `Run` 正常结束
- **THEN** 系统 MUST 发出 `RUN_FINISHED`

#### Scenario: run 执行失败
- **WHEN** 一次 `Run` 在执行过程中失败
- **THEN** 系统 MUST 发出 `RUN_ERROR`
- **THEN** 失败事件 MUST 包含清晰错误信息

### Requirement: 工具与步骤事件映射
系统 MUST 将运行时步骤和工具调用映射为 AG-UI 兼容事件，以便前端实时展示执行过程。

#### Scenario: 步骤开始和结束
- **WHEN** 某个计划步骤开始执行
- **THEN** 系统 MUST 发出 `STEP_STARTED`

- **WHEN** 某个计划步骤完成
- **THEN** 系统 MUST 发出 `STEP_FINISHED`

#### Scenario: 工具调用执行
- **WHEN** 系统调用工具
- **THEN** 系统 MUST 发出 `TOOL_CALL_START`
- **THEN** 系统 MUST 发出 `TOOL_CALL_ARGS`
- **THEN** 工具成功时 MUST 发出 `TOOL_CALL_END` 和 `TOOL_CALL_RESULT`

### Requirement: 无法直接标准化的语义需要保留
系统 MUST 为当前 runtime 中暂时无法直接映射到 AG-UI 标准事件的语义保留扩展出口。

#### Scenario: 计划更新或子任务事件
- **WHEN** 运行时产生 `plan.updated`、`subagent.*` 或类似内部强语义事件
- **THEN** 系统 MUST 通过 `CUSTOM` 或 `RAW` 事件对外暴露
- **THEN** 系统 MUST 不因为 AG-UI 映射而丢失这些信息
