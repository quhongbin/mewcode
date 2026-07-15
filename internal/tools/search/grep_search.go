package search

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"mewcode/internal/tools"
)

// GrepTool 按正则表达式搜索代码内容
type GrepTool struct{}

// NewGrepTool 创建 Grep 工具实例
func NewGrepTool() *GrepTool {
	return &GrepTool{}
}

func (t *GrepTool) Name() string { return "grep" }
func (t *GrepTool) Description() string {
	return "按正则表达式搜索文件内容，返回匹配行及上下文。"
}
func (t *GrepTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "正则表达式搜索模式"},
			"path": {"type": "string", "description": "搜索路径（文件或目录，默认当前目录）"}
		},
		"required": ["pattern"]
	}`)
}
func (t *GrepTool) Meta() tools.ToolMeta { // ToolMeta: tool.go 中定义的结构体
	return tools.ToolMeta{Category: "search", ReadOnly: true, Destructive: false}
}

// grepMatch 一次匹配结果
type grepMatch struct {
	file    string
	lineNum int
	line    string
}

// Execute 执行 grep 搜索
func (t *GrepTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if params.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	if params.Path == "" {
		params.Path = "."
	}

	// 编译正则
	re, err := regexp.Compile(params.Pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %w", err)
	}

	// 搜索
	var matches []grepMatch
	err = filepath.Walk(params.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}
		// 跳过隐藏目录
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}
		// 跳过目录
		if info.IsDir() {
			return nil
		}
		// 检查 context 取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 搜索文件内容
		fileMatches, err := searchFile(path, re)
		if err != nil {
			return nil // 跳过无法读取的文件
		}
		matches = append(matches, fileMatches...)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk error: %w", err)
	}

	if len(matches) == 0 {
		return "未找到匹配内容", nil
	}

	// 格式化输出
	return formatMatches(matches), nil
}

// searchFile 在单个文件中搜索正则
func searchFile(path string, re *regexp.Regexp) ([]grepMatch, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// 快速检测二进制文件
	header := make([]byte, 512)
	n, _ := f.Read(header)
	for i := 0; i < n; i++ {
		if header[i] == 0 {
			return nil, fmt.Errorf("binary file") // 跳过二进制文件
		}
	}
	f.Seek(0, 0)

	var matches []grepMatch
	scanner := bufio.NewScanner(f)
	lineNum := 0
	var lines []string

	// 先读取所有行以支持上下文
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	for i, line := range lines {
		lineNum = i + 1
		if re.MatchString(line) {
			matches = append(matches, grepMatch{
				file:    path,
				lineNum: lineNum,
				line:    line,
			})
		}
	}

	return matches, nil
}

// formatMatches 格式化匹配结果，包含上下文
func formatMatches(matches []grepMatch) string {
	const contextLines = 2 // 前后各 2 行上下文

	var b strings.Builder
	lastFile := ""
	lastLine := -contextLines - 1

	for _, m := range matches {
		// 文件切换时显示文件名
		if m.file != lastFile {
			if lastFile != "" {
				b.WriteString("\n")
			}
			b.WriteString(fmt.Sprintf("--- %s ---\n", m.file))
			lastFile = m.file
			lastLine = -contextLines - 1
		}

		// 读取文件以获取上下文行
		lines := readFileLines(m.file)

		// 上下文范围
		start := m.lineNum - 1 - contextLines
		if start < 0 {
			start = 0
		}
		end := m.lineNum - 1 + contextLines
		if end >= len(lines) {
			end = len(lines) - 1
		}

		// 如果与上一个匹配有间隔，打印分隔符
		if start > lastLine+1 && lastLine >= 0 {
			b.WriteString("  ...\n")
		}

		for i := start; i <= end; i++ {
			if i+1 == m.lineNum {
				// 匹配行加标记
				fmt.Fprintf(&b, ">> %4d│%s\n", i+1, lines[i])
			} else {
				fmt.Fprintf(&b, "   %4d│%s\n", i+1, lines[i])
			}
		}

		lastLine = end
	}

	return b.String()
}

// readFileLines 读取文件所有行（用于上下文展示）
func readFileLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return strings.Split(string(data), "\n")
}
