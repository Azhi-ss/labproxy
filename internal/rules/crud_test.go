package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T, content string) *Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "mixin.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStore_AddRule(t *testing.T) {
	s := newTestStore(t, "rules:\n  - DOMAIN,a.com,DIRECT\n")
	_, err := s.AddRule(Rule{Type: TypeDomainSuffix, Payload: "b.com", Proxy: "PROXY", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	rules, _ := s.LoadRules()
	if len(rules) != 2 || rules[1].Payload != "b.com" {
		t.Errorf("expected 2 rules, got %+v", rules)
	}
}

func TestStore_AddRule_ValidationFail(t *testing.T) {
	s := newTestStore(t, "rules:\n")
	_, err := s.AddRule(Rule{Type: "BOGUS", Payload: "x", Proxy: "y"})
	if err == nil {
		t.Error("expected validation error")
	}
}

func TestStore_DeleteRule(t *testing.T) {
	s := newTestStore(t, "rules:\n  - DOMAIN,a.com,DIRECT\n  - DOMAIN,b.com,DIRECT\n")
	_, err := s.DeleteRule(0)
	if err != nil {
		t.Fatal(err)
	}
	rules, _ := s.LoadRules()
	if len(rules) != 1 || rules[0].Payload != "b.com" {
		t.Errorf("expected 1 rule 'b.com', got %+v", rules)
	}
}

func TestStore_UpdateRule(t *testing.T) {
	s := newTestStore(t, "rules:\n  - DOMAIN,a.com,DIRECT\n")
	_, err := s.UpdateRule(0, Rule{Type: TypeDomainSuffix, Payload: "z.com", Proxy: "PROXY", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	rules, _ := s.LoadRules()
	if rules[0].Payload != "z.com" || rules[0].Type != TypeDomainSuffix {
		t.Errorf("update failed: %+v", rules[0])
	}
}
