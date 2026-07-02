package ruleworkflow

import (
	"fmt"
	"strings"

	"labproxy/internal/rules"
)

type ValidationResult struct {
	Candidate Candidate
	RuleCount int
	Rules     []ProviderRule
}

func ValidateSources(sources []FetchedSource, strategyGroups map[string]bool) ([]ValidationResult, error) {
	results := make([]ValidationResult, 0, len(sources))
	seenProviders := make(map[string]struct{}, len(sources))

	for _, src := range sources {
		provider := src.Candidate.Provider
		if _, exists := seenProviders[provider.Name]; exists {
			return nil, fmt.Errorf("duplicate provider %q", provider.Name)
		}
		seenProviders[provider.Name] = struct{}{}

		if !strategyGroups[src.Candidate.TargetGroup] {
			return nil, fmt.Errorf("candidate %q targets missing strategy group %q", src.Candidate.Name, src.Candidate.TargetGroup)
		}

		if err := rules.ValidateProvider(provider); err != nil {
			return nil, fmt.Errorf("candidate %q provider invalid: %w", src.Candidate.Name, err)
		}

		parsedRules, err := ParseProviderRules(src.Data)
		if err != nil {
			return nil, fmt.Errorf("candidate %q parse failed: %w", src.Candidate.Name, err)
		}
		if len(parsedRules) == 0 {
			return nil, fmt.Errorf("candidate %q has no provider rules", src.Candidate.Name)
		}

		for _, rule := range parsedRules {
			if err := validateRuleBehavior(rule, provider.Behavior); err != nil {
				return nil, fmt.Errorf("candidate %q rule %q invalid: %w", src.Candidate.Name, rule.Raw, err)
			}
		}

		results = append(results, ValidationResult{
			Candidate: src.Candidate,
			RuleCount: len(parsedRules),
			Rules:     parsedRules,
		})
	}

	return results, nil
}

func validateRuleBehavior(rule ProviderRule, behavior string) error {
	ruleType := strings.ToUpper(strings.TrimSpace(rule.Type))
	if !rules.RuleType(ruleType).IsValid() {
		return fmt.Errorf("unsupported rule type %q", rule.Type)
	}

	if behavior == "classical" {
		return nil
	}

	switch behavior {
	case "domain":
		switch rules.RuleType(ruleType) {
		case rules.TypeDomain, rules.TypeDomainSuffix, rules.TypeDomainKeyword:
			return nil
		}
	case "ipcidr":
		switch rules.RuleType(ruleType) {
		case rules.TypeIPCIDR, rules.TypeIPCIDR6:
			return nil
		}
	default:
		return fmt.Errorf("unsupported provider behavior %q", behavior)
	}

	return fmt.Errorf("behavior %q does not support rule type %q", behavior, rule.Type)
}
