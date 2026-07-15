# 工具系统 Checklist

> 每一项通过运行代码或观察行为来验证，聚焦系统行为。

## 实现完整性

- [ ] Tool 接口可被任意工具实现，调用 Execute 返回结构化结果（验证：T1 编译通过，mock 工具通过接口调用成功）
- [ ] ToolMeta 元数据（Category、ReadOnly、Destructive）可通过接口读取（验证：六个工具的 Meta() 返回值符合表格定义）
- [ ] ReadFile 读取文本文件返回带行号前缀的内容（验证：读取一个 20 行文件，每行有 `  N│` 前缀）
- [ ] ReadFile offset/limit 参数正确切片（验证：offset=5 limit=3 只返回第 5-7 行）
- [ ] ReadFile 拒绝二进制文件（验证：读取一个包含 NUL 字节的文件，返回拒绝提示）
- [ ] WriteFile 自动创建不存在的父目录并写入文件（验证：写入 `tmp/a/b/test.txt`，目录被创建，文件内容正确）
- [ ] EditFile 文本匹配模式：唯一匹配替换成功（验证：替换文件中唯一存在的片段，内容正确更新）
- [ ] EditFile 文本匹配模式：未找到匹配返回错误提示（验证：替换不存在的片段，返回"未找到匹配"）
- [ ] EditFile 文本匹配模式：多次匹配返回错误提示（验证：替换重复出现的片段，返回"找到 N 处匹配"）
- [ ] EditFile 行号模式：精准替换指定行范围（验证：start_line=3 end_line=5 替换第 3-5 行）
- [ ] Bash 工具执行命令返回输出（验证：执行 `echo hello`，返回包含 `hello`）
- [ ] Bash 工具执行失败命令返回结构化错误（验证：执行不存在的命令，返回 exit code 和错误信息而非崩溃）
- [ ] Glob 工具按模式查找文件（验证：用 `*.go` 模式，返回当前目录下所有 .go 文件）
- [ ] Grep 工具按正则搜索代码内容（验证：搜索 `func Test`，返回匹配行及上下文）

## 集成

- [ ] Registry 注册工具后按名查找成功（验证：注册 mock 工具后 Get(name) 返回该工具）
- [ ] Registry.Definitions() 返回所有工具的 API 格式定义（验证：注册 6 个工具后 Definitions() 长度为 6，每项含 Name/Description/InputSchema）
- [ ] Executor 超时控制有效（验证：注册一个 sleep 60s 的 mock 工具，timeout=1s，Execute 在 1s 后返回超时错误）
- [ ] Executor 错误包装为结构化结果（验证：mock 工具返回 error，Execute 返回 `ToolResult{IsError: true, Content: err}`）
- [ ] Provider.StreamChat 接受 tools 参数（验证：编译通过，无工具时传 nil 行为不变）
- [ ] Conversation 支持添加工具调用消息和工具结果消息（验证：AddToolCallMessage 和 AddToolResultMessage 正确追加到历史）

## 编译与测试

- [ ] 项目编译无错误（验证：`go build ./...` 成功）
- [ ] 所有单元测试通过（验证：`go test ./...` 全部 PASS）

## 端到端场景

- [ ] 场景 1 — 纯文本对话不受影响：不使用工具时，发送消息获得文本回复，行为与改造前完全一致（验证：启动程序，发送"你好"，收到正常文本回复）
- [ ] 场景 2 — 工具调用完整流程：发送"读取 config.yaml 的内容"，模型调用 ReadFile 工具，界面显示工具调用过程（工具名+参数），执行后显示结果，模型基于文件内容给出文本回复（验证：观察 TUI 输出包含工具调用和最终回复）
- [ ] 场景 3 — ReadFile + EditFile 协同：读取一个文件后，模型使用行号精准编辑指定行（验证：观察 EditFile 的 start_line/end_line 参数与 ReadFile 输出的行号一致）
