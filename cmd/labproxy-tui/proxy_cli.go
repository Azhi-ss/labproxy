package main

import (
	"context"
	"os"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"labproxy/internal/cli"
	"labproxy/internal/proxy"
)

// extractStringFlag 从 args 提取 --name value 或 --name=value，返回剩余 args 与值。
// 用于子命令参数解析（flag.Parse 遇子命令名会停止，无法用全局 flag）。
func extractStringFlag(args []string, name string) ([]string, string) {
	out := make([]string, 0, len(args))
	val := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == name:
			if i+1 < len(args) {
				val = args[i+1]
				i++
			}
		case strings.HasPrefix(a, name+"="):
			val = strings.TrimPrefix(a, name+"=")
		default:
			out = append(out, a)
		}
	}
	return out, val
}

// runProxiesCLI 实现 `labproxy-tui proxies`：列出所有代理分组与节点。
func runProxiesCLI(stdout, stderr io.Writer, args []string, endpoint, secret string) int {
	jsonOut := cli.IsJSONFlag(args)
	if endpoint == "" {
		fmt.Fprintln(stderr, "error: --endpoint is required")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := proxy.NewClient(endpoint, secret)
	resp, err := client.Proxies(ctx)
	if err != nil {
		if jsonOut {
			_ = cli.PrintJSON(stdout, cli.Envelope{OK: false, Error: err.Error()})
			return 1
		}
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if jsonOut {
		_ = cli.PrintJSON(stdout, cli.Envelope{OK: true, Data: resp})
		return 0
	}

	names := make([]string, 0, len(resp.Proxies))
	for n := range resp.Proxies {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		p := resp.Proxies[n]
		now := p.Now
		if now == "" {
			now = "-"
		}
		fmt.Fprintf(stdout, "%-20s %-12s -> %s\n", p.Name, p.Type, now)
	}
	return 0
}

// runConnectionsCLI 实现 `labproxy-tui connections`：列出当前连接。
// 子动作 `close <id|all>`：关闭单条或全部连接。
func runConnectionsCLI(stdout, stderr io.Writer, args []string, endpoint, secret string) int {
	// 识别 close 子动作（首个非 flag 参数）
	rest := stripJSONFlag(args)
	if len(rest) > 0 && rest[0] == "close" {
		return runConnectionsClose(stdout, stderr, args, endpoint, secret)
	}

	jsonOut := cli.IsJSONFlag(args)
	if endpoint == "" {
		fmt.Fprintln(stderr, "error: --endpoint is required")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := proxy.NewClient(endpoint, secret)
	resp, err := client.Connections(ctx)
	if err != nil {
		if jsonOut {
			_ = cli.PrintJSON(stdout, cli.Envelope{OK: false, Error: err.Error()})
			return 1
		}
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if jsonOut {
		_ = cli.PrintJSON(stdout, cli.Envelope{OK: true, Data: resp})
		return 0
	}

	fmt.Fprintf(stdout, "连接数: %d  上传: %d  下载: %d\n", len(resp.Connections), resp.UploadTotal, resp.DownloadTotal)
	for _, c := range resp.Connections {
		host := c.Metadata.Host
		if host == "" {
			host = c.Metadata.Destination
		}
		fmt.Fprintf(stdout, "  %s  %s/%s  %s  chains=%v\n", c.ID, c.Metadata.Network, host, c.Rule, c.Chains)
	}
	return 0
}

// runConnectionsClose 实现 `connections close <id|all>`。
func runConnectionsClose(stdout, stderr io.Writer, args []string, endpoint, secret string) int {
	jsonOut := cli.IsJSONFlag(args)
	// 取 close 之后的第一个非 flag 参数作为目标
	target := ""
	seenClose := false
	for _, a := range args {
		if a == "--json" || strings.HasPrefix(a, "--json=") {
			continue
		}
		if !seenClose {
			if a == "close" {
				seenClose = true
			}
			continue
		}
		if a == "" {
			continue
		}
		target = a
		break
	}

	if target == "" {
		msg := "usage: labproxy connections close <id|all> [--json]"
		if jsonOut {
			_ = cli.PrintJSON(stdout, cli.Envelope{OK: false, Error: msg})
		} else {
			fmt.Fprintln(stderr, msg)
		}
		return 2
	}
	if endpoint == "" {
		fmt.Fprintln(stderr, "error: --endpoint is required")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := proxy.NewClient(endpoint, secret)

	var err error
	if target == "all" {
		err = client.CloseAllConnections(ctx)
	} else {
		err = client.CloseConnection(ctx, target)
	}
	if err != nil {
		if jsonOut {
			_ = cli.PrintJSON(stdout, cli.Envelope{OK: false, Error: err.Error()})
			return 1
		}
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if jsonOut {
		_ = cli.PrintJSON(stdout, cli.Envelope{OK: true, Data: map[string]string{"closed": target}})
		return 0
	}
	fmt.Fprintf(stdout, "已关闭连接: %s\n", target)
	return 0
}

// stripJSONFlag 返回去掉 --json 后的参数列表（保留其他参数顺序）。
func stripJSONFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--json" || strings.HasPrefix(a, "--json=") {
			continue
		}
		out = append(out, a)
	}
	return out
}

// runDelayCLI 实现 `labproxy-tui delay <name>`：测单节点延迟。
func runDelayCLI(stdout, stderr io.Writer, args []string, endpoint, secret string) int {
	jsonOut := cli.IsJSONFlag(args)

	// 剥离 --json，取首个非 flag 参数作为节点名
	var name string
	for _, a := range args {
		if a == "--json" || strings.HasPrefix(a, "--json=") {
			continue
		}
		if a == "" {
			continue
		}
		name = a
		break
	}

	if name == "" {
		fmt.Fprintln(stderr, "usage: labproxy delay <name> [--json]")
		return 2
	}
	if endpoint == "" {
		fmt.Fprintln(stderr, "error: --endpoint is required")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := proxy.NewClient(endpoint, secret)
	delay, err := client.Delay(ctx, name, 5*time.Second)
	if err != nil {
		if jsonOut {
			_ = cli.PrintJSON(stdout, cli.Envelope{OK: false, Error: err.Error()})
			return 1
		}
		fmt.Fprintf(stderr, "delay error: %v\n", err)
		return 1
	}

	if jsonOut {
		_ = cli.PrintJSON(stdout, cli.Envelope{OK: true, Data: map[string]any{"name": name, "delay": delay}})
		return 0
	}
	fmt.Fprintf(stdout, "%s: %d ms\n", name, delay)
	return 0
}

// groupProxyTypes 是 mihomo 中可作为"代理组"的类型。
var groupProxyTypes = map[string]bool{
	"Selector":     true,
	"URLTest":      true,
	"Fallback":     true,
	"LoadBalance":  true,
	"Relay":        true,
}

// isGroupProxy 判断是否为代理组（含多个候选节点）。
func isGroupProxy(p proxy.Proxy) bool {
	return groupProxyTypes[p.Type] && len(p.All) > 0
}

// runTestCLI 实现 `labproxy-tui test [group]`：批量测速并按延迟排序。
// 不指定 group 时默认测 GLOBAL，无 GLOBAL 则取第一个组。
func runTestCLI(stdout, stderr io.Writer, args []string, endpoint, secret string) int {
	jsonOut := cli.IsJSONFlag(args)
	if endpoint == "" {
		fmt.Fprintln(stderr, "error: --endpoint is required")
		return 2
	}

	// 取首个非 flag 参数作为组名
	groupName := ""
	for _, a := range args {
		if a == "--json" || strings.HasPrefix(a, "--json=") {
			continue
		}
		if a == "" {
			continue
		}
		groupName = a
		break
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := proxy.NewClient(endpoint, secret)
	resp, err := client.Proxies(ctx)
	if err != nil {
		if jsonOut {
			_ = cli.PrintJSON(stdout, cli.Envelope{OK: false, Error: err.Error()})
			return 1
		}
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	// 选组：指定名 → GLOBAL → 第一个组
	var group proxy.Proxy
	found := false
	if groupName != "" {
		if g, ok := resp.Proxies[groupName]; ok && isGroupProxy(g) {
			group, found = g, true
		}
	} else {
		if g, ok := resp.Proxies["GLOBAL"]; ok && isGroupProxy(g) {
			group, found = g, true
		} else {
			for _, p := range resp.Proxies {
				if isGroupProxy(p) {
					group, found = p, true
					break
				}
			}
		}
	}

	if !found {
		msg := fmt.Sprintf("group not found: %s", fallbackName(groupName))
		if groupName == "" {
			msg = "no proxy group available"
		}
		if jsonOut {
			_ = cli.PrintJSON(stdout, cli.Envelope{OK: false, Error: msg})
		} else {
			fmt.Fprintf(stderr, "error: %s\n", msg)
		}
		return 1
	}

	results, err := client.DelayGroup(ctx, group, 5*time.Second)
	if err != nil {
		if jsonOut {
			_ = cli.PrintJSON(stdout, cli.Envelope{OK: false, Error: err.Error()})
			return 1
		}
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if jsonOut {
		_ = cli.PrintJSON(stdout, cli.Envelope{OK: true, Data: map[string]any{
			"group":   group.Name,
			"results": results,
		}})
		return 0
	}

	// 人读：按延迟升序排序，-1（失败）排末尾
	type item struct {
		name  string
		delay int
	}
	items := make([]item, 0, len(results))
	for n, d := range results {
		items = append(items, item{n, d})
	}
	sort.Slice(items, func(i, j int) bool {
		if (items[i].delay == -1) != (items[j].delay == -1) {
			return items[j].delay == -1 // 失败排后
		}
		return items[i].delay < items[j].delay
	})
	fmt.Fprintf(stdout, "组: %s\n", group.Name)
	for _, it := range items {
		if it.delay == -1 {
			fmt.Fprintf(stdout, "  %-30s timeout\n", it.name)
		} else {
			fmt.Fprintf(stdout, "  %-30s %d ms\n", it.name, it.delay)
		}
	}
	return 0
}

func fallbackName(s string) string {
	if s == "" {
		return "<empty>"
	}
	return s
}

// logTailLines 是无 -f 时读取的最近日志行数。
const logTailLines = 50

// runLogsCLI 实现 `labproxy-tui logs [-f] [--level LEVEL] [--json]`。
// -f：订阅 mihomo /logs 流式输出；无 -f：读 ~/.labproxy/logs/labproxy.log 最近 N 行。
func runLogsCLI(stdout, stderr io.Writer, args []string, home, endpoint, secret string) int {
	if hasFlag(args, "-f") || hasFlag(args, "--follow") {
		return runLogsCLIFollow(stdout, stderr, args, endpoint, secret)
	}

	// 读文件最近 N 行
	logPath := home + "/.labproxy/logs/labproxy.log"
	data, err := os.ReadFile(logPath)
	if err != nil {
		if cli.IsJSONFlag(args) {
			_ = cli.PrintJSON(stdout, cli.Envelope{OK: false, Error: "read log file: " + err.Error()})
			return 1
		}
		fmt.Fprintf(stderr, "error: read log file: %v\n", err)
		return 1
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > logTailLines {
		lines = lines[len(lines)-logTailLines:]
	}
	for _, l := range lines {
		fmt.Fprintln(stdout, l)
	}
	return 0
}

// runLogsCLIFollow 订阅 mihomo /logs 流并逐行输出。
// 读到流结束（EOF/连接关闭）即返回；生产中由 SIGINT 中断进程。
func runLogsCLIFollow(stdout, stderr io.Writer, args []string, endpoint, secret string) int {
	jsonOut := cli.IsJSONFlag(args)
	level := flagValue(args, "--level")
	if level == "" {
		level = "info"
	}
	if endpoint == "" {
		fmt.Fprintln(stderr, "error: --endpoint is required for -f")
		return 2
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := proxy.NewClient(endpoint, secret)
	ch := client.Logs(ctx, level)

	for e := range ch {
		if jsonOut {
			_ = cli.PrintJSON(stdout, cli.Envelope{OK: true, Data: map[string]string{
				"level":   e.Level,
				"payload": e.Payload,
			}})
		} else {
			fmt.Fprintf(stdout, "[%s] %s\n", e.Level, e.Payload)
		}
	}
	return 0
}

// hasFlag 检测 args 是否含指定 flag。
func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// flagValue 取 --flag value 或 --flag=value 的值。
func flagValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(a, flag+"=") {
			return strings.TrimPrefix(a, flag+"=")
		}
	}
	return ""
}

// runDNSCLI 实现 `labproxy-tui dns <name> [--type TYPE] [--json]`。
// 调 mihomo /dns/query 解析域名，输出 Question/Answer。
func runDNSCLI(stdout, stderr io.Writer, args []string, endpoint, secret string) int {
	jsonOut := cli.IsJSONFlag(args)

	// 提取 --type 与首个非 flag 参数作为 name
	qtype := flagValue(args, "--type")
	if qtype == "" {
		qtype = "A"
	}
	name := ""
	for _, a := range args {
		if a == "--json" || strings.HasPrefix(a, "--json=") {
			continue
		}
		if a == "--type" || strings.HasPrefix(a, "--type=") {
			continue
		}
		if a == "" {
			continue
		}
		name = a
		break
	}

	if name == "" {
		msg := "usage: labproxy dns <name> [--type A|AAAA|CNAME|...] [--json]"
		if jsonOut {
			_ = cli.PrintJSON(stdout, cli.Envelope{OK: false, Error: msg})
		} else {
			fmt.Fprintln(stderr, msg)
		}
		return 2
	}
	if endpoint == "" {
		fmt.Fprintln(stderr, "error: --endpoint is required")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	client := proxy.NewClient(endpoint, secret)
	resp, err := client.DNSQuery(ctx, name, qtype)
	if err != nil {
		if jsonOut {
			_ = cli.PrintJSON(stdout, cli.Envelope{OK: false, Error: err.Error()})
			return 1
		}
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if jsonOut {
		_ = cli.PrintJSON(stdout, cli.Envelope{OK: true, Data: resp})
		return 0
	}

	fmt.Fprintf(stdout, "查询: %s (type=%s)  状态: %d\n", name, qtype, resp.Status)
	if len(resp.Question) > 0 {
		fmt.Fprintf(stdout, "问题: %s\n", resp.Question[0].Name)
	}
	if len(resp.Answer) == 0 {
		fmt.Fprintln(stdout, "无应答记录")
	} else {
		fmt.Fprintln(stdout, "应答:")
		for _, a := range resp.Answer {
			fmt.Fprintf(stdout, "  %s  TTL=%d  %s\n", a.Name, a.TTL, a.Data)
		}
	}
	return 0
}
