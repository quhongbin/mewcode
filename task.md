# MewCode Tasks

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 新建 | `go.mod` | Go 模块定义和依赖管理 |
| 新建 | `internal/config/config.go` | Config、ProviderConfig 结构，LoadConfig 函数 |
| 新建 | `internal/config/config_test.go` | 配置解析和验证测试 |
| 新建 | `internal/provider/provider.go` | Provider、Stream、StreamChunk、Message 接口和类型定义 |
| 新建 | `internal/provider/anthropic.go` | Anthropic Provider 实现，SSE 解析，extended thinking |
| 新建 | `internal/provider/openai.go` | OpenAI Provider 实现，SSE 解析 |
| 新建 | `internal/provider/provider_test.go` | Provider 单元测试 |
| 新建 | `internal/conversation/conversation.go` | Conversation 结构，SendMessage，对话历史管理 |
| 新建 | `internal/conversation/conversation_test.go` | 对话管理测试 |
| 新建 | `internal/tui/styles.go` | lipgloss 样式定义 |
| 新建 | `internal/tui/model.go` | bubbletea Model，状态管理 |
| 新建 | `internal/tui/view.go` | 界面渲染（对话区、输入框、计时） |
| 新建 | `internal/tui/update.go` | 事件处理（按键、流式响应） |
| 新建 | `cmd/mewcode/main.go` | 程序入口，整合各层 |
| 新建 | `mewcode.yaml` | 示例配置文件 |

## T1: 初始化 Go 模块

**文件：** `go.mod`
**依赖：** 无
**步骤：**
1. 在项目根目录执行 `go mod init mewcode`
2. 添加依赖：`go get github.com/charmbracelet/bubbletea`、`go get github.com/charmbracelet/lipgloss`、`go get gopkg.in/yaml.v3`
3. 执行 `go mod tidy` 整理依赖

**验证：** `go mod download` 成功，无报错

## T2: 定义 Config 结构和解析逻辑

**文件：** `internal/config/config.go`
**依赖：** T1
**步骤：**
1. 定义 `ProviderConfig` 结构体，字段：Name、Protocol、Model、BaseURL、APIKey、Thinking（bool 指针，可选）
2. 定义 `Config` 结构体，字段：Providers（[]ProviderConfig）、ActiveProvider（string）
3. 实现 `LoadConfig(path string) (Config, error)` 函数：
   - 读取 YAML 文件
   - 解析到 Config 结构
   - 调用验证函数
4. 实现 `validateConfig(cfg *Config) error` 函数：
   - 检查 Providers 非空
   - 检查每个 Provider 的必填字段（Name、Protocol、Model、BaseURL、APIKey）
   - 检查 Protocol 必须是 "anthropic" 或 "openai"
   - 检查 BaseURL 是合法 URL（使用 url.Parse）
   - 检查 ActiveProvider 在 Providers 列表中存在

**验证：** `go build ./internal/config/...` 编译通过

## T3: 编写 Config 单元测试

**文件：** `internal/config/config_test.go`
**依赖：** T2
**步骤：**
1. 编写测试用例：正常配置解析成功
2. 编写测试用例：缺少必填字段时报错
3. 编写测试用例：Protocol 非法时报错
4. 编写测试用例：ActiveProvider 不存在时报错
5. 使用临时文件创建测试 YAML

**验证：** `go test ./internal/config/...` 全部通过

## T4: 定义 Provider 接口和类型

**文件：** `internal/provider/provider.go`
**依赖：** T1
**步骤：**
1. 定义 `Message` 结构体，字段：Role（string）、Content（string）、Thinking（string）
2. 定义 `StreamChunkType` 类型（string），常量：ChunkTypeText、ChunkTypeThinking、ChunkTypeDone
3. 定义 `StreamChunk` 结构体，字段：Type（StreamChunkType）、Content（string）
4. 定义 `Stream` 接口，方法：Next() (StreamChunk, error)、Close() error
5. 定义 `Provider` 接口，方法：StreamChat(ctx context.Context, messages []Message, thinking bool) (Stream, error)
6. 实现 `NewProvider(cfg config.ProviderConfig) (Provider, error)` 工厂函数，根据 Protocol 返回对应实现

