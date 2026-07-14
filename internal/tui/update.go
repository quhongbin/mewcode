package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"mewcode/internal/provider"
)

// Update 处理事件
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { // Model: model.go 中定义的类型
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width      // width: Model 结构体字段（model.go）
		m.height = msg.Height    // height: Model 结构体字段（model.go）
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

			// 开始流式响应
			m.status = statusStreaming        // status: Model 结构体字段（model.go），statusStreaming: model.go 中定义的常量
			m.timerStart = time.Now()         // timerStart: Model 结构体字段（model.go）
			m.elapsed = 0                     // elapsed: Model 结构体字段（model.go）
			m.err = nil                       // err: Model 结构体字段（model.go）
			m.currentContent = ""             // currentContent: Model 结构体字段（model.go）
			m.currentThinking = ""            // currentThinking: Model 结构体字段（model.go）

			// 启动流式处理
			return m, m.startStream(value)    // startStream: update.go 中定义的函数
		}

		// 其他按键传递给 textarea
		if m.status == statusIdle { // status: Model 结构体字段（model.go），statusIdle: model.go 中定义的常量
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tickMsg: // tickMsg: model.go 中定义的类型
		if m.status == statusStreaming { // status: Model 结构体字段（model.go），statusStreaming: model.go 中定义的常量
			m.elapsed = time.Since(m.timerStart) // elapsed: Model 结构体字段（model.go），timerStart: Model 结构体字段（model.go）
			cmds = append(cmds, m.tick())        // tick: model.go 中定义的函数
		}
		return m, tea.Batch(cmds...)

	case streamChunkMsg: // streamChunkMsg: model.go 中定义的类型
		if msg.chunk.Type == provider.StreamChunkTypeText {
			m.currentContent += msg.chunk.Content // currentContent: Model 结构体字段（model.go）
		} else if msg.chunk.Type == provider.StreamChunkTypeThinking {
			m.currentThinking += msg.chunk.Content // currentThinking: Model 结构体字段（model.go）
		}
		return m, nil

	case streamDoneMsg: // streamDoneMsg: model.go 中定义的类型
		// 添加助手消息到历史
		m.messages = append(m.messages, DisplayMessage{ // messages: Model 结构体字段（model.go），DisplayMessage: model.go 中定义的结构体
			Role:     "assistant",
			Content:  msg.content,
			Thinking: msg.thinking,
		})

		// 更新对话历史
		m.conversation.AddAssistantMessage(msg.content) // conversation: Model 结构体字段（model.go）

		// 重置状态
		m.status = statusIdle        // status: Model 结构体字段（model.go），statusIdle: model.go 中定义的常量
		m.currentContent = ""        // currentContent: Model 结构体字段（model.go）
		m.currentThinking = ""       // currentThinking: Model 结构体字段（model.go）
		m.err = nil                  // err: Model 结构体字段（model.go）
		return m, nil

	case streamErrMsg: // streamErrMsg: model.go 中定义的类型
		m.err = msg.err           // err: Model 结构体字段（model.go）
		m.status = statusIdle     // status: Model 结构体字段（model.go），statusIdle: model.go 中定义的常量
		m.currentContent = ""     // currentContent: Model 结构体字段（model.go）
		m.currentThinking = ""    // currentThinking: Model 结构体字段（model.go）
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

// startStream 启动流式响应处理
func (m Model) startStream(userInput string) tea.Cmd { // Model: model.go 中定义的类型
	return func() tea.Msg {
		ctx := context.Background()
		stream, err := m.conversation.SendMessage(ctx, userInput) // conversation: Model 结构体字段（model.go）
		if err != nil {
			return streamErrMsg{err: err} // streamErrMsg: model.go 中定义的类型
		}
		defer stream.Close()

		var content, thinking string
		for {
			chunk, err := stream.Next()
			if err != nil {
				return streamErrMsg{err: err}
			}

			if chunk.Type == provider.StreamChunkTypeDone {
				break
			}

			// 发送数据块到 UI
			m.program.Send(streamChunkMsg{chunk: chunk}) // program: Model 结构体字段（model.go），streamChunkMsg: model.go 中定义的类型

			if chunk.Type == provider.StreamChunkTypeText {
				content += chunk.Content
			} else if chunk.Type == provider.StreamChunkTypeThinking {
				thinking += chunk.Content
			}
		}

		return streamDoneMsg{content: content, thinking: thinking} // streamDoneMsg: model.go 中定义的类型
	}
}
