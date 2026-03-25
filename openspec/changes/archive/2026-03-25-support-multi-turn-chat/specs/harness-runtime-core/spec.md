## MODIFIED Requirements

### Requirement: 统一的核心领域模型
系统 MUST 定义并持久化 `Task`、`Run`、`Session` 和 `Event` 四类核心对象，并保持它们之间的引用关系清晰可追踪。

#### Scenario: 在已有 session 中创建新的运行
- **WHEN** 用户显式指定一个已有 `session_id` 发起新的输入
- **THEN** 系统 MUST 复用该 `Session`
- **THEN** 系统 MUST 创建新的 `Task`
- **THEN** 系统 MUST 创建新的 `Run`
- **THEN** 系统 MUST 保持该 `Run` 与原 `Session` 的关联关系

### Requirement: 标准运行时事件契约
系统 MUST 为运行时关键阶段使用固定的标准事件名称，以保证 inspect、replay 和后续入口扩展的一致性。

#### Scenario: 记录会话消息事件
- **WHEN** 一轮对话中产生用户输入和助手回复
- **THEN** 系统 MUST 在对应 `Run` 的事件流中记录 `user.message` 事件
- **THEN** 系统 MUST 在对应 `Run` 的事件流中记录 `assistant.message` 事件

### Requirement: Cobra CLI 作为首个入口
系统 MUST 提供基于 Cobra 的 CLI 入口，用于驱动运行时能力，而 CLI 层 MUST 通过应用层访问核心模块。

#### Scenario: 使用 chat 命令进行多轮对话
- **WHEN** 用户执行 `harness chat`
- **THEN** 系统 MUST 进入交互式多轮对话模式
- **THEN** 系统 MUST 在会话结束前持续复用同一个 `Session`
