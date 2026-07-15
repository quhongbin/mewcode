# 工具系统 Tasks

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 新建 | `internal/tools/tool.go` | Tool 接口、ToolMeta、ToolCall、ToolResult 类型 |
| 新建 | `internal/tools/registry.go` | Registry 注册中心、ToolDefinition |
| 新建 | `internal/tools/executor.go` | Executor 统一执行入口 |
| 新建 | `internal/tools/file/read_file.go` | ReadFile 工具 |
| 新建 | `internal/tools/file/write_file.go` | WriteFile 工具 |
| 新建 | `internal/tools/file/edit_file.go` | EditFile 工具 |
| 新建 | `internal/tools/shell/exec_command.go` | Bash 工具 |
| 新建 | `internal/tools/search/glob_search.go` | Glob 工具 |
| 新建 | `internal/tools/search/grep_search.go` | Grep 工具 |
| 新建 | `internal/tools/test/registry_test.go` | Registry 测试 |
| 新建 | `internal/tools/test/executor_test.go` | Executor 测试 |
| 新建 | `internal/tools/file/test/read_file_test.go` | ReadFile 测试 |
| 新建 | `internal/tools/file/test/write_file_test.go` | WriteFile 测试 |
| 新建 | `internal/tools/file/test/edit_file_test.go` | EditFile 测试 |
| 新建 | `internal/tools/shell/test/exec_command_test.go` | Bash 测试 |
| 新建 | `internal/tools/search/test/glob_search_test.go` | Glob 测试 |
| 新建 | `internal/tools/search/test/grep_search_test.go` | Grep 测试 |
| 修改 | `internal/provider/provider.go` | 扩展 Message、StreamChunk、Provider 接口 |
| 修改 | `internal/provider/anthropic.go` | 扩展 Anthropic 工具调用流式解析 |
| 修改 | `internal/provider/openai.go` | 扩展 OpenAI 工具调用流式解析 |
| 修改 | `internal/conversation/conversation.go` | 扩展工具消息支持 |
| 修改 | `internal/conversation/conversation_test.go` | 更新测试 |
| 新建 | `internal/agent/agent.go` | Agent 编排器、agentStream |
| 新建 | `internal/agent/agent_test.go` | Agent 测试 |
| 修改 | `internal/tui/model.go` | Agent 替代 Conversation、新增消息类型 |
| 修改 | `internal/tui/update.go` | 处理工具调用/结果消息 |
| 修改 | `internal/tui/view.go` | 渲染工具调用过程 |
| 修改 | `internal/tui/styles.go` | 工具相关样式 |
| 修改 | `cmd/mewcode/main.go` | 创建 Registry、注册工具、创建 Agent |

## T1: 工具接口与核心类型

**文件：** `internal/tools/tool.go`
**依赖：** 无
**步骤：**
1. 定义 `ToolMeta` 结构体，包含 `Category string`、`ReadOnly bool`、`Destructive bool`
2. 定义 `ToolCall` 结构体，包含 `ID string`、`Name string`、`Args json.RawMessage`
3. 定义 `ToolResult` 结构体，包含 `IsError bool`、`Content string`
4. 定义 `Tool` 接口，方法：`Name() string`、`Description() string`、`Parameters() json.RawMessage`、`Meta() ToolMeta`、`Execute(ctx context.Context, args json.RawMessage) (string, error)`

**验证：** `go build ./internal/tools/...` 编译通过

## T2: 注册中心

**文件：** `internal/tools/registry.go`
**依赖：** T1
**步骤：**
1. 定义 `ToolDefinition` 结构体，包含 `Name string`、`Description string`、`InputSchema json.RawMessage`
2. 定义 `Registry` 结构体，内部 `tools map[string]Tool`
3. 实现 `NewRegistry() *Registry` — 初始化空 map
4. 实现 `Register(t Tool)` — 以 `t.Name()` 为 key 存入 map
5. 实现 `Get(name string) (Tool, bool)` — 按名查找
6. 实现 `List() []Tool` — 返回所有已注册工具
7. 实现 `Definitions() []ToolDefinition` — 遍历 List 转为 ToolDefinition

**验证：** `go test ./internal/tools/test/...` registry 测试通过

## T3: 工具执行器

**文件：** `internal/tools/executor.go`
**依赖：** T1, T2
**步骤：**
1. 定义 `Executor` 结构体，包含 `registry *Registry`、`timeout time.Duration`
2. 实现 `NewExecutor(registry *Registry, timeout time.Duration) *Executor`
3. 实现 `Execute(ctx context.Context, name string, args json.RawMessage) ToolResult`：
   - `registry.Get(name)` 查找工具，未找到返回 `ToolResult{IsError: true, Content: "tool not found: " + name}`
   - `context.WithTimeout(ctx, timeout)` 创建超时 context
   - 调用 `tool.Execute(ctx, args)`
   - 成功：返回 `ToolResult{Content: output}`
   - 失败（含超时）：返回 `ToolResult{IsError: true, Content: err.Error()}`

