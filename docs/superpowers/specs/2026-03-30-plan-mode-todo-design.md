# Plan Mode Design

## Implementation Status (2026-03-30)

当前实现已经落地，实际行为如下：

- `Run.PlanMode` 与 `RunState.Todos` 已持久化，旧 run 在反序列化时会默认回填 `plan_mode=none`。
- `RunRequest` 与 HTTP `start run` 请求都支持 `plan_mode`。
- `todo` action 只在 `plan_mode=todo` 下生效，采用全量 `set` 语义，写入 `todo.updated` 事件，并继续执行后续 loop。
- todo 上下文通过 `prompt.InjectTodoContext(...)` 包装现有 prompt 构造结果注入，而不是改写 prompt builder 接口。
- `none` mode 仍保留 compatibility plan 路径；该兼容 step 会沿用 delegatable heuristic，因此 lead run 的 delegation/replan 行为没有被破坏。


## 背景

当前仓库中的 `plan` 能力仍然是一个 deterministic planner：

- `service.StartRun(...)` 会在 run 开始前无条件调用 `Planner.CreatePlan(...)`
- `planner.DeterministicPlanner` 只生成一个非常薄的 `Plan`
- 执行内核真正的推理和拆解仍然发生在 model/action loop 中

这意味着当前 `plan` 更像执行脚手架，而不是面向通用 harness 的规划层。

本设计的目标不是继续增强现有的 upfront planner，而是把第一版显式规划能力收敛成：

- `none`
- `todo`

也就是让 planning 成为可选的运行模式，而不是默认的系统主路径。

## 设计目标

- 保持 `execution-first`：执行内核仍然是系统中心
- 支持入口显式指定 planning mode
- 在未显式指定时，由系统用轻量规则自动选择默认 mode
- 让 `todo` 成为可恢复、可回放、可流式展示的一等运行态数据
- 让模型通过显式 action 更新 todo，而不是依赖隐式 prompt 解析

## 非目标

本轮不做以下内容：

- 不保留 `structured` 或 `research` 作为第一版 mode
- 不引入 LLM upfront planner
- 不把 todo 设计成强流程控制器
- 不支持 todo patch 语义，如 `append`、`delete`、`update_status`
- 不让 policy 层直接负责生成或写入 todo

## 顶层模式

第一版只定义两个 planning mode：

```go
type PlanMode string

const (
    PlanModeNone PlanMode = "none"
    PlanModeTodo PlanMode = "todo"
)
```

语义如下：

- `none`
  - 不维护显式 todo
  - 执行内核直接运行现有 loop
- `todo`
  - 允许模型维护运行中的任务清单
  - todo 仅提供可见性和轻引导
  - executor 不基于 todo 做强 gating

## Mode 选择规则

当前落地实现的选择优先级如下：

1. 入口显式指定值优先
2. 若入口未指定，则执行轻量 heuristic
3. fallback 为 `none`

说明：设计上预留了 skill metadata 默认值的扩展空间，但 2026-03-30 的实现尚未接入该层

第一版 heuristic 保持克制，只在明显复杂任务时自动升到 `todo`。典型信号包括：

- 指令中出现明显多步骤表达，如“先……再……”“分步骤”“逐步”
- 同时包含读取、分析、修改、总结等多个阶段性动作
- 明显是方案、架构、调研、对比类任务

mode 在 run 创建时确定，整个 run 生命周期内不再切换。

## 数据模型

### Run

`Run` 记录本次执行采用的 planning mode：

```go
type Run struct {
    ...
    PlanMode PlanMode
}
```

`PlanMode` 属于 run 级固定属性，而不是可变运行时状态。

### RunState

todo 快照存放在 `RunState` 中：

```go
type TodoStatus string

const (
    TodoPending    TodoStatus = "pending"
    TodoInProgress TodoStatus = "in_progress"
    TodoDone       TodoStatus = "done"
)

type TodoItem struct {
    ID        string
    Content   string
    Status    TodoStatus
    Priority  int
    UpdatedAt time.Time
}

type RunState struct {
    ...
    Todos []TodoItem
}
```

第一版刻意不包含以下字段：

- `owner`
- `delegated_run_id`
- `dependencies`
- `output_schema`

## Action 设计

