package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"labproxy/internal/rules"
)

func runRulesCLI(stdout, stderr io.Writer, args []string, mixinPath string) int {
	// Allow --mixin-config to appear after `rules` (flag.Parse stops at the
	// first non-flag arg, so a trailing --mixin-config would not populate the
	// global flag). Extract it here and strip it from args before dispatch.
	args, mixinPath = extractMixinFlag(args, mixinPath)
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: labproxy-tui rules <subcommand> [...]")
		return 1
	}
	if mixinPath == "" {
		fmt.Fprintln(stderr, "error: --mixin-config is required for rules CLI")
		return 2
	}
	store, err := rules.NewStore(mixinPath)
	if err != nil {
		fmt.Fprintf(stderr, "open store: %v\n", err)
		return 2
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "list":
		return cliListRules(stdout, store, rest)
	case "add":
		return cliAddRule(stdout, stderr, store, rest)
	case "delete", "rm":
		return cliDeleteRule(stdout, stderr, store, rest)
	case "enable":
		return cliToggleRule(stdout, store, rest, true)
	case "disable":
		return cliToggleRule(stdout, store, rest, false)
	case "move":
		return cliMoveRule(stdout, store, rest)
	case "edit":
		return cliEditRule(stdout, stderr, store, rest)
	case "import":
		return cliImport(stdout, stderr, store, rest)
	case "export":
		return cliExport(stdout, store, rest)
	case "providers":
		return cliProviders(stdout, stderr, store, rest)
	case "workflow":
		return cliWorkflow(stdout, stderr, store, rest)
	case "reset":
		return cliReset(stdout, stderr, store, rest)
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q\n", sub)
		return 1
	}
}

// extractMixinFlag scans args for --mixin-config=PATH or --mixin-config PATH,
// returning the remaining args (with the flag removed) and the resolved path.
// A flag found here overrides the caller-provided mixinPath.
func extractMixinFlag(args []string, mixinPath string) ([]string, string) {
	const flagName = "--mixin-config"
	out := make([]string, 0, len(args))
	resolved := mixinPath
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == flagName:
			if i+1 < len(args) {
				resolved = args[i+1]
				i++
			}
		case strings.HasPrefix(a, flagName+"="):
			resolved = strings.TrimPrefix(a, flagName+"=")
		default:
			out = append(out, a)
		}
	}
	return out, resolved
}

func cliListRules(w io.Writer, s *rules.Store, args []string) int {
	var typeFilter string
	enabledOnly := false
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--type="):
			typeFilter = strings.TrimPrefix(a, "--type=")
		case a == "--enabled":
			enabledOnly = true
		}
	}
	rs, err := s.LoadRules()
	if err != nil {
		fmt.Fprintf(w, "error: %v\n", err)
		return 2
	}
	fmt.Fprintf(w, "%-5s %-7s %-15s %-30s %s\n", "INDEX", "ENABLED", "TYPE", "PAYLOAD", "PROXY")
	for i, r := range rs {
		if typeFilter != "" && string(r.Type) != typeFilter {
			continue
		}
		if enabledOnly && !r.Enabled {
			continue
		}
		marker := "○"
		if r.Enabled {
			marker = "●"
		}
		fmt.Fprintf(w, "%-5d %-7s %-15s %-30s %s\n", i, marker, r.Type, r.Payload, r.Proxy)
	}
	return 0
}

