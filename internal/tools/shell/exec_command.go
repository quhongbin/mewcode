package shell

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"mewcode/internal/tools"
)

// BashTool 在宿主 shell 中执行命令
type BashTool struct{}

// NewBashTool 创建 Bash 工具实例
func NewBashTool() *BashTool {
	return &BashTool{}
}

func (t *BashTool) Name() string { return "bash" }
func (t *BashTool) Description() string {
	return "在宿主 shell 中执行命令，返回标准输出和标准错误。对于长时间运行的进程（如服务器），会在后台启动并返回初始输出。"
}
func (t *BashTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "要执行的 shell 命令"}
		},
		"required": ["command"]
	}`)
}
func (t *BashTool) Meta() tools.ToolMeta { // ToolMeta: tool.go 中定义的结构体
	return tools.ToolMeta{Category: "shell", ReadOnly: false, Destructive: true}
}

// Execute 执行 shell 命令
// 对于短命令：等待完成并返回完整输出
// 对于长运行进程：等待 initialWait 后若进程仍在运行，返回已捕获的输出并让进程在后台继续
func (t *BashTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if params.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	// 根据操作系统选择 shell
	// 使用独立 context 启动进程，避免 Executor 超时杀死长运行进程
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", params.Command)
	} else {
		cmd = exec.Command("sh", "-c", params.Command)
	}

	// 使用 bytes.Buffer 捕获输出
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	// 启动进程
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	// 等待一小段时间让进程输出初始内容
	const initialWait = 5 * time.Second
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// 等待进程完成或 initialWait 超时
	select {
	case err := <-done:
		// 进程已完成（正常退出或出错）
		output := stdout.String()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return fmt.Sprintf("exit code %d:\n%s", exitErr.ExitCode(), output), nil
			}
			return "", fmt.Errorf("failed to execute command: %w", err)
		}
		return output, nil

	case <-time.After(initialWait):
		// 进程仍在运行（长运行进程，如服务器）
		output := stdout.String()
		pid := cmd.Process.Pid
		if output == "" {
			output = "(无初始输出)"
		}
		return fmt.Sprintf("[进程已在后台启动，PID: %d]\n%s\n\n注意：进程仍在运行中。如需停止，请手动终止 PID %d。", pid, output, pid), nil

	case <-ctx.Done():
		// 外部 context 取消（如用户 Ctrl+C）
		_ = cmd.Process.Kill() // 手动杀掉进程
		<-done
		output := stdout.String()
		if output == "" {
			output = "(无输出)"
		}
		return "", fmt.Errorf("command cancelled: %s", output)
	}
}
