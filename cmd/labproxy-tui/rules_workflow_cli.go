package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"labproxy/internal/proxy"
	"labproxy/internal/rules"
	"labproxy/internal/ruleworkflow"
)

type workflowOptions struct {
	candidates   string
	groups       string
	endpoint     string
	secret       string
	reloadConfig string
	backup       string
	overrides    urlOverrides
}

type urlOverrides map[string]string

func (o urlOverrides) String() string {
	if len(o) == 0 {
		return ""
	}
	keys := make([]string, 0, len(o))
	for key := range o {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+o[key])
	}
	return strings.Join(parts, ",")
}

func (o urlOverrides) Set(value string) error {
	name, url, ok := strings.Cut(value, "=")
	if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(url) == "" {
		return fmt.Errorf("--url-override must be name=url")
	}
	o[strings.TrimSpace(name)] = strings.TrimSpace(url)
	return nil
}

func cliWorkflow(stdout, stderr io.Writer, store *rules.Store, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: rules workflow <candidates|inspect|fetch|validate|plan|apply|verify|rollback> [flags]")
		return 1
	}

	subcommand := args[0]
	opts, ok := parseWorkflowOptions(stderr, args[1:])
	if !ok {
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch subcommand {
	case "candidates":
		return workflowCandidates(stdout, stderr, opts)
	case "inspect":
		return workflowInspect(stdout, stderr, store)
	case "fetch":
		return workflowFetch(ctx, stdout, stderr, opts)
	case "validate":
		return workflowValidate(ctx, stdout, stderr, opts)
	case "plan":
		return workflowPlan(stdout, stderr, opts)
	case "apply":
		return workflowApply(ctx, stdout, stderr, store, opts)
	case "verify":
		return workflowVerify(ctx, stdout, stderr, opts)
	case "rollback":
		return workflowRollback(stdout, stderr, store, opts)
	default:
		fmt.Fprintf(stderr, "unknown workflow subcommand %q\n", subcommand)
		return 1
	}
}

func parseWorkflowOptions(stderr io.Writer, args []string) (workflowOptions, bool) {
	opts := workflowOptions{overrides: urlOverrides{}}
	fs := flag.NewFlagSet("rules workflow", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.candidates, "candidates", "", "comma-separated candidate names")
	fs.StringVar(&opts.groups, "groups", "", "comma-separated strategy group names")
	fs.StringVar(&opts.endpoint, "endpoint", "", "mihomo controller endpoint")
	fs.StringVar(&opts.secret, "secret", "", "mihomo controller secret")
	fs.StringVar(&opts.reloadConfig, "reload-config", "", "config path to reload before verify")
	fs.StringVar(&opts.backup, "backup", "", "backup path for rollback")
	fs.Var(opts.overrides, "url-override", "candidate source override as name=url")
	if err := fs.Parse(args); err != nil {
		return workflowOptions{}, false
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n", fs.Arg(0))
		return workflowOptions{}, false
	}
	return opts, true
}

