# Agent Loop 自主循环 Tasks

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 新建 | `internal/agent/event.go` | Event、EventType、DoneReason、EventStream、ToolResultEvent 类型定义 |
| 新建 | `internal/agent/batcher.go` | batchTools 分组逻辑、toolGroup 类型 |
| 修改 | `internal/tools/registry.go` | 新增 Has 方法 |
| 修改 | `internal/conversation/conversation.go` | 新增 AddUserMessage、AddAssistantMessageWithTools 方法 |
| 重写 | `internal/agent/agent.go` | Agent 结构体改造、ReAct 主循环、collectStream、executeGroups |
| 修改 | `internal/tui/model.go` | 删除旧消息类型，新增 agentEventMsg |
| 重写 | `internal/tui/update.go` | startStream 改为消费 EventStream，Update 处理各事件类型，增加 /do |
| 修改 | `cmd/mewcode/main.go` | NewAgent 传入 maxIter、maxParallel 参数 |

## T1: 定义事件类型与 EventStream

**文件：** `internal/agent/event.go`
**依赖：** 无
**步骤：**
1. 定义 `EventType` 字符串类型及 6 个常量：EventText、EventThinking、EventToolCall、EventToolResult、EventIteration、EventDone
2. 定义 `DoneReason` 字符串类型及 5 个常量：DoneCompleted、DoneMaxIter、DoneCancelled、DoneUnknownTool、DoneError
3. 定义 `ToolResultEvent` 结构体，包含 Name string 和 Result tools.ToolResult 字段
4. 定义 `Event` 结构体，包含 Type EventType、Content string、ToolCall *provider.ToolCall、ToolResult *ToolResultEvent、Iteration int、Reason DoneReason 字段
5. 定义 `EventStream` 结构体，包含 ch chan Event（缓冲 32）
6. 实现 `EventStream.Next() (Event, bool)` 方法，从 channel 读取事件，channel 关闭时返回 (Event{}, false)

**验证：** `go build ./internal/agent/...` 编译通过

## T2: 实现工具分组逻辑

**文件：** `internal/agent/batcher.go`
**依赖：** T1
**步骤：**
1. 定义 `toolGroup` 结构体，包含 calls []provider.ToolCall 和 readOnly bool 字段
2. 实现 `batchTools(calls []provider.ToolCall, registry *tools.Registry, maxParallel int) []toolGroup` 函数
3. 遍历 calls 列表，通过 `registry.Get(call.Name)` 查询工具 Meta，判断 ReadOnly 属性
4. 相邻的 ReadOnly 工具合并为一组（toolGroup{readOnly: true}）
5. 非 ReadOnly 工具各自单独成组（toolGroup{readOnly: false}）
6. 对 ReadOnly 组，如果 calls 数量超过 maxParallel，按 maxParallel 拆分为多个子组
7. 如果 registry 中找不到工具，将其视为非 ReadOnly（保守策略，单独成组）

**验证：** `go build ./internal/agent/...` 编译通过

## T3: Registry 新增 Has 方法

**文件：** `internal/tools/registry.go`
**依赖：** 无
**步骤：**
1. 在 Registry 上新增 `Has(name string) bool` 方法
2. 内部实现：`_, ok := r.tools[name]; return ok`

**验证：** `go build ./internal/tools/...` 编译通过

## T4: Conversation 新增消息方法

**文件：** `internal/conversation/conversation.go`
**依赖：** 无
**步骤：**
1. 新增 `AddUserMessage(content string)` 方法：添加 role=user 消息到历史（从 Agent 中拆出，让 Agent 直接控制消息添加时机）
2. 新增 `AddAssistantMessageWithTools(content string, toolCalls []provider.ToolCall)` 方法：添加 role=assistant 消息，同时包含 Content 和 ToolCalls 字段
3. 检查现有 `SendMessage` 方法——保留不删除（向后兼容），但 Agent 将不再使用它

**验证：** `go build ./internal/conversation/...` 编译通过

## T5: 重写 Agent 主循环

**文件：** `internal/agent/agent.go`
**依赖：** T1、T2、T3、T4
**步骤：**
1. 定义 `AgentMode` 类型（int）及 ModeAct（0）、ModePlan（1）常量
2. 改造 `Agent` 结构体：新增 maxIter int、maxParallel int、mode AgentMode 字段
3. 改造 `NewAgent` 函数签名：增加 maxIter、maxParallel 参数，默认 mode 为 ModeAct
4. 新增 `SetMode(mode AgentMode)` 和 `Mode() AgentMode` 方法
5. 改造 `providerToolDefs()` 方法：ModePlan 下只返回 ReadOnly 工具，ModeAct 下返回全部工具
6. 重写 `SendMessage(ctx, userText) *EventStream`：创建 EventStream，启动 runLoop goroutine，返回 EventStream
7. 实现 `runLoop(ctx, userText, eventCh)` 主循环：
   - `conv.AddUserMessage(userText)` 添加用户消息
   - for 循环（iteration 1 到 maxIter）：
     - 检查 ctx.Done()，若已取消则 emit done(cancelled) 并 return
     - emit iteration(iteration)
     - 调用 `providerToolDefs()` 获取工具集
     - 调用 `provider.StreamChat(ctx, conv.GetMessages(), toolDefs, thinking)` 获取流
     - 调用 `collectStream(ctx, stream, eventCh)` 收集响应
     - 若 collectStream 返回 error，emit done(error) 并 return
     - 若无工具调用：`conv.AddAssistantMessage(text)`，emit done(completed)，return
     - 检查每个 toolCall.Name 是否 `registry.Has(name)`，若有不存在的则 emit done(unknown_tool)，return
     - `conv.AddAssistantMessageWithTools(text, toolCalls)` 回灌
     - 调用 `batchTools` 分组，调用 `executeGroups` 执行
   - 循环结束后 emit done(max_iteration)
