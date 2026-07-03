package rules

import "testing"

func TestRuleType_IsValid(t *testing.T) {
	valid := []RuleType{
		TypeDomain, TypeDomainSuffix, TypeDomainKeyword, TypeDomainRegex,
		TypeIPCIDR, TypeIPCIDR6, TypeIPASN, TypeSrcIPCidr, TypeSrcPort,
		TypeGEOIP, TypeGEOSITE, TypeRuleSet, TypeMatch, TypeMatchSrc,
	}
	for _, rt := range valid {
		if !rt.IsValid() {
			t.Errorf("%q should be valid", rt)
		}
	}
	if RuleType("BOGUS").IsValid() {
		t.Error("BOGUS should not be valid")
	}
}

func TestRule_String(t *testing.T) {
	tests := []struct {
		rule Rule
		want string
	}{
		{Rule{Type: TypeDomainSuffix, Payload: "example.com", Proxy: "PROXY"}, "DOMAIN-SUFFIX,example.com,PROXY"},
		{Rule{Type: TypeIPCIDR, Payload: "8.8.8.0/24", Proxy: "DIRECT", NoResolve: true}, "IP-CIDR,8.8.8.0/24,DIRECT,no-resolve"},
		{Rule{Type: TypeMatch, Proxy: "DIRECT"}, "MATCH,,DIRECT"},
	}
	for _, tt := range tests {
		if got := tt.rule.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}
