package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"mewcode/internal/agent"
	"mewcode/internal/config"
	"mewcode/internal/conversation"
	"mewcode/internal/provider"
	"mewcode/internal/tools"
	"mewcode/internal/tools/file"
	"mewcode/internal/tools/search"
	"mewcode/internal/tools/shell"
	"mewcode/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// 加载配置文件
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// 获取当前活跃的 provider 配置
	var activeCfg config.ProviderConfig
	found := false
	for _, p := range cfg.Providers {
		if p.Name == cfg.ActiveProvider {
			activeCfg = p
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "Active provider '%s' not found\n", cfg.ActiveProvider)
		os.Exit(1)
	}

	// 创建 provider
	p, err := provider.NewProvider(activeCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating provider: %v\n", err)
		os.Exit(1)
	}

	// 创建工具注册中心
	reg := tools.NewRegistry()            // tools.NewRegistry: registry.go 中定义的函数
	reg.Register(file.NewReadFileTool())  // file.NewReadFileTool: read_file.go 中定义的函数
	reg.Register(file.NewWriteFileTool()) // file.NewWriteFileTool: write_file.go 中定义的函数
	reg.Register(file.NewEditFileTool())  // file.NewEditFileTool: edit_file.go 中定义的函数
	reg.Register(shell.NewBashTool())     // shell.NewBashTool: exec_command.go 中定义的函数
	reg.Register(search.NewGlobTool())    // search.NewGlobTool: glob_search.go 中定义的函数
	reg.Register(search.NewGrepTool())    // search.NewGrepTool: grep_search.go 中定义的函数

	// 创建工具执行器
	exec := tools.NewExecutor(reg, 30*time.Second) // tools.NewExecutor: executor.go 中定义的函数

	// 创建 conversation
	thinking := activeCfg.Thinking != nil && *activeCfg.Thinking
	conv := conversation.NewConversation(p, thinking)

	// 创建 Agent 编排层
	ag := agent.NewAgent(conv, reg, exec, p, thinking, 25, 5) // agent.NewAgent: agent.go 中定义的函数，maxIter=25，maxParallel=5

	// 创建并运行 TUI
	model := tui.NewModel(ag) // tui.NewModel: model.go 中定义的函数
	program := tea.NewProgram(&model, tea.WithAltScreen())

	// 设置 program 引用（必须在 NewProgram 之后调用，因为使用了指针，两者共享同一实例）
	model.SetProgram(program)

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		debug.PrintStack()
		os.Exit(1)
	}
}
