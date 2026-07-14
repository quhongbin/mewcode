package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"mewcode/internal/provider"
)

// Update 处理事件
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 6 // 预留空间给输入框和状态
		// 输入框样式有 border(2) + padding(2) = 4 字符额外宽度，需要扣除
		m.textarea.SetWidth(msg.Width - 4)
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		// 处理退出
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// 处理 Enter 提交（仅在非流式状态）
		if msg.String() == "enter" && m.status == statusIdle {
			value := strings.TrimSpace(m.textarea.Value())
			if value == "" {
				return m, nil
			}

			// 添加用户消息到显示
			m.messages = append(m.messages, DisplayMessage{
				Role:    "user",
				Content: value,
			})

			// 清空输入框
			m.textarea.Reset()

			// 开始流式响应
			m.status = statusStreaming
			m.timerStart = time.Now()
			m.elapsed = 0
			m.err = nil
			m.currentContent = ""
			m.currentThinking = ""

			// 启动流式处理
			return m, m.startStream(value)
		}

		// 其他按键传递给 textarea
		if m.status == statusIdle {
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tickMsg:
		if m.status == statusStreaming {
			m.elapsed = time.Since(m.timerStart)
			cmds = append(cmds, m.tick())
		}
		return m, tea.Batch(cmds...)

	case streamChunkMsg:
		if msg.chunk.Type == provider.StreamChunkTypeText {
			m.currentContent += msg.chunk.Content
		} else if msg.chunk.Type == provider.StreamChunkTypeThinking {
			m.currentThinking += msg.chunk.Content
		}
		return m, nil

	case streamDoneMsg:
		// 添加助手消息到历史
		m.messages = append(m.messages, DisplayMessage{
			Role:     "assistant",
			Content:  msg.content,
			Thinking: msg.thinking,
		})

		// 更新对话历史
		m.conversation.AddAssistantMessage(msg.content)

		// 重置状态
		m.status = statusIdle
		m.currentContent = ""
		m.currentThinking = ""
		m.err = nil
		return m, nil

	case streamErrMsg:
		m.err = msg.err
		m.status = statusIdle
		m.currentContent = ""
		m.currentThinking = ""
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

// startStream 启动流式响应处理
func (m Model) startStream(userInput string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		stream, err := m.conversation.SendMessage(ctx, userInput)
		if err != nil {
			return streamErrMsg{err: err}
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
			m.program.Send(streamChunkMsg{chunk: chunk})

			if chunk.Type == provider.StreamChunkTypeText {
				content += chunk.Content
			} else if chunk.Type == provider.StreamChunkTypeThinking {
				thinking += chunk.Content
			}
		}

		return streamDoneMsg{content: content, thinking: thinking}
	}
}
