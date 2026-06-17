package main

import (
	"context"
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
