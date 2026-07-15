package agent

import (
	"context"
	"sync"

	"mewcode/internal/conversation"
	"mewcode/internal/provider"
	"mewcode/internal/tools"
)

// AgentMode Agent 运行模式
type AgentMode int

const (
	ModeAct  AgentMode = iota // 全工具模式
	ModePlan                  // 只读工具模式
)

// Agent 编排层，负责驱动 ReAct 自主循环：请求模型 → 检测工具调用 → 执行工具 → 回灌结果 → 再次请求模型
type Agent struct {
	conv        *conversation.Conversation // 对话历史管理
	registry    *tools.Registry            // 工具注册中心
	executor    *tools.Executor            // 工具执行器
	provider    provider.Provider          // LLM 服务提供者
	thinking    bool                       // 是否启用思考模式
	maxIter     int                        // 迭代上限
	maxParallel int                        // 并行工具执行上限
	mode        AgentMode                  // 当前运行模式
}

// NewAgent 创建 Agent 编排器实例
func NewAgent(conv *conversation.Conversation, registry *tools.Registry, executor *tools.Executor, p provider.Provider, thinking bool, maxIter int, maxParallel int) *Agent { // conversation.Conversation: conversation.go 中定义的结构体，tools.Registry: registry.go 中定义的结构体，tools.Executor: executor.go 中定义的结构体
	return &Agent{
		conv:        conv,
		registry:    registry,
		executor:    executor,
		provider:    p,
		thinking:    thinking,
		maxIter:     maxIter,
		maxParallel: maxParallel,
		mode:        ModeAct,
	}
}

// SetMode 切换 Agent 运行模式
func (a *Agent) SetMode(mode AgentMode) {
	a.mode = mode
}

// Mode 获取当前运行模式
func (a *Agent) Mode() AgentMode {
	return a.mode
}

// SendMessage 发送用户消息，返回事件流供外部消费 Agent 的完整执行过程
func (a *Agent) SendMessage(ctx context.Context, userText string) *EventStream { // EventStream: event.go 中定义的结构体
	es := newEventStream() // newEventStream: event.go 中定义的函数
	go a.runLoop(ctx, userText, es)
	return es
}

// runLoop ReAct 主循环：驱动多轮 LLM 调用和工具执行
func (a *Agent) runLoop(ctx context.Context, userText string, es *EventStream) { // EventStream: event.go 中定义的结构体
	defer close(es.ch)

	// 添加用户消息到对话历史
	a.conv.AddUserMessage(userText) // Conversation.AddUserMessage: conversation.go 中定义的方法

	for iteration := 1; iteration <= a.maxIter; iteration++ {
		// 检查 context 是否已取消
		select {
		case <-ctx.Done():
			es.emit(Event{Type: EventDone, Reason: DoneCancelled}) // Event: event.go 中定义的结构体，EventDone, DoneCancelled: event.go 中定义的常量
			return
		default:
		}

		// 发出迭代进度事件
		es.emit(Event{Type: EventIteration, Iteration: iteration}) // EventIteration: event.go 中定义的常量

		// 根据当前模式选择工具集
		toolDefs := a.providerToolDefs() // providerToolDefs: 本文件中定义的方法

		// 调用 LLM 获取流式响应
		stream, err := a.provider.StreamChat(ctx, a.conv.GetMessages(), toolDefs, a.thinking) // provider.StreamChat: provider.go 中定义的方法
		if err != nil {
			es.emit(Event{Type: EventDone, Reason: DoneError}) // DoneError: event.go 中定义的常量
			return
		}

		// 流式双路收集：实时推送 + 完整累积
		text, _, toolCalls, err := a.collectStream(stream, es) // collectStream: 本文件中定义的方法
		if err != nil {
			es.emit(Event{Type: EventDone, Reason: DoneError})
			return
		}

		// 无工具调用 → 任务完成
		if len(toolCalls) == 0 {
			a.conv.AddAssistantMessage(text) // Conversation.AddAssistantMessage: conversation.go 中定义的方法
			es.emit(Event{Type: EventDone, Reason: DoneCompleted})
			return
		}

		// 检查未知工具
		for _, tc := range toolCalls {
			if !a.registry.Has(tc.Name) { // Registry.Has: registry.go 中定义的方法
				es.emit(Event{Type: EventDone, Reason: DoneUnknownTool})
				return
			}
		}

		// 回灌 assistant 消息（含文本和工具调用）
		a.conv.AddAssistantMessageWithTools(text, toolCalls) // Conversation.AddAssistantMessageWithTools: conversation.go 中定义的方法

		// 通知 TUI 提交当前轮文本到消息历史（在工具调用之前，确保渲染顺序正确）
		es.emit(Event{Type: EventCommit}) // EventCommit: event.go 中定义的常量

		// 分组执行工具
		groups := batchTools(toolCalls, a.registry, a.maxParallel) // batchTools: batcher.go 中定义的函数
		a.executeGroups(ctx, groups, es)                           // executeGroups: 本文件中定义的方法
	}

	// 达到迭代上限
	es.emit(Event{Type: EventDone, Reason: DoneMaxIter}) // DoneMaxIter: event.go 中定义的常量
}

