package rules

import "testing"

func TestLoadPreset(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		wantLen int
		wantErr bool
	}{
		{"direct", "direct", 4, false},
		{"private", "private", 5, false},
		{"gfw", "gfw", 3, false},
		{"tld-not-cn", "tld-not-cn", 1, false},
		{"unknown", "unknown", 0, true},
	}
	for _, tt := range tests {
		rules, err := LoadPreset(tt.ref)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: err=%v wantErr=%v", tt.name, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && len(rules) != tt.wantLen {
			t.Errorf("%s: got %d rules, want %d", tt.name, len(rules), tt.wantLen)
		}
	}
}
