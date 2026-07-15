# 工具系统 Plan

## 架构概览

系统分为五层，从上到下依次是：

**Agent 编排层**（新增）— 位于 Conversation 之上，负责工具调用的编排调度。接收用户输入，驱动"请求模型 → 检测工具调用 → 执行工具 → 回灌结果 → 再次请求模型"的流程。当前实现单次工具调用，后续做 Agent Loop 时只需在此层加循环。

**Tool 层**（新增）— 定义统一的工具接口和元数据，提供注册中心集中管理，实现六大核心工具。与 Agent 层通过接口交互，Agent 通过 Registry 查找和执行工具。

**Provider 层**（改造）— 扩展消息类型以支持工具调用和工具结果，扩展流式数据块类型以支持工具调用事件，StreamChat 接受工具列表参数。Anthropic 和 OpenAI 各自实现其协议的工具调用流式解析。

**Conversation 层**（改造）— 消息类型升级以支持 tool_use / tool_result 等角色，保持消息历史管理的核心职责不变。

**TUI 层**（改造）— 新增工具调用相关的消息类型和渲染逻辑，展示工具执行过程和结果。Agent 替代 Conversation 成为 TUI 的直接依赖。

数据流方向：
```
用户输入 → TUI → Agent → Conversation → Provider → LLM API
                              ↓
                         Tool Registry → 工具执行
                              ↓
              Agent ← 工具结果 ← 执行完成
                ↓
         Conversation → Provider → LLM API（第二轮，带工具结果）
                ↓
         TUI ← 最终文本回复
```

## 核心数据结构

### Tool 接口与元数据

```go
// ToolMeta 工具元数据
type ToolMeta struct {
    Category    string // file, shell, search
    ReadOnly    bool
    Destructive bool
}

// Tool 统一工具接口
type Tool interface {
    Name() string
    Description() string
    Parameters() json.RawMessage // JSON Schema
    Meta() ToolMeta
    Execute(ctx context.Context, args json.RawMessage) (string, error)
}
```

### 工具调用与结果

```go
// ToolCall 表示模型发起的工具调用
type ToolCall struct {
    ID   string
    Name string
    Args json.RawMessage
}

// ToolResult 工具执行结果
type ToolResult struct {
    IsError bool
    Content string
}
```

### 扩展的消息类型

```go
// Message 对话消息（扩展现有类型）
type Message struct {
    Role       string     // "user", "assistant", "system", "tool"
    Content    string
    ToolCalls  []ToolCall // assistant 消息中的工具调用列表
    ToolCallID string     // tool 消息对应的工具调用 ID
}
```

向后兼容：无工具调用时 ToolCalls 为 nil，序列化时省略；ToolCallID 仅 tool 角色使用。

### 扩展的流式数据块

```go
const (
    StreamChunkTypeText     StreamChunkType = "text"
    StreamChunkTypeThinking StreamChunkType = "thinking"
    StreamChunkTypeToolCall StreamChunkType = "tool_call" // 新增
    StreamChunkTypeDone     StreamChunkType = "done"
)

type StreamChunk struct {
    Type     StreamChunkType
    Content  string
    ToolCall *ToolCall // tool_call 使用
}
```

### Provider 接口变更

```go
type Provider interface {
    StreamChat(ctx context.Context, messages []Message, tools []ToolDefinition, thinking bool) (Stream, error)
}

type ToolDefinition struct {
    Name        string
    Description string
    InputSchema json.RawMessage
}
```

### Registry 注册中心

```go
type Registry struct {
    tools map[string]Tool
}

func NewRegistry() *Registry
func (r *Registry) Register(t Tool)
func (r *Registry) Get(name string) (Tool, bool)
func (r *Registry) List() []Tool
func (r *Registry) Definitions() []ToolDefinition
```

### Agent 编排器

```go
type Agent struct {
    conversation *conversation.Conversation
    registry     *tools.Registry
    provider     provider.Provider
    thinking     bool
}

func NewAgent(conv, registry, provider, thinking) *Agent
func (a *Agent) SendMessage(ctx context.Context, userText string) (provider.Stream, error)
```

SendMessage 返回的 Stream 中：若模型直接回复文本只产出 Text/Done 块；若模型调用工具先产出 ToolCall 块，然后执行工具，再请求第二轮产出 Text/Done 块。

## 模块设计

### 模块 A：工具接口与注册中心（`internal/tools/`）

**职责：** 定义 Tool 接口、ToolMeta 元数据、Registry 注册中心、ToolDefinition API 格式转换。

**对外接口：** Tool 接口、Registry.NewRegistry()、Registry.Register()、Registry.Get()、Registry.Definitions()

**依赖：** 无外部依赖，仅使用标准库

### 模块 B：六大核心工具

| 工具 | 文件 | 分类 | 只读 | 破坏性 |
|------|------|------|------|--------|
| ReadFile | `file/read_file.go` | file | 是 | 否 |
| WriteFile | `file/write_file.go` | file | 否 | 否 |
| EditFile | `file/edit_file.go` | file | 否 | 否 |
| Bash | `shell/exec_command.go` | shell | 否 | 是 |
| Glob | `search/glob_search.go` | search | 是 | 否 |
| Grep | `search/grep_search.go` | search | 是 | 否 |

