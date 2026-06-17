package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"labproxy/internal/config"
	"labproxy/internal/proxy"
	"labproxy/internal/tui"
	"labproxy/internal/tui/rules"
)

func main() {
	var (
		endpoint       = flag.String("endpoint", "", "mihomo controller endpoint")
		secret         = flag.String("secret", "", "mihomo controller secret")
		mixinConfig    = flag.String("mixin-config", "", "path to mixin config for system-proxy status")
		restartCommand = flag.String("restart-command", "", "shell command used to restart labproxy runtime")
		lang           = flag.String("lang", "en", "ui language: en or zh")
	)
	flag.Parse()

	if len(os.Args) >= 2 && os.Args[1] == "rules" {
		os.Exit(runRulesCLI(os.Stdout, os.Stderr, os.Args[2:], *mixinConfig))
	}

	if len(os.Args) >= 2 && os.Args[1] == "status" {
		home := os.Getenv("HOME")
		os.Exit(runStatusCLI(os.Stdout, os.Stderr, os.Args[2:], home, *endpoint, *secret))
	}

	if len(os.Args) >= 2 && (os.Args[1] == "proxies" || os.Args[1] == "connections" || os.Args[1] == "delay") {
		sub := os.Args[1]
		args := os.Args[2:]
		// flag.Parse 遇子命令名停止，此处从 args 显式提取 --endpoint/--secret
		args, ep := extractStringFlag(args, "--endpoint")
		args, sec := extractStringFlag(args, "--secret")
		if ep == "" {
			ep = *endpoint
		}
		if sec == "" {
			sec = *secret
		}
		switch sub {
		case "proxies":
			os.Exit(runProxiesCLI(os.Stdout, os.Stderr, args, ep, sec))
		case "connections":
			os.Exit(runConnectionsCLI(os.Stdout, os.Stderr, args, ep, sec))
		case "delay":
			os.Exit(runDelayCLI(os.Stdout, os.Stderr, args, ep, sec))
		}
	}

	if *endpoint == "" {
		fmt.Fprintln(os.Stderr, "missing required --endpoint")
		os.Exit(1)
	}

	systemProxyEnabled, err := config.ReadSystemProxyEnabled(*mixinConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read mixin config: %v\n", err)
		os.Exit(1)
	}

	rulesModal := rules.NewModal(*mixinConfig)

	app := tui.NewApp(proxy.NewClient(*endpoint, *secret), tui.Options{
		Endpoint:           *endpoint,
		SystemProxyEnabled: systemProxyEnabled,
		MixinConfigPath:    *mixinConfig,
		RestartCommand:     *restartCommand,
		RulesModal:         rulesModal,
	})

	tui.SetLanguage(tui.Language(*lang))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := app.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
