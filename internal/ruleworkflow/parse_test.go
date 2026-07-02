package ruleworkflow

import "testing"

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
