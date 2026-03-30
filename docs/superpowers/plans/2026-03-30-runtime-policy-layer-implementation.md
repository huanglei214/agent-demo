# Runtime Policy Layer Implementation Status

## 目标

本轮实现目标已经收敛为一条清晰边界：

- 为执行内核提供统一的 `runtime/policy` 控制层
- 先落地 control policies
- 保持副作用仍由 `Executor` 执行
- 不把 context / memory / approval / max-turns 一次性拉进来

这份文档记录当前代码状态与收尾项，不再保留原始的未来时态实施步骤。

## 已完成项

### Task 1: policy skeleton

已落地：

- `internal/runtime/policy/types.go`
- `internal/runtime/policy/policy.go`
- `internal/runtime/policy/policy_test.go`

已确认语义：

- `Continue()` 返回 no-op outcome
- `HasEffect()` 识别 effectful outcome
- `ActionResultKind` 常量固定

### Task 2: Executor 接入 policy pipeline 和 ExecutionMode

已落地：

- `Executor.Policies []policy.RuntimePolicy`
- `runExecution.mode`
- `deriveExecutionMode(...)`
- `policyContext(...)`
- `runPoliciesBeforeRun(...)`
- `runPoliciesAfterModel(...)`
- `runPoliciesAfterAction(...)`

当前真实语义：

- `BeforeRun` 已接入，但 effectful outcome 仍会被视为未实现行为并报错
- `AfterModel` / `AfterAction` 已接入固定顺序 chain
- `BeforeModel` / `BeforeFinish` 仍保留在接口层，尚未接入主流程
- policy 只能通过 outcome 影响流程，不能靠修改副本偷偷生效

### Task 3: ReplanPolicy

已落地：

- `internal/runtime/policy/replan_policy.go`
- `internal/runtime/policy/replan_policy_test.go`

当前真实语义：

- 仅在 delegation action + delegation result 成功时评估
- 复用 `planner.DecideChildReplan(...)`
- 返回 `DecisionReplan`
- `planner.Replan()` 仍由 executor 路径执行

### Task 4: RetrievalPolicy

已落地：

- `internal/runtime/policy/retrieval_policy.go`
- `internal/runtime/policy/retrieval_policy_test.go`

当前真实语义：

- 在 `AfterAction` 中处理 `ActionResultToolBatch`
- 复用 `retrieval.DecideProgress(...)`
- 返回 `DecisionForceFinal`
- forced-final 的执行仍在 policy 之外

### Task 5: DelegationPolicy

已落地：

- `internal/runtime/policy/delegation_policy.go`
- `internal/runtime/policy/delegation_policy_test.go`

当前真实语义：

- 在 `AfterModel` 中拦截 `delegate` action
- 复用 `delegation.Manager.CanDelegate(...)`
- 失败时返回 `DecisionBlock`
- 子任务构造、持久化和 delegation 副作用仍由 executor / delegation 子系统负责

## Task 6: 收尾项

### 组合语义测试

本轮需要锁定的组合规则是：

- policy chain 按注册顺序执行
- 第一条 effectful outcome 生效
- 后续 policy 不再观察同一个 hook

当前已通过 runner 级测试覆盖该语义，测试位于：

- `internal/agent/loop_test.go`

### 回归范围

Task 6 要求的回归命令：

```bash
GOCACHE=/tmp/gocache go test ./internal/runtime/policy ./internal/agent ./internal/delegation ./internal/retrieval ./internal/service
GOCACHE=/tmp/gocache go test ./...
```

### 文档同步要求

文档需要与代码保持一致的要点：

- 已落地：policy skeleton、ExecutionMode、ReplanPolicy、RetrievalPolicy、DelegationPolicy
- `AfterModel` / `AfterAction` 的 runner 语义是固定顺序 + 首个 effectful outcome 短路
- `AfterToolBatch` 当前通过 `AfterAction` + `ActionResultToolBatch` 表达
- `BeforeModel` / `BeforeFinish` 仍只是接口保留位
- 明确保留未做：`ContextPolicy`、`MemoryPolicy`、`ApprovalPolicy`、`MaxTurnsPolicy`

## 当前验收状态

已满足：

- `internal/runtime/policy` 已成为 control policy 的承载层
- `Executor` 已通过固定顺序执行 policy chain
- `ReplanPolicy`、`RetrievalPolicy`、`DelegationPolicy` 已从主流程中抽离为 policy 决策
- policy 不直接执行 planner / delegation / model 副作用

尚未纳入本轮范围：

- `ContextPolicy`
- `MemoryPolicy`
- `ApprovalPolicy`
- `MaxTurnsPolicy`
- `BeforeModel` hook 落地
- `BeforeFinish` hook 落地

## 风险与后续

当前设计仍有两个明确边界需要保留：

- `ExecutionMode` 目前仍来自 `run.Role` + `ResumePhase` 的投影，而不是独立状态来源
- 短路式组合规则是刻意保持简单的现状，不应在没有新增需求时演化成复杂 merge 引擎
