## Context

`agent-demo` 当前已经从单纯的本地 CLI demo 演进为同时具备 CLI、HTTP API、Web UI、skills、subagent delegation 和外部检索工具的本地 Agent Harness。随着近期连续引入 lead-agent / subagent 分层、批量工具调用、检索收口保护、AG-UI 聊天流和 Web-first 入口，核心运行时已经出现几类基础问题：

- [internal/app/run_service.go](/Users/huanglei/repos/src/github.com/huanglei214/agent-demo/internal/app/run_service.go) 体量过大，承担了 run 生命周期、agent loop、tool batch、forced-final、delegation、result 解析等多类职责。
- 事件序号生成仍依赖读取并解析完整 `events.jsonl`，在事件增多时会引入明显的重复开销。
- `Services` 仍直接依赖 memory、context、prompt 等具体实现，测试注入和未来替换实现的成本偏高。
- 文件存储、命令执行、网页抓取和路径解析的安全边界还不够完整。
- 配置加载、错误建模、CLI/Web 入口风格和 prompt 模板组织方式还存在历史包袱。

这次设计的目标不是新增一组对外产品能力，而是在不破坏现有 Makefile、HTTP 路径、兼容 CLI 入口和当前 Web 使用方式的前提下，完成一轮基础设施级重构，使运行时更容易继续演化。

## Goals / Non-Goals

**Goals:**

- 将核心运行时从“单文件堆叠”调整为职责更明确的模块组织，降低后续开发与调试成本。
- 在不改变对外主路径的前提下，提高存储、配置、工具边界和入口装配的一致性。
- 为未来的真实流式输出、更多 provider、更多 skill 和更重的 Web 主工作台提供更稳的内部基础。
- 将本次改造拆成可验证的技术子任务，确保每个部分都能用现有回归脚本和定向测试验证。

**Non-Goals:**

- 不重新设计现有 HTTP API 路径、Web 页面结构或 Makefile 目标名。
- 不在这次改造中新增新的外部产品能力，例如新的工具类型、全新的聊天交互协议或新的前端页面。
- 不要求一次性重写全部 prompt 或 planner 逻辑；prompt 外部化仅解决组织方式，不重写当前提示策略。
- 不将当前系统整体迁移为远程服务或多机架构。

## Decisions

### 1. 将本次改造作为一个总 change 管理，但实现仍按多个技术子主题分批落地

**Decision**

在 OpenSpec 中使用一个总 change `refactor-runtime-foundations` 统一描述这轮基础改造，但在实际实现与提交时按多个技术主题分批推进，例如运行时拆分、事件存储优化、接口抽象、安全加固等。

**Rationale**

- 这些改动都属于“同一轮基础重构”，共享同一组背景、目标和风险。
- 但如果在实现层一次性混成一个大 patch，会显著增加回归和 review 难度。
- 总 change 能保证设计一致性，分批实现能降低落地风险。

**Alternatives considered**

- 为每个任务分别起一个独立 change：更细，但规格和背景会高度重复，管理成本偏高。
- 完全一次性实现：上下文集中，但风险和回滚成本太高。

### 2. `run_service.go` 仅保留运行入口、状态控制和顶层错误处理，agent loop / delegation / action dispatch 按职责拆分

**Decision**

将 [internal/app/run_service.go](/Users/huanglei/repos/src/github.com/huanglei214/agent-demo/internal/app/run_service.go) 重构为一个较薄的运行编排入口，拆出至少以下职责：

- `agent_loop.go`：主循环与阶段推进
- `delegation_executor.go`：child run 创建、等待、结果接回
- `action_dispatch.go`：`final / tool / delegate` 的动作执行与校验
- 必要时保留 `run_service.go` 作为入口门面

**Rationale**

- 当前单文件承载了过多交叉逻辑，是后续改造的主要阻力。
- 最近已经引入 `lead-agent / subagent`、tool batch、forced final、retrieval progress 等逻辑，继续堆叠会让复杂度失控。
- 按职责拆分后更容易写针对性测试，也减少并行开发冲突。

**Alternatives considered**

- 只做函数重排不拆文件：短期改动更小，但仍然无法解决文件级别认知负担。
- 进一步抽出新的大型 service：会引入更多抽象层，当前阶段没有必要。

### 3. 为 memory、context、prompt 抽稳定接口，但保留现有构造函数和默认实现

**Decision**

在 `internal/memory/`、`internal/context/`、`internal/prompt/` 里定义窄接口，并让 [internal/app/services.go](/Users/huanglei/repos/src/github.com/huanglei214/agent-demo/internal/app/services.go) 持有接口类型；同时保留现有 `NewManager()` / `NewBuilder()` 返回具体实现的方式，以免外围构造方式大面积变化。

**Rationale**

- 当前 `Services` 直接绑定具体结构体，不利于测试替身注入。
- 未来引入不同 memory/context/prompt 实现时会更容易切换。
- 保留构造函数与默认实现，能把改动限制在依赖注入边界，而不是扩散到整个调用图。

**Alternatives considered**

- 把所有组件都接口化：过度设计，收益不成比例。
- 完全不抽接口：短期简单，但继续阻碍测试与演化。

### 4. 事件序号优化采用“按行计数”的低成本方案，不引入额外索引文件

**Decision**

