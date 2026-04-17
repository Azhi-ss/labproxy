package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestWriteSystemProxyEnabled(t *testing.T) {
	t.Run("updates inline key", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "mixin.yaml")
		if err := os.WriteFile(path, []byte("system-proxy.enable: false\nmode: rule\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		if err := WriteSystemProxyEnabled(path, true); err != nil {
			t.Fatalf("WriteSystemProxyEnabled: %v", err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if !strings.Contains(string(content), "system-proxy.enable: true") {
			t.Fatalf("expected inline key to be updated, got %q", string(content))
		}
	})

	t.Run("updates nested key", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "mixin.yaml")
		content := "system-proxy:\n  enable: false\nmode: rule\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		if err := WriteSystemProxyEnabled(path, true); err != nil {
			t.Fatalf("WriteSystemProxyEnabled: %v", err)
		}

		enabled, err := ReadSystemProxyEnabled(path)
		if err != nil {
			t.Fatalf("ReadSystemProxyEnabled: %v", err)
		}
		if !enabled {
			t.Fatal("expected nested key to be updated")
		}
	})

	t.Run("creates missing block", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "mixin.yaml")
		if err := WriteSystemProxyEnabled(path, true); err != nil {
			t.Fatalf("WriteSystemProxyEnabled: %v", err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if !strings.Contains(string(content), "system-proxy:") || !strings.Contains(string(content), "enable: true") {
			t.Fatalf("expected system-proxy block to be created, got %q", string(content))
		}
	})
}

func TestWriteMode(t *testing.T) {
	t.Run("updates existing mode", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "mixin.yaml")
		if err := os.WriteFile(path, []byte("mode: rule\nsystem-proxy.enable: false\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		if err := WriteMode(path, "global"); err != nil {
			t.Fatalf("WriteMode: %v", err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if !strings.Contains(string(content), "mode: global") {
			t.Fatalf("expected mode to be updated, got %q", string(content))
		}
	})

	t.Run("creates missing mode", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "mixin.yaml")
		if err := WriteMode(path, "direct"); err != nil {
			t.Fatalf("WriteMode: %v", err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if !strings.Contains(string(content), "mode: direct") {
			t.Fatalf("expected mode to be created, got %q", string(content))
		}
	})

	t.Run("rejects invalid mode", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "mixin.yaml")
		if err := WriteMode(path, "auto"); err == nil {
			t.Fatal("expected invalid mode to fail")
		}
	})
}

func TestAllowLanConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixin.yaml")
	if err := WriteAllowLanEnabled(path, true); err != nil {
		t.Fatalf("WriteAllowLanEnabled: %v", err)
	}

	enabled, err := ReadAllowLanEnabled(path)
	if err != nil {
		t.Fatalf("ReadAllowLanEnabled: %v", err)
	}
	if !enabled {
		t.Fatal("expected allow-lan=true")
	}

	if err := WriteAllowLanEnabled(path, false); err != nil {
		t.Fatalf("WriteAllowLanEnabled(false): %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(string(content), "allow-lan: false") {
		t.Fatalf("expected allow-lan to be updated, got %q", string(content))
	}
}

func TestTunConfig(t *testing.T) {
	t.Run("creates nested block", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "mixin.yaml")
		if err := WriteTunEnabled(path, true); err != nil {
			t.Fatalf("WriteTunEnabled: %v", err)
		}

		enabled, err := ReadTunEnabled(path)
		if err != nil {
			t.Fatalf("ReadTunEnabled: %v", err)
		}
		if !enabled {
			t.Fatal("expected tun.enable=true")
		}
	})

	t.Run("updates inline key", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "mixin.yaml")
		if err := os.WriteFile(path, []byte("tun.enable: false\n"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		if err := WriteTunEnabled(path, true); err != nil {
			t.Fatalf("WriteTunEnabled: %v", err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if !strings.Contains(string(content), "tun.enable: true") {
			t.Fatalf("expected inline tun.enable to be updated, got %q", string(content))
		}
	})
}
