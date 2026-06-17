package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeStatusFixtures 在 home 下写入 status 子命令所需的文件。
func writeStatusFixtures(t *testing.T, home string, running bool, systemProxy bool) {
	t.Helper()
	cfgDir := filepath.Join(home, ".labproxy", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}

	// ports.conf
	ports := "PROXY_PORT=7890\nUI_PORT=9090\nDNS_PORT=15353\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "ports.conf"), []byte(ports), 0o644); err != nil {
		t.Fatalf("write ports.conf: %v", err)
	}

	// mixin.yaml
	mixin := "system-proxy:\n  enable: true\n"
	if !systemProxy {
		mixin = "system-proxy:\n  enable: false\n"
	}
	if err := os.WriteFile(filepath.Join(home, ".labproxy", "mixin.yaml"), []byte(mixin), 0o644); err != nil {
		t.Fatalf("write mixin.yaml: %v", err)
	}

	if running {
		// PID 指向当前测试进程（保证 ps 能查到 etime）
		pid := os.Getpid()
		if err := os.WriteFile(filepath.Join(cfgDir, "labproxy.pid"), []byte(itoa(pid)), 0o644); err != nil {
			t.Fatalf("write pid: %v", err)
		}
	}
}

func itoa(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	var b []byte
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		b = append([]byte{digits[i%10]}, b...)
		i /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func TestRunStatusCLI_JSONRunning(t *testing.T) {
	// mihomo mock /configs
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/configs" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"mode":"rule","mixed-port":7890,"external-controller":"127.0.0.1:9090"}`))
	}))
	defer srv.Close()

	home := t.TempDir()
	writeStatusFixtures(t, home, true, true)

	var out bytes.Buffer
	code := runStatusCLI(&out, &out, []string{"--json"}, home, srv.URL, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}

	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Running     bool   `json:"running"`
			PID         int    `json:"pid"`
			ProxyPort   int    `json:"proxy_port"`
			UIPort      int    `json:"ui_port"`
			DNSPort     int    `json:"dns_port"`
			Mode        string `json:"mode"`
			SystemProxy bool   `json:"system_proxy"`
			Uptime      string `json:"uptime"`
		} `json:"data"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if !env.OK || !env.Data.Running {
		t.Errorf("expected ok+running, got %+v", env)
	}
	if env.Data.ProxyPort != 7890 || env.Data.UIPort != 9090 || env.Data.DNSPort != 15353 {
		t.Errorf("ports wrong: %+v", env.Data)
	}
	if env.Data.Mode != "rule" {
		t.Errorf("mode=%s want rule", env.Data.Mode)
	}
	if !env.Data.SystemProxy {
		t.Errorf("system_proxy should be true")
	}
	if env.Data.PID != os.Getpid() {
		t.Errorf("pid=%d want %d", env.Data.PID, os.Getpid())
	}
}

func TestRunStatusCLI_JSONNotRunning(t *testing.T) {
	home := t.TempDir()
	writeStatusFixtures(t, home, false, false)

	var out bytes.Buffer
	// endpoint 不可达也无所谓：进程未运行时不应查 mihomo
	code := runStatusCLI(&out, &out, []string{"--json"}, home, "http://127.0.0.1:1", "")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}

	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Running     bool   `json:"running"`
			Mode        string `json:"mode"`
			SystemProxy bool   `json:"system_proxy"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if env.Data.Running {
		t.Errorf("should not be running")
	}
	if env.Data.Mode != "" {
		t.Errorf("mode should be empty when not running, got %q", env.Data.Mode)
	}
}

func TestRunStatusCLI_HumanReadable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"mode":"rule","mixed-port":7890}`))
	}))
	defer srv.Close()

	home := t.TempDir()
	writeStatusFixtures(t, home, true, true)

	var out bytes.Buffer
	code := runStatusCLI(&out, &out, []string{}, home, srv.URL, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}
	s := out.String()
	// 人读输出应包含关键字，且不是纯 JSON
	if !contains(s, "运行") && !contains(s, "running") {
		t.Errorf("human output missing status keyword: %s", s)
	}
	// 不应以 { 开头（非 JSON）
	trim := bytes.TrimLeft(out.Bytes(), " \t\n")
	if len(trim) > 0 && trim[0] == '{' {
		t.Errorf("human output looks like JSON: %s", s)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func init() {
	// 触发 runtime 引用避免未使用（跨平台 ps 解析路径）
	_ = runtime.GOOS
}
