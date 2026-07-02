package ruleworkflow

import (
	"fmt"
	"os"

	"labproxy/internal/rules"
)

type ApplyResult struct {
	BackupPath string
}

var (
	backupFn        = func(store *rules.Store) (string, error) { return store.Backup() }
	loadProvidersFn = func(store *rules.Store) ([]rules.Provider, error) { return store.LoadProviders() }
	loadRulesFn     = func(store *rules.Store) ([]rules.Rule, error) { return store.LoadRules() }
	saveProvidersFn = func(store *rules.Store, providers []rules.Provider) error { return store.SaveProviders(providers) }
	saveRulesFn     = func(store *rules.Store, ruleList []rules.Rule) error { return store.SaveRules(ruleList) }
)

func ApplyPlan(store *rules.Store, plan Plan) (ApplyResult, error) {
	backupPath, err := backupFn(store)
	if err != nil {
		return ApplyResult{}, err
	}

	existingProviders, err := loadProvidersFn(store)
	if err != nil {
		return ApplyResult{}, err
	}

	existingRules, err := loadRulesFn(store)
	if err != nil {
		return ApplyResult{}, err
	}

	providers := mergeProviders(existingProviders, plan.Providers)
	ruleList := mergeRules(existingRules, plan.Rules)

	if err := saveProvidersFn(store, providers); err != nil {
		return ApplyResult{}, fmt.Errorf("save providers: %w", err)
	}

	if err := saveRulesFn(store, ruleList); err != nil {
		if backupPath != "" {
			_ = RollbackMixin(store.Path, backupPath)
		}
		return ApplyResult{}, fmt.Errorf("save rules: %w", err)
	}

	return ApplyResult{BackupPath: backupPath}, nil
}

func RollbackMixin(mixinPath, backupPath string) error {
	if mixinPath == "" {
		return fmt.Errorf("mixin path is required")
	}
	if backupPath == "" {
		return fmt.Errorf("backup path is required")
	}

	data, err := os.ReadFile(backupPath)
	if err != nil {
		return err
	}

	tmp := mixinPath + ".rollback.tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmp, mixinPath)
}

func mergeProviders(existing, incoming []rules.Provider) []rules.Provider {
	byName := make(map[string]rules.Provider, len(existing)+len(incoming))
	order := make([]string, 0, len(existing)+len(incoming))

	for _, provider := range existing {
		if _, ok := byName[provider.Name]; !ok {
			order = append(order, provider.Name)
		}
		byName[provider.Name] = provider
	}

	for _, provider := range incoming {
		if _, ok := byName[provider.Name]; !ok {
			order = append(order, provider.Name)
		}
		byName[provider.Name] = provider
	}

	merged := make([]rules.Provider, 0, len(order))
	for _, name := range order {
		merged = append(merged, byName[name])
	}

	return merged
}

func mergeRules(existing, incoming []rules.Rule) []rules.Rule {
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	merged := make([]rules.Rule, 0, len(existing)+len(incoming))

	for _, rule := range existing {
		key := rule.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, rule)
	}

	for _, rule := range incoming {
		key := rule.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, rule)
	}

	return merged
}
