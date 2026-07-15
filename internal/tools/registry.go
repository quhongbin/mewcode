package tools

import "encoding/json"

// ToolDefinition 工具的 API 格式定义，用于发送给 LLM API
type ToolDefinition struct {
	Name        string          `json:"name"`         // 工具名称
	Description string          `json:"description"`  // 工具描述
	InputSchema json.RawMessage `json:"input_schema"` // JSON Schema 参数定义
}

// Registry 工具注册中心，集中管理所有已注册的工具
type Registry struct {
	tools map[string]Tool // tools: tool.go 中定义的接口
}

// NewRegistry 创建一个新的空注册中心
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register 注册一个工具到注册中心
func (r *Registry) Register(t Tool) { // Tool: tool.go 中定义的接口
	r.tools[t.Name()] = t
}

// Get 按名称查找已注册的工具
func (r *Registry) Get(name string) (Tool, bool) { // Tool: tool.go 中定义的接口
	t, ok := r.tools[name]
	return t, ok
}

// Has 检查指定名称的工具是否已注册
func (r *Registry) Has(name string) bool {
	_, ok := r.tools[name]
	return ok
}

// List 返回所有已注册的工具列表
func (r *Registry) List() []Tool { // Tool: tool.go 中定义的接口
	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

// Definitions 将所有已注册工具转为 LLM API 所需的 ToolDefinition 列表
func (r *Registry) Definitions() []ToolDefinition { // ToolDefinition: 本文件中定义的结构体
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.Parameters(),
		})
	}
	return defs
}
