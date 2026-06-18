package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndLoad(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	p := Profile{
		Name:  "work",
		Mixin: []byte("system-proxy:\n  enable: true\n"),
		Rules: []byte("rules:\n  - DOMAIN-SUFFIX,example.com,DIRECT\n"),
	}
	if err := s.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}

	loaded, err := s.Load("work")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Name != "work" {
		t.Errorf("name=%s want work", loaded.Name)
	}
	if string(loaded.Mixin) != "system-proxy:\n  enable: true\n" {
		t.Errorf("mixin mismatch: %q", loaded.Mixin)
	}
	if string(loaded.Rules) != "rules:\n  - DOMAIN-SUFFIX,example.com,DIRECT\n" {
		t.Errorf("rules mismatch: %q", loaded.Rules)
	}
}

func TestCreateWritesToProfileDir(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir)
	p := Profile{Name: "home", Mixin: []byte("a: 1\n"), Rules: []byte("b: 2\n")}
	if err := s.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, f := range []string{"mixin.yaml", "rules.yaml", "meta.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, "profiles", "home", f)); err != nil {
			t.Errorf("expected %s: %v", f, err)
		}
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir)
	for _, n := range []string{"a", "b", "c"} {
		if err := s.Create(Profile{Name: n, Mixin: []byte("x\n"), Rules: []byte("y\n")}); err != nil {
			t.Fatalf("Create %s: %v", n, err)
		}
	}
	names, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 profiles, got %d: %v", len(names), names)
	}
	want := map[string]bool{"a": true, "b": true, "c": true}
	for _, n := range names {
		if !want[n] {
			t.Errorf("unexpected profile %q", n)
		}
	}
}

func TestLoadNotFound(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir)
	_, err := s.Load("nope")
	if err == nil {
		t.Fatal("expected error loading missing profile")
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir)
	if s.Exists("x") {
		t.Error("x should not exist before create")
	}
	s.Create(Profile{Name: "x", Mixin: []byte("x\n"), Rules: []byte("y\n")})
	if !s.Exists("x") {
		t.Error("x should exist after create")
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir)
	s.Create(Profile{Name: "tmp", Mixin: []byte("x\n"), Rules: []byte("y\n")})
	if err := s.Delete("tmp"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if s.Exists("tmp") {
		t.Error("tmp should not exist after delete")
	}
}

func TestDeleteNotFound(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir)
	if err := s.Delete("nope"); err == nil {
		t.Fatal("expected error deleting missing profile")
	}
}

func TestCreateRejectsBadName(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir)
	bad := []string{"", "..", "/abs", "a/b", "a b"}
	for _, n := range bad {
		if err := s.Create(Profile{Name: n, Mixin: []byte("x\n"), Rules: []byte("y\n")}); err == nil {
			t.Errorf("expected error for bad name %q", n)
		}
	}
}

func TestCreateAtomic(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir)
	p := Profile{Name: "atomic", Mixin: []byte("m\n"), Rules: []byte("r\n")}
	if err := s.Create(p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	p.Mixin = []byte("m2\n")
	if err := s.Create(p); err != nil {
		t.Fatalf("Create overwrite: %v", err)
	}
	loaded, _ := s.Load("atomic")
	if string(loaded.Mixin) != "m2\n" {
		t.Errorf("overwrite failed: %q", loaded.Mixin)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "profiles", "atomic"))
	for _, e := range entries {
		if e.Name() != "mixin.yaml" && e.Name() != "rules.yaml" && e.Name() != "meta.yaml" {
			t.Errorf("unexpected file: %s", e.Name())
		}
	}
}
