## ADDED Requirements

### Requirement: 危险命令必须被运行时拒绝
系统 MUST 为 `bash.exec` 提供第一层危险命令限制，并拒绝执行明显高风险的本地命令或等效命令链。

#### Scenario: 直接执行危险命令
- **WHEN** Agent 调用 `bash.exec` 且命令的首个可执行程序命中系统定义的危险命令集合
- **THEN** 系统 MUST 拒绝执行该命令
- **THEN** 系统 MUST 返回结构化错误结果
- **THEN** 系统 MUST 不实际启动该命令

#### Scenario: 在命令链中包含危险命令
- **WHEN** Agent 调用 `bash.exec` 且管道、逻辑操作符或等效命令链中的某一段命中系统定义的危险命令集合
- **THEN** 系统 MUST 拒绝执行整条命令链
- **THEN** 系统 MUST 返回结构化错误结果
- **THEN** 系统 MUST 不执行命中危险命令前后的其它链式命令
