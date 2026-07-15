package tools

import (
	"context"
	"encoding/json"
)

// ToolMeta 工具元数据，描述工具的分类和行为属性
type ToolMeta struct {
	Category    string // 工具分类：file, shell, search
	ReadOnly    bool   // 是否为只读操作
	Destructive bool   // 是否可能造成不可逆变更
}

// ToolCall 表示模型发起的工具调用请求
type ToolCall struct {
	ID   string          `json:"id"`   // 唯一标识
	Name string          `json:"name"` // 工具名称
	Args json.RawMessage `json:"args"` // JSON 格式的参数
}

// ToolResult 工具执行结果
type ToolResult struct {
	IsError bool   // 是否执行失败
	Content string // 成功时为输出内容，失败时为错误描述
}

// Tool 统一工具接口，所有工具必须实现此接口
type Tool interface {
	// Name 返回工具的唯一标识名称
	Name() string
	// Description 返回工具的人类可读描述
	Description() string
	// Parameters 返回工具参数的 JSON Schema 定义
	Parameters() json.RawMessage
	// Meta 返回工具的元数据
	Meta() ToolMeta
	// Execute 执行工具，args 为 JSON 格式的参数，返回文本结果或错误
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}
