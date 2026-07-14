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
func (p *OpenAIProvider) StreamChat(ctx context.Context, messages []Message, thinking bool) (Stream, error) {
	// 构造请求体
	reqBody := map[string]interface{}{
		"model":    p.config.Model,
		"messages": messages,
		"stream":   true,
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
		scanner: scanner,
		body:    resp.Body,
	}, nil
}

// openaiStream 实现 OpenAI 的流式响应
type openaiStream struct {
	scanner *bufio.Scanner
	body    io.ReadCloser
}

// Next 获取下一个数据块
func (s *openaiStream) Next() (*StreamChunk, error) {
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
				return &StreamChunk{Type: StreamChunkTypeDone}, nil
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

			content, ok := delta["content"].(string)
			if ok && content != "" {
				return &StreamChunk{
					Type:    StreamChunkTypeText,
					Content: content,
				}, nil
			}
		}
	}

	if err := s.scanner.Err(); err != nil {
		return nil, fmt.Errorf("stream error: %w", err)
	}

	return &StreamChunk{Type: StreamChunkTypeDone}, nil
}

// Close 关闭流
func (s *openaiStream) Close() error {
	if s.body != nil {
		return s.body.Close()
	}
	return nil
}
