package rules

import "testing"

func TestParseRule(t *testing.T) {
	tests := []struct {
		line    string
		want    Rule
		wantErr bool
	}{
		{line: "DOMAIN-SUFFIX,example.com,PROXY", want: Rule{Type: TypeDomainSuffix, Payload: "example.com", Proxy: "PROXY"}},
		{line: "IP-CIDR,8.8.8.0/24,DIRECT,no-resolve", want: Rule{Type: TypeIPCIDR, Payload: "8.8.8.0/24", Proxy: "DIRECT", NoResolve: true}},
		{line: "MATCH,,DIRECT", want: Rule{Type: TypeMatch, Proxy: "DIRECT"}},
		{line: "", wantErr: true},
		{line: "BOGUS,x,y", wantErr: true},
		{line: "DOMAIN-SUFFIX,example.com", wantErr: true},
	}
	for _, tt := range tests {
		got, err := ParseRule(tt.line)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseRule(%q) err=%v wantErr=%v", tt.line, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseRule(%q) = %+v, want %+v", tt.line, got, tt.want)
		}
	}
}
