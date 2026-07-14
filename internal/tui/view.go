package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View 渲染界面
func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	var b strings.Builder

	// 用于文本换行的样式，限制最大宽度为终端宽度
	wrapStyle := lipgloss.NewStyle().MaxWidth(m.width)

	// 渲染欢迎信息（仅在没有消息时显示）
	if len(m.messages) == 0 {
		b.WriteString(welcomeStyle.Render(welcomeMessage))
		b.WriteString("\n")
	}

	// 渲染对话历史
	for _, msg := range m.messages {
		switch {
		case msg.IsError:
			b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %s", msg.Content)))
			b.WriteString("\n\n")
		case msg.Role == "user":
			b.WriteString(userStyle.Render("You: "))
			b.WriteString(wrapStyle.Render(msg.Content))
			b.WriteString("\n\n")
		case msg.Role == "assistant":
			// 渲染思考过程
			if msg.Thinking != "" {
				b.WriteString(thinkingStyle.Render("💭 Thinking:"))
				b.WriteString("\n")
				b.WriteString(wrapStyle.Render(thinkingStyle.Render(msg.Thinking)))
				b.WriteString("\n\n")
			}
			// 渲染回复
			b.WriteString(assistantStyle.Render("🤖 Assistant:"))
			b.WriteString("\n")
			b.WriteString(wrapStyle.Render(msg.Content))
			b.WriteString("\n\n")
		}
	}

	// 渲染当前流式输出（如果有）
	if m.status == statusStreaming {
		if m.currentThinking != "" {
			b.WriteString(thinkingStyle.Render("💭 Thinking:"))
			b.WriteString("\n")
			b.WriteString(wrapStyle.Render(thinkingStyle.Render(m.currentThinking)))
			b.WriteString("\n\n")
		}
		if m.currentContent != "" {
			b.WriteString(assistantStyle.Render("🤖 Assistant:"))
			b.WriteString("\n")
			b.WriteString(wrapStyle.Render(m.currentContent))
			b.WriteString("\n")
		}
	}

	// 渲染计时器
	if m.status == statusStreaming {
		elapsed := int(m.elapsed.Seconds())
		b.WriteString(timerStyle.Render(fmt.Sprintf("\n⏱️  Imagining… (%ds)", elapsed)))
		b.WriteString("\n")
	} else if m.elapsed > 0 {
		// 显示上一次的总耗时
		elapsed := m.elapsed.Seconds()
		b.WriteString(timerStyle.Render(fmt.Sprintf("✓ Done in %.1fs", elapsed)))
		b.WriteString("\n")
	}

	// 渲染错误（如果有）
	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("\nError: %s", m.err.Error())))
		b.WriteString("\n")
	}

	// 渲染输入框
	b.WriteString("\n")
	b.WriteString(inputStyle.Render(m.textarea.View()))

	return b.String()
}

// 辅助函数：限制字符串长度
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// 辅助函数：添加省略号
func ellipsis(s string) string {
	return s + "…"
}

// 样式渲染辅助
func renderWithPrefix(prefix, content string, style lipgloss.Style) string {
	return style.Render(prefix) + content
}
