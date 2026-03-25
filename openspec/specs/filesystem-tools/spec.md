# filesystem-tools

## Purpose
定义 Harness 平台统一工具运行时中的文件系统工具能力，包括读取、写入、列目录、搜索、查询元信息和工作区边界控制。

## Requirements

### Requirement: 统一的工具注册与执行接口
系统 MUST 提供统一的工具注册与执行机制，使内置工具和未来外部工具可以通过一致的调用模型运行。

#### Scenario: 注册文件系统工具
- **WHEN** 系统启动工具运行时
- **THEN** 系统 MUST 注册文件系统相关工具
- **THEN** 每个工具 MUST 拥有唯一名称和结构化输入输出定义

#### Scenario: 暴露工具访问级别
- **WHEN** 系统列出已注册的工具描述信息
- **THEN** 每个工具描述 MUST 包含唯一名称
- **THEN** 每个工具描述 MUST 包含用户可读的说明
- **THEN** 每个工具描述 MUST 包含结构化访问级别，例如 `read_only` 或 `write`

### Requirement: 读取文件内容
系统 MUST 提供 `fs.read_file` 工具，用于读取工作区内指定文件的内容。

#### Scenario: 读取工作区内文件
- **WHEN** Agent 调用 `fs.read_file` 并提供工作区内有效文件路径
- **THEN** 系统 MUST 返回该文件的内容
- **THEN** 系统 MUST 记录对应的工具调用与成功事件

#### Scenario: 读取不存在的文件
- **WHEN** Agent 调用 `fs.read_file` 且目标文件不存在
- **THEN** 系统 MUST 返回结构化错误结果
- **THEN** 系统 MUST 记录 `tool.failed` 事件

### Requirement: 写入文件内容
系统 MUST 提供 `fs.write_file` 工具，用于在工作区范围内写入文件内容。

#### Scenario: 写入工作区内文件
- **WHEN** Agent 调用 `fs.write_file` 并提供工作区内目标路径及内容
- **THEN** 系统 MUST 将内容写入目标文件
- **THEN** 系统 MUST 记录对应的工具调用与成功事件

#### Scenario: 区分新建与更新事件
- **WHEN** Agent 调用 `fs.write_file` 成功写入目标路径
- **THEN** 系统 MUST 在目标文件原本不存在时记录 `fs.file_created` 事件
- **THEN** 系统 MUST 在目标文件原本已存在且被覆盖时记录 `fs.file_updated` 事件

#### Scenario: 写入工作区外路径
- **WHEN** Agent 调用 `fs.write_file` 且目标路径超出工作区范围
- **THEN** 系统 MUST 拒绝此次写入
- **THEN** 系统 MUST 返回结构化错误结果

#### Scenario: 未显式允许覆盖已有文件
- **WHEN** Agent 调用 `fs.write_file` 写入一个已存在的文件且未显式指定允许覆盖
- **THEN** 系统 MUST 拒绝此次写入
- **THEN** 系统 MUST 返回结构化错误结果

### Requirement: 列出目录内容
系统 MUST 提供 `fs.list_dir` 工具，用于列出工作区内目录的直接内容。

#### Scenario: 列出目录
- **WHEN** Agent 调用 `fs.list_dir` 并提供工作区内有效目录路径
- **THEN** 系统 MUST 返回该目录下的条目列表

### Requirement: 搜索工作区内容
系统 MUST 提供 `fs.search` 工具，用于在工作区范围内按名称或文本模式搜索文件。

#### Scenario: 搜索匹配内容
- **WHEN** Agent 调用 `fs.search` 并提供搜索模式
- **THEN** 系统 MUST 返回匹配的文件或文本位置结果

### Requirement: 查询文件信息
系统 MUST 提供 `fs.stat` 工具，用于返回工作区内文件或目录的元信息。

#### Scenario: 获取文件元信息
- **WHEN** Agent 调用 `fs.stat` 并提供工作区内有效路径
- **THEN** 系统 MUST 返回该路径的类型、大小或修改时间等元信息

### Requirement: 文件系统工具边界控制
系统 MUST 将文件系统工具的可操作范围限制在工作区内，并阻止越界访问。

#### Scenario: 工具尝试访问工作区外路径
- **WHEN** 任一文件系统工具收到工作区外的路径参数
- **THEN** 系统 MUST 拒绝执行该操作
- **THEN** 系统 MUST 返回结构化错误信息
