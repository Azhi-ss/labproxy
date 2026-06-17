package rules

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestStore_Import_Preset(t *testing.T) {
	s := newTestStore(t, "rules:\n")
	_, err := s.Import(ImportSource{Kind: "preset", Ref: "direct"}, "append")
	if err != nil {
		t.Fatal(err)
	}
	rules, _ := s.LoadRules()
	if len(rules) != 4 {
		t.Errorf("expected 4 rules from direct preset, got %d", len(rules))
	}
}

func TestStore_Import_File(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.yaml")
	os.WriteFile(src, []byte("- DOMAIN-SUFFIX,foo.com,PROXY\n- DOMAIN-SUFFIX,bar.com,PROXY\n"), 0o644)
	s := newTestStore(t, "rules:\n")
	_, err := s.Import(ImportSource{Kind: "file", Ref: src}, "append")
	if err != nil {
		t.Fatal(err)
	}
	rules, _ := s.LoadRules()
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}
}

func TestStore_Import_URL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("- DOMAIN-SUFFIX,remote.com,PROXY\n"))
	}))
	defer server.Close()
	s := newTestStore(t, "rules:\n")
	_, err := s.Import(ImportSource{Kind: "url", Ref: server.URL}, "append")
	if err != nil {
		t.Fatal(err)
	}
	rules, _ := s.LoadRules()
	if len(rules) != 1 || rules[0].Payload != "remote.com" {
		t.Errorf("unexpected: %+v", rules)
	}
}

func TestStore_Import_RejectBadScheme(t *testing.T) {
	s := newTestStore(t, "rules:\n")
	_, err := s.Import(ImportSource{Kind: "url", Ref: "file:///etc/passwd"}, "append")
	if err == nil {
		t.Error("expected scheme rejection")
	}
}