todo 更新通过专门 action 完成，而不是普通 tool 调用。

```go
type Action struct {
    Action string `json:"action"`

    Answer         string      `json:"answer,omitempty"`
    Calls          []ToolCall  `json:"calls,omitempty"`
    DelegationGoal string      `json:"delegation_goal,omitempty"`
    Todo           *TodoAction `json:"todo,omitempty"`
}

type TodoAction struct {
    Operation string     `json:"operation"`
    Items     []TodoItem `json:"items"`
}
```

第一版约束如下：

- `action == "todo"`
- `todo.operation` 固定为 `"set"`
- `items` 允许为空
- `set []` 表示清空整个 todo 列表

每次 model response 只能表达一个 action，`todo` 不能与 `tool`、`delegate`、`final` 混合。

## Executor 行为

在 `plan_mode=todo` 下，执行流新增一条中间动作路径：

```text
model -> todo action -> validate -> save RunState.Todos -> append todo.updated -> continue loop
```

具体语义：

- `plan_mode=none` 下，如果模型返回 `todo action`，executor 直接报错
- `plan_mode=todo` 下，executor 校验 payload 后整体替换 `RunState.Todos`
- todo 更新不是终点，不会结束 run
- todo 更新后继续下一轮模型调用
- `todo action` 计入 `TurnCount`

executor 不会基于 todo 做以下硬约束：

- 不要求存在 `current todo item`
- 不要求按 priority 顺序执行
- 不自动根据 tool/delegation/final 推断某项完成

## Prompt 与上下文

`plan_mode=todo` 下，prompt builder 需要额外注入：

- 当前 todo 快照
- 很短的使用规则

推荐规则包括：

- 复杂任务可以先写 todo，再继续执行
- 如果 todo 需要更新，应提交完整最新列表
- 保持 todo 与当前执行状态一致
- 简单任务不要过度维护 todo

只向模型注入当前 todo 快照，不注入 todo 变更历史。历史仍由 event/replay 消费。

## 事件与恢复

第一版只新增一个 todo 相关事件：

- `todo.updated`

推荐 payload：

- `operation`
- `count`
- `items`

恢复语义如下：

- resume 直接读取 `RunState.Todos`
- replay 通过 `todo.updated` 重放 todo 演化过程
- SSE / AGUI 可以同时使用快照和事件流

## 与现有 planner 的关系

第一版 `todo` 设计不再以现有 deterministic planner 为核心。

当前已落地的关系如下：

- `todo` mode 在 start run 路径上仍会调用 `Planner.CreatePlan(...)`
- `none` mode 不再强制走 upfront planner，而是生成最小 compatibility plan
- child run 仍通过 planner 创建 plan，兼容现有 delegation 流程
- 若未来需要更强规划能力，应在新的 mode 或 profile 下演进，而不是继续给 deterministic planner 叠职责

也就是说，系统已经不再要求“所有 run 都先 CreatePlan 再执行”，但 compatibility path 仍然保留，尚未完全移除 planner

## 边界

本设计明确保持以下边界：

- `service`
  - 负责接收 `PlanMode`、选择默认 mode、创建 run/state
- `executor`
  - 负责消费 `todo action`、校验、落库、发事件、继续 loop
- `runtime/policy`
  - 第一版不拥有 todo 生成或写入职责
- `planner`
  - 第一版不是 todo 模式的中心能力

## 测试要求

第一版实现至少应覆盖以下场景：

- 显式 `plan_mode=none` 时拒绝 `todo action`
- 显式 `plan_mode=todo` 时允许 `todo action`
- `todo set` 能正确写入 `RunState`
- `set []` 能正确清空 todo 列表
- todo 更新会写入 `todo.updated` 事件
- resume 能读取最新 todo 快照
- todo action 会计入 turn count
- 自动 mode 选择在典型简单任务与复杂任务上行为稳定

## 结论

第一版 plan 能力不再追求“强 planner”，而是把 planning 收敛成一个可选的运行时任务清单：

- `none` 用于直接执行
- `todo` 用于显式维护运行中任务清单

这条路线更贴近通用 harness 的中心能力，也更贴近 DeerFlow 2.0 / Deep Agents 这类 `todo-driven execution` 的现实做法。
