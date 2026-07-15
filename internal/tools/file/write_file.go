package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"mewcode/internal/tools"
)

// WriteFileTool 写入文件，自动创建不存在的父目录
type WriteFileTool struct{}

// NewWriteFileTool 创建 WriteFile 工具实例
func NewWriteFileTool() *WriteFileTool {
	return &WriteFileTool{}
}

func (t *WriteFileTool) Name() string { return "write_file" }
func (t *WriteFileTool) Description() string {
	return "写入文件内容。若父目录不存在则自动创建。"
}
func (t *WriteFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "文件路径"},
			"content": {"type": "string", "description": "要写入的文件内容"}
		},
		"required": ["path", "content"]
	}`)
}
func (t *WriteFileTool) Meta() tools.ToolMeta { // ToolMeta: tool.go 中定义的结构体
	return tools.ToolMeta{Category: "file", ReadOnly: false, Destructive: false}
}

// Execute 执行写文件操作
func (t *WriteFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	// 递归创建父目录
	dir := filepath.Dir(params.Path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directories: %w", err)
		}
	}

	// 写入文件
	data := []byte(params.Content)
	if err := os.WriteFile(params.Path, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("[成功写入 %d 字节到 %s]", len(data), params.Path), nil
}
