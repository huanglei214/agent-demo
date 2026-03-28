## ADDED Requirements

### Requirement: 核心服务装配支持替换关键组件实现
系统 MUST 通过清晰的依赖注入边界装配 memory、context 和 prompt 等关键组件，而不是在服务层强绑定唯一具体实现。

#### Scenario: 服务层使用可替换组件
- **WHEN** 系统创建运行时 `Services` 或等效依赖集合
- **THEN** memory、context 和 prompt 相关组件 MUST 可通过接口或等效抽象进行替换
- **THEN** 系统 MUST 保留默认实现作为开箱即用路径
- **THEN** 系统 MUST 不要求调用方重写现有公共构造入口

### Requirement: 事件序号生成需要避免全量解析事件文件
系统 MUST 为追加事件时的序号生成提供与事件总数线性可扩展的实现，而不得在每次追加前重复解析全部历史事件对象。

#### Scenario: 生成下一条事件序号
- **WHEN** 系统准备向某个 `Run` 的 `events.jsonl` 追加新事件
- **THEN** 系统 MUST 能在不反序列化全部历史事件对象的前提下得到下一条序号
- **THEN** 该实现 MUST 与现有追加式 JSONL 事件存储兼容

### Requirement: 运行时配置支持文件配置与环境变量覆盖
系统 MUST 支持从本地配置文件加载运行时配置，并允许环境变量与显式参数在其之上覆盖。

#### Scenario: 工作区存在本地配置文件
- **WHEN** 系统启动并在工作区检测到项目级配置文件
- **THEN** 系统 MUST 读取该配置文件中的运行时配置
- **THEN** 系统 MUST 将其作为默认配置来源之一

#### Scenario: 同时存在用户级配置与环境变量
- **WHEN** 系统检测到用户级配置文件且环境变量也提供了同名配置项
- **THEN** 系统 MUST 允许环境变量覆盖配置文件值
- **THEN** 系统 MUST 保持显式 flag 或等效调用参数的优先级高于环境变量

### Requirement: Web 服务入口需要与 Cobra 风格保持一致
系统 MUST 让本地 Web 服务入口与 CLI 入口保持一致的参数装配风格，以降低入口层维护差异。

#### Scenario: 启动本地 Web 服务
- **WHEN** 用户启动本地 Web 服务入口
- **THEN** 该入口 MUST 通过 Cobra 风格的命令装配解析参数
- **THEN** 该入口 MUST 继续复用现有应用层与 HTTP 适配层
- **THEN** 系统 MUST 不因为入口风格调整而改变现有服务能力

### Requirement: 运行时需要提供可分类的基础错误类型
系统 MUST 为常见的运行时基础错误提供集中定义且可识别的错误类型，以支持一致的错误处理和测试判断。

#### Scenario: 返回不支持的 provider 或缺失对象错误
- **WHEN** 系统遇到不支持的模型 provider、缺失的 run 或缺失的 session
- **THEN** 系统 MUST 返回集中定义的基础错误类型或基于其包装的错误
- **THEN** 系统 MUST 允许调用方通过标准错误判断识别这些错误类别
