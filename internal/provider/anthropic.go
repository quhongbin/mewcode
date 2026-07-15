package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"mewcode/internal/config"
)

// AnthropicProvider 实现 Anthropic Claude API
type AnthropicProvider struct {
	config config.ProviderConfig
	client *http.Client
}

// NewAnthropicProvider 创建 Anthropic Provider 实例
func NewAnthropicProvider(cfg config.ProviderConfig) (*AnthropicProvider, error) {
	return &AnthropicProvider{
		config: cfg,
		client: &http.Client{},
	}, nil
}

// StreamChat 发送消息并返回流式响应
func (p *AnthropicProvider) StreamChat(ctx context.Context, messages []Message, tools []ToolDefinition, thinking bool) (Stream, error) { // Message: provider.go 中定义的结构体，Stream: provider.go 中定义的接口，ToolDefinition: provider.go 中定义的结构体
	// 将内部消息转为 Anthropic API 格式
	anthropicMessages := convertToAnthropicMessages(messages) // convertToAnthropicMessages: 本文件中定义的函数

	// 构造请求体
	reqBody := map[string]interface{}{
		"model":    p.config.Model,
		"messages": anthropicMessages,
		"stream":   true,
	}

	if len(tools) > 0 {
		// 将 ToolDefinition 转为 Anthropic API 格式
		anthropicTools := make([]map[string]interface{}, 0, len(tools))
		for _, t := range tools {
			anthropicTools = append(anthropicTools, map[string]interface{}{
				"name":         t.Name,
				"description":  t.Description,
				"input_schema": t.InputSchema,
			})
		}
		reqBody["tools"] = anthropicTools
	}

	if thinking {
		reqBody["thinking"] = map[string]interface{}{
			"type": "enabled",
		}
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建 HTTP 请求
	url := strings.TrimRight(p.config.BaseURL, "/") + "/v1/messages"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.config.APIKey)

	// 发送请求
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// 返回流
	scanner := bufio.NewScanner(resp.Body)
	return &anthropicStream{
		scanner: scanner,
		body:    resp.Body,
	}, nil
}

// anthropicStream 实现 Anthropic 的流式响应
type anthropicStream struct {
	scanner *bufio.Scanner
	body    io.ReadCloser
	// 工具调用流式拼接状态
	toolID    string // 当前工具调用的 ID
	toolName  string // 当前工具调用的名称
	toolArgs  string // 当前工具调用的 JSON 参数碎片累积
	inToolUse bool   // 是否正在接收 tool_use 内容块
}

// Next 获取下一个数据块
func (s *anthropicStream) Next() (*StreamChunk, error) { // StreamChunk: provider.go 中定义的结构体
	for s.scanner.Scan() {
		line := s.scanner.Text()

		// 跳过空行和 event 行
		if line == "" || strings.HasPrefix(line, "event:") {
			continue
		}

		// 解析 data 行
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// 检查是否是结束标记
			if data == "[DONE]" {
				return &StreamChunk{Type: StreamChunkTypeDone}, nil // StreamChunk: provider.go 中定义的结构体，StreamChunkTypeDone: provider.go 中定义的常量
			}

			// 解析 JSON
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			// 处理不同类型的事件
			eventType, _ := event["type"].(string)

			switch eventType {
			case "content_block_start":
				block, ok := event["content_block"].(map[string]interface{})
				if !ok {
					continue
				}
				blockType, _ := block["type"].(string)
				if blockType == "tool_use" {
					s.inToolUse = true
					s.toolID, _ = block["id"].(string)
					s.toolName, _ = block["name"].(string)
					s.toolArgs = ""
				}

			case "content_block_delta":
				delta, ok := event["delta"].(map[string]interface{})
				if !ok {
					continue
				}

				deltaType, _ := delta["type"].(string)

				switch deltaType {
				case "text_delta":
					text, _ := delta["text"].(string)
					return &StreamChunk{
						Type:    StreamChunkTypeText, // StreamChunkTypeText: provider.go 中定义的常量
						Content: text,
					}, nil
				case "thinking_delta":
					thinking, _ := delta["thinking"].(string)
					return &StreamChunk{
						Type:    StreamChunkTypeThinking, // StreamChunkTypeThinking: provider.go 中定义的常量
						Content: thinking,
					}, nil
				case "input_json_delta":
					if s.inToolUse {
						partial, _ := delta["partial_json"].(string)
						s.toolArgs += partial
					}
				}

			case "content_block_stop":
				if s.inToolUse {
					s.inToolUse = false
					return &StreamChunk{
						Type: StreamChunkTypeToolCall, // StreamChunkTypeToolCall: provider.go 中定义的常量
						ToolCall: &ToolCall{ // ToolCall: provider.go 中定义的结构体
							ID:   s.toolID,
							Name: s.toolName,
							Args: json.RawMessage(s.toolArgs),
						},
					}, nil
				}

			case "message_stop":
				return &StreamChunk{Type: StreamChunkTypeDone}, nil // StreamChunk: provider.go 中定义的结构体，StreamChunkTypeDone: provider.go 中定义的常量
			}
		}
	}

	if err := s.scanner.Err(); err != nil {
		return nil, fmt.Errorf("stream error: %w", err)
	}

	return &StreamChunk{Type: StreamChunkTypeDone}, nil // StreamChunk: provider.go 中定义的结构体，StreamChunkTypeDone: provider.go 中定义的常量
}

// Close 关闭流
func (s *anthropicStream) Close() error {
	if s.body != nil {
		return s.body.Close()
	}
	return nil
}

// convertToAnthropicMessages 将内部 Message 列表转为 Anthropic API 所需的消息格式
// Anthropic 要求：assistant 消息用 content 数组（text/tool_use 块），工具结果用 role=user + tool_result 块
func convertToAnthropicMessages(messages []Message) []map[string]interface{} { // Message: provider.go 中定义的结构体
	var result []map[string]interface{}
	for _, msg := range messages {
		switch msg.Role {
		case "user", "system":
			result = append(result, map[string]interface{}{
				"role":    msg.Role,
				"content": msg.Content,
			})

		case "assistant":
			// Anthropic 要求 assistant 消息使用 content 数组
			var content []map[string]interface{}
			if msg.Content != "" {
				content = append(content, map[string]interface{}{
					"type": "text",
					"text": msg.Content,
				})
			}
			for _, tc := range msg.ToolCalls { // ToolCalls: Message 结构体字段（provider.go），ToolCall: provider.go 中定义的结构体
				var input interface{}
				if err := json.Unmarshal(tc.Args, &input); err != nil { // json: 标准库
					input = map[string]interface{}{}
				}
				content = append(content, map[string]interface{}{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Name,
					"input": input,
				})
			}
			if len(content) == 0 {
				content = append(content, map[string]interface{}{
					"type": "text",
					"text": "",
				})
			}
			result = append(result, map[string]interface{}{
				"role":    "assistant",
				"content": content,
			})

		case "tool":
			// Anthropic 用 role=user + tool_result 内容块表示工具结果
			result = append(result, map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type":        "tool_result",
						"tool_use_id": msg.ToolCallID, // ToolCallID: Message 结构体字段（provider.go）
						"content":     msg.Content,    // Content: Message 结构体字段（provider.go）
					},
				},
			})
		}
	}
	return result
}
