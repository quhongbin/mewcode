package conversation

import (
	"context"
	"errors"
	"mewcode/internal/provider"
	"testing"
)

// mockProvider 用于测试的模拟 Provider
type mockProvider struct {
	responses []provider.StreamChunk
	callCount int
}

func (m *mockProvider) StreamChat(ctx context.Context, messages []provider.Message, tools []provider.ToolDefinition, thinking bool) (provider.Stream, error) { // provider.Message, provider.ToolDefinition, provider.Stream: provider.go 中定义的类型
	m.callCount++
	return &mockStream{chunks: m.responses, index: 0}, nil
}

// mockStream 用于测试的模拟 Stream
type mockStream struct {
	chunks []provider.StreamChunk
	index  int
}

func (s *mockStream) Next() (*provider.StreamChunk, error) {
	if s.index >= len(s.chunks) {
		return nil, errors.New("stream ended")
	}
	chunk := s.chunks[s.index]
	s.index++
	return &chunk, nil
}

func (s *mockStream) Close() error {
	return nil
}

func TestConversation_SendMessage(t *testing.T) {
	mock := &mockProvider{
		responses: []provider.StreamChunk{
			{Type: provider.StreamChunkTypeText, Content: "Hello"},
			{Type: provider.StreamChunkTypeText, Content: " World"},
		},
	}

	conv := NewConversation(mock, false) // NewConversation: conversation.go 中定义的函数

	// 发送第一条消息
	stream, err := conv.SendMessage(context.Background(), "Hi", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// 验证消息已添加到历史
	messages := conv.GetMessages()
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
	if messages[0].Role != "user" || messages[0].Content != "Hi" {
		t.Errorf("Unexpected message: %+v", messages[0])
	}

	// 读取流
	var content string
	for {
		chunk, err := stream.Next()
		if err != nil {
			break
		}
		if chunk.Type == provider.StreamChunkTypeText {
			content += chunk.Content
		}
	}

	// 添加助手回复
	conv.AddAssistantMessage(content)

	// 验证助手回复已添加
	messages = conv.GetMessages()
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}
	if messages[1].Role != "assistant" || messages[1].Content != "Hello World" {
		t.Errorf("Unexpected assistant message: %+v", messages[1])
	}
}

func TestConversation_MultipleRounds(t *testing.T) {
	mock := &mockProvider{
		responses: []provider.StreamChunk{
			{Type: provider.StreamChunkTypeText, Content: "Response"},
		},
	}

	conv := NewConversation(mock, false) // NewConversation: conversation.go 中定义的函数

	// 第一轮对话
	_, err := conv.SendMessage(context.Background(), "First question", nil)
	if err != nil {
		t.Fatalf("First SendMessage failed: %v", err)
	}
	conv.AddAssistantMessage("First response")

	// 第二轮对话
	_, err = conv.SendMessage(context.Background(), "Second question", nil)
	if err != nil {
		t.Fatalf("Second SendMessage failed: %v", err)
	}
	conv.AddAssistantMessage("Second response")

	// 验证历史消息
	messages := conv.GetMessages()
	if len(messages) != 4 {
		t.Errorf("Expected 4 messages, got %d", len(messages))
	}

	expectedRoles := []string{"user", "assistant", "user", "assistant"}
	expectedContents := []string{"First question", "First response", "Second question", "Second response"}

	for i, msg := range messages {
		if msg.Role != expectedRoles[i] {
			t.Errorf("Message %d: expected role %s, got %s", i, expectedRoles[i], msg.Role)
		}
		if msg.Content != expectedContents[i] {
			t.Errorf("Message %d: expected content %s, got %s", i, expectedContents[i], msg.Content)
		}
	}

	// 验证 Provider 被调用了两次
	if mock.callCount != 2 {
		t.Errorf("Expected Provider to be called 2 times, got %d", mock.callCount)
	}
}

func TestConversation_WithThinking(t *testing.T) {
	mock := &mockProvider{
		responses: []provider.StreamChunk{
			{Type: provider.StreamChunkTypeThinking, Content: "Thinking..."},
			{Type: provider.StreamChunkTypeText, Content: "Answer"},
		},
	}

	conv := NewConversation(mock, true) // NewConversation: conversation.go 中定义的函数

	stream, err := conv.SendMessage(context.Background(), "Question", nil)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// 读取流，收集思考和回答
	var thinking, answer string
	for {
		chunk, err := stream.Next()
		if err != nil {
			break
		}
		switch chunk.Type {
		case provider.StreamChunkTypeThinking:
			thinking += chunk.Content
		case provider.StreamChunkTypeText:
			answer += chunk.Content
		}
	}

	if thinking != "Thinking..." {
		t.Errorf("Expected thinking 'Thinking...', got '%s'", thinking)
	}
	if answer != "Answer" {
		t.Errorf("Expected answer 'Answer', got '%s'", answer)
	}
}
