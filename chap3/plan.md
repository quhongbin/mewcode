# Agent Loop 自主循环 Plan

## 架构概览

改造后的 Agent 层从"硬编码两轮编排"升级为"ReAct 事件驱动循环"。核心变化：

1. **Agent** 不再返回 `provider.Stream`，改为返回 `EventStream`——一条基于 channel 的结构化事件流。Agent 内部的 goroutine 驱动 ReAct 循环，每轮调用 LLM、收集事件、执行工具、回灌历史，直到满足停止条件。

2. **Event 体系** 独立于 `provider.StreamChunk`，新增 tool_result、iteration、done（含结束原因）等类型，让 TUI 层只关心事件，不关心 Agent 内部逻辑。

3. **StreamCollector** 在 Agent 内部使用，消费 LLM 的 `provider.Stream`，同时完成两件事：实时转发 text/thinking/tool_call 事件到 EventStream，以及在内部累积完整响应供回灌对话历史。不单独建文件，作为 runLoop 内的内联逻辑。

4. **ToolBatcher** 将一批工具调用按 ReadOnly 属性分组，按并行上限拆分子组，供 Agent 调度执行。

5. **AgentMode** 维护 Plan/Act 模式，决定每轮传给 LLM 的工具集。

## 核心数据结构

### Event / EventType / DoneReason（event.go）

```go
type EventType string

const (
    EventText       EventType = "text"
    EventThinking   EventType = "thinking"
    EventToolCall   EventType = "tool_call"
    EventToolResult EventType = "tool_result"
    EventIteration  EventType = "iteration"
    EventDone       EventType = "done"
)

type DoneReason string

const (
    DoneCompleted   DoneReason = "completed"
    DoneMaxIter     DoneReason = "max_iteration"
    DoneCancelled   DoneReason = "cancelled"
    DoneUnknownTool DoneReason = "unknown_tool"
    DoneError       DoneReason = "error"
)

type Event struct {
    Type       EventType
    Content    string              // text/thinking 使用
    ToolCall   *provider.ToolCall  // tool_call 使用
    ToolResult *ToolResultEvent    // tool_result 使用
    Iteration  int                 // iteration 使用
    Reason     DoneReason          // done 使用
}

type ToolResultEvent struct {
    Name   string // 工具名称，用于 UI 显示
    Result tools.ToolResult
}
```

### EventStream（event.go）

```go
type EventStream struct {
    ch chan Event
}

// Next 阻塞获取下一个事件，channel 关闭时返回 (Event{}, false)
func (s *EventStream) Next() (Event, bool)
```

### Agent（agent.go）

```go
type AgentMode int

const (
    ModeAct  AgentMode = iota // 全工具模式
    ModePlan                  // 只读工具模式
)

type Agent struct {
    conv        *conversation.Conversation
    registry    *tools.Registry
    executor    *tools.Executor
    provider    provider.Provider
    thinking    bool
    maxIter     int       // 迭代上限，默认 25
    maxParallel int       // 并行上限，默认 5
    mode        AgentMode
}

func NewAgent(conv, registry, executor, p, thinking, maxIter, maxParallel) *Agent
func (a *Agent) SendMessage(ctx context.Context, userText string) *EventStream
func (a *Agent) SetMode(mode AgentMode)
func (a *Agent) Mode() AgentMode
```

### toolGroup / batchTools（batcher.go）

```go
type toolGroup struct {
    calls    []provider.ToolCall
    readOnly bool
}

// batchTools 将工具调用列表按 ReadOnly 属性分组，并按 maxParallel 拆分只读子组
func batchTools(calls []provider.ToolCall, registry *tools.Registry, maxParallel int) []toolGroup
```

## 模块设计

### Agent（agent.go）

**职责：** ReAct 主循环编排，驱动 LLM 调用 → 事件收集 → 工具执行 → 历史回灌的循环

**对外接口：**
- `NewAgent(conv, registry, executor, p, thinking, maxIter, maxParallel) *Agent`
- `SendMessage(ctx, userText) *EventStream`
- `SetMode(mode AgentMode)` / `Mode() AgentMode`

