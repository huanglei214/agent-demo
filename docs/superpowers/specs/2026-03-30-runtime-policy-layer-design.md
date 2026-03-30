# Runtime Policy Layer Design

## 背景

本轮已经把一批影响执行路径的运行时规则，从 `agent` 主流程中收拢到 `internal/runtime/policy`：

- `Executor` 继续拥有 run loop、action dispatch 和副作用执行。
- `runtime/policy` 负责返回控制类决策，不直接调用 planner、delegation 或 model。
- `planner`、`delegation`、`retrieval` 中原有的纯规则 helper 仍然保留，由 policy 复用。

这份文档记录的是当前代码真实状态，而不是待实现蓝图。

## 已落地范围

- policy skeleton：`types.go`、`policy.go`、`policy_test.go`
- `ExecutionMode`
- `ReplanPolicy`
- `RetrievalPolicy`
- `DelegationPolicy`
- `Executor` 中的固定顺序 policy chain

## 明确保留未做

以下能力本轮没有进入 policy pipeline，仍然保持现状：

- `ContextPolicy`
- `MemoryPolicy`
- `ApprovalPolicy`
- `MaxTurnsPolicy`

## 核心类型

### ExecutionMode

当前代码中显式落地了三种模式：

```go
type ExecutionMode string

const (
    ExecutionModeStructured ExecutionMode = "structured"
    ExecutionModeResume     ExecutionMode = "resume"
    ExecutionModeDelegated  ExecutionMode = "delegated"
)
```

`deriveExecutionMode(...)` 目前仍由 `run.Role` 和 `state.ResumePhase` 推导：

- subagent run -> `delegated`
- resume turn -> `resume`
- 其他情况 -> `structured`

这让 policy 不必在各处重复推断模式，但 `ExecutionMode` 目前仍是投影值，不是独立持久化来源。

### ExecutionContext

`ExecutionContext` 是 `runExecution` 的只读快照投影，包含：

- `Task`、`Session`、`Run`、`State`
- `Plan`、`CurrentStep`
- `Mode`、`Flags`、`Metadata`
- `Memories`、`Summaries`
- `RetrievalProgress`
- `WorkingEvidence`
- `ExplicitCandidates`
- `FinalAnswer`
- `TurnCount`

当前实现会在进入 policy hook 前拷贝结构化输入，测试已锁定：

- policy 看到的是隔离副本
- policy 对 action / result 副本的无效果修改会被 runner 拒绝

### ActionResult

当前使用的结果类型如下：

```go
const (
    ActionResultModel      ActionResultKind = "model"
    ActionResultToolBatch  ActionResultKind = "tool_batch"
    ActionResultDelegation ActionResultKind = "delegation"
    ActionResultFinal      ActionResultKind = "final"
)
```

其中首批真正用到的是：

- `tool_batch`
- `delegation`
- `final`

### PolicyOutcome

第一版 no-op 约定已经固定：

- `Continue()` 返回零值 `PolicyOutcome{}`
- `HasEffect()` 用于判断 outcome 是否真正影响执行路径
- payload-only outcome 也会被视为 effectful

## Runner 接入现状

### 已接入的 hook

当前 `Executor` 实际接入了三个 hook：

- `BeforeRun`
- `AfterModel`
- `AfterAction`

`BeforeModel` 和 `BeforeFinish` 只存在于接口中，当前没有接入执行主流程；这两个 hook 仍然是后续扩展位。

### after-model runner 语义

`runPoliciesAfterModel(...)` 的当前语义是：

- 按 `Executor.Policies` 的固定顺序执行
- 每个 policy 拿到 `ExecutionContext` 快照和 `action` 副本
- 如果 policy 修改了 `action` 副本，但最终 outcome 没有效果，runner 返回错误
- 遇到第一条 effectful outcome 时立即短路，后续 policy 不再观察该 hook
- 只有调用点显式允许的 decision 才能返回；否则 runner 报错

这条短路语义已经由组合测试锁定。

### after-action runner 语义

`runPoliciesAfterAction(...)` 的当前语义与 after-model 保持一致：

- 按固定顺序执行
- policy 拿到 `action` / `result` 的隔离副本
- 无效果的副本修改会被拒绝
- 第一条 effectful outcome 生效，后续 policy 不再执行
- decision 是否允许由调用点控制

`ActionResultToolBatch` 用来表达原设计中的 “after-tool-batch” 决策点；当前没有单独的 `AfterToolBatch` 接口。

## Policy Chain

### 注册顺序

`Executor` 当前显式注册的 policy 顺序是：

1. `DelegationPolicy`
2. `ReplanPolicy`
3. `RetrievalPolicy`

这一顺序直接决定同一 hook 的观察顺序和短路行为。

### 组合规则

当前实现采用简单短路语义：

- policy 按固定顺序执行
- 第一条 effectful outcome 生效
- 不做多 outcome merge
- 后续 policy 不会再观察到同一个 hook

## 已实现的 Policy

### DelegationPolicy

职责：

- 在 `AfterModel` 阶段拦截 `delegate` action
- 通过 `delegation.Manager.CanDelegate(...)` 复用既有 guard helper
- 失败时返回 `DecisionBlock`

不负责：

- 构造子任务
- 持久化 child run
- 执行 delegation 副作用

### ReplanPolicy

职责：

- 在 `AfterAction` 阶段处理 delegation 结果
- 复用 `planner.DecideChildReplan(...)`
- 在需要时返回 `DecisionReplan`

不负责：

- 直接执行 `planner.Replan()`

### RetrievalPolicy

职责：

- 在 `AfterAction` + `ActionResultToolBatch` 阶段评估 retrieval 进度
- 复用 `retrieval.DecideProgress(...)`
- 在需要时返回 `DecisionForceFinal`

不负责：

- 直接发起 forced-final model call

## 边界

当前边界已经固定为：

- policy 只做决策
- `Executor` 消费 `PolicyOutcome`
- planner / delegation / retrieval 仍保留各自的规则 helper 和副作用入口

这意味着文档、测试和后续扩展都应围绕 “policy 决策层 + executor 执行层” 继续推进，而不是把副作用重新挪回 policy。

## 后续扩展位

后续如果继续扩展 policy layer，建议沿着现有 runner 语义推进：

- 如需接入 `ContextPolicy` 或 `MemoryPolicy`，优先评估是否需要真正接通 `BeforeModel`
- 如需接入 `ApprovalPolicy` 或 `MaxTurnsPolicy`，优先评估 `BeforeFinish` 的调用点和允许 decision 集合
- 如需 richer merge 语义，应先证明短路规则不足，再考虑更复杂的 outcome 组合
