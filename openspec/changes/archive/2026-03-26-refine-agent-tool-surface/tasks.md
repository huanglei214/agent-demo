## 1. Refine Filesystem Tool Surface

- [x] 1.1 从工具注册表中移除 `fs.stat`
- [x] 1.2 更新文件系统工具描述和提示词，使默认核心集不再包含 `fs.stat`
- [x] 1.3 补充或更新测试，确保工具列表中不再暴露 `fs.stat`

## 2. Add `fs.str_replace`

- [x] 2.1 新增 `fs.str_replace` 工具，实现单次替换与可选多次替换
- [x] 2.2 约束 `fs.str_replace` 在找不到目标文本时返回结构化错误
- [x] 2.3 为 `fs.str_replace` 增加测试，覆盖成功替换、找不到目标文本和越界路径

## 3. Strengthen `fs.search`

- [x] 3.1 保持 `pattern` 为主输入，同时兼容 `query`
- [x] 3.2 明确空查询拒绝和结果截断行为
- [x] 3.3 更新规格、提示词或工具描述以反映新的搜索契约

## 4. Add Web Retrieval Tools

- [x] 4.1 新增 `web.search` 工具，返回结构化搜索结果
- [x] 4.2 新增 `web.fetch` 工具，返回结构化页面内容
- [x] 4.3 为 `web.search` 和 `web.fetch` 增加超时、输出截断和测试

## 5. Add Command Execution Tool

- [x] 5.1 新增 `bash.exec` 工具
- [x] 5.2 为 `bash.exec` 定义 `workdir`、`timeout_seconds`、输出截断和结构化返回
- [x] 5.3 将工具访问级别扩展为 `read_only | write | exec`
- [x] 5.4 为 `bash.exec` 增加测试，覆盖成功执行、非零退出码和超时

## 6. Surface and Documentation

- [x] 6.1 更新 README 和工具清单说明，反映新的核心 8 工具
- [x] 6.2 更新 HTTP `/api/tools` 和相关前端展示，使访问级别正确暴露 `exec`
- [x] 6.3 确保 AG-UI / Web UI / CLI 中的工具描述与新工具面一致

## 7. Verification

- [x] 7.1 运行 Go 测试，验证所有工具相关路径
- [x] 7.2 手动验证工具列表输出，确认 `fs.stat` 已移除且新增工具可见
- [x] 7.3 手动验证至少一条联网查询和一条命令执行路径
