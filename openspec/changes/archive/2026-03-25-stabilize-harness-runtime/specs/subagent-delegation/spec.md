## MODIFIED Requirements

### Requirement: 子代理执行边界控制
系统 MUST 对子代理的执行深度、并发数和工具权限施加限制。

#### Scenario: 子代理默认仅获得只读工具
- **WHEN** 系统为 child `Run` 构建委派任务
- **THEN** `allowed_tools` MUST 从已注册工具中的只读工具集合推导得到
- **THEN** 系统 MUST 默认排除写入类工具

### Requirement: 主运行结果合并
系统 MUST 支持主 `Run` 收集 child `Run` 的结果，并将其合并回当前执行过程。

#### Scenario: 子代理结果触发主运行重规划
- **WHEN** child `Run` 返回结构化结果，并且 `needs_replan=true`
- **THEN** 主 `Run` MUST 在继续后续推理前评估是否需要更新当前 `Plan`
- **THEN** 当结果包含可消费信号时，主 `Run` MUST 记录 `plan.updated` 事件
