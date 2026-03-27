## Overview

本次变更把 skills 作为一等能力引入到 agent-demo，但范围控制在最小可用闭环：

- 发现本地 skill
- 索引 skill 元数据
- 在运行时激活 skill
- 将 skill 指令注入 prompt
- 按 skill 白名单收窄可用 tools
- 只提供一个示范 skill：`weather-lookup`

技能系统不替代 tools。tool 仍负责原子能力，skill 负责复用工作流、工具使用策略和输出约束。

## Goals

- 兼容文件系统原生的 skill 组织方式，优先支持 `SKILL.md`
- 让运行时可以在不新增专用天气 tool 的前提下，通过 `weather-lookup` skill 改善联网查询体验
- 保持工具面收敛，避免把每个领域需求都实现成新的 tool
- 为未来支持社区 skill 留出格式兼容空间

## Non-Goals

- 不实现远程 skill marketplace 或社区安装命令
- 不实现复杂的 skill 排名、依赖图或版本求解
- 不实现前端 skill 管理页面
- 不把 skill 和 subagent 混成同一抽象

## Skill Package Format

第一版采用目录即 skill 的形式，兼容如下结构：

```text
.skills/
  weather-lookup/
    SKILL.md
    references/
    scripts/
    assets/
```

第一版只要求 `SKILL.md` 必须存在；其他目录都是可选的。

`SKILL.md` 第一版支持 YAML frontmatter + 正文，至少解析以下字段：

- `name`
- `description`
- `allowed-tools`
- `compatibility`
- `tags`

正文部分作为完整 skill instructions，在 skill 被激活时注入 prompt。

## Skill Sources And Precedence

第一版支持两类来源：

1. 项目级：`<workspace>/.skills/`
2. 用户级：`~/.agent-demo/skills/`

若存在同名 skill，优先级如下：

1. 项目级
2. 用户级

设计上预留第三类来源“社区缓存目录”，但本次不实现安装与同步逻辑。

## Loading Model

采用 progressive disclosure：

1. 启动或首次访问时，只索引 skill 目录和 `SKILL.md` frontmatter
2. 真正命中某个 skill 时，再读取完整 `SKILL.md`
3. 只有在 `SKILL.md` 明确引用 `references/` 或 `scripts/` 时，才继续按需读取这些资源

这样可以避免把所有 skill 内容都无差别塞进上下文。

## Runtime Activation

第一版支持两种激活方式：

1. 显式指定 skill 名称
2. 基于 `description` 的轻量匹配

第一批只要求 `weather-lookup` 能在以下两种情况下被激活：

- 用户显式指定使用 `weather-lookup`
- 用户问题明显是在查询天气、温度、降雨等实时天气信息

## Prompt Integration

当 skill 被激活时，prompt builder 需要追加：

- skill 名称和描述
- `SKILL.md` 正文指令
- 可用工具约束

注入顺序：

1. 基础运行时 prompt
2. 当前任务与上下文
3. 激活 skill 指令
4. tool surface 与约束说明

## Tool Allowlist

skill 可以声明 `allowed-tools`。运行时激活该 skill 后，需要将当前 run 的可用工具面收窄到：

- `allowed-tools` 中声明的工具
- 运行时所需的最小系统工具（若存在）

第一版 `weather-lookup` 允许的核心工具为：

- `web.search`
- `web.fetch`

## First Bundled Skill: weather-lookup

`weather-lookup` 是第一批唯一示范 skill，目标是：

- 搜索权威天气来源
- 必须读取至少一个结果页面，而不是只返回链接
- 回答中给出简洁天气结论和来源
- 若信息不一致或内容不足，明确说明不确定性

它复用现有 `web.search` 和 `web.fetch`，不新增专用天气 tool。

## Verification

第一版验收至少覆盖：

- 能发现项目级 `weather-lookup`
- 能在天气问题下激活 `weather-lookup`
- 激活后 tool allowlist 被收窄到 `web.search` / `web.fetch`
- 一条天气查询 run 中出现 `web.search -> web.fetch -> answer`

