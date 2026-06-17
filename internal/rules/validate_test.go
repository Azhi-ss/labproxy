package rules

import (
	"strings"
	"testing"
)

func TestRule_Validate(t *testing.T) {
	tests := []struct {
		name    string
		rule    Rule
		wantErr bool
		errSub  string
	}{
		{"valid domain-suffix", Rule{Type: TypeDomainSuffix, Payload: "example.com", Proxy: "PROXY"}, false, ""},
		{"valid match empty payload", Rule{Type: TypeMatch, Proxy: "DIRECT"}, false, ""},
		{"empty type", Rule{Payload: "x", Proxy: "y"}, true, "type"},
		{"invalid type", Rule{Type: "BOGUS", Payload: "x", Proxy: "y"}, true, "type"},
		{"empty proxy", Rule{Type: TypeIPCIDR, Payload: "8.8.8.0/24"}, true, "proxy"},
		{"empty payload for non-MATCH", Rule{Type: TypeIPCIDR, Proxy: "DIRECT"}, true, "payload"},
		{"whitespace proxy is invalid", Rule{Type: TypeDomainSuffix, Payload: "x", Proxy: "  "}, true, "proxy"},
	}
	for _, tt := range tests {
		err := tt.rule.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: err=%v wantErr=%v", tt.name, err, tt.wantErr)
			continue
		}
		if tt.wantErr && tt.errSub != "" && !strings.Contains(err.Error(), tt.errSub) {
			t.Errorf("%s: err=%q want substring %q", tt.name, err, tt.errSub)
		}
	}
}
