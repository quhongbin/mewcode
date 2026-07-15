package agent

import (
	"mewcode/internal/provider"
	"mewcode/internal/tools"
)

// toolGroup 工具执行分组，同一组内的工具具有相同的 ReadOnly 属性
type toolGroup struct {
	calls    []provider.ToolCall // provider.ToolCall: provider.go 中定义的结构体
	readOnly bool                // 是否为只读组
}

// batchTools 将工具调用列表按 ReadOnly 属性和并行上限分组
// 相邻的只读工具合并为一组，非只读工具各自单独成组
// 只读组超过 maxParallel 时按上限拆分为子组
func batchTools(calls []provider.ToolCall, registry *tools.Registry, maxParallel int) []toolGroup { // provider.ToolCall: provider.go 中定义的结构体，tools.Registry: registry.go 中定义的结构体
	if len(calls) == 0 {
		return nil
	}

	// 第一轮：按相邻 ReadOnly 属性合并分组
	var groups []toolGroup
	i := 0
	for i < len(calls) {
		readOnly := isReadOnly(calls[i].Name, registry) // registry: 参数，Registry.Get: registry.go 中定义的方法
		if readOnly {
			// 收集相邻的只读工具
			j := i + 1
			for j < len(calls) && isReadOnly(calls[j].Name, registry) {
				j++
			}
			groups = append(groups, toolGroup{calls: calls[i:j], readOnly: true})
			i = j
		} else {
			// 非只读工具单独成组
			groups = append(groups, toolGroup{calls: []provider.ToolCall{calls[i]}, readOnly: false})
			i++
		}
	}

	// 第二轮：对只读组按 maxParallel 拆分
	if maxParallel <= 0 {
		maxParallel = 1
	}
	var result []toolGroup
	for _, g := range groups {
		if g.readOnly && len(g.calls) > maxParallel {
			// 拆分子组
			for k := 0; k < len(g.calls); k += maxParallel {
				end := k + maxParallel
				if end > len(g.calls) {
					end = len(g.calls)
				}
				result = append(result, toolGroup{calls: g.calls[k:end], readOnly: true})
			}
		} else {
			result = append(result, g)
		}
	}

	return result
}

// isReadOnly 判断指定名称的工具是否为只读
// 如果工具未注册，保守返回 false（视为有副作用）
func isReadOnly(name string, registry *tools.Registry) bool { // tools.Registry: registry.go 中定义的结构体
	tool, ok := registry.Get(name) // Registry.Get: registry.go 中定义的方法
	if !ok {
		return false
	}
	return tool.Meta().ReadOnly // Tool.Meta: tool.go 中定义的方法，ToolMeta.ReadOnly: tool.go 中定义的字段
}
