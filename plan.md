# MewCode Plan

## 架构概览

MewCode 采用分层架构，核心分为四个模块：

### 1. Provider 层（API 抽象层）
负责与不同 LLM API 后端通信。定义统一的 Provider 接口，Anthropic 和 OpenAI 分别实现该接口。支持流式响应（SSE）和 extended thinking。新增 API 后端只需实现接口，无需修改其他代码。

### 2. Config 层（配置管理层）
负责 YAML 配置文件的解析、验证和管理。支持多个供应商配置，通过 name 字段选择当前使用的供应商。启动时校验必填字段，缺失时给出明确错误提示。

### 3. Conversation 层（对话管理层）
维护对话历史，管理多轮对话的上下文。接收用户消息，调用 Provider 获取响应，将响应加入历史。与 UI 层解耦，可独立测试。

### 4. TUI 层（用户界面层）
基于 bubbletea 框架构建交互式终端界面。负责用户输入、流式输出显示、计时显示、错误提示等。接收用户输入后调用 Conversation 层，将响应实时渲染到终端。

### 5. Main（主程序）
整合各层，初始化配置、Provider、Conversation，启动 TUI 循环。

## 核心数据结构

### Message 类型
对话消息的基础结构：
- Role: 消息角色（user / assistant / system）
- Content: 文本内容
- Thinking: 思考内容（仅 assistant 消息，用于 extended thinking）

### Provider 接口
统一的 API 抽象接口：
- StreamChat(ctx, messages, thinking): 发送消息并返回流式响应
- 返回一个 Stream 对象，包含：
  - Next(): 获取下一个增量（文本片段或 thinking 片段）
  - Close(): 关闭流

### StreamChunk 类型
流式响应的增量单元：
- Type: 增量类型（text / thinking / done）
- Content: 增量内容

### Config 结构
配置文件解析结果：
- Providers: 供应商配置列表
- ActiveProvider: 当前使用的供应商 name

### ProviderConfig 结构
单个供应商配置：
- Name: 供应商标识名
- Protocol: 协议类型（anthropic / openai）
- Model: 模型名称
- BaseURL: 请求地址
- APIKey: 认证密钥
- Thinking: 是否启用扩展思考（可选）

### Conversation 结构
对话状态管理：
- Messages: 对话历史切片
- Provider: 当前使用的 Provider 实例

## 模块设计

### Provider 模块

**职责：** 封装与 LLM API 的通信，屏蔽不同协议的差异，提供统一的流式响应接口。

**对外接口：**
- `Provider` 接口：`StreamChat(ctx, messages, thinking) → Stream`
- `Stream` 接口：`Next() → (StreamChunk, error)` 和 `Close()`
- `NewProvider(config) → Provider`：根据 protocol 字段创建对应实现

**依赖：** 仅依赖标准库的 HTTP 客户端和 JSON 编解码，不依赖其他业务模块。

**内部实现：**
- `anthropicProvider`：实现 Anthropic Messages API，支持 SSE 流式解析和 extended thinking（thinking 块 + text 块交替）
- `openaiProvider`：实现 OpenAI Chat Completions API，支持 SSE 流式解析

### Config 模块

**职责：** 读取、解析、验证 YAML 配置文件。

**对外接口：**
- `LoadConfig(path) → (Config, error)`：加载并验证配置
- `Config` 结构：包含 `Providers []ProviderConfig` 和 `ActiveProvider string`

**依赖：** 使用 `gopkg.in/yaml.v3` 解析 YAML。

**验证规则：**
- 必填字段：name、protocol、model、base_url、api_key
- protocol 必须是 `anthropic` 或 `openai`
- base_url 必须是合法 URL
- ActiveProvider 必须在 Providers 列表中存在

### Conversation 模块

**职责：** 维护对话历史，协调 Provider 调用。

**对外接口：**
- `NewConversation(provider) → Conversation`
- `SendMessage(ctx, userText) → Stream`：将用户消息加入历史，调用 Provider 返回流
- `OnStreamComplete(stream)`：流结束后将完整 assistant 回复加入历史

**依赖：** 依赖 Provider 接口。

### TUI 模块

**职责：** 终端交互界面，基于 bubbletea 框架。

**对外接口：**
- `NewModel(conversation) → Model`：创建 bubbletea Model
- `Run(conversation) error`：启动 TUI 主循环

**内部状态：**
- 对话历史显示区（滚动）
- 输入框（支持多行编辑，Alt+Enter 换行）
- 当前状态：idle / streaming / error
- 计时器：请求发出时间、首个 token 时间、总耗时

**依赖：** 依赖 Conversation 模块，使用 `charmbracelet/bubbletea` 和 `charmbracelet/lipgloss`。

### Main 模块

**职责：** 程序入口，整合各层。