**验证：** `go build ./internal/provider/...` 编译通过

## T5: 实现 Anthropic Provider

**文件：** `internal/provider/anthropic.go`
**依赖：** T4
**步骤：**
1. 定义 `anthropicProvider` 结构体，字段：config（ProviderConfig）、client（*http.Client）
2. 实现 `StreamChat` 方法：
   - 构造请求体（model、messages、stream、thinking 参数）
   - 创建 HTTP POST 请求到 BaseURL + "/v1/messages"
   - 设置 Header：x-api-key、anthropic-version、content-type
   - 发送请求，检查响应状态
3. 定义 `anthropicStream` 结构体，字段：scanner（*bufio.Scanner）、done（bool）
4. 实现 `Next()` 方法：
   - 逐行读取 SSE 数据（以 "data: " 开头）
   - 解析 JSON，判断 event 类型（content_block_delta、message_stop 等）
   - 提取 delta 中的 type（text_delta、thinking_delta）和内容
   - 返回对应的 StreamChunk
5. 实现 `Close()` 方法：清理资源

**验证：** `go build ./internal/provider/...` 编译通过

## T6: 实现 OpenAI Provider

**文件：** `internal/provider/openai.go`
**依赖：** T4
**步骤：**
1. 定义 `openaiProvider` 结构体，字段：config（ProviderConfig）、client（*http.Client）
2. 实现 `StreamChat` 方法：
   - 构造请求体（model、messages、stream 参数）
   - 创建 HTTP POST 请求到 BaseURL + "/v1/chat/completions"
   - 设置 Header：Authorization（Bearer api_key）、content-type
   - 发送请求，检查响应状态
3. 定义 `openaiStream` 结构体，字段：scanner（*bufio.Scanner）、done（bool）
4. 实现 `Next()` 方法：
   - 逐行读取 SSE 数据（以 "data: " 开头）
   - 解析 JSON，提取 choices[0].delta.content
   - 遇到 "[DONE]" 时返回 ChunkTypeDone
5. 实现 `Close()` 方法：清理资源

**验证：** `go build ./internal/provider/...` 编译通过

## T7: 实现 Conversation 模块

**文件：** `internal/conversation/conversation.go`
**依赖：** T4
**步骤：**
1. 定义 `Conversation` 结构体，字段：messages（[]provider.Message）、provider（provider.Provider）、thinking（bool）
2. 实现 `NewConversation(p provider.Provider, thinking bool) *Conversation`
3. 实现 `SendMessage(ctx context.Context, userText string) (provider.Stream, error)` 方法：
   - 创建 user Message，加入 messages 切片
   - 调用 provider.StreamChat，返回 Stream
4. 实现 `AddAssistantMessage(content, thinking string)` 方法：
   - 创建 assistant Message，加入 messages 切片
5. 实现 `GetMessages() []provider.Message` 方法：返回当前历史

**验证：** `go build ./internal/conversation/...` 编译通过

## T8: 编写 Conversation 单元测试

**文件：** `internal/conversation/conversation_test.go`
**依赖：** T7
**步骤：**
1. 创建 mock Provider 实现（返回固定的 Stream）
2. 编写测试用例：SendMessage 后 messages 包含 user 消息
3. 编写测试用例：AddAssistantMessage 后 messages 包含 assistant 消息
4. 编写测试用例：多轮对话历史正确累积

**验证：** `go test ./internal/conversation/...` 全部通过

## T9: 定义 TUI 样式

**文件：** `internal/tui/styles.go`
**依赖：** T1
**步骤：**
1. 定义样式常量：userStyle（蓝色）、assistantStyle（白色）、thinkingStyle（灰色/斜体）、errorStyle（红色）、timerStyle（黄色）
2. 使用 lipgloss 创建对应样式对象
3. 定义欢迎信息文本

**验证：** `go build ./internal/tui/...` 编译通过

## T10: 实现 TUI Model 和状态管理

