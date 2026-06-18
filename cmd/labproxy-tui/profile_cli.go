package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"labproxy/internal/cli"
	"labproxy/internal/profile"
)

// runProfileCLI 实现 `labproxy-tui profile <subcommand> [name] [--json]`。
// 子命令：list / create <name> / delete <name> / use <name>。
// home 为 labproxy home 目录（含 ~/.labproxy）。
func runProfileCLI(stdout, stderr io.Writer, args []string, home string) int {
	jsonOut := cli.IsJSONFlag(args)

	sub, name := parseProfileArgs(args)
	if sub == "" {
		msg := "usage: labproxy profile <list|create|delete|use> [name] [--json]"
		if jsonOut {
			_ = cli.PrintJSON(stdout, cli.Envelope{OK: false, Error: msg})
		} else {
			fmt.Fprintln(stderr, msg)
		}
		return 2
	}

	labDir := filepath.Join(home, ".labproxy")
	store, err := profile.NewStore(labDir)
	if err != nil {
		return profileErr(stdout, stderr, jsonOut, "init store: %v", err)
	}

	switch sub {
	case "list":
		names, err := store.List()
		if err != nil {
			return profileErr(stdout, stderr, jsonOut, "list: %v", err)
		}
		if jsonOut {
			_ = cli.PrintJSON(stdout, cli.Envelope{OK: true, Data: names})
			return 0
		}
		if len(names) == 0 {
			fmt.Fprintln(stdout, "无 profile（用 'labproxy profile create <name>' 从当前配置创建）")
			return 0
		}
		for _, n := range names {
			fmt.Fprintln(stdout, n)
		}
		return 0

	case "create":
		if name == "" {
			return profileErr(stdout, stderr, jsonOut, "create 需要 profile 名称")
		}
		mixin, err := os.ReadFile(filepath.Join(labDir, "mixin.yaml"))
		if err != nil {
			return profileErr(stdout, stderr, jsonOut, "read current mixin: %v", err)
		}
		rules, err := os.ReadFile(filepath.Join(labDir, "rules.yaml"))
		if err != nil {
			rules = []byte("rules: []\n")
		}
		p := profile.Profile{Name: name, Mixin: mixin, Rules: rules}
		if err := store.Create(p); err != nil {
			return profileErr(stdout, stderr, jsonOut, "create: %v", err)
		}
		if jsonOut {
			_ = cli.PrintJSON(stdout, cli.Envelope{OK: true, Data: map[string]string{"name": name}})
		} else {
			fmt.Fprintf(stdout, "已创建 profile: %s\n", name)
		}
		return 0

	case "delete":
		if name == "" {
			return profileErr(stdout, stderr, jsonOut, "delete 需要 profile 名称")
		}
		if err := store.Delete(name); err != nil {
			return profileErr(stdout, stderr, jsonOut, "delete: %v", err)
		}
		if jsonOut {
			_ = cli.PrintJSON(stdout, cli.Envelope{OK: true, Data: map[string]string{"deleted": name}})
		} else {
			fmt.Fprintf(stdout, "已删除 profile: %s\n", name)
		}
		return 0

	case "use":
		if name == "" {
			return profileErr(stdout, stderr, jsonOut, "use 需要 profile 名称")
		}
		p, err := store.Load(name)
		if err != nil {
			return profileErr(stdout, stderr, jsonOut, "load: %v", err)
		}
		if err := atomicWriteFile(filepath.Join(labDir, "mixin.yaml"), p.Mixin); err != nil {
			return profileErr(stdout, stderr, jsonOut, "apply mixin: %v", err)
		}
		if len(p.Rules) > 0 {
			if err := atomicWriteFile(filepath.Join(labDir, "rules.yaml"), p.Rules); err != nil {
				return profileErr(stdout, stderr, jsonOut, "apply rules: %v", err)
			}
		}
		if jsonOut {
			_ = cli.PrintJSON(stdout, cli.Envelope{OK: true, Data: map[string]string{"used": name}})
		} else {
			fmt.Fprintf(stdout, "已切换到 profile: %s\n", name)
			fmt.Fprintln(stdout, "提示：执行 'labproxy restart' 使配置生效")
		}
		return 0

	default:
		return profileErr(stdout, stderr, jsonOut, "未知子命令: %s", sub)
	}
}

// parseProfileArgs 从 args 提取子命令与名称（跳过 --json）。
func parseProfileArgs(args []string) (sub, name string) {
	picked := 0
	for _, a := range args {
		if a == "--json" || (len(a) > 7 && a[:7] == "--json=") {
			continue
		}
		if a == "" {
			continue
		}
		switch picked {
		case 0:
			sub = a
		case 1:
			name = a
		}
		picked++
	}
	return sub, name
}

// atomicWriteFile 原子写入文件（tmp + rename）。
func atomicWriteFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// profileErr 输出错误并返回非零退出码。
func profileErr(stdout, stderr io.Writer, jsonOut bool, format string, args ...any) int {
	msg := fmt.Sprintf(format, args...)
	if jsonOut {
		_ = cli.PrintJSON(stdout, cli.Envelope{OK: false, Error: msg})
	} else {
		fmt.Fprintf(stderr, "error: %s\n", msg)
	}
	return 1
}
