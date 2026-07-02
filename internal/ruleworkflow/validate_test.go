package ruleworkflow

import (
	"strings"
	"testing"
)

func TestValidateSourcesRejectsDuplicateProviderNames(t *testing.T) {
	sources := []FetchedSource{
		{
			Candidate: candidate("github", "GitHub", "https://example.test/github-1.yaml", "Proxies", "classical", "./rule-providers/github-1.yaml"),
			Data:      []byte("payload:\n  - DOMAIN-SUFFIX,github.com\n"),
		},
		{
			Candidate: candidate("github", "GitHub mirror", "https://example.test/github-2.yaml", "Proxies", "classical", "./rule-providers/github-2.yaml"),
			Data:      []byte("payload:\n  - DOMAIN-SUFFIX,githubusercontent.com\n"),
		},
	}

	_, err := ValidateSources(sources, map[string]bool{"Proxies": true})
	if err == nil || !strings.Contains(err.Error(), "duplicate provider") {
		t.Fatalf("ValidateSources error = %v, want duplicate provider", err)
	}
}

func TestValidateSourcesRejectsMissingTargetGroup(t *testing.T) {
	src := FetchedSource{
		Candidate: candidate("github", "GitHub", "https://example.test/github.yaml", "Proxies", "classical", "./rule-providers/github.yaml"),
		Data:      []byte("payload:\n  - DOMAIN-SUFFIX,github.com\n"),
	}

	_, err := ValidateSources([]FetchedSource{src}, map[string]bool{"DIRECT": true})
	if err == nil || !strings.Contains(err.Error(), "missing strategy group") {
		t.Fatalf("ValidateSources error = %v, want missing strategy group", err)
	}
}

func TestValidateSourcesRejectsEmptyParsedRules(t *testing.T) {
	src := FetchedSource{
		Candidate: candidate("github", "GitHub", "https://example.test/github.yaml", "Proxies", "classical", "./rule-providers/github.yaml"),
		Data:      []byte("# comment only\n"),
	}

	_, err := ValidateSources([]FetchedSource{src}, map[string]bool{"Proxies": true})
	if err == nil || !strings.Contains(err.Error(), "no provider rules") {
		t.Fatalf("ValidateSources error = %v, want no provider rules", err)
	}
}

func TestValidateSourcesRejectsBehaviorMismatches(t *testing.T) {
	tests := []struct {
		name      string
		behavior  string
		ruleLine  string
		targetErr string
	}{
		{
			name:      "domain rejects ipcidr",
			behavior:  "domain",
			ruleLine:  "IP-CIDR,1.1.1.0/24",
			targetErr: "does not support rule type",
		},
		{
			name:      "ipcidr rejects domain",
			behavior:  "ipcidr",
			ruleLine:  "DOMAIN-SUFFIX,github.com",
			targetErr: "does not support rule type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := FetchedSource{
				Candidate: candidate("provider-"+tc.behavior, "Provider "+tc.behavior, "https://example.test/provider.yaml", "Proxies", tc.behavior, "./rule-providers/provider.yaml"),
				Data:      []byte("payload:\n  - " + tc.ruleLine + "\n"),
			}

			_, err := ValidateSources([]FetchedSource{src}, map[string]bool{"Proxies": true})
			if err == nil || !strings.Contains(err.Error(), tc.targetErr) {
				t.Fatalf("ValidateSources error = %v, want substring %q", err, tc.targetErr)
			}
		})
	}
}

func TestValidateSourcesSuccess(t *testing.T) {
	sources := []FetchedSource{
		{
			Candidate: candidate("domains", "Domains", "https://example.test/domains.yaml", "Proxies", "domain", "./rule-providers/domains.yaml"),
			Data: []byte(`
payload:
  - DOMAIN,github.com
  - DOMAIN-SUFFIX,githubusercontent.com
  - DOMAIN-KEYWORD,gitlab
`),
		},
		{
			Candidate: candidate("ips", "IPs", "https://example.test/ips.yaml", "Streaming", "ipcidr", "./rule-providers/ips.yaml"),
			Data: []byte(`
payload:
  - IP-CIDR,1.1.1.0/24
  - IP-CIDR6,2001:db8::/32
`),
		},
		{
			Candidate: candidate("mixed", "Mixed", "https://example.test/mixed.yaml", "Fallback", "classical", "./rule-providers/mixed.yaml"),
			Data: []byte(`
payload:
  - DOMAIN-SUFFIX,openai.com
  - IP-CIDR,8.8.8.0/24
`),
		},
	}

	results, err := ValidateSources(sources, map[string]bool{
		"Proxies":   true,
		"Streaming": true,
		"Fallback":  true,
	})
	if err != nil {
		t.Fatalf("ValidateSources: %v", err)
	}
	if len(results) != len(sources) {
		t.Fatalf("len(results) = %d, want %d", len(results), len(sources))
	}
	if results[0].Candidate.Name != "domains" || results[0].RuleCount != 3 {
		t.Fatalf("first result = %+v", results[0])
	}
	if results[1].Candidate.Name != "ips" || results[1].RuleCount != 2 {
		t.Fatalf("second result = %+v", results[1])
	}
	if results[2].Candidate.Name != "mixed" || results[2].RuleCount != 2 {
		t.Fatalf("third result = %+v", results[2])
	}
	if results[0].Rules[0].Type != "DOMAIN" {
		t.Fatalf("unexpected parsed rules: %+v", results[0].Rules)
	}
}
