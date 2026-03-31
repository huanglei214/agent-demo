# Prompt Context Trim Design

## Goal

在不改变现有 run loop、tool schema 和外部接口的前提下，压缩传给模型的 prompt input，去掉明显重复和低价值上下文，减少 token 噪声。

## Current Problems

当前 prompt 组装存在四类问题：

1. 同一信息重复出现。
   - `Workspace` 既在顶层 input 中出现，也在 `Pinned Context` 中再次出现。
   - 当前 step 既在 `Plan Context` 中出现，也在 builder 末尾再次出现。
2. follow-up prompt 里，最新工具结果和累计证据重复。
   - `New tool results` 包含本轮工具结果。
   - `Working evidence` 又会按工具名重新分组并再次包含同一批结果。
3. 空值上下文仍然被注入。
   - todo 模式下，即使 `state.Todos` 为空，也会拼接 `Todo snapshot: null`。
4. 弱语义上下文占 prompt。
   - `Recent Events` 只包含 `actor` 和 `sequence`，对模型几乎没有可操作价值。

这些内容叠加后，会让 lead-agent 的初始 prompt 和 post-tool follow-up prompt 都出现不必要的 token 消耗，并且增加模型在真正关键信息上的注意力稀释。

## Approaches

### Approach 1: 只删最明显的重复块

做法：
- 删除重复的 `Workspace`
- 删除重复的 `Current step`
- 保留其余结构不变

优点：
- 改动最小
- 风险最低

缺点：
- 不能解决 `New tool results` / `Working evidence` 的双写问题
- `Todo snapshot: null` 和 `Recent Events` 噪声仍然存在

### Approach 2: 保留现有结构，但把 follow-up 证据语义拆清楚

做法：
- `Workspace` 只保留一次
- 当前 step 只保留一次
- `Todo snapshot` 仅在非空时注入
- 默认不渲染 `Recent Events`
- follow-up prompt 中：
  - `New tool results` 只表示当前批次的新结果
  - `Working evidence` 只表示历史累计证据，不包含当前批次

优点：
- 解决最主要的重复问题
- 保留现有 prompt 结构和大部分调用方语义
- 不需要改 tool event 或 runtime store 结构

缺点：
- 需要在 prompt builder 内引入“从 working evidence 中排除当前批次”的逻辑
- 相关单测需要系统性更新

### Approach 3: 重写为统一 `Evidence` 模型

做法：
- 移除 `Pinned Context` / `Plan Context` / `Recent Events` 等 section 命名
- 移除 `New tool results` / `Working evidence` 二分法
- 将所有上下文改为一个压缩后的 `Evidence` 块

优点：
- 长期最干净
- 信息结构更统一

缺点：
- 改动面过大
- 会波及 prompt 测试、mock provider 行为、调试体验和已有观察习惯
- 不适合作为本轮的最小修复

## Chosen Design

采用 Approach 2。

这次只做“去重、空块过滤、低价值块下线、follow-up 证据拆清”的收敛，不重写整个 prompt 框架。

## Design Details

### 1. Run prompt 去重

对 lead-agent 的 run prompt 做以下调整：

- 顶层 `Workspace` 保留。
- `Pinned Context` 中移除 `Workspace` 项，仅保留 `Goal`。
- `Current step` 只保留一处。
  - 保留 `Plan Context` 中的 `Active Step`
  - 删除 builder 额外追加的 `Current step` 块

这样可以保持现有 section 结构基本稳定，同时消除两处明显重复。

### 2. Recent Events 默认不进入模型输入

`ModelContext.Render()` 不再默认渲染 `Recent Events` section。

原因：
- 当前事件内容只有 `event.Type`、`actor`、`sequence`
- 缺乏结果摘要、错误摘要或状态变化语义
- 对模型几乎不可消费

保留 `ModelContext.Recent` 数据结构本身，先只调整 render 路径，不影响后续如果想把它用于调试 UI 或后续更高质量摘要的可能性。

