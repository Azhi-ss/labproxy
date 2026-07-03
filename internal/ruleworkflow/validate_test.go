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
			targetErr: "expected bare domain payload",
		},
		{
			name:      "ipcidr rejects domain",
			behavior:  "ipcidr",
			ruleLine:  "DOMAIN-SUFFIX,github.com",
			targetErr: "expected bare CIDR payload",
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

func TestValidateSourcesRejectsNativeBehaviorTypedLines(t *testing.T) {
	tests := []struct {
		name      string
		behavior  string
		ruleLine  string
		targetErr string
	}{
		{
			name:      "domain rejects typed line",
			behavior:  "domain",
			ruleLine:  "DOMAIN-SUFFIX,github.com",
			targetErr: "expected bare domain payload",
		},
		{
			name:      "ipcidr rejects typed line",
			behavior:  "ipcidr",
			ruleLine:  "IP-CIDR,1.1.1.0/24",
			targetErr: "expected bare CIDR payload",
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

func TestValidateSourcesRejectsInvalidNativeDomainText(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		targetErr string
	}{
		{
			name:      "rejects payload mapping fallback",
			data:      []byte("payload: []\n"),
			targetErr: "expected bare domain payload",
		},
		{
			name:      "rejects title mapping fallback",
			data:      []byte("title: Not Found\n"),
			targetErr: "expected bare domain payload",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := FetchedSource{
				Candidate: candidate("domains", "Domains", "https://example.test/domains.yaml", "Proxies", "domain", "./rule-providers/domains.yaml"),
				Data:      tc.data,
			}

			_, err := ValidateSources([]FetchedSource{src}, map[string]bool{"Proxies": true})
			if err == nil || !strings.Contains(err.Error(), tc.targetErr) {
				t.Fatalf("ValidateSources error = %v, want substring %q", err, tc.targetErr)
			}
		})
	}
}

func TestValidateSourcesAcceptsDocumentedNativeDomainExamples(t *testing.T) {
	src := FetchedSource{
		Candidate: candidate("domains", "Domains", "https://example.test/domains.yaml", "Proxies", "domain", "./rule-providers/domains.yaml"),
		Data: []byte(`
payload:
  - '.blogger.com'
  - '*.*.microsoft.com'
  - books.itunes.apple.com
`),
	}

	results, err := ValidateSources([]FetchedSource{src}, map[string]bool{"Proxies": true})
	if err != nil {
		t.Fatalf("ValidateSources: %v", err)
	}
	if len(results) != 1 || results[0].RuleCount != 3 {
		t.Fatalf("results = %+v, want single validated domain provider", results)
	}
}

func TestValidateSourcesRejectsInvalidNativeIPCIDR(t *testing.T) {
	src := FetchedSource{
		Candidate: candidate("ips", "IPs", "https://example.test/ips.yaml", "Streaming", "ipcidr", "./rule-providers/ips.yaml"),
		Data:      []byte("payload:\n  - 2001:db8::/129\n"),
	}

	_, err := ValidateSources([]FetchedSource{src}, map[string]bool{"Streaming": true})
	if err == nil || !strings.Contains(err.Error(), "invalid ipcidr provider rule") {
		t.Fatalf("ValidateSources error = %v, want invalid ipcidr provider rule", err)
	}
}

func TestValidateSourcesRejectsInvalidClassicalRuleType(t *testing.T) {
	src := FetchedSource{
		Candidate: candidate("mixed", "Mixed", "https://example.test/mixed.yaml", "Fallback", "classical", "./rule-providers/mixed.yaml"),
		Data:      []byte("payload:\n  - BOGUS,example.com\n"),
	}

	_, err := ValidateSources([]FetchedSource{src}, map[string]bool{"Fallback": true})
	if err == nil || !strings.Contains(err.Error(), "unsupported rule type") {
		t.Fatalf("ValidateSources error = %v, want unsupported rule type", err)
	}
}

func TestValidateSourcesAcceptsClassicalIPASNRule(t *testing.T) {
	src := FetchedSource{
		Candidate: candidate("openai", "OpenAI", "https://example.test/openai.yaml", "OpenAI", "classical", "./rule-providers/openai.yaml"),
		Data: []byte(`
payload:
  - DOMAIN-SUFFIX,openai.com
  - IP-ASN,20473
`),
	}

	results, err := ValidateSources([]FetchedSource{src}, map[string]bool{"OpenAI": true})
	if err != nil {
		t.Fatalf("ValidateSources: %v", err)
	}
	if len(results) != 1 || results[0].RuleCount != 2 {
		t.Fatalf("results = %+v, want single validated provider with 2 rules", results)
	}
	if results[0].Rules[1].Type != "IP-ASN" || results[0].Rules[1].Payload != "20473" {
		t.Fatalf("unexpected parsed rules: %+v", results[0].Rules)
	}
}

func TestValidateSourcesSuccess(t *testing.T) {
	sources := []FetchedSource{
		{
			Candidate: candidate("domains", "Domains", "https://example.test/domains.yaml", "Proxies", "domain", "./rule-providers/domains.yaml"),
			Data: []byte(`
payload:
  - '.blogger.com'
  - '*.*.microsoft.com'
  - books.itunes.apple.com
`),
		},
		{
			Candidate: candidate("ips", "IPs", "https://example.test/ips.yaml", "Streaming", "ipcidr", "./rule-providers/ips.yaml"),
			Data: []byte(`
payload:
  - 1.1.1.0/24
  - 2001:db8::/32
`),
		},
		{
			Candidate: candidate("mixed", "Mixed", "https://example.test/mixed.yaml", "Fallback", "classical", "./rule-providers/mixed.yaml"),
			Data: []byte(`
payload:
  - DOMAIN-SUFFIX,openai.com
  - IP-CIDR,8.8.8.0/24
  - IP-ASN,20473
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
	if results[2].Candidate.Name != "mixed" || results[2].RuleCount != 3 {
		t.Fatalf("third result = %+v", results[2])
	}
	if results[0].Rules[0].Type != "DOMAIN" {
		t.Fatalf("unexpected parsed rules: %+v", results[0].Rules)
	}
	if results[1].Rules[1].Type != "IP-CIDR6" {
		t.Fatalf("unexpected parsed ip rules: %+v", results[1].Rules)
	}
	if results[2].Rules[2].Type != "IP-ASN" {
		t.Fatalf("unexpected parsed classical rules: %+v", results[2].Rules)
	}
}