**文件：** `internal/tui/model.go`
**依赖：** T9, T7
**步骤：**
1. 定义 `Model` 结构体，字段：conversation、textarea（bubbletea textarea）、messages（[]DisplayMessage）、status（idle/streaming）、timerStart（time.Time）、elapsed（time.Duration）、err（error）、width/height（int）
2. 定义 `DisplayMessage` 结构体，字段：Role、Content、Thinking、IsError
3. 实现 `NewModel(conv *conversation.Conversation, thinking bool) Model`
4. 实现 bubbletea 接口：Init()、Update()、View()（先写空实现，后续补充）

**验证：** `go build ./internal/tui/...` 编译通过

## T11: 实现 TUI View 渲染

**文件：** `internal/tui/view.go`
**依赖：** T10
**步骤：**
1. 实现 `View()` 方法：
   - 渲染欢迎信息（如果 messages 为空）
   - 渲染对话历史（遍历 messages，根据 Role 应用不同样式）
   - 渲染计时状态（streaming 时显示 "Imagining… (Xs)"，完成后显示 "Done in Xs"）
   - 渲染错误信息（如果有 err）
   - 渲染输入框（textarea.View()）
2. 使用 lipgloss 布局，确保各区域正确排列

**验证：** `go build ./internal/tui/...` 编译通过

## T12: 实现 TUI Update 事件处理

**文件：** `internal/tui/update.go`
**依赖：** T10, T11
**步骤：**
1. 实现 `Update(msg tea.Msg) (tea.Model, tea.Cmd)` 方法：
   - 处理 `tea.KeyMsg`：
     - Ctrl+C 或 `/exit`：退出程序
     - Enter（且 status == idle）：提交输入，调用 SendMessage，启动流式处理
     - 其他按键：转发给 textarea
   - 处理自定义消息 `streamChunkMsg`：更新当前 assistant 消息内容
   - 处理自定义消息 `streamDoneMsg`：设置 status = idle，记录总耗时
   - 处理自定义消息 `streamErrMsg`：设置 err，status = idle
   - 处理 `tea.WindowSizeMsg`：更新 width/height
2. 实现 `submitInput()` 方法：
   - 获取 textarea 内容，清空
   - 设置 status = streaming，记录 timerStart
   - 启动 goroutine 调用 conversation.SendMessage，循环读取 Stream，发送 streamChunkMsg
   - 流结束后发送 streamDoneMsg 或 streamErrMsg

**验证：** `go build ./internal/tui/...` 编译通过

## T13: 实现 Main 入口

**文件：** `cmd/mewcode/main.go`
**依赖：** T2, T4, T7, T10
**步骤：**
1. 解析命令行参数：配置文件路径（默认 "./mewcode.yaml"）
2. 调用 config.LoadConfig 加载配置
3. 根据 ActiveProvider 找到对应 ProviderConfig
4. 调用 provider.NewProvider 创建 Provider
5. 调用 conversation.NewConversation 创建 Conversation
6. 调用 tui.NewModel 创建 Model
7. 创建 bubbletea.Program 并运行

**验证：** `go build ./cmd/mewcode/...` 编译通过，生成可执行文件

## T14: 创建示例配置文件

**文件：** `mewcode.yaml`
**依赖：** 无
**步骤：**
1. 编写示例配置，包含两个供应商（anthropic 和 openai）
2. 添加注释说明各字段含义
3. 使用占位符替代真实 API Key

**验证：** 文件格式正确，可被 T2 的 LoadConfig 解析

## T15: 端到端测试

**文件：** 无
**依赖：** T13, T14
**步骤：**
1. 运行 `go run ./cmd/mewcode`，观察欢迎信息
2. 输入问题，观察流式响应
3. 测试多轮对话，验证上下文记忆
4. 测试错误处理（配置错误 API Key）
5. 测试退出命令

**验证：** 所有功能正常，无崩溃

## 执行顺序

```
T1 → T2 → T3
  ↘
   T4 → T5
     ↘
      T6
     ↗
   T7 → T8
  ↗
T9 → T10 → T11 → T12
              ↗
           T13
          ↗
       T14
      ↗
   T15
```

简化版（串行）：
```
T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8 → T9 → T10 → T11 → T12 → T13 → T14 → T15
```
