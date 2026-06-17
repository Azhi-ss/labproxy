package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"labproxy/internal/cli"
	"labproxy/internal/proxy"

	"gopkg.in/yaml.v3"
)

// StatusData 是 status --json 的 data 负载。
type StatusData struct {
	Running     bool   `json:"running"`
	PID         int    `json:"pid"`
	ProxyPort   int    `json:"proxy_port"`
	UIPort      int    `json:"ui_port"`
	DNSPort     int    `json:"dns_port"`
	Mode        string `json:"mode"`
	SystemProxy bool   `json:"system_proxy"`
	Uptime      string `json:"uptime"`
}

// runStatusCLI 实现 `labproxy-tui status` 子命令。
// home: labproxy home 目录（含 ~/.labproxy）。
// endpoint: mihomo controller URL（如 http://127.0.0.1:9090），可为空。
// secret: mihomo controller secret。
func runStatusCLI(stdout, stderr io.Writer, args []string, home, endpoint, secret string) int {
	_ = stderr
	jsonOut := cli.IsJSONFlag(args)

	data := StatusData{}
	data.SystemProxy = readSystemProxy(home)

	pid := readPIDFile(home)
	if pid > 0 && processAlive(pid) {
		data.Running = true
		data.PID = pid
		data.Uptime = processUptime(pid)
	}

	ports := readPortsFile(home)
	data.ProxyPort = ports["PROXY_PORT"]
	data.UIPort = ports["UI_PORT"]
	data.DNSPort = ports["DNS_PORT"]

	// 仅当进程运行时才查 mihomo /configs 获取 mode
	if data.Running && endpoint != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		client := proxy.NewClient(endpoint, secret)
		if cfg, err := client.Config(ctx); err == nil {
			data.Mode = cfg.Mode
		}
		// mode 查询失败不影响整体 status 输出，仅留空
	}

	if jsonOut {
		_ = cli.PrintJSON(stdout, cli.Envelope{OK: true, Data: data})
		return 0
	}
	printStatusHuman(stdout, data)
	return 0
}

func printStatusHuman(w io.Writer, d StatusData) {
	if d.Running {
		fmt.Fprintf(w, "🟢 LabProxy 运行中 (PID: %d)\n", d.PID)
	} else {
		fmt.Fprintln(w, "🔴 LabProxy 未运行")
	}
	if d.Uptime != "" {
		fmt.Fprintf(w, "运行时间: %s\n", d.Uptime)
	}
	fmt.Fprintf(w, "代理端口: %d\n", d.ProxyPort)
	fmt.Fprintf(w, "管理端口: %d\n", d.UIPort)
	fmt.Fprintf(w, "DNS 端口: %d\n", d.DNSPort)
	if d.Mode != "" {
		fmt.Fprintf(w, "代理模式: %s\n", d.Mode)
	}
	sp := "关闭"
	if d.SystemProxy {
		sp = "开启"
	}
	fmt.Fprintf(w, "系统代理: %s\n", sp)
}

// readPIDFile 读取 config/labproxy.pid，无效返回 0。
func readPIDFile(home string) int {
	b, err := os.ReadFile(filepath.Join(home, ".labproxy", "config", "labproxy.pid"))
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

// readPortsFile 解析 config/ports.conf 的 KEY=VALUE 行。
func readPortsFile(home string) map[string]int {
	out := map[string]int{}
	b, err := os.ReadFile(filepath.Join(home, ".labproxy", "config", "ports.conf"))
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		port, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			continue
		}
		out[strings.TrimSpace(k)] = port
	}
	return out
}

// mixinSystemProxy 结构，仅解析 system-proxy.enable。
type mixinSystemProxy struct {
	SystemProxy struct {
		Enable bool `yaml:"enable"`
	} `yaml:"system-proxy"`
}

// readSystemProxy 读 mixin.yaml 的 system-proxy.enable，默认 true（与 bash 行为一致）。
func readSystemProxy(home string) bool {
	b, err := os.ReadFile(filepath.Join(home, ".labproxy", "mixin.yaml"))
	if err != nil {
		return true
	}
	// 未显式写 enable 时视为 true（与 proxyctl 的 `.system-proxy.enable // true` 一致）
	if !hasSystemProxyEnable(b) {
		return true
	}
	var m mixinSystemProxy
	if err := yaml.Unmarshal(b, &m); err != nil {
		return true
	}
	return m.SystemProxy.Enable
}

// hasSystemProxyEnable 检测 mixin.yaml 是否显式写了 system-proxy.enable。
func hasSystemProxyEnable(b []byte) bool {
	var node yaml.Node
	if err := yaml.Unmarshal(b, &node); err != nil {
		return false
	}
	if node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		return false
	}
	root := node.Content[0]
	if root.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "system-proxy" {
			sp := root.Content[i+1]
			if sp.Kind != yaml.MappingNode {
				return false
			}
			for j := 0; j+1 < len(sp.Content); j += 2 {
				if sp.Content[j].Value == "enable" {
					return true
				}
			}
		}
	}
	return false
}

// processAlive 判断进程是否存活。
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 探活
	if err := proc.Signal(syscall.Signal(0)); err == nil {
		return true
	}
	return false
}

// processUptime 返回进程运行时长字符串（best-effort，失败返回空）。
func processUptime(pid int) string {
	cmd := exec.Command("ps", "-o", "etime=", "-p", strconv.Itoa(pid))
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
