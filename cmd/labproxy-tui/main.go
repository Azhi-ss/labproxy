package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"labproxy/internal/config"
	"labproxy/internal/proxy"
	"labproxy/internal/tui"
)

func main() {
	var (
		endpoint       = flag.String("endpoint", "", "mihomo controller endpoint")
		secret         = flag.String("secret", "", "mihomo controller secret")
		mixinConfig    = flag.String("mixin-config", "", "path to mixin config for system-proxy status")
		restartCommand = flag.String("restart-command", "", "shell command used to restart labproxy runtime")
	)
	flag.Parse()

	if *endpoint == "" {
		fmt.Fprintln(os.Stderr, "missing required --endpoint")
		os.Exit(1)
	}

	systemProxyEnabled, err := config.ReadSystemProxyEnabled(*mixinConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read mixin config: %v\n", err)
		os.Exit(1)
	}

	app := tui.NewApp(proxy.NewClient(*endpoint, *secret), tui.Options{
		Endpoint:           *endpoint,
		SystemProxyEnabled: systemProxyEnabled,
		MixinConfigPath:    *mixinConfig,
		RestartCommand:     *restartCommand,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := app.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
