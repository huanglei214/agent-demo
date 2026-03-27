## ADDED Requirements

### Requirement: 运行时显式区分 lead-agent 与 subagent 角色
系统 MUST 在运行时 prompt 构建与执行过程中显式区分 `lead-agent` 和 `subagent` 两种角色，而不是让所有运行默认复用同一套 agent 语义。

#### Scenario: 主运行使用 lead-agent 角色
- **WHEN** 系统为一次面向用户的主 `Run` 构建首次模型调用 prompt
- **THEN** 系统 MUST 使用 `lead-agent` 角色模板
- **THEN** 该角色模板 MUST 明确主 `Run` 对规划、委派决策和最终用户答案负责

#### Scenario: child run 使用 subagent 角色
- **WHEN** 系统为一次 delegated child `Run` 构建首次模型调用 prompt
- **THEN** 系统 MUST 使用 `subagent` 角色模板
- **THEN** 该角色模板 MUST 明确 child `Run` 不直接面向用户
- **THEN** 该角色模板 MUST 明确 child `Run` 只处理被委派的单一子任务

### Requirement: 角色语义在同一运行的 follow-up 调用中保持一致
系统 MUST 在同一 `Run` 的后续模型调用中保持既定角色语义，避免一次运行在工具调用或子结果合并后切换角色。

#### Scenario: 主运行在工具后续推理中保持 lead-agent 语义
- **WHEN** 主 `Run` 在工具调用成功后继续进行 `post_tool` 推理
- **THEN** 系统 MUST 继续使用 `lead-agent` 角色语义
- **THEN** 系统 MUST 不将主 `Run` 降级为仅做局部摘要的 worker 角色

#### Scenario: child run 在工具后续推理中保持 subagent 语义
- **WHEN** child `Run` 在工具调用成功后继续进行 `post_tool` 推理
- **THEN** 系统 MUST 继续使用 `subagent` 角色语义
- **THEN** 系统 MUST 不允许 child `Run` 在后续推理中转变为直接面向用户的最终答复角色

### Requirement: 运行时与 inspect 路径可识别运行角色
系统 MUST 让主运行与 child run 的角色标识可在运行时调试与 inspect 路径中被识别，以便通过 CLI 验证 lead-agent / subagent 分层是否生效。

#### Scenario: 运行时记录主运行角色
- **WHEN** 系统创建并启动一次主 `Run`
- **THEN** 系统 MUST 为该 `Run` 记录可区分为 `lead-agent` 的结构化角色标识

#### Scenario: inspect 能查看 child run 角色
- **WHEN** 用户通过 inspect 或等效调试路径查看某个包含 child run 的主 `Run`
- **THEN** 系统 MUST 能区分主 `Run` 与 child `Run` 的角色
- **THEN** 角色信息 MUST 可用于 CLI 验证 lead-agent 和 subagent 的职责边界

### Requirement: delegation 权限必须与运行角色绑定
系统 MUST 将 delegation 权限绑定到运行角色：只有 `lead-agent` 可以发起委派，`subagent` 不得继续创建新的 child run。

#### Scenario: lead-agent 允许发起委派
- **WHEN** 一次主 `Run` 以 `lead-agent` 角色执行并判断当前子任务适合委派
- **THEN** 系统 MAY 接受该运行产生的 `delegate` 决策
- **THEN** 系统 MUST 将该委派转化为新的 child `Run`

#### Scenario: subagent 不允许继续委派
- **WHEN** 一次 child `Run` 以 `subagent` 角色执行
- **THEN** 系统 MUST 不允许该运行通过模型输出或运行时决策继续创建新的 child `Run`
- **THEN** 如该运行无法完成任务，系统 MUST 要求其通过结构化结果返回阻塞信息，而不是继续 delegation

### Requirement: child run 的输入必须是 task-scoped 的
系统 MUST 将 child run 的模型输入限制为委派任务本身及最小必要的 task-local context，而不能默认继承主运行的完整对话和父目标信息。

#### Scenario: child run 不继承完整会话历史
- **WHEN** 系统为一次 `subagent` child `Run` 构建模型输入
- **THEN** 系统 MUST 不默认注入主 `Run` 的完整 `Conversation History`
- **THEN** 系统 MUST 不把主 `Run` 的多轮用户对话作为 child 的主要输入来源

#### Scenario: child run 不继承 parent goal
- **WHEN** 系统为一次 `subagent` child `Run` 构建模型输入
- **THEN** 系统 MUST 不默认注入 `parent_goal`
- **THEN** 系统 MUST 不默认注入 `parent_goal` 的摘要版本
- **THEN** child `Run` 的主要输入 MUST 是当前 delegation task 的 `goal`、`allowed_tools`、`constraints` 与 `completion_criteria`
