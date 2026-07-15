package search

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"mewcode/internal/tools"
)

// GlobTool 按 glob 模式查找文件
type GlobTool struct{}

// NewGlobTool 创建 Glob 工具实例
func NewGlobTool() *GlobTool {
	return &GlobTool{}
}

func (t *GlobTool) Name() string { return "glob" }
func (t *GlobTool) Description() string {
	return "按 glob 模式查找文件，返回匹配的文件路径列表。"
}
func (t *GlobTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "glob 匹配模式，如 *.go、**/*.txt"}
		},
		"required": ["pattern"]
	}`)
}
func (t *GlobTool) Meta() tools.ToolMeta { // ToolMeta: tool.go 中定义的结构体
	return tools.ToolMeta{Category: "search", ReadOnly: true, Destructive: false}
}

// Execute 执行 glob 查找
func (t *GlobTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if params.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	matches, err := filepath.Glob(params.Pattern)
	if err != nil {
		return "", fmt.Errorf("invalid glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return "未找到匹配文件", nil
	}

	return strings.Join(matches, "\n"), nil
}
