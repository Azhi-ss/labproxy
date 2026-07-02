package ruleworkflow

import (
	"strings"
	"testing"
)

func TestParseProviderRulesFromPayloadYAML(t *testing.T) {
	data := []byte(`
payload:
  - DOMAIN-SUFFIX,github.com
  - DOMAIN,raw.githubusercontent.com
`)
	got, err := ParseProviderRules(data)
	if err != nil {
		t.Fatalf("ParseProviderRules: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Payload != "github.com" || got[0].Type != "DOMAIN-SUFFIX" {
		t.Fatalf("first rule = %+v", got[0])
	}
}

func TestParseProviderRulesFromPlainList(t *testing.T) {
	data := []byte(`
# comment
- DOMAIN-SUFFIX,youtube.com
DOMAIN-SUFFIX,googlevideo.com
`)
	got, err := ParseProviderRules(data)
	if err != nil {
		t.Fatalf("ParseProviderRules: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestParseProviderRulesForBehaviorDomainFromPayloadYAML(t *testing.T) {
	data := []byte(`
payload:
  - '.blogger.com'
  - '*.*.microsoft.com'
  - books.itunes.apple.com
`)

	got, err := ParseProviderRulesForBehavior(data, "domain")
	if err != nil {
		t.Fatalf("ParseProviderRulesForBehavior: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].Type != "DOMAIN" || got[0].Payload != ".blogger.com" {
		t.Fatalf("first rule = %+v", got[0])
	}
	if got[1].Payload != "*.*.microsoft.com" || got[2].Payload != "books.itunes.apple.com" {
		t.Fatalf("parsed rules = %+v", got)
	}
}

func TestParseProviderRulesForBehaviorDomainRejectsClassicalTypedLines(t *testing.T) {
	data := []byte(`
payload:
  - DOMAIN-SUFFIX,example.com
`)

	_, err := ParseProviderRulesForBehavior(data, "domain")
	if err == nil || !strings.Contains(err.Error(), "expected bare domain payload") {
		t.Fatalf("ParseProviderRulesForBehavior error = %v, want bare domain payload", err)
	}
}

func TestParseProviderRulesForBehaviorDomainRejectsMappingTextFallback(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "payload mapping line",
			data: []byte("payload: []\n"),
		},
		{
			name: "arbitrary yaml title line",
			data: []byte("title: Not Found\n"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseProviderRulesForBehavior(tc.data, "domain")
			if err == nil || !strings.Contains(err.Error(), "expected bare domain payload") {
				t.Fatalf("ParseProviderRulesForBehavior error = %v, want bare domain payload", err)
			}
		})
	}
}

func TestParseProviderRulesForBehaviorIPCIDRFromText(t *testing.T) {
	data := []byte(`
# comment
192.168.1.0/24
2001:db8::/32
`)

	got, err := ParseProviderRulesForBehavior(data, "ipcidr")
	if err != nil {
		t.Fatalf("ParseProviderRulesForBehavior: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Type != "IP-CIDR" || got[0].Payload != "192.168.1.0/24" {
		t.Fatalf("first rule = %+v", got[0])
	}
	if got[1].Type != "IP-CIDR6" || got[1].Payload != "2001:db8::/32" {
		t.Fatalf("second rule = %+v", got[1])
	}
}

func TestParseProviderRulesForBehaviorIPCIDRRejectsInvalidEntries(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		targetErr string
	}{
		{
			name:      "invalid cidr",
			data:      []byte("payload:\n  - 192.168.1.0/33\n"),
			targetErr: "invalid ipcidr provider rule",
		},
		{
			name:      "classical typed line",
			data:      []byte("payload:\n  - IP-CIDR,192.168.1.0/24\n"),
			targetErr: "expected bare CIDR payload",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseProviderRulesForBehavior(tc.data, "ipcidr")
			if err == nil || !strings.Contains(err.Error(), tc.targetErr) {
				t.Fatalf("ParseProviderRulesForBehavior error = %v, want substring %q", err, tc.targetErr)
			}
		})
	}
}
