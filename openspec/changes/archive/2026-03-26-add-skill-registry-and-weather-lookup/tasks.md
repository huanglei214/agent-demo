## 1. Skill Registry

- [x] 1.1 定义本地 skill 目录约定，支持项目级 `.skills/` 和用户级 `~/.agent-demo/skills/`
- [x] 1.2 实现 `SKILL.md` frontmatter 解析与元数据索引
- [x] 1.3 为同名 skill 定义项目级优先于用户级的覆盖规则

## 2. Runtime Integration

- [x] 2.1 为运行时增加 skill 激活入口，支持显式指定 skill
- [x] 2.2 增加基于 skill 描述的最小自动匹配路径，确保天气问题可命中 `weather-lookup`
- [x] 2.3 在 prompt builder 中注入 skill 正文指令
- [x] 2.4 根据 `allowed-tools` 收窄当前 run 的工具面

## 3. First Skill Package

- [x] 3.1 新增首个示范 skill `weather-lookup`
- [x] 3.2 为 `weather-lookup` 编写 `SKILL.md`，要求搜索后必须读取页面并给出来源
- [x] 3.3 如有必要，为 `weather-lookup` 补充最小 references 资源

## 4. Verification

- [x] 4.1 为 skill registry 与 frontmatter 解析补充测试
- [x] 4.2 为 skill 激活与 tool allowlist 收窄补充运行时测试
- [x] 4.3 手动验证一次天气问题的端到端路径，确认出现 `web.search -> web.fetch -> final answer`
- [x] 4.4 更新 README 或相关文档，说明 skill 目录约定、作用域和第一批 `weather-lookup`