优化 [internal/store/filesystem/event_store.go](/Users/huanglei/repos/src/github.com/huanglei214/agent-demo/internal/store/filesystem/event_store.go) 中 `NextSequence` 的实现，改为读取 `events.jsonl` 后按换行数计数，而不是解析全部 JSON 事件；不引入额外的序号索引文件或独立元数据文件。

**Rationale**

- 当前问题主要是“反复全量解析 JSON”过重，而不是文件读取本身不可接受。
- 按行计数已经能把复杂度从“读取+解析所有事件对象”降到“读取+计数”，实现简单且稳妥。
- 不引入额外索引文件可以避免新的持久化一致性问题。

**Alternatives considered**

- 维护独立 sequence 元数据文件：性能更好，但需要额外的一致性控制。
- 完全保留现状：对事件较多的 run 成本不必要地偏高。

### 5. 并发与安全加固优先走“最小但明确”的基础保护

**Decision**

本轮安全与并发加固采用保守策略：

- `memory.FileStore` 增加文件锁和原子写入，解决 read-all + write-all 的竞争覆盖风险。
- `bash.exec` 增加第一层危险命令限制，但不将本轮目标扩大为完整 shell policy engine。
- `web.fetch` 增加本地/内网地址阻断，优先覆盖典型 SSRF 入口。
- filesystem 路径解析在 symlink 展开后继续校验工作区边界。

**Rationale**

- 当前最需要的是把明显薄弱点补齐，而不是一次性做成完整安全框架。
- 这些保护都能以较小改动显著降低风险。
- 如果后续需要更严格的 allowlist/policy 机制，可以在此基础上再演进。

**Alternatives considered**

- 一次性引入完整命令 allowlist 和网络策略系统：更强，但超出本轮范围。
- 只依赖 prompt 约束，不做运行时硬限制：不符合当前系统的安全边界要求。

### 6. `cmd/web` 与配置、错误建模一并收敛到更统一的运行时风格

**Decision**

- 将 [cmd/web/main.go](/Users/huanglei/repos/src/github.com/huanglei214/agent-demo/cmd/web/main.go) 从 `flag` 风格调整为 Cobra 风格，使 CLI 与 Web 入口在参数装配方式上更一致。
- 用集中定义的 sentinel errors 替代零散的“手写 not found / unsupported”错误风格。
- 配置加载增加“文件配置 + 环境变量覆盖”的层次，但保持现有函数签名和外部调用方式尽量不变。

**Rationale**

- 入口风格统一会降低维护心智负担。
- sentinel errors 更利于错误分类、日志和测试判断。
- 配置层增强后，后续 Web-first 与本地开发会更容易管理 provider、timeout 和 runtime root。

**Alternatives considered**

- 保持 `cmd/web` 使用 `flag`：继续维持入口风格不一致。
- 仅做错误建模，不动配置：可以，但会错过一次统一整理基础装配层的机会。

### 7. Prompt 模板外部化只解决“组织与迭代”问题，不改变当前提示职责边界

**Decision**

将 prompt 模板迁移到 `embed.FS` 加载的模板文件，但不借这次改造顺手重写 prompt 策略；运行时当前已有的 `lead-agent / subagent`、forced final、tool batch 等语义保持不变，只改变模板的存放和加载方式。

**Rationale**

- 当前 prompt 还在快速演进，先把模板组织方式独立出来更利于迭代。
- 如果模板外部化同时伴随大规模 prompt 重写，会把验证难度显著拉高。

**Alternatives considered**

- 继续硬编码：短期简单，但后续修改 prompt 组织成本持续偏高。
- 外部化并同时重写全部 prompt：风险过大。

## Risks / Trade-offs

- [大范围重构回归风险] → 按子主题分批实现，并在每一批至少运行定向测试和 `make verify-scenarios`
- [接口抽象过度导致复杂度上升] → 只抽 memory/context/prompt 的最小必要接口，不做全仓库接口化
- [安全加固影响现有工具行为] → 保留兼容参数与公共入口不变，新增保护以显式错误结果呈现
- [配置加载增强带来优先级混乱] → 明确顺序为“配置文件 → 环境变量 → flag/显式参数覆盖”，并补优先级测试
- [prompt 外部化使调试分散] → 保持模板命名与职责边界清晰，并在 builder 层保留统一装配入口

## Migration Plan

1. 先完成不改变外部行为但能降低基础风险的改造：
   - `run_service.go` 拆分
   - `NextSequence` 优化
   - 核心接口抽象
2. 再补并发与安全边界：
   - memory 文件锁
   - `bash.exec` / `web.fetch` / filesystem 安全加固
3. 然后统一入口、错误和配置层：
   - `cmd/web` Cobra 化
   - sentinel errors
   - 配置文件加载
4. 最后做 prompt 模板外部化，并重新跑完整回归

回滚策略：

- 每个子主题保持独立提交或清晰 patch 边界，出现回归时优先回滚对应子主题，而不是整轮重构全部回退。
- 外部入口、Makefile 目标和 HTTP 路径在本轮不变，确保出现问题时用户面冲击最小。

## Resolved Decisions During Implementation

- 配置文件格式在实现阶段确定为 JSON，当前支持：
  - 用户级配置：`~/.agent-demo/config.json`
  - 工作区配置：`<workspace>/.agent-demo.json`
- `bash.exec` 第一版危险命令限制采用“显式危险命令与命令链黑名单 + 运行时拒绝”的保守方案，为后续更严格的 allowlist / policy 机制预留演进空间。
