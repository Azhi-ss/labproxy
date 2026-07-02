package ruleworkflow

import (
	"fmt"
	"strings"

	"labproxy/internal/rules"
)

func BuildPlan(candidates []Candidate) Plan {
	plan := Plan{
		Candidates: append([]Candidate(nil), candidates...),
		Providers:  make([]rules.Provider, 0, len(candidates)),
		Rules:      make([]rules.Rule, 0, len(candidates)),
	}

	for _, candidate := range candidates {
		plan.Providers = append(plan.Providers, candidate.Provider)
		plan.Rules = append(plan.Rules, rules.Rule{
			Type:    rules.TypeRuleSet,
			Payload: candidate.Provider.Name,
			Proxy:   candidate.TargetGroup,
			Enabled: true,
		})
	}

	return plan
}

func RenderPlan(plan Plan) string {
	var builder strings.Builder

	builder.WriteString("Rule workflow plan\n")
	builder.WriteString("Providers:\n")
	for _, provider := range plan.Providers {
		fmt.Fprintf(
			&builder,
			"- %s behavior=%s type=%s path=%s url=%s interval=%d\n",
			provider.Name,
			provider.Behavior,
			provider.Type,
			provider.Path,
			provider.URL,
			provider.Interval,
		)
	}

	builder.WriteString("Rules:\n")
	for _, rule := range plan.Rules {
		fmt.Fprintf(&builder, "- %s enabled=%t\n", rule.String(), rule.Enabled)
	}

	return builder.String()
}