func cliAddRule(w, e io.Writer, s *rules.Store, args []string) int {
	var rType, payload, proxy string
	noResolve := false
	at := -1
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--type="):
			rType = strings.TrimPrefix(a, "--type=")
		case strings.HasPrefix(a, "--payload="):
			payload = strings.TrimPrefix(a, "--payload=")
		case strings.HasPrefix(a, "--proxy="):
			proxy = strings.TrimPrefix(a, "--proxy=")
		case a == "--no-resolve":
			noResolve = true
		case strings.HasPrefix(a, "--at="):
			at, _ = strconv.Atoi(strings.TrimPrefix(a, "--at="))
		}
	}
	if rType == "" || proxy == "" {
		fmt.Fprintln(e, "error: --type and --proxy are required")
		return 1
	}
	r := rules.Rule{Type: rules.RuleType(rType), Payload: payload, Proxy: proxy, NoResolve: noResolve, Enabled: true}
	if err := r.Validate(); err != nil {
		fmt.Fprintln(e, err)
		return 1
	}
	if at >= 0 {
		existing, _ := s.LoadRules()
		if at > len(existing) {
			at = len(existing)
		}
		out := make([]rules.Rule, 0, len(existing)+1)
		out = append(out, existing[:at]...)
		out = append(out, r)
		out = append(out, existing[at:]...)
		if err := s.SaveRules(out); err != nil {
			fmt.Fprintln(e, err)
			return 2
		}
	} else {
		if _, err := s.AddRule(r); err != nil {
			fmt.Fprintln(e, err)
			return 1
		}
	}
	fmt.Fprintln(w, "ok")
	return 0
}

func cliDeleteRule(w, e io.Writer, s *rules.Store, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(e, "usage: rules delete <index>")
		return 1
	}
	idx, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintln(e, err)
		return 1
	}
	if _, err := s.DeleteRule(idx); err != nil {
		fmt.Fprintln(e, err)
		return 2
	}
	fmt.Fprintln(w, "ok")
	return 0
}

func cliToggleRule(w io.Writer, s *rules.Store, args []string, wantEnabled bool) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: rules <enable|disable> <index>")
		return 1
	}
	idx, err := strconv.Atoi(args[0])
	if err != nil {
		return 1
	}
	rulesList, err := s.LoadRules()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if idx < 0 || idx >= len(rulesList) {
		return 1
	}
	if rulesList[idx].Enabled != wantEnabled {
		if _, err := s.ToggleRule(idx); err != nil {
			return 2
		}
	}
	fmt.Fprintln(w, "ok")
	return 0
}

func cliMoveRule(w io.Writer, s *rules.Store, args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: rules move <from> <to>")
		return 1
	}
	from, _ := strconv.Atoi(args[0])
	to, _ := strconv.Atoi(args[1])
	if _, err := s.MoveRule(from, to); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	fmt.Fprintln(w, "ok")
	return 0
}

func cliEditRule(w, e io.Writer, s *rules.Store, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(e, "usage: rules edit <index> [--type=X ...]")
		return 1
	}
	idx, err := strconv.Atoi(args[0])
	if err != nil {
		return 1
	}
	existing, _ := s.LoadRules()
	if idx < 0 || idx >= len(existing) {
		return 1
	}
	updated := existing[idx]
	for _, a := range args[1:] {
		switch {
		case strings.HasPrefix(a, "--type="):
			updated.Type = rules.RuleType(strings.TrimPrefix(a, "--type="))
		case strings.HasPrefix(a, "--payload="):
			updated.Payload = strings.TrimPrefix(a, "--payload=")
		case strings.HasPrefix(a, "--proxy="):
			updated.Proxy = strings.TrimPrefix(a, "--proxy=")
		}
	}
	if _, err := s.UpdateRule(idx, updated); err != nil {
		fmt.Fprintln(e, err)
		return 1
	}
	fmt.Fprintln(w, "ok")
	return 0
}

func cliImport(w, e io.Writer, s *rules.Store, args []string) int {
	var source, mode string
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--source="):
			source = strings.TrimPrefix(a, "--source=")
		case strings.HasPrefix(a, "--mode="):
			mode = strings.TrimPrefix(a, "--mode=")
		}
	}
	if source == "" {
		fmt.Fprintln(e, "usage: rules import --source=url|file:/path|preset:NAME")
		return 1
	}
	parts := strings.SplitN(source, ":", 2)
	if len(parts) != 2 {
		fmt.Fprintln(e, "invalid --source format")
		return 1
	}
	if mode == "" {
		mode = "append"
	}
	src := rules.ImportSource{Kind: parts[0], Ref: parts[1]}
	if _, err := s.Import(src, mode); err != nil {
		fmt.Fprintln(e, err)
		return 1
	}
	fmt.Fprintln(w, "ok")
	return 0
}

