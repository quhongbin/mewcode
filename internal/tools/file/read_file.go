package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"mewcode/internal/tools"
)

// ReadFileTool 读取文件内容，支持行号前缀、offset/limit 和二进制检测
type ReadFileTool struct{}

// NewReadFileTool 创建 ReadFile 工具实例
func NewReadFileTool() *ReadFileTool {
	return &ReadFileTool{}
}

func (t *ReadFileTool) Name() string { return "read_file" }
func (t *ReadFileTool) Description() string {
	return "读取文件内容，返回带行号前缀的文本。支持 offset/limit 分段读取，自动检测并拒绝二进制文件。"
}
func (t *ReadFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "文件路径"},
			"offset": {"type": "integer", "description": "从第几行开始读取（1-based，默认1）"},
			"limit": {"type": "integer", "description": "最多读取几行（默认全部）"}
		},
		"required": ["path"]
	}`)
}
func (t *ReadFileTool) Meta() tools.ToolMeta { // ToolMeta: tool.go 中定义的结构体
	return tools.ToolMeta{Category: "file", ReadOnly: true, Destructive: false}
}

// Execute 执行读文件操作
func (t *ReadFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Path   string `json:"path"`
		Offset *int   `json:"offset"`
		Limit  *int   `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	// 打开文件
	f, err := os.Open(params.Path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	// 二进制检测：读取前 512 字节检查 NUL 字符
	header := make([]byte, 512)
	n, _ := f.Read(header)
	for i := 0; i < n; i++ {
		if header[i] == 0 {
			return "该文件为二进制文件，请使用命令行工具（如 file、xxd）处理", nil
		}
	}

	// 回到文件开头读取全部内容
	f.Seek(0, 0)
	data, err := readAll(f)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// 按行分割
	lines := strings.Split(string(data), "\n")

	// 应用 offset 和 limit
	offset := 1
	if params.Offset != nil && *params.Offset > 0 {
		offset = *params.Offset
	}
	limit := len(lines)
	if params.Limit != nil && *params.Limit > 0 {
		limit = *params.Limit
	}

	startIdx := offset - 1 // 转为 0-based
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx >= len(lines) {
		return fmt.Sprintf("offset %d 超出文件行数（共 %d 行）", offset, len(lines)), nil
	}

	endIdx := startIdx + limit
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	// 格式化带行号的输出
	var b strings.Builder
	for i := startIdx; i < endIdx; i++ {
		lineNum := i + 1 // 1-based 行号
		fmt.Fprintf(&b, "%4d│%s\n", lineNum, lines[i])
	}

	return b.String(), nil
}

// readAll 读取全部内容（兼容 io.Reader）
func readAll(r *os.File) ([]byte, error) {
	info, err := r.Stat()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, info.Size())
	_, err = r.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf, nil
}
