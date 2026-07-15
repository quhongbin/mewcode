package tui

import (
	"time"

	"mewcode/internal/agent"
	"mewcode/internal/provider"
	"mewcode/internal/tools"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// 状态类型
type statusType int

const (
	statusIdle statusType = iota
	statusStreaming
)

// DisplayMessage 用于显示的消息结构
type DisplayMessage struct {
	Role        string
	Content     string
	Thinking    string
	IsError     bool
	ToolCall    *provider.ToolCall // provider.ToolCall: provider.go 中定义的结构体
	ToolResult  *tools.ToolResult  // tools.ToolResult: tool.go 中定义的结构体
	Iteration   int                // 迭代轮次（可选，用于显示进度）
	IsExecuting bool               // 工具是否正在执行中（仅 tool_call 角色使用）
}

// Model 是 TUI 的主模型
type Model struct {
	agent            *agent.Agent // Agent 编排层（agent.go 中定义的结构体）
	textarea         textarea.Model
	viewport         viewport.Model
	messages         []DisplayMessage // DisplayMessage: 本文件中定义的结构体
	status           statusType       // statusType: 本文件中定义的类型
	timerStart       time.Time
	elapsed          time.Duration
	err              error
	width            int
	height           int
	ready            bool
	currentThinking  string
	currentContent   string
	currentIteration int // 当前迭代轮次
	tickCounter      int // tick 计数器，用于标识当前活跃的 tick 链（防止多个并发 tick 链）
	spinnerFrame     int // spinner 动画帧索引
	program          *tea.Program
}

// NewModel 创建新的 TUI 模型
func NewModel(ag *agent.Agent) Model { // agent.Agent: agent.go 中定义的结构体
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Enter to send, Ctrl+C to exit)"
	ta.Focus()
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	vp := viewport.New(0, 0)

	return Model{
		agent:    ag,
		textarea: ta,
		viewport: vp,
		messages: []DisplayMessage{}, // DisplayMessage: model.go 中定义的结构体
		status:   statusIdle,         // statusIdle: model.go 中定义的常量
	}
}

// SetProgram 设置 program 引用（在创建 Program 后调用）
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

// Init 初始化模型
func (m Model) Init() tea.Cmd { // Model: model.go 中定义的类型
	return m.tick() // tick: model.go 中定义的函数
}

// tick 每秒更新计时器，携带当前 tickCounter 用于过滤过期 tick
func (m Model) tick() tea.Cmd { // Model: model.go 中定义的类型
	tickCounter := m.tickCounter
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{time: t, counter: tickCounter} // tickMsg: model.go 中定义的类型
	})
}

// tickMsg 计时器消息（携带 counter 用于过滤过期 tick）
type tickMsg struct {
	time    time.Time
	counter int
}

// agentEventMsg Agent 事件消息
type agentEventMsg struct {
	event agent.Event // agent.Event: event.go 中定义的结构体
}

// hasExecutingTool 检查是否有工具正在执行中
func (m Model) hasExecutingTool() bool {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "tool_call" && m.messages[i].IsExecuting {
			return true
		}
	}
	return false
}

// markToolDone 根据工具名称将对应的执行中状态标记为已完成
func (m *Model) markToolDone(toolName string) {
	// 从后向前查找匹配的正在执行的工具调用
	for i := len(m.messages) - 1; i >= 0; i-- {
		msg := &m.messages[i]
		if msg.Role == "tool_call" && msg.IsExecuting && msg.ToolCall != nil && msg.ToolCall.Name == toolName {
			msg.IsExecuting = false
			return
		}
	}
}