**关键内部方法：**
- `runLoop(ctx, eventCh)` — 主循环 goroutine
- `collectStream(ctx, stream, eventCh) (text, thinking string, toolCalls []provider.ToolCall, err error)` — 流式双路收集
- `executeGroups(ctx, groups []toolGroup, eventCh)` — 按组执行工具
- `providerToolDefs() []provider.ToolDefinition` — 根据当前 mode 返回工具集

**依赖：** Conversation、Registry、Executor、Provider、batchTools

**主循环伪代码：**
```
func runLoop(ctx, eventCh):
    defer close(eventCh)
    // 添加用户消息到历史
    conv.AddUserMessage(userText)

    for iteration := 1; iteration <= maxIter; iteration++:
        if ctx cancelled: emit done(cancelled); return
        emit iteration(iteration)

        // 根据 mode 选择工具集
        toolDefs = providerToolDefs()
        stream = provider.StreamChat(ctx, messages, toolDefs, thinking)

        // 流式双路收集
        text, thinking, toolCalls, err = collectStream(ctx, stream, eventCh)
        if err: emit done(error); return

        // 无工具调用 → 完成
        if len(toolCalls) == 0:
            conv.AddAssistantMessage(text)
            emit done(completed); return

        // 检查未知工具
        for _, tc := range toolCalls:
            if !registry.Has(tc.Name): emit done(unknown_tool); return

        // 回灌 assistant 消息（含文本和工具调用）
        conv.AddAssistantMessageWithTools(text, toolCalls)

        // 分组执行工具
        groups = batchTools(toolCalls, registry, maxParallel)
        executeGroups(ctx, groups, eventCh)
        // 工具结果已逐个回灌到 conv

    // 达到迭代上限
    emit done(max_iteration)
```

### Event（event.go）

**职责：** 定义 Event、EventType、DoneReason、EventStream、ToolResultEvent 类型

**对外接口：** `EventStream.Next() (Event, bool)`

**依赖：** provider.ToolCall、tools.ToolResult

### ToolBatcher（batcher.go）

**职责：** 将工具调用列表按 ReadOnly 属性和并行上限分组

**对外接口：** `batchTools(calls []provider.ToolCall, registry *tools.Registry, maxParallel int) []toolGroup`

**分组逻辑：**
1. 遍历 calls，相邻的 ReadOnly 工具合并为一组，非 ReadOnly 工具各自单独成组
2. 对 ReadOnly 组，如果长度超过 maxParallel，按 maxParallel 拆分为多个子组
3. 返回有序的 toolGroup 列表

**依赖：** Registry（查询工具 Meta）

### Conversation（conversation.go，改造）

**职责：** 新增方法支持同时记录助手文本和工具调用

**新增接口：**
- `AddUserMessage(content string)` — 从 Agent 中拆出，显式添加用户消息
- `AddAssistantMessageWithTools(content string, toolCalls []provider.ToolCall)` — 记录含文本和工具调用的 assistant 消息

**依赖：** provider.Message

### TUI（model.go / update.go，改造）

**职责：** 消费 EventStream 事件渲染界面，替代直接消费 provider.Stream；处理 `/do` 命令

**改造要点：**
- 删除 streamChunkMsg、streamDoneMsg 等旧消息类型
- 新增 agentEventMsg 包装 agent.Event
- startStream 改为消费 EventStream，每个 Event 转为 agentEventMsg 发给 Bubble Tea
- Update 中处理各事件类型：text 追加到 currentContent、tool_call/tool_result 追加到 messages、iteration 更新进度、done 结束 streaming
- 识别 `/do` 输入，调用 agent.SetMode(ModeAct)

### Registry（registry.go，改造）

**职责：** 新增 Has 方法

**新增接口：** `Has(name string) bool` — 检查工具是否已注册

### main.go（改造）

**职责：** 传入新增的 maxIter 和 maxParallel 参数

## 模块交互

