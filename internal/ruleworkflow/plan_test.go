package ruleworkflow

import (
	"strings"
	"testing"

	"labproxy/internal/rules"
)

func TestBuildPlanCreatesRuleSetRules(t *testing.T) {
	candidates, err := SelectedCandidates([]string{"github", "openai"})
	if err != nil {
		t.Fatalf("SelectedCandidates: %v", err)
	}

	plan := BuildPlan(candidates)
	if len(plan.Candidates) != 2 {
		t.Fatalf("len(plan.Candidates) = %d, want 2", len(plan.Candidates))
	}
	if len(plan.Providers) != 2 || len(plan.Rules) != 2 {
		t.Fatalf("unexpected plan sizes: %+v", plan)
	}

	if plan.Providers[0].Name != "github" || plan.Providers[1].Name != "openai" {
		t.Fatalf("provider order = %+v", plan.Providers)
	}

	first := plan.Rules[0]
	if first.Type != rules.TypeRuleSet || first.Payload != "github" || first.Proxy != "Proxies" || !first.Enabled {
		t.Fatalf("first rule = %+v", first)
	}

	second := plan.Rules[1]
	if second.Type != rules.TypeRuleSet || second.Payload != "openai" || second.Proxy != "OpenAI" || !second.Enabled {
		t.Fatalf("second rule = %+v", second)
	}
}

func TestBuildPlanPreservesCandidateOrdering(t *testing.T) {
	candidates := []Candidate{
		candidate("third", "Third", "https://example.test/third.yaml", "C", "classical", "./rule-providers/third.yaml"),
		candidate("first", "First", "https://example.test/first.yaml", "A", "classical", "./rule-providers/first.yaml"),
		candidate("second", "Second", "https://example.test/second.yaml", "B", "classical", "./rule-providers/second.yaml"),
	}

	plan := BuildPlan(candidates)
	if got := []string{plan.Candidates[0].Name, plan.Candidates[1].Name, plan.Candidates[2].Name}; strings.Join(got, ",") != "third,first,second" {
		t.Fatalf("candidate order = %v", got)
	}
	if got := []string{plan.Providers[0].Name, plan.Providers[1].Name, plan.Providers[2].Name}; strings.Join(got, ",") != "third,first,second" {
		t.Fatalf("provider order = %v", got)
	}
	if got := []string{plan.Rules[0].Payload, plan.Rules[1].Payload, plan.Rules[2].Payload}; strings.Join(got, ",") != "third,first,second" {
		t.Fatalf("rule order = %v", got)
	}
}

func TestRenderPlanIncludesProviderAndRuleSetDetails(t *testing.T) {
	candidates, err := SelectedCandidates([]string{"github"})
	if err != nil {
		t.Fatalf("SelectedCandidates: %v", err)
	}

	text := RenderPlan(BuildPlan(candidates))
	for _, want := range []string{
		"github",
		"behavior=classical",
		"./rule-providers/github.yaml",
		"RULE-SET,github,Proxies",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("RenderPlan() missing %q in %q", want, text)
		}
	}
}
