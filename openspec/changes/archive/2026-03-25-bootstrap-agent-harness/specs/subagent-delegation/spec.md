## ADDED Requirements

### Requirement: 受控的子代理委派
系统 MUST 支持主 `Run` 将特定子任务以 child run 的形式委派给子代理执行，并保持父子运行关系可追踪。

#### Scenario: 为可委派步骤创建 child run
- **WHEN** 主 `Run` 识别当前 `PlanStep` 可委派且满足委派策略
- **THEN** 系统 MUST 创建一个新的 child `Run`
- **THEN** 该 child `Run` MUST 记录父 `Run` 标识
- **THEN** 系统 MUST 记录 `subagent.spawned` 事件

### Requirement: 委派必须绑定计划步骤
系统 MUST 仅允许围绕明确的 `PlanStep` 进行委派，不能创建与计划无关的开放式子代理任务。

#### Scenario: 绑定计划步骤发起委派
- **WHEN** 主 `Run` 发起一次委派
- **THEN** 该委派 MUST 关联到一个明确的 `PlanStep`
- **THEN** 系统 MUST 记录该 `PlanStep` 与 child `Run` 的映射关系

### Requirement: 子代理上下文最小化
系统 MUST 向 child `Run` 传递最小充分上下文，而不是完整复制父运行的全部上下文。

#### Scenario: 构建子代理上下文
- **WHEN** 系统为 child `Run` 准备执行上下文
- **THEN** 系统 MUST 包含父目标、子目标、对应计划步骤、约束和被选中的记忆或摘要
- **THEN** 系统 MUST 不将无关的完整历史上下文默认注入 child `Run`

### Requirement: 子代理执行边界控制
系统 MUST 对子代理的执行深度、并发数和工具权限施加限制。

#### Scenario: 超过最大委派深度
- **WHEN** 一次新的委派会导致委派深度超过预设上限
- **THEN** 系统 MUST 拒绝该委派请求
- **THEN** 系统 MUST 记录 `subagent.rejected` 事件

#### Scenario: 子代理使用未授权工具
- **WHEN** child `Run` 尝试调用未被允许的工具
- **THEN** 系统 MUST 拒绝该工具调用
- **THEN** 系统 MUST 返回结构化错误结果
- **THEN** 系统 MUST 记录 `subagent.rejected` 事件

### Requirement: 子代理结果摘要
系统 MUST 要求 child `Run` 返回结构化摘要，以供主 `Run` 汇总和继续执行。

#### Scenario: 子代理完成后返回摘要
- **WHEN** child `Run` 正常完成
- **THEN** 系统 MUST 产出至少包含完成内容、产物、风险和是否建议重规划的摘要结果
- **THEN** 系统 MUST 记录 `subagent.completed` 事件

#### Scenario: 子代理结果字段完整
- **WHEN** child `Run` 返回结构化结果
- **THEN** 结果 MUST 包含 `summary` 字段
- **THEN** 结果 MUST 包含 `needs_replan` 字段
- **THEN** 结果 MUST 包含 `artifacts`、`findings`、`risks`、`recommendations` 数组字段，即使这些数组为空

### Requirement: 主运行结果合并
系统 MUST 支持主 `Run` 收集 child `Run` 的结果，并将其合并回当前执行过程。

#### Scenario: 合并子代理结果
- **WHEN** 一个或多个 child `Run` 已返回结果摘要
- **THEN** 系统 MUST 将这些摘要结果提供给主 `Run`
- **THEN** 主 `Run` MUST 能够基于这些结果继续执行、更新计划或输出最终结果

### Requirement: 长期记忆写入由主运行控制
系统 MUST 防止 child `Run` 直接修改长期记忆，长期记忆写入应由主 `Run` 或统一的记忆管理流程控制。

#### Scenario: 子代理产生长期记忆候选
- **WHEN** child `Run` 生成了可能具有长期价值的信息
- **THEN** 系统 MUST 将其作为候选结果返回给主 `Run` 或记忆管理流程
- **THEN** child `Run` MUST 不直接提交该信息到长期记忆存储
