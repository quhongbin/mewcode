package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// View 渲染界面
func (m Model) View() string { // Model: model.go 中定义的类型
	if !m.ready { // ready: Model 结构体字段（model.go）
		return "\n  Initializing..."
	}

	var b strings.Builder

	// 计算内容区域的最大宽度（减去输入框的 border(2) + padding(2) = 4）
	contentWidth := m.width - 4 // width: Model 结构体字段（model.go）

	// 渲染欢迎信息（仅在没有消息时显示）
	if len(m.messages) == 0 { // messages: Model 结构体字段（model.go）
		b.WriteString(welcomeStyle.Render(welcomeMessage)) // welcomeStyle, welcomeMessage: styles.go 中定义的变量
		b.WriteString("\n")
	}

	// 渲染对话历史
	for _, msg := range m.messages { // messages: Model 结构体字段（model.go）
		switch {
		case msg.IsError: // IsError: DisplayMessage 结构体字段（model.go）
			b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %s", msg.Content))) // errorStyle: styles.go 中定义的变量，Content: DisplayMessage 结构体字段（model.go）
			b.WriteString("\n\n")
		case msg.Role == "user": // Role: DisplayMessage 结构体字段（model.go）
			b.WriteString(userStyle.Render("You: ")) // userStyle: styles.go 中定义的变量
			b.WriteString(ansi.Wrap(msg.Content, contentWidth, "")) // Content: DisplayMessage 结构体字段（model.go）
			b.WriteString("\n\n")
		case msg.Role == "assistant": // Role: DisplayMessage 结构体字段（model.go）
			// 渲染思考过程
			if msg.Thinking != "" { // Thinking: DisplayMessage 结构体字段（model.go）
				b.WriteString(thinkingStyle.Render(" Thinking:")) // thinkingStyle: styles.go 中定义的变量
				b.WriteString("\n")
				b.WriteString(thinkingStyle.Render(ansi.Wrap(msg.Thinking, contentWidth, ""))) // thinkingStyle: styles.go 中定义的变量，Thinking: DisplayMessage 结构体字段（model.go）
				b.WriteString("\n\n")
			}
			// 渲染回复
			b.WriteString(assistantStyle.Render(" Assistant:")) // assistantStyle: styles.go 中定义的变量
			b.WriteString("\n")
			b.WriteString(ansi.Wrap(msg.Content, contentWidth, "")) // Content: DisplayMessage 结构体字段（model.go）
			b.WriteString("\n\n")
		}
	}

	// 渲染当前流式输出（如果有）
	if m.status == statusStreaming { // status: Model 结构体字段（model.go），statusStreaming: model.go 中定义的常量
		if m.currentThinking != "" { // currentThinking: Model 结构体字段（model.go）
			b.WriteString(thinkingStyle.Render(" Thinking:")) // thinkingStyle: styles.go 中定义的变量
			b.WriteString("\n")
			b.WriteString(thinkingStyle.Render(ansi.Wrap(m.currentThinking, contentWidth, ""))) // thinkingStyle: styles.go 中定义的变量，currentThinking: Model 结构体字段（model.go）
			b.WriteString("\n\n")
		}
		if m.currentContent != "" { // currentContent: Model 结构体字段（model.go）
			b.WriteString(assistantStyle.Render(" Assistant:")) // assistantStyle: styles.go 中定义的变量
			b.WriteString("\n")
			b.WriteString(ansi.Wrap(m.currentContent, contentWidth, "")) // currentContent: Model 结构体字段（model.go）
			b.WriteString("\n")
		}
	}

	// 渲染计时器
	if m.status == statusStreaming { // status: Model 结构体字段（model.go），statusStreaming: model.go 中定义的常量
		elapsed := int(m.elapsed.Seconds()) // elapsed: Model 结构体字段（model.go）
		b.WriteString(timerStyle.Render(fmt.Sprintf("\n⏱️  Imagining… (%ds)", elapsed))) // timerStyle: styles.go 中定义的变量
		b.WriteString("\n")
	} else if m.elapsed > 0 { // elapsed: Model 结构体字段（model.go）
		// 显示上一次的总耗时
		elapsed := m.elapsed.Seconds() // elapsed: Model 结构体字段（model.go）
		b.WriteString(timerStyle.Render(fmt.Sprintf("✓ Done in %.1fs", elapsed))) // timerStyle: styles.go 中定义的变量
		b.WriteString("\n")
	}

	// 渲染错误（如果有）
	if m.err != nil { // err: Model 结构体字段（model.go）
		b.WriteString(errorStyle.Render(fmt.Sprintf("\nError: %s", m.err.Error()))) // errorStyle: styles.go 中定义的变量，err: Model 结构体字段（model.go）
		b.WriteString("\n")
	}

	// 渲染输入框
	b.WriteString("\n")
	b.WriteString(inputStyle.Render(m.textarea.View())) // inputStyle: styles.go 中定义的变量，textarea: Model 结构体字段（model.go）

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