**验证：** `go test ./internal/tools/test/...` executor 测试通过（含超时测试）

## T4: ReadFile 工具

**文件：** `internal/tools/file/read_file.go`
**依赖：** T1
**步骤：**
1. 定义 `ReadFileTool` 结构体，实现 Tool 接口
2. `Name()` 返回 `"read_file"`，`Meta()` 返回 `{Category: "file", ReadOnly: true, Destructive: false}`
3. `Parameters()` 返回 JSON Schema：`path`(string,required)、`offset`(int,optional)、`limit`(int,optional)
4. `Execute` 实现：
   - `os.Open(path)` 打开文件
   - 读取前 512 字节，检查是否包含 `\x00`（NUL），若有返回 `"该文件为二进制文件，请使用命令行工具处理"`
   - `io.ReadAll` 读取全部内容，按 `\n` 分割为行数组
   - 应用 offset（默认 1）和 limit（默认全部）切片行数组
   - 每行格式化为 `  42│行内容`（行号右对齐 + │ 分隔符）
   - 返回拼接后的字符串

**验证：** `go test ./internal/tools/file/test/...` ReadFile 测试通过（正常读取、offset/limit、二进制拒绝）

## T5: WriteFile 工具

**文件：** `internal/tools/file/write_file.go`
**依赖：** T1
**步骤：**
1. 定义 `WriteFileTool`，实现 Tool 接口
2. `Name()` 返回 `"write_file"`，`Meta()` 返回 `{Category: "file", ReadOnly: false, Destructive: false}`
3. `Parameters()` 返回 JSON Schema：`path`(string,required)、`content`(string,required)
4. `Execute` 实现：
   - `filepath.Dir(path)` 获取父目录
   - `os.MkdirAll(dir, 0755)` 递归创建父目录
   - `os.WriteFile(path, []byte(content), 0644)` 写入文件
   - 返回 `fmt.Sprintf("[成功写入 %d 字节到 %s]", len(content), path)`

**验证：** `go test ./internal/tools/file/test/...` WriteFile 测试通过（正常写入、深层目录创建）

## T6: EditFile 工具

**文件：** `internal/tools/file/edit_file.go`
**依赖：** T1
**步骤：**
1. 定义 `EditFileTool`，实现 Tool 接口
2. `Name()` 返回 `"edit_file"`，`Meta()` 返回 `{Category: "file", ReadOnly: false, Destructive: false}`
3. `Parameters()` 返回 JSON Schema：`path`(string,required)、`old_text`(string,required)、`new_text`(string,required)、`start_line`(int,optional)、`end_line`(int,optional)
4. `Execute` 实现：
   - `os.ReadFile(path)` 读取文件全部内容
   - 解析参数，判断模式：
   - **行号模式**（start_line 和 end_line 均提供）：
     - 按 `\n` 分割为行数组
     - 提取 start_line 到 end_line 范围的行作为 old_text
     - 用 new_text 替换这些行
     - 拼接回完整文件内容并写回
   - **文本匹配模式**（不提供行号）：
     - `strings.Count(content, old_text)` 检查匹配次数
     - =0：返回 `"未找到匹配：所提供的文本在文件中不存在"`
     - \>1：返回 `"找到 N 处匹配，请提供 start_line/end_line 或更多上下文以唯一定位"`
     - =1：`strings.Replace(content, old_text, new_text, 1)` 并写回
   - 写回文件：`os.WriteFile(path, newContent, originalPerm)`

**验证：** `go test ./internal/tools/file/test/...` EditFile 测试通过（行号模式替换、文本匹配替换、未找到、多次匹配）

## T7: Bash 工具

**文件：** `internal/tools/shell/exec_command.go`
**依赖：** T1
**步骤：**
1. 定义 `BashTool`，实现 Tool 接口
2. `Name()` 返回 `"bash"`，`Meta()` 返回 `{Category: "shell", ReadOnly: false, Destructive: true}`
3. `Parameters()` 返回 JSON Schema：`command`(string,required)
4. `Execute` 实现：
   - 判断 `runtime.GOOS == "windows"` → `exec.CommandContext(ctx, "cmd", "/c", command)`
   - 否则 → `exec.CommandContext(ctx, "sh", "-c", command)`
   - 捕获 stdout 和 stderr：`cmd.CombinedOutput()`
   - 成功：返回输出内容
   - 失败：返回 `fmt.Sprintf("exit code %d:\n%s", exitCode, output)`（非 error，是结构化结果）

