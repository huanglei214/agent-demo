## 1. 定义 store 接口

- [x] 1.1 在 `internal/store/` 中新增 `EventStore` 与 `StateStore` 接口
- [x] 1.2 让 `internal/store/filesystem/` 的现有实现满足这些接口
- [x] 1.3 更新相关测试，确保接口抽象后行为不变

## 2. 引入 agent.Runner

- [x] 2.1 在 `internal/agent/` 中定义最小可用的 `Runner` 接口
- [x] 2.2 让 `Executor` 实现 `Runner`，并保持现有执行与恢复语义不变
- [x] 2.3 补充或调整 agent 层测试，验证 `Runner` 契约

## 3. 收紧 service 层边界

- [x] 3.1 将 `service.Services` 从嵌入 `agent.Executor` 改为显式持有 `runner` 与 store 接口
- [x] 3.2 调整 `NewServices` / `NewDependencies` / `NewServicesFromDependencies` 的装配方式
- [x] 3.3 更新 service 层测试，确保 service 仍只暴露业务 API

## 4. 拆解 Executor 依赖结构

- [x] 4.1 按 `RuntimeDeps / AgentDeps / ExecutionDeps` 重组 `Executor` 依赖
- [x] 4.2 去掉当前平铺的大依赖结构，同时保持内部 loop / dispatch / delegation 行为不变
- [x] 4.3 清理重构过程中出现的临时适配代码和重复字段

## 5. 切换入口与验证

- [x] 5.1 更新 CLI / HTTP / Web 入口装配到新的 `service -> runner -> store interface` 边界
- [x] 5.2 确认 `.runtime/` 工件结构和外部行为保持兼容
- [x] 5.3 运行定向测试：`go test ./internal/agent ./internal/service ./internal/store/...`
- [x] 5.4 运行场景回归：`make verify-scenarios`
- [x] 5.5 运行全量回归：`go test ./...`