**流程：**
1. 解析命令行参数（配置文件路径，默认 `./mewcode.yaml`）
2. 调用 Config 模块加载配置
3. 根据 ActiveProvider 创建 Provider 实例
4. 创建 Conversation 实例
5. 启动 TUI 主循环

## 模块交互

### 启动流程
```
main()
  ├─ LoadConfig(path) → Config
  ├─ 根据 ActiveProvider 找到对应 ProviderConfig
  ├─ NewProvider(config) → Provider
  ├─ NewConversation(provider) → Conversation
  └─ TUI.Run(conversation)
```

### 用户发送消息流程
```
TUI (用户按 Enter)
  ├─ 将用户输入添加到对话显示区（user 样式）
  ├─ 启动计时器，显示 "Imagining… (0s)"
  ├─ 设置状态为 streaming，禁用输入
  └─ 调用 conversation.SendMessage(ctx, text)
       ├─ 将 user message 加入 Messages 历史
       ├─ 调用 provider.StreamChat(ctx, messages, thinking) → Stream
       └─ 返回 Stream 给 TUI

TUI (接收流式响应)
  ├─ 循环调用 stream.Next()
  │    ├─ 收到 text chunk → 追加到对话显示区（assistant 样式）
  │    ├─ 收到 thinking chunk → 追加到显示区（thinking 样式，可折叠/灰色）
  │    ├─ 更新计时显示
  │    └─ 收到 done → 退出循环
  ├─ 调用 stream.Close()
  ├─ 将完整 assistant 回复加入 Conversation.Messages
  ├─ 显示总耗时（如 "Done in 3.2s"）
  └─ 设置状态为 idle，重新启用输入
```

### 错误处理流程
```
TUI (stream.Next() 返回 error)
  ├─ 调用 stream.Close()
  ├─ 在对话区显示错误信息（error 样式，红色）
  ├─ 显示总耗时
  └─ 设置状态为 idle，重新启用输入
```

### 退出流程
```
TUI (用户输入 /exit 或 Ctrl+C)
  ├─ 清理资源（关闭流、释放终端）
  └─ 程序退出
```

## 文件组织

```
mewcode/
├── cmd/
│   └── mewcode/
│       └── main.go              — 程序入口，整合各层
├── internal/
│   ├── config/
│   │   ├── config.go            — Config、ProviderConfig 结构，LoadConfig
│   │   └── config_test.go       — 配置解析和验证测试
│   ├── provider/
│   │   ├── provider.go          — Provider、Stream、StreamChunk 接口和类型定义
│   │   ├── anthropic.go         — Anthropic Provider 实现，SSE 解析，extended thinking
│   │   ├── openai.go            — OpenAI Provider 实现，SSE 解析
│   │   └── provider_test.go     — Provider 单元测试
│   ├── conversation/
│   │   ├── conversation.go      — Conversation 结构，SendMessage，对话历史管理
│   │   └── conversation_test.go — 对话管理测试
│   └── tui/
│       ├── model.go             — bubbletea Model，状态管理
│       ├── view.go              — 界面渲染（对话区、输入框、计时）
│       ├── update.go            — 事件处理（按键、流式响应）
│       └── styles.go            — lipgloss 样式定义
├── go.mod
├── go.sum
├── mewcode.yaml                 — 示例配置文件
└── README.md
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| TUI 框架 | charmbracelet/bubbletea + lipgloss | Go 生态最成熟的 TUI 框架，原生支持流式更新、按键事件、多行输入，社区活跃 |
| HTTP 客户端 | 标准库 net/http | 无需引入第三方依赖，SSE 流式解析用 bufio.Scanner 逐行读取即可 |
| YAML 解析 | gopkg.in/yaml.v3 | Go 生态标准 YAML 库，稳定可靠 |
| 流式响应实现 | SSE（Server-Sent Events） | Anthropic 和 OpenAI 均使用 SSE 协议，逐行解析 `data:` 字段即可 |
| Provider 接口设计 | 返回 Stream 对象，由调用方迭代 | 解耦生产和消费，TUI 可控制迭代节奏，便于错误处理和取消 |
| 对话历史存储 | 内存中的 []Message 切片 | 简单直接，符合"不做持久化"的边界，后续加上下文管理策略时易于扩展 |
| 配置校验时机 | 启动时一次性校验 | 快速失败，避免运行时才发现配置错误 |
| Extended Thinking 处理 | StreamChunk 区分 text 和 thinking 类型 | TUI 可用不同样式渲染，用户可区分思考过程和最终回复 |
| 多行输入实现 | bubbletea textarea 组件 | 原生支持 Alt+Enter 换行、Enter 提交，无需自行实现按键映射 |
| 计时显示 | 使用 bubbletea 的 ticker 机制 | 每秒更新计时显示，与 TUI 事件循环集成，无需额外 goroutine 管理 |
