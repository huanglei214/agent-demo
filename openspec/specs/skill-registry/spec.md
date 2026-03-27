# skill-registry

## Purpose
定义本地 skills 的目录格式、元数据索引、按需加载、运行时激活与工具白名单约束。

## Requirements

### Requirement: 本地 skill 发现
系统 MUST 能从本地文件系统发现 skill，并支持项目级与用户级两个作用域。

#### Scenario: 发现项目级与用户级 skill
- **WHEN** 系统初始化或首次访问 skills registry
- **THEN** 系统 MUST 扫描项目级 `.skills/` 目录
- **THEN** 系统 MUST 扫描用户级 `~/.agent-demo/skills/` 目录
- **THEN** 系统 MUST 将每个包含 `SKILL.md` 的目录视为一个 skill 包

#### Scenario: 同名 skill 覆盖
- **WHEN** 项目级和用户级存在同名 skill
- **THEN** 系统 MUST 优先使用项目级 skill

### Requirement: 兼容社区常见的 skill 包格式
系统 MUST 采用文件系统原生 skill 包格式，以便后续兼容社区已有 skill。

#### Scenario: skill 目录包含标准入口文件
- **WHEN** 某个 skill 目录包含 `SKILL.md`
- **THEN** 系统 MUST 将该文件作为 skill 的规范入口
- **THEN** 系统 MAY 忽略不存在的 `references/`、`scripts/` 或 `assets/`

### Requirement: 渐进式加载
系统 MUST 避免在索引阶段读取所有 skill 的完整正文和资源。

#### Scenario: 初始化阶段只索引元数据
- **WHEN** 系统建立 skills registry
- **THEN** 系统 MUST 只读取 `SKILL.md` 的最小元数据
- **THEN** 系统 MUST 不在该阶段批量加载完整正文或附属资源

#### Scenario: 命中 skill 时按需加载
- **WHEN** 某次运行激活一个 skill
- **THEN** 系统 MUST 读取完整 `SKILL.md`
- **THEN** 系统 MAY 继续按需读取该 skill 引用的 `references/` 或 `scripts/`

### Requirement: 运行时 skill 激活
系统 MUST 支持在一次运行中激活 skill，并让该 skill 影响 prompt 与工具面。

#### Scenario: 显式指定 skill
- **WHEN** 用户或调用方显式指定一个存在的 skill
- **THEN** 系统 MUST 激活该 skill
- **THEN** 系统 MUST 将 skill 指令注入 prompt

#### Scenario: 自动命中 weather-lookup
- **WHEN** 用户请求明显是在查询实时天气信息
- **THEN** 系统 MUST 能自动命中 `weather-lookup`
- **THEN** 系统 MUST 将该 skill 指令注入 prompt

### Requirement: Skill 限制可用工具
系统 MUST 支持 skill 通过 `allowed-tools` 收窄当前运行中的可用工具集。

#### Scenario: weather-lookup 收窄工具面
- **WHEN** 系统激活 `weather-lookup`
- **THEN** 当前运行中的可用工具 MUST 至少收窄为 `web.search` 和 `web.fetch`
- **THEN** 系统 MUST 不要求其它无关工具对该 skill 可见

### Requirement: 首个示范 skill 为 weather-lookup
系统 MUST 提供一个可工作的 `weather-lookup` skill，用于验证 skill 系统的最小闭环。

#### Scenario: 使用 weather-lookup 查询天气
- **WHEN** 用户询问某个城市当前天气
- **THEN** 系统 MUST 先调用 `web.search`
- **THEN** 系统 MUST 再调用 `web.fetch`
- **THEN** 最终回答 MUST 给出天气结论而不只是链接
- **THEN** 最终回答 MUST 包含来源
