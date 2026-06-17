// Package cli 提供命令行工具共享的输出工具：统一 JSON envelope 与 --json 标志解析。
// agent-native 设计：所有 CLI 命令对人类输出彩色文本，对机器（--json）输出稳定 schema 的 JSON。
package cli

import (
	"encoding/json"
	"io"
	"strings"
)

// Envelope 是所有 --json 输出的统一信封。
// Data 为 nil 时序列化为 null，保证字段始终存在，便于 agent 稳定解析。
type Envelope struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data"`
	Error string `json:"error"`
}

// PrintJSON 将 envelope 序列化为单行 JSON 并写入 w，以换行结尾。
func PrintJSON(w io.Writer, env Envelope) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(env)
}

// IsJSONFlag 检测 args 中是否包含 --json（任意位置）。
// 支持 --json 与 --json=true/--json=false 两种形式。
// 不识别短旗 -j，保持显式。
func IsJSONFlag(args []string) bool {
	for _, a := range args {
		switch {
		case a == "--json":
			return true
		case strings.HasPrefix(a, "--json="):
			val := strings.TrimPrefix(a, "--json=")
			return val == "true"
		}
	}
	return false
}
