package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	"mewcode/internal/conversation"
	"mewcode/internal/provider"
)

// 状态类型
type statusType int

const (
	statusIdle statusType = iota
	statusStreaming
)

// DisplayMessage 用于显示的消息结构
type DisplayMessage struct {
	Role     string
	Content  string
	Thinking string
	IsError  bool
}

// Model 是 TUI 的主模型
type Model struct {
	conversation    *conversation.Conversation
	textarea        textarea.Model
	viewport        viewport.Model
	messages        []DisplayMessage // DisplayMessage: 本文件中定义的结构体
	status          statusType       // statusType: 本文件中定义的类型
	timerStart      time.Time
	elapsed         time.Duration
	err             error
	width           int
	height          int
	ready           bool
	currentThinking string
	currentContent  string
	program         *tea.Program
}

// NewModel 创建新的 TUI 模型
func NewModel(conv *conversation.Conversation) Model {
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Enter to send, Ctrl+C to exit)"
	ta.Focus()
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	vp := viewport.New(0, 0)

	return Model{
		conversation: conv,
		textarea:     ta,
		viewport:     vp,
		messages:     []DisplayMessage{}, // DisplayMessage: model.go 中定义的结构体
		status:       statusIdle,         // statusIdle: model.go 中定义的常量
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

// tick 每秒更新计时器
func (m Model) tick() tea.Cmd { // Model: model.go 中定义的类型
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t) // tickMsg: model.go 中定义的类型
	})
}

// tickMsg 计时器消息
type tickMsg time.Time

// streamChunkMsg 流式数据块消息
type streamChunkMsg struct {
	chunk *provider.StreamChunk
}

// streamDoneMsg 流式完成消息
type streamDoneMsg struct {
	content  string
	thinking string
}

// streamErrMsg 流式错误消息
type streamErrMsg struct {
	err error
}