```
TUI.startStream(userText)
  │
  ├─→ Agent.SendMessage(ctx, userText) → *EventStream
  │     │
  │     ├─→ goroutine: runLoop(ctx, eventCh)
  │     │     │
  │     │     ├─ conv.AddUserMessage(userText)
  │     │     │
  │     │     ├─ LOOP: for iteration := 1; iteration <= maxIter; iteration++
  │     │     │     ├─ emit EventIteration(iteration)
  │     │     │     ├─ providerToolDefs() → 根据 mode 返回工具集
  │     │     │     ├─ provider.StreamChat(ctx, messages, toolDefs, thinking)
  │     │     │     ├─ collectStream: 消费 Stream，同时：
  │     │     │     │     ├─ text chunk → emit EventText → eventCh
  │     │     │     │     ├─ thinking chunk → emit EventThinking → eventCh
  │     │     │     │     ├─ tool_call chunk → emit EventToolCall → eventCh
  │     │     │     │     └─ 内部累积完整 text + toolCalls 列表
  │     │     │     │
  │     │     │     ├─ if 无工具调用:
  │     │     │     │     ├─ conv.AddAssistantMessage(text)
  │     │     │     │     └─ emit EventDone(completed) → return
  │     │     │     │
  │     │     │     ├─ 检查未知工具 → 若有 emit EventDone(unknown_tool) → return
  │     │     │     │
  │     │     │     ├─ conv.AddAssistantMessageWithTools(text, toolCalls)
  │     │     │     ├─ batchTools(toolCalls, registry, maxParallel)
  │     │     │     ├─ 按组执行工具:
  │     │     │     │     ├─ readOnly 组: goroutine 并发 + WaitGroup
  │     │     │     │     └─ 副作用组: 串行
  │     │     │     │     └─ 每个完成 → conv.AddToolResultMessage + emit EventToolResult
  │     │     │     │
  │     │     │     └─ 继续下一轮循环
  │     │     │
  │     │     └─ emit EventDone(max_iteration) [如果跳出循环]
  │     │
  │     └─ close(eventCh)
  │
  └─→ TUI 消费 EventStream.Next() 渲染界面
```

## 文件组织

```
internal/
├── agent/
│   ├── agent.go      — Agent 结构体、NewAgent、SendMessage、runLoop、collectStream、executeGroups、providerToolDefs
│   ├── event.go      — Event、EventType、DoneReason、EventStream、ToolResultEvent 类型定义
│   └── batcher.go    — batchTools 分组逻辑、toolGroup 类型
├── conversation/
│   └── conversation.go — 新增 AddUserMessage、AddAssistantMessageWithTools 方法
├── tools/
│   └── registry.go    — 新增 Has 方法
└── tui/
    ├── model.go      — 删除旧消息类型，新增 agentEventMsg
    └── update.go     — startStream 改为消费 EventStream，Update 处理各事件类型，增加 /do 处理
cmd/
└── mewcode/
    └── main.go       — NewAgent 传入 maxIter、maxParallel 参数
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| Agent.SendMessage 返回值 | `*EventStream`（自定义 channel 包装） | 与 provider.Stream 解耦，支持 Agent 自定义事件类型（iteration、done 等） |
| 事件传递方式 | 有缓冲 channel（容量 32） | Agent goroutine 和 TUI 消费方不需要紧密同步，缓冲减少阻塞 |
| 并行工具执行方式 | `sync.WaitGroup` + goroutine | Go 标准方案，简单可靠 |
| 对话历史回灌时机 | 工具执行前回灌 assistant（含 text+toolCalls），每个工具完成后立即回灌 tool result | 确保对话历史完整，模型能看到每一步的结果 |
| Plan/Act 默认模式 | Act | 保持向后兼容，Plan 是可选功能 |
| StreamCollector 位置 | 不单独建文件，作为 runLoop 内的 collectStream 方法 | 逻辑不复杂，提取成独立模块反而增加理解成本 |
| Conversation 新增 AddUserMessage | 从 SendMessage 中拆出用户消息添加逻辑 | Agent 需要直接控制消息添加时机，不再依赖 Conversation.SendMessage 的内部逻辑 |
| Agent 直接调用 Provider.StreamChat | 绕过 Conversation.SendMessage | Agent 需要控制完整的循环流程，Conversation.SendMessage 的单次调用模式不适合循环场景 |
