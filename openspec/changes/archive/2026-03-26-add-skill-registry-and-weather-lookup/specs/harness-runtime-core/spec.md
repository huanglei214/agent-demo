# harness-runtime-core

## MODIFIED Requirements

### Requirement: 统一的核心领域模型
系统 MUST 定义并持久化 `Task`、`Run`、`Session` 和 `Event` 四类核心对象，并保持它们之间的引用关系清晰可追踪。

#### Scenario: 运行时按激活 skill 收窄工具面
- **WHEN** 一次 `Run` 激活了某个 skill，且该 skill 声明了 `allowed-tools`
- **THEN** 系统 MUST 在该次 `Run` 中根据 skill 约束收窄可用工具集
- **THEN** 系统 MUST 不影响其它未激活该 skill 的运行

### Requirement: 事件优先的执行轨迹
系统 MUST 将执行过程中的关键行为记录为结构化事件，并以追加方式保存为事件流。

#### Scenario: 运行时记录 skill 激活
- **WHEN** 某次 `Run` 激活了一个 skill
- **THEN** 系统 MUST 记录该 skill 的名称或等效结构化标识
- **THEN** 该记录 MUST 能在 inspect、replay 或调试路径中被查看
