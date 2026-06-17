package rules

import (
	"testing"

	"labproxy/internal/rules"
)

func TestModal_Open(t *testing.T) {
	m := NewModal("/tmp/mixin.yaml")
	m.Open()
	if !m.IsOpen() {
		t.Error("modal should be open after Open()")
	}
	if m.View() == "" {
		t.Error("modal should produce non-empty view")
	}
}

func TestModal_Close(t *testing.T) {
	m := NewModal("/tmp/mixin.yaml")
	m.Open()
	m.Close()
	if m.IsOpen() {
		t.Error("modal should be closed after Close()")
	}
}

func TestList_FormatRow(t *testing.T) {
	row := formatRow(0, rules.Rule{Type: rules.TypeDomainSuffix, Payload: "example.com", Proxy: "PROXY", Enabled: true}, 60)
	if !containsStr(row, "DOMAIN-SUFFIX") || !containsStr(row, "example.com") {
		t.Errorf("missing fields in row: %q", row)
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
