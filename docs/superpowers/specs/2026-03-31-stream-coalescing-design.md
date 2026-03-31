# 2026-03-31 Stream Coalescing Design

## Background

当前仓库已经支持回答流式输出，但真实 Ark provider 在接入 SDK 流式接口后，会把非常细碎的 delta 直接传到下游。这样虽然延迟最低，但在 Web 和 CLI 中都会表现成接近逐 token 或逐字跳动的输出，中文尤其明显，观感上不接近 ChatGPT 那种连续但平滑的打字感。

这次设计的目标不是改变最终答案内容，而是统一整理回答流的 chunk 粒度，让所有流式出口都具备更稳定、更自然的输出节奏。

## Goals

- 统一处理所有回答流出口的 chunk 粒度，包括 Web / AG-UI、CLI，以及其他基于 answer stream 的消费方。
- 保持 provider 尽快产出原始 delta，不把 UX 特化逻辑塞进 Ark provider。
- 在不改变最终答案文本的前提下，让流式输出更接近 ChatGPT 的连续打字感。
- 保持 structured non-final action 不进入答案流。
- 让 mock 和 ark 走同一套流式整理逻辑，便于稳定验证。

## Non-Goals

- 不引入新的用户配置项或前端开关。
- 不修改最终持久化的 assistant message 文本。
- 不改变 planner、tool call、runtime event 的语义。
- 不做语义级别的重写、断句或润色，只调整 delta 的发送时机和分块方式。

## Approach Options

### Option 1: Provider-level coalescing

在 Ark `GenerateStream` 中直接把细碎 token 合并后再发给 sink。

优点：
- 改动面最小。
- 能最快改善当前 Ark 的真实体验。

缺点：
- 只能覆盖 Ark provider。
- mock 或未来其他 provider 仍会有不同的输出手感。
- 把展示层面的 UX 逻辑耦合进 provider，实现边界不清晰。

### Option 2: Common answer-stream coalescing

在回答流公共出口前新增一个通用 coalescing sink。provider 继续尽快输出原始 delta，统一由公共层决定何时向真实 sink flush。

优点：
- Web、CLI、AG-UI、mock、ark 共用一套规则。
- provider 只负责模型流式数据，边界最清楚。
- 后续接入新 provider 时自动获得一致的流式体验。

缺点：
- 需要梳理当前 answer stream 的公共封装点。
- 需要新增一组时间相关的测试。

### Option 3: Per-client throttling

在 Web、CLI 等各自的渲染层做节流或聚合。

优点：
- 不需要触碰 provider 或 agent 层。

缺点：
- 会形成多处实现，行为容易漂移。
- 不能保证“统一处理”。
- 真实运行工件和不同输出面之间更难对齐。

## Chosen Design

采用 Option 2：在回答流公共层新增统一的 stream coalescing sink。

核心原则：
- provider 负责尽快产出原始流。
- 公共 coalescing sink 负责把原始流整理成更适合人眼感知的 chunk。
- 所有消费回答流的出口都透过同一层获得一致行为。

## Architecture

### Placement

新增一个公共的 `coalescing stream sink`，放在 `internal/agent` 或其直接依赖的 answer stream 组装路径上，而不是放在：

- Ark provider 内部
- Web SSE 映射层
- CLI 渲染层

接入方式为：
- 现有上游 provider 仍接收一个 `model.StreamSink` 风格的 sink。
- 创建回答流 sink 时，先构建真实 sink，再包上一层 coalescing sink。
- provider 的 `Start/Delta/Complete/Fail` 调用先进入 coalescing sink，由它决定何时真正转发给下游。

### Sink Behavior Contract

coalescing sink 需要完整实现现有 sink 生命周期：
- `Start()`
- `Delta(text string)`
- `Complete()`
- `Fail(error)`

行为约定：
- `Start()` 只向下游转发一次。
- `Delta()` 将原始 delta 追加到内部缓冲，并按规则决定是否 flush。
- `Complete()` 在结束前必须先 flush 所有剩余内容，再调用下游 `Complete()`。
- `Fail()` 在失败前也必须 flush 所有剩余内容，再调用下游 `Fail()`，避免结尾内容丢失。
- 不改变 delta 内容本身，不重排字符，只改变何时往下游发。

## Chunking Policy

默认策略以“接近 ChatGPT 的连续打字感”为目标。

### Timing

- 常规刷新窗口：`50ms`
- 提前刷新阈值：`40ms`
- 静默上限：`80ms`

含义：
- 如果持续收到细碎 delta，默认在短时间窗口内先合并。
- 如果命中了更自然的边界，可以提前 flush。
- 即使一直没有理想边界，也不能让用户等待过久，最迟 `80ms` 必须 flush 一次。

### Boundary Rules

优先级从高到低如下：

1. 标点边界立即 flush
   - 中文：`，。！？；：`
   - 英文：`, . ! ? ; :`
   - 换行：`\n`

2. 空格边界允许提前 flush
   - 如果缓冲末尾是空格，且距上次发送已超过 `40ms`，允许立即 flush。

3. 长度上限提前 flush
   - 单个待发送缓冲达到约 `12-24` 个可见字符时提前 flush。
   - 具体实现上可以先从 `16` 个可见字符起步，后续按体验微调。

