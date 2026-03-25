## MODIFIED Requirements

### Requirement: 上下文组装
系统 MUST 在每轮模型调用前构造统一的模型上下文，并按优先级组装不同来源的信息。

#### Scenario: 构建多轮会话上下文
- **WHEN** 当前 `Run` 属于一个已经存在历史消息的 `Session`
- **THEN** 系统 MUST 将最近的用户/助手消息历史注入模型上下文
- **THEN** 系统 MUST 保持这些消息的时间顺序
- **THEN** 系统 MUST 限制注入消息数量，避免无界增长