func workflowCandidates(stdout, stderr io.Writer, opts workflowOptions) int {
	candidates, err := selectedWorkflowCandidates(opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for _, candidate := range candidates {
		fmt.Fprintf(stdout, "%s target=%s behavior=%s url=%s description=%s\n", candidate.Name, candidate.TargetGroup, candidate.Provider.Behavior, candidate.SourceURL, candidate.Description)
	}
	return 0
}

func workflowInspect(stdout, stderr io.Writer, store *rules.Store) int {
	ruleList, err := store.LoadRules()
	if err != nil {
		fmt.Fprintf(stderr, "load rules: %v\n", err)
		return 2
	}
	providers, err := store.LoadProviders()
	if err != nil {
		fmt.Fprintf(stderr, "load providers: %v\n", err)
		return 2
	}
	names := make([]string, 0, len(providers))
	for _, provider := range providers {
		names = append(names, provider.Name)
	}
	sort.Strings(names)
	fmt.Fprintf(stdout, "rules=%d providers=%d", len(ruleList), len(providers))
	if len(names) > 0 {
		fmt.Fprintf(stdout, " names=%s", strings.Join(names, ","))
	}
	fmt.Fprintln(stdout)
	return 0
}

func workflowFetch(ctx context.Context, stdout, stderr io.Writer, opts workflowOptions) int {
	candidates, err := selectedWorkflowCandidates(opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	sources, err := fetchWorkflowSources(ctx, candidates)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	for _, source := range sources {
		parsed, err := ruleworkflow.ParseProviderRulesForBehavior(source.Data, source.Candidate.Provider.Behavior)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", source.Candidate.Name, err)
			return 2
		}
		fmt.Fprintf(stdout, "%s rules=%d\n", source.Candidate.Name, len(parsed))
	}
	return 0
}

func workflowValidate(ctx context.Context, stdout, stderr io.Writer, opts workflowOptions) int {
	results, err := validateWorkflow(ctx, opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for _, result := range results {
		fmt.Fprintf(stdout, "%s rules=%d\n", result.Candidate.Name, result.RuleCount)
	}
	return 0
}

func workflowPlan(stdout, stderr io.Writer, opts workflowOptions) int {
	candidates, err := selectedWorkflowCandidates(opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprint(stdout, ruleworkflow.RenderPlan(ruleworkflow.BuildPlan(candidates)))
	return 0
}

func workflowApply(ctx context.Context, stdout, stderr io.Writer, store *rules.Store, opts workflowOptions) int {
	if _, err := validateWorkflow(ctx, opts); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	candidates, err := selectedWorkflowCandidates(opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	result, err := ruleworkflow.ApplyPlan(store, ruleworkflow.BuildPlan(candidates))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintln(stdout, "ok")
	fmt.Fprintf(stdout, "backup=%s\n", result.BackupPath)
	return 0
}

func workflowVerify(ctx context.Context, stdout, stderr io.Writer, opts workflowOptions) int {
	if strings.TrimSpace(opts.endpoint) == "" {
		fmt.Fprintln(stderr, "error: --endpoint is required for verify")
		return 1
	}
	client := proxy.NewClient(opts.endpoint, opts.secret)
	if opts.reloadConfig != "" {
		if err := client.ReloadConfig(ctx, opts.reloadConfig); err != nil {
			fmt.Fprintf(stderr, "reload config: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "reloaded=%s\n", opts.reloadConfig)
	}
	summary, err := ruleworkflow.InspectRuntime(ctx, client)
	if err != nil {
		fmt.Fprintf(stderr, "inspect runtime: %v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "strategy-groups=%s connections=%d\n", strings.Join(sortedTrueKeys(summary.StrategyGroups), ","), summary.ConnectionCount)
	return 0
}

func workflowRollback(stdout, stderr io.Writer, store *rules.Store, opts workflowOptions) int {
	if strings.TrimSpace(opts.backup) == "" {
		fmt.Fprintln(stderr, "error: --backup is required for rollback")
		return 1
	}
	if err := ruleworkflow.RollbackMixin(store.Path, opts.backup); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintln(stdout, "ok")
	return 0
}

func validateWorkflow(ctx context.Context, opts workflowOptions) ([]ruleworkflow.ValidationResult, error) {
	groups, err := workflowStrategyGroups(ctx, opts)
	if err != nil {
		return nil, err
	}
	candidates, err := selectedWorkflowCandidates(opts)
	if err != nil {
		return nil, err
	}
	sources, err := fetchWorkflowSources(ctx, candidates)
	if err != nil {
		return nil, err
	}
	return ruleworkflow.ValidateSources(sources, groups)
}

func workflowStrategyGroups(ctx context.Context, opts workflowOptions) (map[string]bool, error) {
	groups := csvSet(opts.groups)
	if len(groups) > 0 {
		return groups, nil
	}
	if strings.TrimSpace(opts.endpoint) == "" {
		return nil, fmt.Errorf("error: --groups or --endpoint is required to validate target strategy groups")
	}
	summary, err := ruleworkflow.InspectRuntime(ctx, proxy.NewClient(opts.endpoint, opts.secret))
	if err != nil {
		return nil, fmt.Errorf("inspect runtime: %w", err)
	}
	return summary.StrategyGroups, nil
}

func selectedWorkflowCandidates(opts workflowOptions) ([]ruleworkflow.Candidate, error) {
	candidates, err := ruleworkflow.SelectedCandidates(csvList(opts.candidates))
	if err != nil {
		return nil, err
	}
	for i := range candidates {
		if override := opts.overrides[candidates[i].Name]; override != "" {
			candidates[i].SourceURL = override
			candidates[i].Provider.URL = override
		}
	}
	return candidates, nil
}

func fetchWorkflowSources(ctx context.Context, candidates []ruleworkflow.Candidate) ([]ruleworkflow.FetchedSource, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	sources := make([]ruleworkflow.FetchedSource, 0, len(candidates))
	for _, candidate := range candidates {
		source, err := ruleworkflow.FetchCandidate(ctx, client, candidate)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, nil
}

func csvList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func csvSet(value string) map[string]bool {
	items := csvList(value)
	out := make(map[string]bool, len(items))
	for _, item := range items {
		out[item] = true
	}
	return out
}

func sortedTrueKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key, ok := range values {
		if ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}