8. 实现 `collectStream(ctx, stream, eventCh) (text, thinking string, toolCalls []provider.ToolCall, err error)`：
   - 循环调用 stream.Next()
   - text chunk → emit EventText，累积到 text
   - thinking chunk → emit EventThinking，累积到 thinking
   - tool_call chunk → emit EventToolCall，追加到 toolCalls
   - done chunk → 跳出循环
   - error → 返回 err
   - 关闭 stream
9. 实现 `executeGroups(ctx, groups []toolGroup, eventCh)`：
   - 遍历 groups
   - readOnly 组：用 sync.WaitGroup + goroutine 并发执行组内所有工具
   - 非 readOnly 组：串行执行组内工具（实际每组只有 1 个）
   - 每个工具执行后：`executor.Execute(ctx, call.Name, call.Args)` → `conv.AddToolResultMessage(call.ID, result.Content, result.IsError)` → emit EventToolResult
10. 删除旧的 `agentStream` 类型及其 Next/Close 方法

**验证：** `go build ./internal/agent/...` 编译通过

## T6: TUI Model 适配事件流

**文件：** `internal/tui/model.go`
**依赖：** T5
**步骤：**
1. 删除旧消息类型：streamChunkMsg、streamDoneMsg、toolCallMsg、toolResultMsg
2. 新增 `agentEventMsg` 消息类型，包含 event agent.Event 字段
3. Model 结构体新增 `currentIteration int` 字段（显示当前迭代轮次）
4. 更新 DisplayMessage 结构体：新增 Iteration int 字段（可选，用于显示迭代进度）

**验证：** `go build ./internal/tui/...` 编译通过

## T7: TUI Update 适配事件流与 /do 命令

**文件：** `internal/tui/update.go`
**依赖：** T6
**步骤：**
1. 重写 `startStream` 函数：
   - 调用 `agent.SendMessage(ctx, userInput)` 获取 *EventStream
   - 循环调用 `eventStream.Next()`
   - 每个 Event 通过 `m.program.Send(agentEventMsg{event: event})` 发给 Bubble Tea
   - 当 Next() 返回 false 时，发送完成信号
2. 重写 Update 中的事件处理：
   - 删除旧的 streamChunkMsg、streamDoneMsg、toolResultMsg case
   - 新增 `agentEventMsg` case，按 event.Type 分发：
     - EventText: 追加到 currentContent
     - EventThinking: 追加到 currentThinking
     - EventToolCall: 添加 DisplayMessage{Role: "tool_call", ToolCall: event.ToolCall}
     - EventToolResult: 添加 DisplayMessage{Role: "tool_result", ToolResult: &event.ToolResult.Result}
     - EventIteration: 更新 currentIteration
     - EventDone: 添加 assistant DisplayMessage，重置 status 为 idle
   - 删除 streamErrMsg 处理（错误通过 EventDone 传递）
3. 在 Enter 键处理中增加 `/do` 命令识别：
   - 如果输入为 `/do`，调用 `agent.SetMode(agent.ModeAct)`，添加系统提示消息到显示，不发送给 Agent
4. 删除 `executeTool` 方法（不再需要）

**验证：** `go build ./internal/tui/...` 编译通过

## T8: main.go 适配新 Agent 签名

**文件：** `cmd/mewcode/main.go`
**依赖：** T5
**步骤：**
1. 修改 `agent.NewAgent` 调用，增加 maxIter（25）和 maxParallel（5）参数
2. 调用方式：`agent.NewAgent(conv, reg, exec, p, thinking, 25, 5)`

**验证：** `go build ./cmd/mewcode/...` 编译通过

## T9: 全量编译验证

**文件：** 全项目
**依赖：** T1-T8
**步骤：**
1. 执行 `go build ./...` 确保全项目编译通过
2. 执行 `go vet ./...` 确保无静态分析问题

**验证：** `go build ./...` 和 `go vet ./...` 均无错误

## 执行顺序

```
T1（event.go）──→ T2（batcher.go）──→ T5（agent.go 重写）──→ T6（model.go）──→ T7（update.go）──→ T8（main.go）──→ T9（全量验证）
                                                              ↑
T3（registry Has）─────────────────────────────────────────────┘
T4（conversation 新方法）──────────────────────────────────────┘
```

T1、T3、T4 无依赖可并行。T2 依赖 T1。T5 依赖 T1-T4。T6-T8 依赖 T5。T9 最后执行。
