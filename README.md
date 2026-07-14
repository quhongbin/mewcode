# MewCode

一个命令行 AI 助手，支持 Anthropic Claude 和 OpenAI API。

## 功能特性

- 🤖 支持 Anthropic Claude 和 OpenAI API
- 💬 多轮对话，AI 能记住完整对话历史
- ⚡ 流式输出，实时显示 AI 回复
- 💭 支持 Claude 的 extended thinking 模式
- 🎨 美观的 TUI 界面
- ⏱️ 响应计时显示
- ⚙️ YAML 配置文件管理

## 安装

```bash
# 编译项目
go build ./cmd/mewcode

# 运行
./mewcode
```

## 配置

1. 复制示例配置文件：
```bash
cp config.example.yaml config.yaml
```

2. 编辑 `config.yaml`，填入你的 API 密钥：
```yaml
active_provider: anthropic

providers:
  - name: anthropic
    protocol: anthropic
    model: claude-3-5-sonnet-20241022
    base_url: https://api.anthropic.com
    api_key: your-api-key-here
    thinking: false
```

配置字段说明：
- `name`: 供应商标识名
- `protocol`: 协议类型（`anthropic` 或 `openai`）
- `model`: 模型名称
- `base_url`: API 基础 URL
- `api_key`: API 密钥
- `thinking`: 是否启用 extended thinking（仅 Anthropic）

## 使用方法

1. 启动程序：
```bash
./mewcode
```

2. 在输入框中输入问题，按 Enter 发送

3. 使用 Alt+Enter 输入多行文本

4. 按 Ctrl+C 或输入 `/exit` 退出程序

## 开发

```bash
# 运行测试
go test ./...

# 编译
go build ./cmd/mewcode
```

## 项目结构

```
mewcode/
├── cmd/mewcode/          # 主程序入口
├── internal/
│   ├── config/           # 配置管理
│   ├── provider/         # API Provider 抽象
│   ├── conversation/     # 对话管理
│   └── tui/              # TUI 界面
├── config.example.yaml   # 配置示例
└── README.md
```

## License

MIT
