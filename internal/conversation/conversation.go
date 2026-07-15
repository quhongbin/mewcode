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
func (c *Conversation) SendMessage(ctx context.Context, userText string, tools []provider.ToolDefinition) (provider.Stream, error) { // provider.ToolDefinition: provider.go 中定义的结构体
	// 添加用户消息到历史
	c.messages = append(c.messages, provider.Message{
		Role:    "user",
		Content: userText,
	})

	// 调用 Provider 获取响应
	stream, err := c.provider.StreamChat(ctx, c.messages, tools, c.thinking)
	if err != nil {
		return nil, err
	}

	return stream, nil
}

// AddUserMessage 添加用户消息到历史
func (c *Conversation) AddUserMessage(content string) {
	c.messages = append(c.messages, provider.Message{
		Role:    "user",
		Content: content,
	})
}

// AddAssistantMessage 添加助手回复到历史
func (c *Conversation) AddAssistantMessage(content string) {
	c.messages = append(c.messages, provider.Message{
		Role:    "assistant",
		Content: content,
	})
}

// AddAssistantMessageWithTools 添加含工具调用的助手消息到历史
func (c *Conversation) AddAssistantMessageWithTools(content string, toolCalls []provider.ToolCall) { // provider.ToolCall: provider.go 中定义的结构体
	c.messages = append(c.messages, provider.Message{
		Role:      "assistant",
		Content:   content,
		ToolCalls: toolCalls,
	})
}

// AddToolCallMessage 添加助手的工具调用消息到历史
func (c *Conversation) AddToolCallMessage(toolCalls []provider.ToolCall) { // provider.ToolCall: provider.go 中定义的结构体
	c.messages = append(c.messages, provider.Message{
		Role:      "assistant",
		ToolCalls: toolCalls,
	})
}

// AddToolResultMessage 添加工具执行结果消息到历史
func (c *Conversation) AddToolResultMessage(toolCallID string, content string, isError bool) {
	role := "tool"
	if isError {
		// 错误结果也通过 tool 角色传递，模型可从 content 中识别错误
	}
	c.messages = append(c.messages, provider.Message{
		Role:       role,
		Content:    content,
		ToolCallID: toolCallID,
	})
}

// GetMessages 获取当前对话历史
func (c *Conversation) GetMessages() []provider.Message {
	return c.messages
}
