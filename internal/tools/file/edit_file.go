package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"mewcode/internal/tools"
)

// EditFileTool 编辑文件，支持行号精准模式和文本匹配模式
type EditFileTool struct{}

// NewEditFileTool 创建 EditFile 工具实例
func NewEditFileTool() *EditFileTool {
	return &EditFileTool{}
}

func (t *EditFileTool) Name() string { return "edit_file" }
func (t *EditFileTool) Description() string {
	return "编辑文件内容。支持两种模式：行号模式（提供 start_line/end_line 精准定位）和文本匹配模式（唯一匹配替换）。"
}
func (t *EditFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "文件路径"},
			"old_text": {"type": "string", "description": "要替换的原文内容（文本匹配模式必填，行号模式可省略）"},
			"new_text": {"type": "string", "description": "替换后的新内容"},
			"start_line": {"type": "integer", "description": "起始行号（1-based，行号模式使用）"},
			"end_line": {"type": "integer", "description": "结束行号（1-based，行号模式使用）"}
		},
		"required": ["path", "new_text"]
	}`)
}
func (t *EditFileTool) Meta() tools.ToolMeta { // ToolMeta: tool.go 中定义的结构体
	return tools.ToolMeta{Category: "file", ReadOnly: false, Destructive: false}
}

// Execute 执行编辑文件操作
func (t *EditFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Path      string `json:"path"`
		OldText   string `json:"old_text"`
		NewText   string `json:"new_text"`
		StartLine *int   `json:"start_line"`
		EndLine   *int   `json:"end_line"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	// 读取文件
	content, err := os.ReadFile(params.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	original := string(content)
	var newContent string

	if params.StartLine != nil && params.EndLine != nil {
		// 行号模式
		newContent, err = editByLine(original, *params.StartLine, *params.EndLine, params.NewText)
		if err != nil {
			return err.Error(), nil // 返回结构化错误而非 Go error
		}
	} else {
		// 文本匹配模式
		if params.OldText == "" {
			return "", fmt.Errorf("old_text is required for text matching mode")
		}
		newContent, err = editByMatch(original, params.OldText, params.NewText)
		if err != nil {
			return err.Error(), nil // 返回结构化错误而非 Go error
		}
	}

	// 获取原文件权限并写回
	info, err := os.Stat(params.Path)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}
	if err := os.WriteFile(params.Path, []byte(newContent), info.Mode()); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return "[编辑成功]", nil
}

// editByLine 行号模式编辑：提取 startLine 到 endLine 范围的行并替换
func editByLine(content string, startLine, endLine int, newText string) (string, error) {
	lines := strings.Split(content, "\n")

	if startLine < 1 || startLine > len(lines) {
		return "", fmt.Errorf("start_line %d 超出范围（文件共 %d 行）", startLine, len(lines))
	}
	if endLine < startLine || endLine > len(lines) {
		return "", fmt.Errorf("end_line %d 超出范围（start_line=%d, 文件共 %d 行）", endLine, startLine, len(lines))
	}

	// 构建新内容：前段 + 替换 + 后段
	startIdx := startLine - 1 // 转 0-based
	endIdx := endLine         // endLine 是 inclusive，所以 endIdx = endLine

	var result []string
	result = append(result, lines[:startIdx]...)
	result = append(result, newText)
	result = append(result, lines[endIdx:]...)

	return strings.Join(result, "\n"), nil
}

// editByMatch 文本匹配模式编辑：唯一匹配后替换
func editByMatch(content, oldText, newText string) (string, error) {
	count := strings.Count(content, oldText)

	switch {
	case count == 0:
		return "", fmt.Errorf("未找到匹配：所提供的文本在文件中不存在")
	case count > 1:
		return "", fmt.Errorf("找到 %d 处匹配，请提供 start_line/end_line 或更多上下文以唯一定位", count)
	default:
		return strings.Replace(content, oldText, newText, 1), nil
	}
}