4. 静默上限强制 flush
   - 即使没有命中任何自然边界，只要累计等待达到 `80ms`，也必须 flush。

### Rationale

这套规则的效果是：
- 中文更倾向于按词组、短句、标点后输出，而不是逐字跳动。
- 英文更倾向于按单词片段或短短语输出，而不是长时间不动后突然一大截。
- 对极细粒度 token 流和较粗粒度 chunk 流都能兼容。

## Data Flow

1. provider 产生原始 delta。
2. 原始 delta 进入 coalescing sink。
3. coalescing sink 维护内部缓冲、上次发送时间、是否已开始等状态。
4. 满足 flush 条件时，把当前缓冲作为一个合并后的 delta 发给下游真实 sink。
5. 下游继续进入现有 answer stream 事件管道。
6. Web / AG-UI / CLI 收到的都是整理后的 chunk。
7. 最终 assistant message 的完整文本保持不变。

## Structured Response Handling

当前对 structured non-final action 的规则保持不变：
- 如果模型返回的是非 final action，不进入答案流。
- 如果模型返回 final answer，才进入答案流。

coalescing sink 只处理“已经确定要进入答案流的 delta”，不负责判断 structured action 是否属于 final。该判断仍保留在 provider 与现有 answer stream 语义边界内。

## Compatibility

### Providers

- Ark provider：继续尽快透传原始流，不再承担 UI 手感调优责任。
- Mock provider：也经过同一层 coalescing，确保测试环境与真实模型行为更一致。
- 未来 provider：只要接入既有流式 sink 接口，就自动继承相同策略。

### Output Surfaces

- Web / AG-UI：自动获得更顺滑的 `TEXT_MESSAGE_CONTENT` chunk。
- CLI：自动获得更稳定的终端流式输出。
- 其他基于 answer stream 的消费方：自动获得一致 chunk 策略。

### Persistence

- 最终 assistant message 内容不变。
- runtime 结果和消息存档不应因 coalescing 改变文本语义。
- 仅事件中 delta 的切分方式发生变化。

## Error Handling

- 如果下游 sink 的 `Start/Delta/Complete/Fail` 返回错误，coalescing sink 立即中止并返回该错误。
- 如果上游在流中途失败，coalescing sink 需要先尽量 flush 已缓冲的文本，再把失败传给下游。
- 不引入后台 goroutine 常驻发送逻辑，优先保持实现同步、可预测，避免额外并发清理问题。

## Testing Strategy

采用 TDD，优先补公共聚合层的单元测试，再接入实现。

### New Unit Tests

为 coalescing sink 新增测试，至少覆盖：

- 标点到达时立即 flush。
- 高频细碎 delta 在窗口内被合并成更大的 chunk。
- 没有自然边界时，超过静默上限会 flush。
- `Complete()` 前会 flush 剩余缓冲。
- `Fail()` 前会 flush 剩余缓冲。
- 最终只会有一个 `Start()` 和一个终态调用。

为避免时间相关测试脆弱，coalescing sink 应允许注入时钟或 `now()` 函数，测试里用可控时间推进，而不是依赖真实 sleep。

### Existing Flow Tests

更新现有回答流或 AG-UI 相关测试：
- 保持只有一个 `TEXT_MESSAGE_START`。
- 保持只有一个 `TEXT_MESSAGE_END`。
- 保持 `TEXT_MESSAGE_END` 仍然先于 `RUN_FINISHED`。
- 将原来“至少多个 chunk”的断言细化为“chunk 被合理聚合，而不是逐字一跳”。

### Real Verification

在实现完成后做两类真实验证：

1. Provider/CLI
   - `make run PROVIDER=ark MODEL=doubao-1.8 ...`
   - 观察终端输出是否从逐 token 碎片变为短词组/短句输出。

2. AG-UI/SSE
   - `make serve PROVIDER=ark MODEL=doubao-1.8`
   - `curl -N POST /api/agui/chat`
   - 检查 `TEXT_MESSAGE_CONTENT` 是否从单字流变成更自然的 chunk。

## Rollout Plan

1. 确认回答流公共接入点。
2. 先写 coalescing sink 测试，验证时间窗和边界规则。
3. 实现 coalescing sink。
4. 把它接到统一的 answer stream sink 构建路径。
5. 修正现有测试断言。
6. 用 mock 跑单元和集成测试。
7. 用 Ark 做一次 CLI 和 AG-UI 的真实回归。

## Risks

- 如果时间窗设置过大，会让输出显得卡顿。
- 如果长度阈值过小，仍会过碎，改善不明显。
- 如果把逻辑放在错误的层，会导致 Web 和 CLI 表现不一致。
- 如果时间相关测试使用真实 sleep，测试会变慢且不稳定。

## Decision Summary

本次设计将“接近 ChatGPT 的流式输出手感”定义为回答流公共行为，而不是 Ark 特例。实现上通过在公共 answer stream 路径加入统一 coalescing sink，按 `50ms` 小窗口、标点优先、空格辅助、`80ms` 上限与长度阈值来整理原始 delta，从而在 Web、CLI 和不同 provider 之间获得一致、自然的打字体验。
