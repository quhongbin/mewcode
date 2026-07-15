# Agent Loop 自主循环 Checklist

> 每一项通过运行代码或观察行为来验证，聚焦系统行为。

## 实现完整性

- [ ] Event 类型体系已定义且可被引用（验证：`go build ./internal/agent/...` 编译通过）
- [ ] EventStream 可通过 Next() 方法逐个获取事件，channel 关闭后返回 false（验证：编译通过 + 单元逻辑可追踪）
- [ ] batchTools 能将混合工具调用按 ReadOnly 属性正确分组（验证：编译通过 + 分组逻辑可追踪）
- [ ] Registry.Has 方法能正确判断工具是否已注册（验证：编译通过）
- [ ] Conversation.AddUserMessage 和 AddAssistantMessageWithTools 方法正确添加到对话历史（验证：编译通过）
- [ ] Agent 主循环能驱动多轮 LLM 调用和工具执行（验证：编译通过 + AC1 端到端验证）
- [ ] Agent.SetMode / Agent.Mode 接口可正确切换和查询模式（验证：编译通过）

## 集成

- [ ] TUI 层通过 EventStream 消费事件渲染界面，不再直接引用 provider.Stream（验证：编译通过 + grep 确认 update.go 中无 provider.StreamChunkMsg 等旧类型引用）
- [ ] /do 命令触发 Agent 模式切换为 Act（验证：输入 /do 后观察 TUI 显示模式切换提示）
- [ ] main.go 正确传入 maxIter 和 maxParallel 参数创建 Agent（验证：编译通过）
- [ ] Agent 循环中每轮调用 providerToolDefs 根据当前 mode 返回正确的工具集（验证：Plan 模式下 grep 确认只传只读工具定义）

## 编译与测试

- [ ] `go build ./...` 全项目编译无错误
- [ ] `go vet ./...` 无静态分析问题

## 端到端场景

- [ ] 场景 1（多步自主循环）：发送"读取 main.go 的内容"→ Agent 自动调用 ReadFile → 拿到结果 → 输出文本总结 → 循环结束，无需用户二次催促（验证：观察 TUI 中自动出现 tool_call → tool_result → assistant 文本）

- [ ] 场景 2（纯文本向后兼容）：发送"你好"→ Agent 直接返回文本回复，界面正常显示，与改造前行为一致（验证：观察 TUI 只显示 assistant 文本，无工具调用事件）

- [ ] 场景 3（/do 模式切换）：启动后输入 `/do` → TUI 显示"已切换到 Act 模式"提示 → 后续消息使用全部工具（验证：观察 /do 后发送包含工具需求的消息，模型可调用写工具）

- [ ] 场景 4（Ctrl+C 取消）：Agent 正在执行多轮循环时按 Ctrl+C → 循环停止，不启动新的 LLM 调用，程序退出（验证：观察程序正常退出，无残余 goroutine 导致的 hang）
