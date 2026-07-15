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

// OpenAIProvider 实现 OpenAI API
type OpenAIProvider struct {
	config config.ProviderConfig
	client *http.Client
}

// NewOpenAIProvider 创建 OpenAI Provider 实例
func NewOpenAIProvider(cfg config.ProviderConfig) (*OpenAIProvider, error) {
	return &OpenAIProvider{
		config: cfg,
		client: &http.Client{},
	}, nil
}

// StreamChat 发送消息并返回流式响应
func (p *OpenAIProvider) StreamChat(ctx context.Context, messages []Message, tools []ToolDefinition, thinking bool) (Stream, error) { // Message: provider.go 中定义的结构体，Stream: provider.go 中定义的接口，ToolDefinition: provider.go 中定义的结构体
	// 将内部消息转为 OpenAI API 格式
	openaiMessages := convertToOpenAIMessages(messages) // convertToOpenAIMessages: 本文件中定义的函数

	// 构造请求体
	reqBody := map[string]interface{}{
		"model":    p.config.Model,
		"messages": openaiMessages,
		"stream":   true,
	}

	if len(tools) > 0 {
		// 将 ToolDefinition 转为 OpenAI API 格式
		openaiTools := make([]map[string]interface{}, 0, len(tools))
		for _, t := range tools {
			openaiTools = append(openaiTools, map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  t.InputSchema,
				},
			})
		}
		reqBody["tools"] = openaiTools
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建 HTTP 请求
	url := strings.TrimRight(p.config.BaseURL, "/") + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

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
	return &openaiStream{
		scanner:   scanner,
		body:      resp.Body,
		toolCalls: make(map[int]*toolCallAccum),
		emitted:   make(map[int]bool),
	}, nil
}

// openaiStream 实现 OpenAI 的流式响应
type openaiStream struct {
	scanner *bufio.Scanner
	body    io.ReadCloser
	// 工具调用流式拼接状态
	toolCalls map[int]*toolCallAccum // 按 index 追踪工具调用碎片
	emitted   map[int]bool           // 已发送的工具调用 index
}

// toolCallAccum 工具调用碎片累积器
type toolCallAccum struct {
	id   string
	name string
	args string
}

// Next 获取下一个数据块
func (s *openaiStream) Next() (*StreamChunk, error) { // StreamChunk: provider.go 中定义的结构体
	for s.scanner.Scan() {
		line := s.scanner.Text()

		// 跳过空行
		if line == "" {
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

			// 提取 choices[0].delta.content
			choices, ok := event["choices"].([]interface{})
			if !ok || len(choices) == 0 {
				continue
			}

			choice, ok := choices[0].(map[string]interface{})
			if !ok {
				continue
			}

			delta, ok := choice["delta"].(map[string]interface{})
			if !ok {
				continue
			}

			// 检查文本内容
			content, ok := delta["content"].(string)
			if ok && content != "" {
				return &StreamChunk{
					Type:    StreamChunkTypeText, // StreamChunkTypeText: provider.go 中定义的常量
					Content: content,
				}, nil
			}

			// 检查工具调用碎片
			if tcRaw, ok := delta["tool_calls"].([]interface{}); ok {
				for _, tc := range tcRaw {
					tcMap, ok := tc.(map[string]interface{})
					if !ok {
						continue
					}

					idx := 0
					if idxVal, ok := tcMap["index"].(float64); ok {
						idx = int(idxVal)
					}

					acc, exists := s.toolCalls[idx]
					if !exists {
						acc = &toolCallAccum{}
						s.toolCalls[idx] = acc
					}

					// 首个碎片包含 id 和 function.name
					if id, ok := tcMap["id"].(string); ok && id != "" {
						acc.id = id
					}
					if fn, ok := tcMap["function"].(map[string]interface{}); ok {
						if name, ok := fn["name"].(string); ok {
							acc.name = name
						}
						if args, ok := fn["arguments"].(string); ok {
							acc.args += args
						}
					}
				}
			}

			// 检查 finish_reason（可能在单独的 chunk 中）
			finishReason, _ := choice["finish_reason"].(string)
			if finishReason == "tool_calls" {
				for idx, acc := range s.toolCalls {
					if !s.emitted[idx] && acc.name != "" {
						s.emitted[idx] = true
						return &StreamChunk{
							Type: StreamChunkTypeToolCall, // StreamChunkTypeToolCall: provider.go 中定义的常量
							ToolCall: &ToolCall{ // ToolCall: provider.go 中定义的结构体
								ID:   acc.id,
								Name: acc.name,
								Args: json.RawMessage(acc.args),
							},
						}, nil
					}
				}
			}
		}
	}

	if err := s.scanner.Err(); err != nil {
		return nil, fmt.Errorf("stream error: %w", err)
	}

	return &StreamChunk{Type: StreamChunkTypeDone}, nil // StreamChunk: provider.go 中定义的结构体，StreamChunkTypeDone: provider.go 中定义的常量
}

// Close 关闭流
func (s *openaiStream) Close() error {
	if s.body != nil {
		return s.body.Close()
	}
	return nil
}

// convertToOpenAIMessages 将内部 Message 列表转为 OpenAI API 所需的消息格式
// OpenAI 要求：assistant 带工具调用时用 tool_calls 数组，工具结果用 role=tool + tool_call_id
func convertToOpenAIMessages(messages []Message) []map[string]interface{} { // Message: provider.go 中定义的结构体
	var result []map[string]interface{}
	for _, msg := range messages {
		switch msg.Role {
		case "user", "system":
			result = append(result, map[string]interface{}{
				"role":    msg.Role,
				"content": msg.Content,
			})

		case "assistant":
			m := map[string]interface{}{
				"role":    "assistant",
				"content": msg.Content,
			}
			if len(msg.ToolCalls) > 0 { // ToolCalls: Message 结构体字段（provider.go）
				var toolCalls []map[string]interface{}
				for _, tc := range msg.ToolCalls { // ToolCall: provider.go 中定义的结构体
					toolCalls = append(toolCalls, map[string]interface{}{
						"id":   tc.ID,
						"type": "function",
						"function": map[string]interface{}{
							"name":      tc.Name,
							"arguments": string(tc.Args),
						},
					})
				}
				m["tool_calls"] = toolCalls
			}
			result = append(result, m)

		case "tool":
			result = append(result, map[string]interface{}{
				"role":         "tool",
				"content":      msg.Content,    // Content: Message 结构体字段（provider.go）
				"tool_call_id": msg.ToolCallID, // ToolCallID: Message 结构体字段（provider.go）
			})
		}
	}
	return result
}
