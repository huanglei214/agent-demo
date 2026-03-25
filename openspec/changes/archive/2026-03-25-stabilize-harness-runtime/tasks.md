## 1. 运行稳定性与恢复

- [x] 1.1 为 Ark provider 增加超时、错误分类和失败原因透传
- [x] 1.2 在模型调用失败、工具失败和内部异常时统一收口 `Run` 终态，避免遗留长期 `running` 状态
- [x] 1.3 明确 `resume` 的可恢复条件，并为不可恢复场景返回清晰提示
- [x] 1.4 为“模型调用中断后恢复”和“工具执行后恢复继续运行”补充端到端测试

## 2. 场景化回归与验收

- [x] 2.1 固化基础规划、文件系统工具、多轮 chat、delegation 四条回归场景
- [x] 2.2 为每条场景定义必须出现的关键事件和运行工件
- [x] 2.3 增加统一回归入口，例如 `make verify-scenarios`
- [x] 2.4 为回归失败输出补充缺失事件、缺失工件和失败场景定位信息

## 3. 会话级可观察性与调试体验

- [x] 3.1 增加 session 维度的查看能力，用于展示最近消息和关联 run 列表
- [x] 3.2 优化 `inspect` 输出，突出当前状态、当前步骤、失败点和 child run 摘要
- [x] 3.3 区分 `replay` 与 `debug events` 的输出定位，分别面向人读摘要和原始事件排障
- [x] 3.4 为新的会话查看和调试输出补充 CLI 测试

## 4. 文档与工件说明收口

- [x] 4.1 更新 `README.md`，同步当前已实现能力和推荐验证方式
- [x] 4.2 更新 `docs/step1/TECHNICAL_SOLUTION.md`，去除过期的 greenfield 描述
- [x] 4.3 补充 `.runtime/` 工件说明，明确 `run.json`、`state.json`、`events.jsonl`、`children/`、`sessions/` 的职责
- [x] 4.4 统一文档中关于 sub-agent / delegation 的表述，避免歧义

## 5. 下一轮能力扩展准备

- [x] 5.1 明确 `plan.updated` 的触发规则，特别是 child run 返回 `needs_replan=true` 的处理逻辑
- [x] 5.2 梳理工具权限模型，为后续增加更多受控只读工具做准备
- [x] 5.3 评估并整理应用层服务边界，为后续 HTTP API 或 MCP 接入预留接口
