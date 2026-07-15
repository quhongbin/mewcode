package tui

import "github.com/charmbracelet/lipgloss"

// 定义 TUI 样式
var (
	// 用户消息样式（在 view.go 中使用）
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")). // 蓝色
			Bold(true)

	// AI 回复样式（在 view.go 中使用）
	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")) // 白色

	// 思考过程样式（在 view.go 中使用）
	thinkingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")). // 灰色
			Italic(true)

	// 错误消息样式（在 view.go 中使用）
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")). // 红色
			Bold(true)

	// 计时器样式（在 view.go 中使用）
	timerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")) // 黄色

	// 欢迎信息样式（在 view.go 中使用）
	welcomeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("28")). // 粉色
			Bold(true)

	// 输入框样式（在 view.go 中使用）
	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	// 状态栏样式（预留，暂未使用）
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	// 工具调用样式（在 view.go 中使用）
	toolCallStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("38")) // 青色

	// 工具执行中样式（在 view.go 中使用）
	toolExecutingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")) // 橙黄色

	// 工具执行完成样式（在 view.go 中使用）
	toolDoneStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("78")) // 绿色

	// 工具结果样式（在 view.go 中使用）
	toolResultStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")) // 灰色
)

const (
	// 欢迎信息文本（在 view.go 中使用）
	welcomeMessage = `
#############################################################
#                                                           #
#    MewCode - AI Coding Assistant                          #
#                                                           #
#   Type your message and press Enter to send.              #
#   Press Ctrl+C to exit.                                   #
#                                                           #
#############################################################
`

	// spinner 动画帧（在 view.go 中使用，工具执行时的旋转动画）
	spinnerFrames = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"
)