### 3. Todo 空块过滤

`InjectTodoContext()` 改为：

- 仅在 `run.PlanMode == todo` 时考虑注入 todo context
- 仅在 `len(state.Todos) > 0` 时注入 `Todo snapshot`
- `Todo rules` 也只在存在 todo 列表时一起注入

这样可以避免出现 `Todo snapshot: null` 这类纯噪声文本。

### 4. Follow-up prompt 证据去重

保留两个概念，但语义分离：

- `New tool results`
  - 只包含当前这一次 post-tool follow-up 要消费的工具结果
- `Working evidence`
  - 只包含历史累计证据
  - 不再重复包含当前批次工具结果

具体做法：

- 保持 `BuildWorkingEvidenceForPrompt()` 作为“聚合成功工具结果”的 helper。
- 在 `BuildFollowUpPrompt()` 内增加一个 helper，从传入的 `workingEvidence` 中剔除当前 `toolResults` 对应的条目。
- 如果剔除后没有历史证据，则整个 `Working evidence` section 不渲染。

这样 follow-up prompt 会表达为：

- “这是刚得到的新结果”
- “这是更早已经累积的历史证据”

而不是把同一批结果发两遍。

### 5. 子代理 delegation follow-up 仍复用同一规则

delegation 完成后的 follow-up 也走 `BuildFollowUpPrompt()`，因此会自动获得同样的去重行为：

- 当前 child result 出现在 `New tool results`
- 历史 evidence 如存在，则出现在 `Working evidence`
- 当前 child result 不会在两边重复

## File Changes

- `internal/context/manager.go`
  - 移除 pinned workspace
  - 停止在 render 中输出 `Recent Events`
- `internal/prompt/builder.go`
  - 删除追加的顶层 `Current step`
  - follow-up prompt 中剔除与当前批次重叠的 working evidence
- `internal/prompt/todo.go`
  - 只在 todo 非空时注入 todo snapshot 和 todo rules
- `internal/context/manager_test.go`
  - 更新 context section 断言
- `internal/prompt/builder_test.go`
  - 更新 run prompt / follow-up prompt 断言
  - 增加“working evidence 不包含当前批次结果”的覆盖
- `internal/service/run_test.go`
  - 更新 todo snapshot 相关断言，确保空 todo 不再注入

## Error Handling

这次不引入新的运行时错误路径。

唯一需要注意的是 follow-up 证据去重逻辑必须是“宽容”的：

- 如果 `workingEvidence` 结构不完整或类型不符合预期，不报错
- 直接按原值保留，避免因 prompt 压缩逻辑反向破坏执行链路

## Testing Strategy

### Targeted tests

- `go test ./internal/context ./internal/prompt ./internal/service -count=1`

验证：
- `Workspace` 不再重复
- `Current step` 不再重复
- `Recent Events` 不再渲染
- 空 todo 不再渲染
- follow-up prompt 里 `Working evidence` 不再重复当前批次结果

### Broader verification

- `go test ./internal/agent ./internal/context ./internal/prompt ./internal/service -count=1`

用于确认 executor 路径、resume 路径和 delegation follow-up 没有被 prompt 结构调整打坏。

## Risks

### Risk: 某些测试隐式依赖旧 prompt 文本

影响：
- 断言可能大量依赖具体 section 名称和旧文本布局

控制：
- 只改最少必要断言
- 保留主要 section 名称，避免无意义重命名

### Risk: 去重 helper 错删历史 evidence

影响：
- follow-up prompt 可能丢失跨轮累积证据

控制：
- 仅按 `tool_call_id` 精确删除当前批次项
- 如果结构不匹配则保守保留

## Non-Goals

- 不重写整个 prompt 分层体系
- 不改变 model metadata 结构
- 不改变 event store 或 memory store schema
- 不为 `Recent Events` 设计新的摘要语义，本轮只先移除其渲染
