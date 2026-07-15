package agent

import (
	"mewcode/internal/provider"
	"mewcode/internal/tools"
)

// EventType 事件类型标识
type EventType string

const (
	EventText       EventType = "text"        // 模型输出的文本片段
	EventThinking   EventType = "thinking"    // 模型思考过程的文本片段
	EventToolCall   EventType = "tool_call"   // 模型请求执行工具
	EventToolResult EventType = "tool_result" // 工具执行结果
	EventIteration  EventType = "iteration"   // 迭代进度（每轮 LLM 调用开始时发出）
	EventCommit     EventType = "commit"      // 提交当前轮文本到消息历史（工具调用前发出）
	EventDone       EventType = "done"        // 循环结束信号
)

// DoneReason 循环结束原因
type DoneReason string

const (
	DoneCompleted   DoneReason = "completed"     // 模型返回纯文本，任务完成
	DoneMaxIter     DoneReason = "max_iteration" // 达到迭代上限
	DoneCancelled   DoneReason = "cancelled"     // 用户取消
	DoneUnknownTool DoneReason = "unknown_tool"  // 模型调用了未知工具
	DoneError       DoneReason = "error"         // 出错
)

// ToolResultEvent 工具执行结果事件数据
type ToolResultEvent struct {
	Name   string           // 工具名称，用于 UI 显示
	Result tools.ToolResult // tools.ToolResult: tool.go 中定义的结构体
}

// Event Agent 产出的结构化事件
type Event struct {
	Type       EventType
	Content    string             // text/thinking 使用
	ToolCall   *provider.ToolCall // tool_call 使用（provider.ToolCall: provider.go 中定义的结构体）
	ToolResult *ToolResultEvent   // tool_result 使用
	Iteration  int                // iteration 使用（当前轮次，从 1 开始）
	Reason     DoneReason         // done 使用
}

// EventStream 基于 channel 的事件流，供外部消费 Agent 产出的事件
type EventStream struct {
	ch chan Event
}

// newEventStream 创建带缓冲的 EventStream
func newEventStream() *EventStream { // EventStream: 本文件中定义的结构体
	return &EventStream{
		ch: make(chan Event, 32),
	}
}

// Next 阻塞获取下一个事件，channel 关闭时返回 (Event{}, false)
func (s *EventStream) Next() (Event, bool) {
	event, ok := <-s.ch
	return event, ok
}

// emit 向 EventStream 发送事件（内部使用）
func (s *EventStream) emit(event Event) { // Event: 本文件中定义的结构体
	s.ch <- event
}
