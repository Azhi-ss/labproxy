package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"labproxy/internal/cli"
	"labproxy/internal/profile"
	"labproxy/internal/proxy"
)

// doctorCheck 是单项诊断结果。
type doctorCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// runDoctorCLI 实现 `labproxy-tui doctor [--json]`：结构化诊断。
// 覆盖：内核 API 可达、连接数、配置文件、profile、rules、config 目录。
// 单项失败不阻断整体，envelope.ok 始终 true（data.checks 内含每项 ok）。
func runDoctorCLI(stdout, stderr io.Writer, args []string, home, endpoint, secret string) int {
	_ = stderr
	jsonOut := cli.IsJSONFlag(args)
	labDir := filepath.Join(home, ".labproxy")

	checks := []doctorCheck{}

	// 1. 内核 API 可达 + 连接数
	if endpoint != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		client := proxy.NewClient(endpoint, secret)
		if _, err := client.Version(ctx); err != nil {
			checks = append(checks, doctorCheck{Name: "kernel_api", OK: false, Detail: err.Error()})
		} else {
			checks = append(checks, doctorCheck{Name: "kernel_api", OK: true, Detail: endpoint})
		}
		cancel()

		ctx2, cancel2 := context.WithTimeout(context.Background(), 4*time.Second)
		if conns, err := client.Connections(ctx2); err == nil {
			checks = append(checks, doctorCheck{Name: "connections", OK: true, Detail: fmt.Sprintf("%d active", len(conns.Connections))})
		} else {
			checks = append(checks, doctorCheck{Name: "connections", OK: false, Detail: err.Error()})
		}
		cancel2()
	} else {
		checks = append(checks, doctorCheck{Name: "kernel_api", OK: false, Detail: "endpoint not configured"})
	}

	// 2. mixin.yaml
	mixinPath := filepath.Join(labDir, "mixin.yaml")
	if info, err := os.Stat(mixinPath); err == nil && !info.IsDir() {
		checks = append(checks, doctorCheck{Name: "mixin_config", OK: true, Detail: fmt.Sprintf("%d bytes", info.Size())})
	} else {
		checks = append(checks, doctorCheck{Name: "mixin_config", OK: false, Detail: "mixin.yaml missing"})
	}

	// 3. rules.yaml
	rulesPath := filepath.Join(labDir, "rules.yaml")
	if info, err := os.Stat(rulesPath); err == nil && !info.IsDir() {
		checks = append(checks, doctorCheck{Name: "rules_config", OK: true, Detail: fmt.Sprintf("%d bytes", info.Size())})
	} else {
		checks = append(checks, doctorCheck{Name: "rules_config", OK: false, Detail: "rules.yaml missing"})
	}

	// 4. profiles
	if store, err := profile.NewStore(labDir); err == nil {
		names, _ := store.List()
		checks = append(checks, doctorCheck{Name: "profiles", OK: true, Detail: fmt.Sprintf("%d profiles", len(names))})
	} else {
		checks = append(checks, doctorCheck{Name: "profiles", OK: false, Detail: err.Error()})
	}

	// 5. config 目录
	if info, err := os.Stat(filepath.Join(labDir, "config")); err == nil && info.IsDir() {
		checks = append(checks, doctorCheck{Name: "config_dir", OK: true, Detail: "exists"})
	} else {
		checks = append(checks, doctorCheck{Name: "config_dir", OK: false, Detail: "missing"})
	}

	if jsonOut {
		_ = cli.PrintJSON(stdout, cli.Envelope{OK: true, Data: map[string]any{"checks": checks}})
		return 0
	}

	// 人读
	fmt.Fprintln(stdout, "🩺 LabProxy 诊断 (agent-native)")
	failed := 0
	for _, c := range checks {
		mark := "✅"
		if !c.OK {
			mark = "❌"
			failed++
		}
		fmt.Fprintf(stdout, "  %s %-16s %s\n", mark, c.Name, c.Detail)
	}
	if failed > 0 {
		fmt.Fprintf(stdout, "\n%d 项检查失败\n", failed)
	} else {
		fmt.Fprintln(stdout, "\n所有检查通过")
	}
	return 0
}
