package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_LoadRules_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mixin.yaml")
	if err := os.WriteFile(path, []byte("# empty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := s.LoadRules()
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}
}

func TestStore_LoadRules_PreservesEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mixin.yaml")
	yaml := `rules:
  - DOMAIN,foo.com,DIRECT
  # - DOMAIN-SUFFIX,bar.com,PROXY
  - MATCH,,DIRECT
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := NewStore(path)
	rules, err := s.LoadRules()
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
	if !rules[0].Enabled {
		t.Error("rule 0 should be enabled")
	}
	if rules[1].Enabled {
		t.Error("rule 1 should be disabled (commented)")
	}
	if !rules[2].Enabled {
		t.Error("rule 2 should be enabled")
	}
}

func TestStore_SaveRules_AtomicAndBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mixin.yaml")
	original := `mode: rule
rules:
  - DOMAIN,foo.com,DIRECT
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := NewStore(path)
	rules := []Rule{{Type: TypeDomainSuffix, Payload: "x.com", Proxy: "PROXY", Enabled: true}}
	if err := s.SaveRules(rules); err != nil {
		t.Fatal(err)
	}
	rules2, err := s.LoadRules()
	if err != nil {
		t.Fatal(err)
	}
	if len(rules2) != 1 || rules2[0].Payload != "x.com" {
		t.Errorf("unexpected load result: %+v", rules2)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "mixin.yaml.bak.*"))
	if len(matches) == 0 {
		t.Error("expected backup file")
	}
}
