package tui

import (
	"context"
	"strings"
	"time"

	"mewcode/internal/agent"

	tea "github.com/charmbracelet/bubbletea"
)

// Update 处理事件
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { // Model: model.go 中定义的类型
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width   // width: Model 结构体字段（model.go）
		m.height = msg.Height // height: Model 结构体字段（model.go）
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 6 // 预留空间给输入框和状态
		// 输入框样式有 border(2) + padding(2) = 4 字符额外宽度，需要扣除
		m.textarea.SetWidth(msg.Width - 4)
		m.ready = true // ready: Model 结构体字段（model.go）
		return m, nil

	case tea.KeyMsg:
		// 处理退出
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// 处理 Enter 提交（仅在非流式状态）
		if msg.String() == "enter" && m.status == statusIdle { // status: Model 结构体字段（model.go），statusIdle: model.go 中定义的常量
			value := strings.TrimSpace(m.textarea.Value())
			if value == "" {
				return m, nil
			}

			// 添加用户消息到显示
			m.messages = append(m.messages, DisplayMessage{ // messages: Model 结构体字段（model.go），DisplayMessage: model.go 中定义的结构体
				Role:    "user",
				Content: value,
			})

			// 清空输入框
			m.textarea.Reset()

			// 处理 /do 命令
			if value == "/do" {
				m.agent.SetMode(agent.ModeAct) // agent.Agent.SetMode: agent.go 中定义的方法，agent.ModeAct: agent.go 中定义的常量
				m.messages = append(m.messages, DisplayMessage{
					Role:    "system",
					Content: "已切换到 Act 模式，现在可以使用全部工具。",
				})
				return m, nil
			}

			// 开始流式响应
			m.status = statusStreaming // status: Model 结构体字段（model.go），statusStreaming: model.go 中定义的常量
			m.timerStart = time.Now()  // timerStart: Model 结构体字段（model.go）
			m.elapsed = 0              // elapsed: Model 结构体字段（model.go）
			m.err = nil                // err: Model 结构体字段（model.go）
			m.currentContent = ""      // currentContent: Model 结构体字段（model.go）
			m.currentThinking = ""     // currentThinking: Model 结构体字段（model.go）
			m.currentIteration = 0     // currentIteration: Model 结构体字段（model.go）

			// 启动流式响应，同时启动计时器 tick 链（确保计时器从流式开始就有活跃的 tick）
			m.tickCounter++                                     // 递增计数器，使旧的 tick 链失效
			return m, tea.Batch(m.startStream(value), m.tick()) // startStream: update.go 中定义的函数
		}

		// 其他按键传递给 textarea
		if m.status == statusIdle { // status: Model 结构体字段（model.go），statusIdle: model.go 中定义的常量
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tickMsg: // tickMsg: model.go 中定义的类型
		// 仅处理当前活跃的 tick（过滤掉过期的 tick，防止多个并发 tick 链）
		if msg.counter == m.tickCounter && m.status == statusStreaming { // status: Model 结构体字段（model.go），statusStreaming: model.go 中定义的常量
			m.elapsed = time.Since(m.timerStart) // elapsed: Model 结构体字段（model.go），timerStart: Model 结构体字段（model.go）
			// 如果有工具正在执行，推进 spinner 动画帧
			if m.hasExecutingTool() {
				m.spinnerFrame++
			}
			return m, m.tick() // 继续 tick 链：安排下一个 tick
		}
		return m, nil

	case agentEventMsg: // agentEventMsg: model.go 中定义的类型
		switch msg.event.Type {
		case agent.EventText: // agent.EventText: event.go 中定义的常量
			m.currentContent += msg.event.Content // currentContent: Model 结构体字段（model.go）

		case agent.EventThinking: // agent.EventThinking: event.go 中定义的常量
			m.currentThinking += msg.event.Content // currentThinking: Model 结构体字段（model.go）

		case agent.EventToolCall: // agent.EventToolCall: event.go 中定义的常量
			if msg.event.ToolCall != nil {
				m.messages = append(m.messages, DisplayMessage{
					Role:        "tool_call",
					ToolCall:    msg.event.ToolCall,
					Iteration:   m.currentIteration,
					IsExecuting: true, // 工具刚调用，标记为执行中
				})
			}

		case agent.EventToolResult: // agent.EventToolResult: event.go 中定义的常量
			if msg.event.ToolResult != nil {
				// 找到对应的正在执行的工具调用，标记为已完成
				m.markToolDone(msg.event.ToolResult.Name)
				m.messages = append(m.messages, DisplayMessage{
					Role:       "tool_result",
					ToolResult: &msg.event.ToolResult.Result,
					IsError:    msg.event.ToolResult.Result.IsError,
				})
			}

		case agent.EventIteration: // agent.EventIteration: event.go 中定义的常量
			m.currentIteration = msg.event.Iteration // currentIteration: Model 结构体字段（model.go）

		case agent.EventCommit: // agent.EventCommit: event.go 中定义的常量
			// 提交当前轮文本到消息历史（在工具调用之前，确保渲染顺序正确）
			if m.currentContent != "" || m.currentThinking != "" {
				m.messages = append(m.messages, DisplayMessage{
					Role:     "assistant",
					Content:  m.currentContent,
					Thinking: m.currentThinking,
				})
				m.currentContent = ""  // currentContent: Model 结构体字段（model.go）
				m.currentThinking = "" // currentThinking: Model 结构体字段（model.go）
			}

		case agent.EventDone: // agent.EventDone: event.go 中定义的常量
			// 添加最终助手消息到历史（仅在未被 EventCommit 提交的情况下）
			if m.currentContent != "" || m.currentThinking != "" {
				m.messages = append(m.messages, DisplayMessage{
					Role:     "assistant",
					Content:  m.currentContent,
					Thinking: m.currentThinking,
				})
			}

			// 重置状态
			m.status = statusIdle  // status: Model 结构体字段（model.go），statusIdle: model.go 中定义的常量
			m.currentContent = ""  // currentContent: Model 结构体字段（model.go）
			m.currentThinking = "" // currentThinking: Model 结构体字段（model.go）
			m.currentIteration = 0 // currentIteration: Model 结构体字段（model.go）
			m.err = nil            // err: Model 结构体字段（model.go）
		}

		// 流式期间防御性重启 tick 链：确保计时器不会因为 tick 链意外断裂而停止更新
		if m.status == statusStreaming {
			return m, m.tick()
		}
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

// startStream 启动流式响应处理，消费 Agent 返回的 EventStream
// 立即返回，后台 goroutine 通过 program.Send 发送事件，不阻塞 Bubble Tea 消息循环
func (m Model) startStream(userInput string) tea.Cmd { // Model: model.go 中定义的类型
	return func() tea.Msg {
		ctx := context.Background()
		eventStream := m.agent.SendMessage(ctx, userInput) // agent.Agent.SendMessage: agent.go 中定义的方法

		// 启动独立 goroutine 消费事件流，不阻塞 Bubble Tea 命令处理
		go func() {
			for {
				event, ok := eventStream.Next() // EventStream.Next: event.go 中定义的方法
				if !ok {
					return
				}
				m.program.Send(agentEventMsg{event: event}) // program: Model 结构体字段（model.go），agentEventMsg: model.go 中定义的类型
			}
		}()

		return nil // 立即返回，让 Bubble Tea 继续处理消息
	}
}
