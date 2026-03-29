## Why

当前仓库虽然已经完成了 `agent / service` 的目录拆分，但核心边界仍未真正收紧：`service.Services` 仍然直接嵌入 `agent.Executor`，`Executor` 仍以平铺的大结构体方式持有大量依赖，同时 `agent / service` 还直接依赖 `filesystem` 的具体 store 实现。  
这会让服务层继续暴露执行引擎内部能力，也让后续替换存储后端、继续拆解执行逻辑或收敛依赖方向时成本偏高，因此需要补上这一轮边界重构。

## What Changes

- 去掉 `service.Services` 对 `agent.Executor` 的直接嵌入，改为通过窄接口与 agent 层交互
- 在 `internal/agent/` 中引入最小可用的 `Runner` 接口，承接执行与恢复能力
- 在 `internal/store/` 中定义 `EventStore` 与 `StateStore` 接口，让 `filesystem` 成为其中一个实现
- 拆解 `agent.Executor` 的依赖结构，按职责域分组，避免继续平铺大量字段
- 收敛 `service -> agent -> store/runtime` 的依赖方向，避免 service 继续感知 agent 内部执行细节
- 保持 CLI / HTTP / Web / `.runtime/` 外部行为不变，不把本轮改造成产品能力变更

## Capabilities

### New Capabilities

无

### Modified Capabilities

- `harness-runtime-core`: 收紧 service-agent-store 边界，要求 service 通过窄接口调用 agent，要求 agent/service 依赖 store 接口而不是 filesystem 具体实现，并要求 Executor 依赖结构按职责分组

## Impact

- `internal/service/`
- `internal/agent/`
- `internal/store/`
- `internal/store/filesystem/`
- `internal/interfaces/cli/`
- `internal/interfaces/http/`
- `cmd/cli/`
- `cmd/web/`
- 相关测试、README 与 OpenSpec delta spec