// collectStream 消费 LLM 流式响应，同时实时推送事件并累积完整响应
func (a *Agent) collectStream(stream provider.Stream, es *EventStream) (text string, thinking string, toolCalls []provider.ToolCall, err error) { // provider.Stream: provider.go 中定义的接口，EventStream: event.go 中定义的结构体
	defer stream.Close() // provider.Stream.Close: provider.go 中定义的方法

	var textBuilder, thinkingBuilder string
	var toolCallList []provider.ToolCall

	for {
		chunk, err := stream.Next() // provider.Stream.Next: provider.go 中定义的方法
		if err != nil {
			return textBuilder, thinkingBuilder, toolCallList, err
		}

		switch chunk.Type {
		case provider.StreamChunkTypeText: // provider.StreamChunkTypeText: provider.go 中定义的常量
			textBuilder += chunk.Content
			es.emit(Event{Type: EventText, Content: chunk.Content}) // EventText: event.go 中定义的常量

		case provider.StreamChunkTypeThinking: // provider.StreamChunkTypeThinking: provider.go 中定义的常量
			thinkingBuilder += chunk.Content
			es.emit(Event{Type: EventThinking, Content: chunk.Content}) // EventThinking: event.go 中定义的常量

		case provider.StreamChunkTypeToolCall: // provider.StreamChunkTypeToolCall: provider.go 中定义的常量
			if chunk.ToolCall != nil {
				toolCallList = append(toolCallList, *chunk.ToolCall)
				es.emit(Event{Type: EventToolCall, ToolCall: chunk.ToolCall}) // EventToolCall: event.go 中定义的常量
			}

		case provider.StreamChunkTypeDone: // provider.StreamChunkTypeDone: provider.go 中定义的常量
			return textBuilder, thinkingBuilder, toolCallList, nil
		}
	}
}

// executeGroups 按分组执行工具，只读组并发，副作用组串行
func (a *Agent) executeGroups(ctx context.Context, groups []toolGroup, es *EventStream) { // toolGroup: batcher.go 中定义的结构体，EventStream: event.go 中定义的结构体
	for _, g := range groups {
		if g.readOnly && len(g.calls) > 1 {
			// 只读组并发执行
			var wg sync.WaitGroup
			for _, call := range g.calls {
				wg.Add(1)
				go func(c provider.ToolCall) { // provider.ToolCall: provider.go 中定义的结构体
					defer wg.Done()
					a.executeOneTool(ctx, c, es) // executeOneTool: 本文件中定义的方法
				}(call)
			}
			wg.Wait()
		} else {
			// 串行执行（副作用组或单工具只读组）
			for _, call := range g.calls {
				a.executeOneTool(ctx, call, es)
			}
		}
	}
}

// executeOneTool 执行单个工具并回灌结果到对话历史
func (a *Agent) executeOneTool(ctx context.Context, call provider.ToolCall, es *EventStream) { // provider.ToolCall: provider.go 中定义的结构体，EventStream: event.go 中定义的结构体
	result := a.executor.Execute(ctx, call.Name, call.Args) // Executor.Execute: executor.go 中定义的方法

	// 回灌对话历史
	a.conv.AddToolResultMessage(call.ID, result.Content, result.IsError) // Conversation.AddToolResultMessage: conversation.go 中定义的方法

	// 发出工具结果事件
	es.emit(Event{
		Type: EventToolResult, // EventToolResult: event.go 中定义的常量
		ToolResult: &ToolResultEvent{
			Name:   call.Name,
			Result: result,
		},
	})
}

// providerToolDefs 根据当前模式返回 LLM 所需的工具定义列表
func (a *Agent) providerToolDefs() []provider.ToolDefinition { // provider.ToolDefinition: provider.go 中定义的结构体
	regTools := a.registry.List() // Registry.List: registry.go 中定义的方法

	var defs []provider.ToolDefinition
	for _, t := range regTools {
		// Plan 模式下只暴露只读工具
		if a.mode == ModePlan && !t.Meta().ReadOnly { // Tool.Meta: tool.go 中定义的方法，ToolMeta.ReadOnly: tool.go 中定义的字段
			continue
		}
		defs = append(defs, provider.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.Parameters(),
		})
	}
	return defs
}