**验证：** `go test ./internal/tools/shell/test/...` Bash 测试通过（正常命令、失败命令）

## T8: Glob 工具

**文件：** `internal/tools/search/glob_search.go`
**依赖：** T1
**步骤：**
1. 定义 `GlobTool`，实现 Tool 接口
2. `Name()` 返回 `"glob"`，`Meta()` 返回 `{Category: "search", ReadOnly: true, Destructive: false}`
3. `Parameters()` 返回 JSON Schema：`pattern`(string,required)
4. `Execute` 实现：
   - `filepath.Glob(pattern)` 查找匹配文件
   - 有结果：每行一个路径，拼接返回
   - 无结果：返回 `"未找到匹配文件"`

**验证：** `go test ./internal/tools/search/test/...` Glob 测试通过

## T9: Grep 工具

**文件：** `internal/tools/search/grep_search.go`
**依赖：** T1
**步骤：**
1. 定义 `GrepTool`，实现 Tool 接口
2. `Name()` 返回 `"grep"`，`Meta()` 返回 `{Category: "search", ReadOnly: true, Destructive: false}`
3. `Parameters()` 返回 JSON Schema：`pattern`(string,required)、`path`(string,optional,默认 ".")
4. `Execute` 实现：
   - `regexp.Compile(pattern)` 编译正则
   - `filepath.Walk(path, ...)` 遍历目录
   - 跳过隐藏目录和二进制文件
   - 逐行扫描，匹配时记录 `文件名:行号│行内容`
   - 附加匹配行前后各 2 行上下文
   - 无匹配时返回 `"未找到匹配内容"`

**验证：** `go test ./internal/tools/search/test/...` Grep 测试通过

## T10: Provider 层扩展

**文件：** `internal/provider/provider.go`、`internal/provider/anthropic.go`、`internal/provider/openai.go`
**依赖：** T1
**步骤：**
1. provider.go：Message 新增字段 `ToolCalls []ToolCall \`json:"tool_calls,omitempty"\``、`ToolCallID string \`json:"tool_call_id,omitempty"\``
2. provider.go：新增 `ToolCall` 结构体（ID, Name, Args json.RawMessage）
3. provider.go：新增常量 `StreamChunkTypeToolCall StreamChunkType = "tool_call"`
4. provider.go：StreamChunk 新增字段 `ToolCall *ToolCall \`json:"tool_call,omitempty"\``
5. provider.go：新增 `ToolDefinition` 结构体（Name, Description, InputSchema）
6. provider.go：`Provider.StreamChat` 签名改为 `StreamChat(ctx, messages, tools []ToolDefinition, thinking bool) (Stream, error)`
7. anthropic.go：
   - StreamChat 请求体增加 `"tools"` 字段（将 ToolDefinition 转为 Anthropic 格式）
   - 流式解析新增：`content_block_start`(type=tool_use) 记录 tool ID 和 name
   - 流式解析新增：`content_block_delta`(type=input_json_delta) 拼接 JSON 参数碎片
   - 流式解析新增：`content_block_stop` 时若当前是 tool_use 块，产出 `StreamChunkTypeToolCall`
8. openai.go：
   - StreamChat 请求体增加 `"tools"` 字段（将 ToolDefinition 转为 OpenAI 格式）
   - 流式解析新增：处理 `choices[0].delta.tool_calls`，按 index 拼接 name 和 arguments 碎片
   - 流结束时产出完整的 `StreamChunkTypeToolCall`

**验证：** `go build ./internal/provider/...` 编译通过；`go test ./internal/conversation/...` 现有测试通过

## T11: Conversation 层扩展

**文件：** `internal/conversation/conversation.go`
**依赖：** T10
**步骤：**
1. `SendMessage` 签名改为 `SendMessage(ctx context.Context, userText string, tools []provider.ToolDefinition) (provider.Stream, error)`
2. 内部调用 `provider.StreamChat(ctx, messages, tools, thinking)` 传入 tools
3. 新增 `AddToolCallMessage(toolCalls []provider.ToolCall)` — 添加 role="assistant" 且带 ToolCalls 的消息
4. 新增 `AddToolResultMessage(toolCallID string, content string, isError bool)` — 添加 role="tool" 且带 ToolCallID 和 Content 的消息

**验证：** `go test ./internal/conversation/...` 测试通过

## T12: Agent 编排层