func cliExport(w io.Writer, s *rules.Store, args []string) int {
	out := "./labproxy-rules-export.yaml"
	for _, a := range args {
		if strings.HasPrefix(a, "--out=") {
			out = strings.TrimPrefix(a, "--out=")
		}
	}
	abs, _ := filepath.Abs(out)
	if err := rules.Export(s, abs, false); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	fmt.Fprintln(w, "ok")
	return 0
}

func cliProviders(w, e io.Writer, s *rules.Store, args []string) int {
	if len(args) == 0 {
		return cliProvidersList(w, s)
	}
	switch args[0] {
	case "list":
		return cliProvidersList(w, s)
	case "add":
		return cliProviderAdd(w, e, s, args[1:])
	case "delete", "rm":
		return cliProviderDelete(w, e, s, args[1:])
	case "refresh":
		if len(args) < 2 {
			fmt.Fprintln(e, "usage: rules providers refresh <name>")
			return 1
		}
		if err := s.RefreshProvider(args[1]); err != nil {
			fmt.Fprintln(e, err)
			return 2
		}
		fmt.Fprintln(w, "ok")
		return 0
	default:
		fmt.Fprintln(e, "unknown providers subcommand")
		return 1
	}
}

func cliProvidersList(w io.Writer, s *rules.Store) int {
	ps, _ := s.LoadProviders()
	fmt.Fprintf(w, "%-15s %-6s %-10s %-10s %s\n", "NAME", "TYPE", "BEHAVIOR", "INTERVAL", "URL")
	for _, p := range ps {
		fmt.Fprintf(w, "%-15s %-6s %-10s %-10d %s\n", p.Name, p.Type, p.Behavior, p.Interval, p.URL)
	}
	return 0
}

func cliProviderAdd(w, e io.Writer, s *rules.Store, args []string) int {
	p := rules.Provider{}
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--name="):
			p.Name = strings.TrimPrefix(a, "--name=")
		case strings.HasPrefix(a, "--type="):
			p.Type = strings.TrimPrefix(a, "--type=")
		case strings.HasPrefix(a, "--behavior="):
			p.Behavior = strings.TrimPrefix(a, "--behavior=")
		case strings.HasPrefix(a, "--url="):
			p.URL = strings.TrimPrefix(a, "--url=")
		case strings.HasPrefix(a, "--path="):
			p.Path = strings.TrimPrefix(a, "--path=")
		case strings.HasPrefix(a, "--interval="):
			n, _ := strconv.Atoi(strings.TrimPrefix(a, "--interval="))
			p.Interval = n
		}
	}
	if _, err := s.AddProvider(p); err != nil {
		fmt.Fprintln(e, err)
		return 1
	}
	fmt.Fprintln(w, "ok")
	return 0
}

func cliProviderDelete(w, e io.Writer, s *rules.Store, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(e, "usage: rules providers delete <name>")
		return 1
	}
	if _, err := s.DeleteProvider(args[0]); err != nil {
		fmt.Fprintln(e, err)
		return 1
	}
	fmt.Fprintln(w, "ok")
	return 0
}

func cliReset(w, e io.Writer, s *rules.Store, args []string) int {
	force := false
	for _, a := range args {
		if a == "-y" {
			force = true
		}
	}
	if !force {
		fmt.Fprintln(e, "this will remove all user rules; pass -y to confirm")
		return 1
	}
	if _, err := s.ResetRules(); err != nil {
		fmt.Fprintln(e, err)
		return 2
	}
	fmt.Fprintln(w, "ok")
	return 0
}
