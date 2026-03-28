## Why

当前 `agent-demo` 在连续迭代后，运行时主链路、存储层、配置层和安全边界已经出现明显的复杂度堆积：`run_service.go` 过大、事件序号生成存在低效路径、关键组件难以替换或注入测试替身，且工具安全边界、入口一致性与配置管理也还不够稳。现在需要做一轮集中式基础重构，在不破坏现有 Makefile、HTTP 路径和兼容行为的前提下，为后续 Web 主入口、真实流式输出和更多运行时能力打下更稳定的底座。

## What Changes

- 拆分 `internal/app/run_service.go`，将 agent loop、delegation 执行、action 分发与顶层运行状态管理解耦。
- 优化事件序号生成路径，避免 `NextSequence` 在事件增多时反复全量解析 `events.jsonl`。
- 为 memory、context、prompt 等核心组件抽出接口，降低 `Services` 对具体实现的绑定强度。
- 为 memory 文件存储增加并发读写保护，减少多 run 并发写入时的数据覆盖风险。
- 为 `bash.exec`、`web.fetch` 和 filesystem 工具补齐更严格的安全边界，包括危险命令限制、SSRF 防护和 symlink 逃逸检测。
- 统一 deprecated API、错误定义和入口风格，包括替换 `strings.Title`、将 `cmd/web` 对齐到 Cobra 风格、收敛 sentinel errors。
- 增强配置层，支持从工作区或用户目录读取配置文件，并保持环境变量覆盖能力。
- 将 prompt 模板从 Go 硬编码迁移为 `embed.FS` 加载的模板文件，便于独立迭代。

## Capabilities

### New Capabilities

无

### Modified Capabilities

- `harness-runtime-core`: 调整运行时核心实现约束，包括事件序号生成效率、组件依赖方式、CLI/Web 入口一致性以及配置加载方式。
- `filesystem-tools`: 强化文件系统工具的工作区边界控制，明确 symlink 解析后仍不得逃逸工作区。
- `web-retrieval-tools`: 强化 `web.fetch` 的目标地址校验与抓取安全边界，阻止对本地或内网地址的访问。
- `command-execution-tools`: 强化 `bash.exec` 的受控执行约束，禁止执行危险命令或等效高风险命令链。

## Impact

- `internal/app/` 运行时主流程与服务装配
- `internal/store/filesystem/` 事件存储与序号生成逻辑
- `internal/memory/`、`internal/context/`、`internal/prompt/` 的接口与实现边界
- `internal/tool/bash/`、`internal/tool/web/`、`internal/tool/filesystem/` 的安全控制
- `internal/config/` 配置加载逻辑
- `cmd/web` 与 CLI/入口一致性
- 回归测试、基准测试和相关文档
