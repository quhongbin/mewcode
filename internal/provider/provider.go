package provider

import (
	"context"
	"fmt"
	"mewcode/internal/config"
)

// Message 表示对话消息
type Message struct {
	Role    string `json:"role"`    // user, assistant, system
	Content string `json:"content"`
}

// StreamChunkType 表示流式数据块的类型
type StreamChunkType string

const (
	StreamChunkTypeText    StreamChunkType = "text"
	StreamChunkTypeThinking StreamChunkType = "thinking"
	StreamChunkTypeDone    StreamChunkType = "done"
)

// StreamChunk 表示流式响应中的一个数据块
type StreamChunk struct {
	Type    StreamChunkType
	Content string
}

// Stream 接口用于迭代流式响应
type Stream interface {
	Next() (*StreamChunk, error)
	Close() error
}

// Provider 接口定义了与 LLM 服务交互的方法
type Provider interface {
	StreamChat(ctx context.Context, messages []Message, thinking bool) (Stream, error) // Message: 本文件中定义的结构体，Stream: 本文件中定义的接口
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
