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
func (p *AnthropicProvider) StreamChat(ctx context.Context, messages []Message, thinking bool) (Stream, error) {
	// 构造请求体
	reqBody := map[string]interface{}{
		"model":      p.config.Model,
		"messages":   messages,
		"stream":     true,

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
}

// Next 获取下一个数据块
func (s *anthropicStream) Next() (*StreamChunk, error) {
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
				return &StreamChunk{Type: StreamChunkTypeDone}, nil
			}

			// 解析 JSON
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			// 处理不同类型的事件
			eventType, _ := event["type"].(string)

			switch eventType {
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
						Type:    StreamChunkTypeText,
						Content: text,
					}, nil
				case "thinking_delta":
					thinking, _ := delta["thinking"].(string)
					return &StreamChunk{
						Type:    StreamChunkTypeThinking,
						Content: thinking,
					}, nil
				}

			case "message_stop":
				return &StreamChunk{Type: StreamChunkTypeDone}, nil
			}
		}
	}

	if err := s.scanner.Err(); err != nil {
		return nil, fmt.Errorf("stream error: %w", err)
	}

	return &StreamChunk{Type: StreamChunkTypeDone}, nil
}

// Close 关闭流
func (s *anthropicStream) Close() error {
	if s.body != nil {
		return s.body.Close()
	}
	return nil
}
