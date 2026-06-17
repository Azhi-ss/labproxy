package rules

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStore_AddProvider(t *testing.T) {
	s := newTestStore(t, "rule-providers:\n")
	p := Provider{Name: "g", Type: "http", Behavior: "domain", URL: "https://x.com/g.yaml", Path: "./p/g.yaml", Interval: 3600}
	if _, err := s.AddProvider(p); err != nil {
		t.Fatal(err)
	}
	providers, _ := s.LoadProviders()
	if len(providers) != 1 || providers[0].Name != "g" {
		t.Errorf("unexpected: %+v", providers)
	}
}

func TestStore_RefreshProvider(t *testing.T) {
	body := []byte("- DOMAIN-SUFFIX,example.com,DIRECT\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	defer server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "mixin.yaml")
	os.WriteFile(path, []byte("rule-providers:\n"), 0o644)
	s, _ := NewStore(path)
	p := Provider{Name: "g", Type: "http", Behavior: "domain", URL: server.URL, Path: filepath.Join(dir, "g.yaml"), Interval: 3600}
	if _, err := s.AddProvider(p); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshProvider("g"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p.Path)
	if !strings.Contains(string(data), "example.com") {
		t.Errorf("expected cached file to contain example.com, got %s", data)
	}
}
