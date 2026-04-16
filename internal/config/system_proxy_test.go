package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadSystemProxyEnabled(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "mixin.yaml")
		if err := os.WriteFile(path, []byte("system-proxy.enable: true\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		enabled, err := ReadSystemProxyEnabled(path)
		if err != nil {
			t.Fatalf("ReadSystemProxyEnabled: %v", err)
		}
		if !enabled {
			t.Fatalf("expected enabled=true")
		}
	})

	t.Run("nested yaml", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "mixin.yaml")
		content := "" +
			"# comment\n" +
			"system-proxy:\n" +
			"  enable: true\n" +
			"mode: rule\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		enabled, err := ReadSystemProxyEnabled(path)
		if err != nil {
			t.Fatalf("ReadSystemProxyEnabled: %v", err)
		}
		if !enabled {
			t.Fatalf("expected enabled=true for nested yaml")
		}
	})

	t.Run("nested yaml false", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "mixin.yaml")
		content := "" +
			"system-proxy:\n" +
			"  enable: false\n" +
			"allow-lan: false\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		enabled, err := ReadSystemProxyEnabled(path)
		if err != nil {
			t.Fatalf("ReadSystemProxyEnabled: %v", err)
		}
		if enabled {
			t.Fatalf("expected enabled=false for nested yaml")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		enabled, err := ReadSystemProxyEnabled(filepath.Join(t.TempDir(), "missing.yaml"))
		if err != nil {
			t.Fatalf("ReadSystemProxyEnabled: %v", err)
		}
		if enabled {
			t.Fatalf("expected enabled=false")
		}
	})
}
