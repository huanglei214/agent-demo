## Why

当前 agent-demo 已经有一组收敛后的核心 tools，但领域能力仍主要依赖通用 prompt 和模型自行决定工具链。像“查天气”这类需求更适合通过可复用的 skill 承载：它能复用现有 `web.search` / `web.fetch`，同时把步骤、来源要求和工具约束收成可维护的能力包。

此外，skills 系统如果希望后续兼容社区已有 skill，就不应一开始设计成私有数据库对象，而应优先支持文件系统原生格式和渐进加载模型。Codex、Claude Code、DeerFlow 这类系统都在往“skill 目录 + 描述文件 + 按需加载”的方向收敛，本仓库也适合沿这条路线扩展。

## What Changes

- 新增本地 skills registry/loading 能力，支持从项目级和用户级目录发现 skill。
- 采用兼容社区习惯的 skill 目录结构，核心入口为 `SKILL.md`，允许后续扩展 `references/`、`scripts/`、`assets/`。
- 为运行时增加 skill 激活和 prompt 注入路径，先支持显式 skill 和基于描述的轻量匹配。
- 为 skill 增加可选的工具白名单约束，避免 skill 激活后无限制扩大工具面。
- 第一批只提供一个示范 skill：`weather-lookup`，复用 `web.search` 与 `web.fetch`。

## Capabilities

### New Capabilities
- `skill-registry`: 本地 skill 发现、元数据索引、按需加载与运行时激活。

### Modified Capabilities
- `harness-runtime-core`: 运行时需要支持 skill 激活、prompt 注入和 tool allowlist 收窄。

## Impact

- 新增 `skills/` 或等效 skill 目录约定与加载器。
- 影响 `internal/prompt/`、`internal/app/` 和工具可见性装配逻辑。
- 需要为 `weather-lookup` 提供一个首个示范 skill 包。
- 本次变更不要求同时实现 skill marketplace、远程安装或前端 skill 管理 UI。