**关键实现细节：**
- 读文件：前 512 字节检测 NUL → 二进制拒绝；全文读取后按行号加前缀；offset/limit 切片行数组
- 写文件：`os.MkdirAll(dir, 0755)` + `os.WriteFile(path, data, 0644)`
- 编辑文件：`strings.Count` 检查匹配次数，=0 报错，>1 报错，=1 执行 `strings.Replace`
- 执行命令：`exec.CommandContext(ctx, shell, flag, command)` — Windows `cmd /c`，Unix `sh -c`
- 查找文件：`filepath.Glob(pattern)`
- 搜索代码：`regexp.Compile` + 逐文件逐行扫描，匹配行及前后各 2 行上下文

### 模块 C：工具执行器（`internal/tools/executor.go`）

**职责：** 封装"按名查找 + 超时执行 + 错误包装"的统一执行入口。

```go
type Executor struct {
    registry *Registry
    timeout  time.Duration // 默认 30 秒
}

func NewExecutor(registry, timeout) *Executor
func (e *Executor) Execute(ctx context.Context, name string, args json.RawMessage) ToolResult
```

### 模块 D：Provider 层改造

**改造点：**
- Message 新增 ToolCalls 和 ToolCallID 字段
- StreamChunk 新增 ToolCall 字段和 StreamChunkTypeToolCall 常量
- Provider.StreamChat 签名增加 tools 参数
- Anthropic：解析 content_block_start(tool_use)、content_block_delta(input_json_delta)
- OpenAI：解析 choices[0].delta.tool_calls

### 模块 E：Conversation 层改造

**改造点：**
- SendMessage 内部传入工具列表
- 新增 AddToolCallMessage(toolCalls)
- 新增 AddToolResultMessage(toolCallID, content, isError)

### 模块 F：Agent 编排层

**核心流程：**
1. 将 userText 添加到对话历史
2. 调用 StreamChat 获取第一轮流
3. 创建 agentStream 包装
4. 后台 goroutine：消费第一轮流 → 检测工具调用 → 执行工具 → 回灌历史 → 第二轮流 → 透传

### 模块 G：TUI 层改造

**改造点：**
- Model 中 conversation 改为 agent
- DisplayMessage 新增 ToolCall 和 ToolResult 字段
- 新增 toolCallMsg 和 toolResultMsg 消息类型
- startStream 改为调用 agent.SendMessage
- View 新增工具调用过程渲染

## 模块交互

**场景 1：无工具调用**
```
用户输入 → TUI → Agent.SendMessage → Provider.StreamChat → Text/Done → TUI 渲染
```

**场景 2：模型调用工具**
```
用户输入 → TUI → Agent.SendMessage
  → Provider.StreamChat（第一轮）→ ToolCall + Done
  → Executor.Execute → ToolResult
  → Provider.StreamChat（第二轮，带工具结果）→ Text + Done
  → TUI 渲染工具过程 + 最终回复
```

## 文件组织

```
project/
├── internal/
│   ├── tools/
│   │   ├── tool.go              — Tool 接口、ToolMeta、ToolResult、ToolCall
│   │   ├── registry.go          — Registry、ToolDefinition
│   │   ├── executor.go          — Executor
│   │   ├── file/
│   │   │   ├── read_file.go     — ReadFile
│   │   │   ├── write_file.go    — WriteFile
│   │   │   ├── edit_file.go     — EditFile
│   │   │   └── test/
│   │   │       ├── read_file_test.go
│   │   │       ├── write_file_test.go
│   │   │       └── edit_file_test.go
│   │   ├── shell/
│   │   │   ├── exec_command.go  — Bash
│   │   │   └── test/
│   │   │       └── exec_command_test.go
│   │   ├── search/
│   │   │   ├── glob_search.go   — Glob
│   │   │   ├── grep_search.go   — Grep
│   │   │   └── test/
│   │   │       ├── glob_search_test.go
│   │   │       └── grep_search_test.go
│   │   └── test/
│   │       ├── registry_test.go
│   │       └── executor_test.go
│   ├── agent/
│   │   ├── agent.go             — Agent 编排器
│   │   └── agent_test.go
│   ├── provider/
│   │   ├── provider.go          — 扩展 Message、StreamChunk、Provider 接口
│   │   ├── anthropic.go         — 扩展工具调用流式解析
│   │   └── openai.go            — 扩展工具调用流式解析
│   ├── conversation/
│   │   ├── conversation.go      — 扩展工具消息支持
│   │   └── conversation_test.go
│   ├── tui/
│   │   ├── model.go             — Agent 替代 Conversation
│   │   ├── update.go            — 处理工具消息
│   │   ├── view.go              — 渲染工具过程
│   │   └── styles.go            — 工具样式
│   └── config/
│       └── config.go            — 无变更
└── cmd/
    └── mewcode/
        └── main.go              — 创建 Registry、注册工具、创建 Agent
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| Agent 编排方式 | 独立 internal/agent/ 包 | 与 Conversation 职责分离，后续 Agent Loop 只改此层 |
| 工具调用流式解析 | 扩展 StreamChunk 新增 ToolCall 类型 | 复用现有 Stream 接口，Agent 和 TUI 通过同一接口消费 |
| 工具参数格式 | JSON Schema | Anthropic 和 OpenAI API 均原生支持 |
| Agent Stream 实现 | agentStream 包装 Provider Stream | 两轮调用对外表现为一个连续流 |
| 编辑文件匹配 | strings.Count + strings.Replace | 标准库足够，无需引入 diff 库 |
| 命令执行 Shell | Windows: cmd /c，Unix: sh -c | 与用户当前 shell 环境一致 |
| 工具超时 | 默认 30 秒，context.WithTimeout | 与 Go context 体系天然集成 |
| Message 序列化 | omitempty 处理工具字段 | 向后兼容，无工具时 JSON 与现有一致 |
