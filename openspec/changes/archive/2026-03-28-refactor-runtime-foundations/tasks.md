## 1. 运行时主流程拆分

- [x] 1.1 将 `internal/app/run_service.go` 中的 agent loop 主循环提取到独立文件，并保持现有运行行为不变
- [x] 1.2 将 delegation / child run 执行与结果接回逻辑提取到独立文件，并保持现有事件记录语义不变
- [x] 1.3 将 action dispatch 与相关校验逻辑提取到独立文件，并让 `run_service.go` 主要保留入口、状态转换和顶层错误处理
- [x] 1.4 为拆分后的运行时主流程补充或调整定向测试，并运行 `make verify-scenarios`

## 2. 存储与依赖注入基础优化

- [x] 2.1 优化 `internal/store/filesystem/event_store.go` 的 `NextSequence`，避免每次追加事件前全量解析历史事件
- [x] 2.2 为 `internal/memory/`、`internal/context/`、`internal/prompt/` 定义最小必要接口，并让 `internal/app/services.go` 依赖接口而非具体实现
- [x] 2.3 为 `NextSequence` 优化补充定向测试或 benchmark，并为接口化后的装配路径补充测试

## 3. 并发与安全加固

- [x] 3.1 为 memory 文件存储增加文件锁与原子写入，防止并发写覆盖
- [x] 3.2 为 `bash.exec` 增加危险命令与危险命令链拦截，并补充对应测试
- [x] 3.3 为 `web.fetch` 增加本地/内网地址与解析后受限地址的阻断，并补充对应测试
- [x] 3.4 为 filesystem 路径解析增加 symlink 逃逸检测，并补充对应测试
- [x] 3.5 运行相关定向测试与 `make verify-scenarios`，确认安全加固未破坏现有能力

## 4. 入口、错误与配置统一

- [x] 4.1 替换 `strings.Title` 等已废弃调用，消除当前实现中的 deprecated API 使用
- [x] 4.2 将 `cmd/web` 入口调整为 Cobra 风格装配，同时保持现有启动能力与参数语义兼容
- [x] 4.3 集中定义运行时基础错误类型，并将现有零散错误建模迁移到统一风格
- [x] 4.4 为配置层增加“工作区配置文件 / 用户配置文件 / 环境变量 / 显式参数”覆盖链路，并补充优先级测试

## 5. Prompt 模板外部化

- [x] 5.1 在 `internal/prompt/` 下建立基于 `embed.FS` 的模板加载机制，并迁移现有核心 prompt 模板
- [x] 5.2 调整 prompt builder 以使用外部模板文件，同时保持现有 `lead-agent / subagent / forced-final` 等语义不变
- [x] 5.3 为模板加载和 prompt 构建路径补充测试，并确认回归场景输出未被破坏

## 6. 文档与最终验证

- [x] 6.1 更新 README、Tech 文档或相关说明，使其与新的入口、配置和安全边界保持一致
- [x] 6.2 运行 `make build`、`make verify-scenarios` 和必要的定向测试，确认整轮基础重构通过验证
- [x] 6.3 复核本次 change 的 proposal / design / specs 与实现一致性，为后续 verification 和 archive 做准备
