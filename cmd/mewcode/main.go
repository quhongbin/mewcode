package main

import (
	"fmt"
	"os"
	"runtime/debug"
	

	tea "github.com/charmbracelet/bubbletea"
	"mewcode/internal/config"
	"mewcode/internal/conversation"
	"mewcode/internal/provider"
	"mewcode/internal/tui"
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

	// 创建 conversation
	thinking := activeCfg.Thinking != nil && *activeCfg.Thinking
	conv := conversation.NewConversation(p, thinking)

	// 创建并运行 TUI
	model := tui.NewModel(conv)
	program := tea.NewProgram(&model, tea.WithAltScreen())

	// 设置 program 引用（必须在 NewProgram 之后调用，因为使用了指针，两者共享同一实例）
	model.SetProgram(program)

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		debug.PrintStack()
		os.Exit(1)
	}
}
