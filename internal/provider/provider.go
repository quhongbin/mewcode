package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"mewcode/internal/config"
)

// ToolCall 表示模型发起的工具调用请求
type ToolCall struct {
	ID   string          `json:"id"`   // 唯一标识
	Name string          `json:"name"` // 工具名称
	Args json.RawMessage `json:"args"` // JSON 格式的参数
}

// Message 表示对话消息
type Message struct {
	Role       string     `json:"role"`                   // user, assistant, system, tool
	Content    string     `json:"content"`                // 文本内容
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // assistant 消息中的工具调用列表
	ToolCallID string     `json:"tool_call_id,omitempty"` // tool 消息对应的工具调用 ID
}

// StreamChunkType 表示流式数据块的类型
type StreamChunkType string

const (
	StreamChunkTypeText     StreamChunkType = "text"
	StreamChunkTypeThinking StreamChunkType = "thinking"
	StreamChunkTypeToolCall StreamChunkType = "tool_call" // 工具调用
	StreamChunkTypeDone     StreamChunkType = "done"
)

// StreamChunk 表示流式响应中的一个数据块
type StreamChunk struct {
	Type     StreamChunkType
	Content  string    // text/thinking 使用
	ToolCall *ToolCall // tool_call 使用（流式拼接完成后的完整调用）
}

// Stream 接口用于迭代流式响应
type Stream interface {
	Next() (*StreamChunk, error)
	Close() error
}

// ToolDefinition 工具的 API 格式定义，用于发送给 LLM API
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// Provider 接口定义了与 LLM 服务交互的方法
type Provider interface {
	StreamChat(ctx context.Context, messages []Message, tools []ToolDefinition, thinking bool) (Stream, error) // Message: 本文件中定义的结构体，Stream: 本文件中定义的接口，ToolDefinition: 本文件中定义的结构体
}

// NewProvider 根据配置创建对应的 Provider 实例
func NewProvider(cfg config.ProviderConfig) (Provider, error) {
	switch cfg.Protocol {
	case "anthropic":
		return NewAnthropicProvider(cfg) // NewAnthropicProvider: anthropic.go 中定义的函数
	case "openai":
		return NewOpenAIProvider(cfg) // NewOpenAIProvider: openai.go 中定义的函数
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", cfg.Protocol)
	}
}