**文件：** `internal/agent/agent.go`
**依赖：** T3, T10, T11
**步骤：**
1. 定义 `Agent` 结构体：conversation, registry, executor, provider, thinking
2. 实现 `NewAgent(conv, registry, executor, provider, thinking) *Agent`
3. 定义 `agentStream` 结构体：channel chan *provider.StreamChunk, done chan struct{}, body io.Closer
4. 实现 `agentStream.Next()` 从 channel 读取，`agentStream.Close()` 关闭
5. 实现 `Agent.SendMessage(ctx, userText) (provider.Stream, error)`：
   - `conv.SendMessage(ctx, userText, registry.Definitions())` 获取第一轮 provider Stream
   - 创建 agentStream，启动后台 goroutine
   - goroutine 逻辑：
     a. 循环消费第一轮 stream.Next()
     b. Text/Thinking 块 → 发送到 channel
     c. ToolCall 块 → 记录 toolCall 变量 → 发送到 channel
     d. Done → 若无 toolCall → 关闭 channel 结束
     e. 若有 toolCall：
        - `executor.Execute(ctx, toolCall.Name, toolCall.Args)` → result
        - 发送 ToolResult chunk（可复用 ToolCall 类型或新增）
        - `conv.AddToolCallMessage([]ToolCall{*toolCall})`
        - `conv.AddToolResultMessage(toolCall.ID, result.Content, result.IsError)`
        - `provider.StreamChat(ctx, conv.GetMessages(), registry.Definitions(), thinking)` 获取第二轮 stream
        - 循环消费第二轮，透传 Text/Thinking/Done 到 channel
        - 关闭 channel

**验证：** `go test ./internal/agent/...` Agent 测试通过（mock provider，无工具调用场景 + 有工具调用场景）

## T13: TUI 层改造

**文件：** `internal/tui/model.go`、`internal/tui/update.go`、`internal/tui/view.go`、`internal/tui/styles.go`
**依赖：** T12
**步骤：**
1. model.go：Model 的 `conversation` 字段替换为 `agent *agent.Agent`
2. model.go：`NewModel` 参数改为接收 `*agent.Agent`
3. model.go：DisplayMessage 新增 `ToolCall *provider.ToolCall`、`ToolResult *tools.ToolResult`
4. model.go：新增 `toolCallMsg struct { call *provider.ToolCall }`、`toolResultMsg struct { result *tools.ToolResult }`
5. update.go：`startStream` 改为调用 `m.agent.SendMessage(ctx, userInput)`
6. update.go：`streamChunkMsg` 处理新增 `StreamChunkTypeToolCall` → 发送 `toolCallMsg`
7. update.go：新增 `case toolCallMsg` → 追加 DisplayMessage（带 ToolCall）
8. update.go：新增 `case toolResultMsg` → 更新最后一条消息的 ToolResult 或追加新消息
9. update.go：`streamDoneMsg` 处理 → `conv.AddAssistantMessage` 保留兼容
10. view.go：新增渲染逻辑 — ToolCall 显示为 `🔧 工具: name(args摘要)`，ToolResult 显示为 `📋 结果: 内容摘要（截断到 200 字符）`
11. styles.go：新增 `toolCallStyle`（蓝色/青色）、`toolResultStyle`（灰色）

**验证：** `go build ./internal/tui/...` 编译通过

## T14: main.go 接线 + 集成验证

**文件：** `cmd/mewcode/main.go`
**依赖：** T13
**步骤：**
1. 创建 Registry：`reg := tools.NewRegistry()`
2. 注册六个工具：`reg.Register(file.NewReadFileTool())`、`reg.Register(file.NewWriteFileTool())`、`reg.Register(file.NewEditFileTool())`、`reg.Register(shell.NewBashTool())`、`reg.Register(search.NewGlobTool())`、`reg.Register(search.NewGrepTool())`
3. 创建 Executor：`exec := tools.NewExecutor(reg, 30*time.Second)`
4. 创建 Conversation（保持原有方式）
5. 创建 Agent：`ag := agent.NewAgent(conv, reg, exec, p, thinking)`
6. `tui.NewModel` 改为传入 `ag`
7. 编译验证：`go build ./cmd/mewcode/...`
8. 全部测试：`go test ./...`

**验证：** `go build ./...` 编译通过；`go test ./...` 全部通过

## 执行顺序

```
T1
├──→ T2 → T3
├──→ T4（可并行）
├──→ T5（可并行）
├──→ T6（可并行）
├──→ T7（可并行）
├──→ T8（可并行）
├──→ T9（可并行）
└──→ T10 → T11
              ↘
T3, T10, T11 → T12 → T13 → T14
```
