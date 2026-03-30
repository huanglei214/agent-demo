# Architecture Boundary Follow-Ups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 收紧 service/agent/store 边界，补齐请求生命周期传递，并把流式事件链路从“能用”整理到“可维护”。

**Architecture:** 先处理结构边界，再处理 `context.Context` 贯穿，随后优化 SSE/event 路径，最后做低风险加固与清理。每一批都保持外部 API 和 `.runtime/` 工件兼容，并用定向测试和全量回归兜底。

**Tech Stack:** Go, Chi HTTP, SSE, filesystem-backed runtime store, OpenSpec

---

### Task 1: 收紧 Service 与 Runner 边界

**Files:**
- Modify: `internal/agent/runner.go`
- Modify: `internal/service/runner.go`
- Modify: `internal/service/services.go`
- Modify: `internal/interfaces/http/handlers_runs.go`
- Modify: `internal/interfaces/http/agui/service.go`
- Modify: `internal/service/run_test.go`
- Modify: `internal/service/services_test.go`

- [x] 盘点所有越过 service API 直接访问 `Runner` / `StateStore` / `EventStore` 的入口，确认本批只收口边界，不改变 HTTP 返回形状。
- [x] 在 `internal/agent/runner.go` 决定 `GenerateWithModelTimeout` 的归属：要么纳入 `Runner`，要么定义单独小接口，并同步调整 compile-time check。
- [x] 改造 `internal/service/runner.go`，去掉对底层 runner 的匿名 type assertion 穿透。
- [x] 在 `internal/service/services.go` 为 HTTP/AGUI 需要的读取行为补 service 方法，例如“加载 run + state + recent messages”这类组合查询，避免上层再直接碰 store。
- [x] 更新 [internal/interfaces/http/handlers_runs.go](/Users/huanglei/repos/src/github.com/huanglei214/agent-demo/internal/interfaces/http/handlers_runs.go) 和 [internal/interfaces/http/agui/service.go](/Users/huanglei/repos/src/github.com/huanglei214/agent-demo/internal/interfaces/http/agui/service.go) 让它们只通过 `service.Services` 暴露的方法取数。
- [x] 清理测试里对 `services.Runner.(agent.Executor)` 的具体类型断言，改成验证行为或从装配结果验证依赖连线。
- [x] Run: `go test ./internal/service ./internal/interfaces/http/...`
- [x] Expected: 所有 service 和 HTTP 相关测试通过，且不再出现新的具体类型断言。

### Task 2: 贯穿 Context 生命周期

**Files:**
- Modify: `internal/agent/runner.go`
- Modify: `internal/agent/loop.go`
- Modify: `internal/agent/resume.go`
- Modify: `internal/service/run.go`
- Modify: `internal/service/resume.go`
- Modify: `internal/interfaces/http/handlers_runs.go`
- Modify: `internal/interfaces/http/handlers_agui.go`
- Modify: `internal/interfaces/http/agui/service.go`
- Modify: `cmd/web/main.go`
- Modify: related tests under `internal/service/` and `internal/interfaces/http/`

- [x] 为 `service` 和 `agent.Runner` 的执行/恢复入口统一补 `ctx context.Context` 参数，约定它始终作为第一个参数。
- [x] 把 [internal/service/run.go](/Users/huanglei/repos/src/github.com/huanglei214/agent-demo/internal/service/run.go) 里 `planner.CreatePlan(context.Background(), ...)` 改成使用上层传入的请求上下文。
- [x] 让 HTTP handler 和 AGUI 入口把 `r.Context()` 继续往 service / runner / model 调用链透传。
- [x] 为 `cmd/web/main.go` 增加 signal 监听和 `server.Shutdown(ctx)`，保证活跃 SSE 连接在退出时有序关闭。
- [x] 为“请求取消”至少补一条测试：请求上下文取消后，执行链应尽快返回 `context.Canceled` 或等价失败。
- [x] Run: `go test ./internal/agent ./internal/service ./internal/interfaces/http/... ./cmd/web`
- [x] Expected: 执行和恢复入口全部收 `ctx`，现有 API 语义保持不变，新增 graceful shutdown 路径通过测试。

### Task 3: 优化 SSE 与事件写入路径

**Files:**
- Modify: `internal/interfaces/http/handlers_runs.go`
- Modify: `internal/service/replay.go`
- Modify: `internal/service/run.go`
- Modify: `internal/agent/helpers.go`
- Modify: `internal/store/interface.go` or relevant store interface file
- Modify: `internal/store/filesystem/...`
- Modify: tests for replay/streaming/event append

- [x] 先决定本批的最小落地方案：优先实现 `ReadAfter(runID, afterSeq)` 之类的增量读取，而不是直接把整个流式模型改成 channel-only。
- [x] 给 event store 接口和 filesystem 实现补增量读取能力，并保持现有 `ReadAll` 调用兼容。
- [x] 把 [internal/interfaces/http/handlers_runs.go](/Users/huanglei/repos/src/github.com/huanglei214/agent-demo/internal/interfaces/http/handlers_runs.go) 的 SSE 轮询从“每次全量 replay”改为“按 after sequence 增量读取”。
- [x] 评估并合并 [internal/service/run.go](/Users/huanglei/repos/src/github.com/huanglei214/agent-demo/internal/service/run.go) 与 [internal/agent/helpers.go](/Users/huanglei/repos/src/github.com/huanglei214/agent-demo/internal/agent/helpers.go) 中重复的 `newEvent` / `appendEvent` / `appendEvents` 逻辑；如果一轮内能安全收口，就抽到统一 helper，否则至少保证两边字段构造一致。
- [x] 为 SSE 新增测试，覆盖 `after` 参数、终态 run、空增量和错误路径。
- [x] Run: `go test ./internal/service ./internal/interfaces/http/... ./internal/store/...`
- [x] Expected: SSE 不再依赖全量读取事件文件，event append/read 行为对外保持兼容。

### Task 4: 加固与代码形态清理

**Files:**
- Modify: `internal/agent/executor.go`
- Modify: `internal/agent/helpers.go`
- Modify: `internal/agent/dispatch.go`
- Modify: `internal/agent/delegation.go`
- Modify: `internal/agent/loop.go`
- Modify: `internal/agent/resume.go`
- Modify: `internal/delegation/manager.go`
- Modify: `internal/runtime/sequence.go`
- Modify: related tests under `internal/agent/` and `internal/delegation/`

- [x] 把 `Executor` 的方法接收者统一成 pointer receiver，避免继续按值复制大结构体，并同步更新接口满足性检查。
- [x] 给 `delegation.Manager.depth()` 增加循环或最大深度保护，避免损坏的 run 链把遍历卡死。
- [x] 处理 `SequenceCursor`：要么改成原子计数，要么明确注释它只能在串行或 reserve 后并行的约束下使用。
- [x] 如果前 3 个任务已经稳定，再考虑把 `startRun` 适度拆分为准备实体、持久化、发事件三段；不强求在本批做参数对象化。
- [x] Run: `go test ./internal/agent ./internal/delegation ./internal/runtime ./...`
- [x] Run: `make verify-scenarios`
- [x] Expected: 全量测试和场景回归通过，且本轮没有对外行为漂移。

### 验收清单

- [x] `service` 不再通过具体类型断言穿透到 `Executor`
- [x] HTTP / AGUI 不再直接读取 `StateStore` / `EventStore`
- [x] 执行与恢复链路统一接收并传递 `context.Context`
- [x] Web 入口支持 graceful shutdown
- [x] SSE 改为增量事件读取
- [x] 关键回归通过：`go test ./...` 与 `make verify-scenarios`
