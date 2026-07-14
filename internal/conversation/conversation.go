package conversation

import (
	"context"
	"mewcode/internal/provider"
)

// Conversation 管理对话历史
type Conversation struct {
	messages []provider.Message
	provider provider.Provider
	thinking bool
}

// NewConversation 创建新的对话实例
func NewConversation(p provider.Provider, thinking bool) *Conversation {
	return &Conversation{
		messages: []provider.Message{},
		provider: p,
		thinking: thinking,
	}
}

// SendMessage 发送用户消息并返回流式响应
func (c *Conversation) SendMessage(ctx context.Context, userText string) (provider.Stream, error) {
	// 添加用户消息到历史
	c.messages = append(c.messages, provider.Message{
		Role:    "user",
		Content: userText,
	})

	// 调用 Provider 获取响应
	stream, err := c.provider.StreamChat(ctx, c.messages, c.thinking)
	if err != nil {
		return nil, err
	}

	return stream, nil
}

// AddAssistantMessage 添加助手回复到历史
func (c *Conversation) AddAssistantMessage(content string) {
	c.messages = append(c.messages, provider.Message{
		Role:    "assistant",
		Content: content,
	})
}

// GetMessages 获取当前对话历史
func (c *Conversation) GetMessages() []provider.Message {
	return c.messages
}
