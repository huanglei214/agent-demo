## MODIFIED Requirements

### Requirement: 读取文件内容
系统 MUST 提供 `fs.read_file` 工具，用于读取工作区内指定文件的内容。

#### Scenario: 读取文件内容时允许结果截断
- **WHEN** Agent 调用 `fs.read_file` 且目标文件内容超过系统定义的安全上限
- **THEN** 系统 MUST 返回被截断的内容
- **THEN** 系统 MUST 明确标记该结果已截断

### Requirement: 写入文件内容
系统 MUST 提供 `fs.write_file` 工具，用于在工作区范围内写入文件内容。

#### Scenario: 明确区分整体写入
- **WHEN** Agent 调用 `fs.write_file`
- **THEN** 系统 MUST 将其解释为创建新文件或整体覆盖文件内容
- **THEN** 系统 MUST 不将其作为局部替换工具使用

### Requirement: 搜索工作区内容
系统 MUST 提供 `fs.search` 工具，用于在工作区范围内按名称或文本模式搜索文件。

#### Scenario: 兼容搜索别名
- **WHEN** Agent 调用 `fs.search` 并提供 `pattern`
- **THEN** 系统 MUST 使用该值作为搜索模式
- **THEN** 系统 MAY 为兼容模型行为接受 `query` 作为等价别名

#### Scenario: 拒绝空查询
- **WHEN** Agent 调用 `fs.search` 但既未提供有效 `pattern`，也未提供有效 `query`
- **THEN** 系统 MUST 拒绝执行该搜索
- **THEN** 系统 MUST 返回结构化错误结果

#### Scenario: 搜索结果截断
- **WHEN** `fs.search` 的匹配结果超过系统定义的安全上限
- **THEN** 系统 MUST 截断返回的匹配项
- **THEN** 系统 MUST 明确标记该结果已截断

### Requirement: 局部替换文件内容
系统 MUST 提供 `fs.str_replace` 工具，用于在工作区范围内对文件内容进行局部文本替换。

#### Scenario: 成功替换指定文本
- **WHEN** Agent 调用 `fs.str_replace` 并提供存在于目标文件中的 `old_str`
- **THEN** 系统 MUST 只替换指定的目标文本
- **THEN** 系统 MUST 返回替换次数

#### Scenario: 找不到待替换文本
- **WHEN** Agent 调用 `fs.str_replace` 但目标文件中不存在 `old_str`
- **THEN** 系统 MUST 拒绝静默成功
- **THEN** 系统 MUST 返回结构化错误结果

#### Scenario: 工作区外替换被拒绝
- **WHEN** Agent 调用 `fs.str_replace` 且目标路径超出工作区范围
- **THEN** 系统 MUST 拒绝此次替换
- **THEN** 系统 MUST 返回结构化错误结果

## REMOVED Requirements

### Requirement: 查询文件信息
系统 MUST 提供 `fs.stat` 工具，用于返回工作区内文件或目录的元信息。

#### Scenario: 获取文件元信息
- **WHEN** Agent 调用 `fs.stat` 并提供工作区内有效路径
- **THEN** 系统 MUST 返回该路径的类型、大小或修改时间等元信息
