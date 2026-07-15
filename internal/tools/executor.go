package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Executor 工具执行器，封装按名查找、超时控制和错误包装
type Executor struct {
	registry *Registry // registry: registry.go 中定义的结构体
	timeout  time.Duration
}

// NewExecutor 创建工具执行器实例
func NewExecutor(registry *Registry, timeout time.Duration) *Executor { // Registry: registry.go 中定义的结构体
	return &Executor{
		registry: registry,
		timeout:  timeout,
	}
}

// Execute 按名称查找工具并执行，返回结构化结果
// 查找失败、超时、执行错误均返回 ToolResult{IsError: true}，不会 panic
func (e *Executor) Execute(ctx context.Context, name string, args json.RawMessage) ToolResult { // ToolResult: tool.go 中定义的结构体
	// 查找工具
	tool, ok := e.registry.Get(name) // registry: 本结构体字段，Get: registry.go 中定义的方法
	if !ok {
		return ToolResult{
			IsError: true,
			Content: fmt.Sprintf("tool not found: %s", name),
		}
	}

	// 创建超时 context
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// 执行工具
	output, err := tool.Execute(ctx, args)
	if err != nil {
		return ToolResult{
			IsError: true,
			Content: err.Error(),
		}
	}

	return ToolResult{
		Content: output,
	}
}
