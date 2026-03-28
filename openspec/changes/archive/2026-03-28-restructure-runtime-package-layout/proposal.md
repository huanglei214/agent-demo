## Why

当前仓库的目录结构已经比早期版本清晰很多，但运行时核心仍然存在几处可以继续收敛的边界：

- `internal/app/` 同时承载了 agent 执行引擎、运行时编排服务和部分领域策略，职责仍然偏混合
- `retrieval_guard`、`delegation_executor`、`replan_policy` 等策略文件尚未完全归位到各自领域包
- `internal/interfaces/`、`internal/app/`、`internal/store/` 之间的依赖方向虽然可用，但在目录语义上还不够明确
- 后续要继续推进真实流式输出、Web 主入口、运行时可观测性时，现有目录结构会增加理解和修改成本

这次变更的目标不是引入新的用户可见能力，而是进一步优化运行时目录布局，让执行引擎、服务编排、领域策略和接口适配层的边界更清楚，为后续功能演进和维护留出更稳定的结构基础。

## What Changes

本次变更将以“不改变用户可见行为”为前提，对运行时核心目录进行重构：

- 将 `internal/app/` 按职责拆分为更清晰的执行引擎层与服务编排层
- 将运行时领域策略文件归位到对应包，例如 delegation、planner、retrieval 等
- 保留 `internal/interfaces/` 作为内部接口适配层，不将其提升为顶层公开包
- 继续收紧 `service -> agent -> runtime/store/model/tool` 的依赖方向，避免循环依赖
- 统一迁移期间的装配点、测试入口和文档说明，确保 CLI、HTTP、Web 不发生行为回归

本次变更明确不包含：

- 新的用户交互能力
- 新的工具能力
- 新的模型协议或前端协议
- 对外 HTTP/CLI 行为的有意变更

## Capabilities

### Modified Capabilities
- `harness-runtime-core`: 调整运行时核心目录布局、依赖边界和内部职责分层，但保持现有运行行为与对外接口不变

## Impact

预期会影响这些模块和文件区域：

- `internal/app/`
- `internal/service/`（新增）
- `internal/agent/`（新增）
- `internal/delegation/`
- `internal/planner/`
- `internal/store/`
- `internal/interfaces/cli/`
- `internal/interfaces/http/`
- `cmd/`
- `README.md`
- `docs/`

这次重构会涉及较多 import 路径与包名调整，因此需要以可分步验证的方式推进，并保持每一步都可编译、可测试、可回归。
