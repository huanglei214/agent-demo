## 1. 拆分执行引擎层

- [x] 1.1 新建 `internal/agent/` 包，并迁移执行引擎相关文件
- [x] 1.2 将 `agent_loop`、`action_dispatch`、`action_parser`、`run_observer` 的 import 与包引用调整到新路径
- [x] 1.3 保持执行引擎对外暴露清晰入口，不反向依赖服务编排层
- [x] 1.4 为迁移后的 `internal/agent/` 补齐或更新相关测试

## 2. 拆分服务编排层

- [x] 2.1 新建 `internal/service/` 包，并迁移 run/session/inspect/replay/resume/list/tools 等服务文件
- [x] 2.2 迁移 `services.go` 与依赖装配逻辑，使 interfaces 与 cmd 层改为依赖 `internal/service/`
- [x] 2.3 调整 `internal/interfaces/cli/`、`internal/interfaces/http/`、`cmd/` 的 import 路径
- [x] 2.4 为迁移后的服务层补齐或更新相关测试

## 3. 归位领域策略文件

- [x] 3.1 将 `delegation_executor` 迁移到 `internal/delegation/`
- [x] 3.2 将 `replan_policy` 迁移到 `internal/planner/`
- [x] 3.3 新建 `internal/retrieval/`（或等效语义位置），迁移 `retrieval_guard`
- [x] 3.4 调整 `agent`、`service` 与领域包之间的依赖关系，避免循环依赖
- [x] 3.5 为归位后的 delegation / planner / retrieval 相关逻辑补齐或更新测试

## 4. 收敛接口与依赖边界

- [x] 4.1 审查 `service -> agent -> runtime/store/model/tool` 的依赖方向并修正不清晰的引用
- [x] 4.2 在需要的地方用窄接口替代具体实现引用，避免包级双向依赖
- [x] 4.3 确认 `internal/interfaces/` 保持内部适配层语义，不引入顶层公开 `app/` 包迁移
- [x] 4.4 确认 `runtime` 继续保持底层共享类型与错误语义

## 5. 清理旧路径与文档同步

- [x] 5.1 删除 `internal/app/` 中已迁移且不再使用的旧文件
- [x] 5.2 清理迁移过程中产生的重复 helper、过时 import 和冗余包装代码
- [x] 5.3 更新 README / docs 中涉及目录结构与内部包路径的说明

## 6. 验证

- [x] 6.1 运行受影响包的定向测试
- [x] 6.2 运行 `make build`
- [x] 6.3 运行 `make verify-scenarios`
- [x] 6.4 运行 `go test ./...`
