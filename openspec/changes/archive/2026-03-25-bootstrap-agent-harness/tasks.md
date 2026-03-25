## 1. 项目初始化与脚手架

- [x] 1.1 初始化 Go 模块并添加 Cobra 依赖
- [x] 1.2 创建 `cmd/harness`、`internal/cli`、`internal/app`、`internal/runtime`、`internal/store`、`internal/model`、`internal/tool`、`internal/planner`、`internal/context`、`internal/memory`、`internal/delegation`、`internal/prompt`、`internal/loop` 的基础目录
- [x] 1.3 建立基础配置加载模块，支持读取 Ark 相关环境变量并映射到统一的 `ModelConfig`
- [x] 1.4 建立 `.runtime/` 目录约定和运行时工件路径工具

## 2. 核心领域模型与存储

- [x] 2.1 定义 `Task`、`Run`、`Session`、`Event` 的核心结构体和状态枚举
- [x] 2.2 定义 `Plan`、`PlanStep`、`Summary`、`MemoryEntry`、`DelegationTask` 等扩展模型
- [x] 2.3 实现文件型 `EventStore`，支持向 `events.jsonl` 追加结构化事件
- [x] 2.4 实现文件型 `StateStore`，支持读写 `run.json`、`state.json`、`plan.json`、`result.json`
- [x] 2.5 为核心对象和文件存储编写基础单元测试

## 3. CLI 与应用层

- [x] 3.1 创建 Cobra 根命令和全局配置入口
- [x] 3.2 实现 `harness run` 命令及其应用层服务
- [x] 3.3 实现 `harness inspect` 命令及其应用层服务
- [x] 3.4 实现 `harness replay` 命令及其应用层服务
- [x] 3.5 实现 `harness resume` 命令及其应用层服务
- [x] 3.6 实现 `harness tools list` 和 `harness debug events` 命令

## 4. 模型适配与 Prompt 构建

- [x] 4.1 定义统一的 `Model` 接口、请求结构和响应结构
- [x] 4.2 实现 Ark Provider，支持通过统一 `ModelConfig` 发起模型调用
- [x] 4.3 实现 Mock Provider，用于本地测试和流程验证
- [x] 4.4 实现 Starter Prompts 模板结构，支持 `base`、`role`、`task`、`tooling` 四层组装
- [x] 4.5 提供默认角色模板 `default-agent` 并接入 Prompt Builder

## 5. 规划、上下文与记忆

- [x] 5.1 定义 `Planner` 接口并实现首版结构化计划生成
- [x] 5.2 在 `PlanStep` 中加入可选的 `estimated_cost` 和 `estimated_effort` 字段
- [x] 5.3 实现 `ContextManager.Build`，按优先级组装 Pinned Context、当前步骤、摘要和记忆
- [x] 5.4 实现 compaction 检测与摘要生成流程，支持 `step summary` 和 `run summary`
- [x] 5.5 实现结构化长期记忆存储，固定 `kind` 并支持自由 `tags`
- [x] 5.6 实现“过滤 + 稳定排序 + Top N”的 Memory 召回逻辑
- [x] 5.7 为 planning、context、compaction、memory 编写流程级测试

## 6. 工具运行时与文件系统工具

- [x] 6.1 定义统一的 Tool 接口、注册中心和执行器
- [x] 6.2 实现 `fs.read_file`、`fs.list_dir`、`fs.search`、`fs.stat`
- [x] 6.3 实现 `fs.write_file` 的安全写入模式，支持 `overwrite=true` 控制覆盖
- [x] 6.4 为 `fs.write_file` 区分“新建文件”和“更新文件”事件
- [x] 6.5 实现工作区路径边界校验，阻止工具访问工作区外路径
- [x] 6.6 为文件系统工具编写成功、失败和越界访问测试

## 7. Agent Loop 与运行状态机

- [x] 7.1 实现 `Run` 生命周期状态机和状态变更事件
- [x] 7.2 实现主 Agent Loop，串联计划生成、模型调用、工具执行和事件回写
- [x] 7.3 在模型调用前接入 Prompt Builder、ContextManager 和 Memory Recall
- [x] 7.4 在执行过程中接入 compaction 触发与摘要回写
- [x] 7.5 在运行结束时接入最终结果持久化和 memory candidate 提取
- [x] 7.6 为主执行链编写端到端样例测试

## 8. 子代理委派

- [x] 8.1 定义 `DelegationManager` 和 child run 结果结构
- [x] 8.2 实现基于 `PlanStep.delegatable` 的委派检查逻辑
- [x] 8.3 实现 child run 创建、父子关系持久化和 `subagent.spawned` 事件
- [x] 8.4 实现子代理最小充分上下文构建
- [x] 8.5 实现子代理执行边界控制，包括最大深度、最大并发和只读工具限制
- [x] 8.6 实现 child run 结构化结果校验，确保返回 `summary`、`needs_replan` 和数组字段
- [x] 8.7 实现主运行对 child run 结果的收集、合并和 replan 触发
- [x] 8.8 为 delegation 流程编写成功、拒绝和结果合并测试

## 9. 回放、恢复与可观测性

- [x] 9.1 实现按事件流回放 `Run` 的输出逻辑
- [x] 9.2 实现基于持久化状态恢复未完成 `Run` 的逻辑
- [x] 9.3 统一关键事件类型，覆盖 run、plan、tool、context、memory、subagent 主流程
- [x] 9.4 为 replay、resume 和事件完整性编写验证测试

## 10. 收尾与示例验证

- [x] 10.1 增加最小可运行示例，演示从 `harness run` 到结果落盘的闭环
- [x] 10.2 补充 README，说明 CLI 用法、运行时目录结构和环境变量要求
- [x] 10.3 对照 proposal、design 和 specs 进行一次实现前自检，确认任务覆盖所有 requirement
