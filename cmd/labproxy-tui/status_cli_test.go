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

// writeRuntimeYaml 在 home/.labproxy/runtime.yaml 写入端口配置。
func writeRuntimeYaml(t *testing.T, home string, body string) {
	t.Helper()
	dir := filepath.Join(home, ".labproxy")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir .labproxy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "runtime.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write runtime.yaml: %v", err)
	}
}

// TestReadPortsFromRuntime 验证 ports.conf 缺失时从 runtime.yaml 回退解析端口。
func TestReadPortsFromRuntime(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want map[string]int
	}{
		{
			name: "full config",
			yaml: "mixed-port: 7893\nexternal-controller: 0.0.0.0:9090\ndns:\n  listen: 0.0.0.0:15353\n",
			want: map[string]int{"PROXY_PORT": 7893, "UI_PORT": 9090, "DNS_PORT": 15353},
		},
		{
			name: "only mixed-port",
			yaml: "mixed-port: 7890\n",
			want: map[string]int{"PROXY_PORT": 7890},
		},
		{
			name: "empty file",
			yaml: "",
			want: map[string]int{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			writeRuntimeYaml(t, home, tc.yaml)
			got := readPortsFromRuntime(home)
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch: got %d want %d (%v)", len(got), len(tc.want), got)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("port %s: got %d want %d", k, got[k], v)
				}
			}
		})
	}
}

// TestReadPortsFromRuntime_MissingFile 文件不存在时返回空 map（不 panic）。
func TestReadPortsFromRuntime_MissingFile(t *testing.T) {
	got := readPortsFromRuntime(t.TempDir())
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}
}

// TestReadPIDFile_FromPIDFile PID 文件存在且有效时直接返回该 PID。
func TestReadPIDFile_FromPIDFile(t *testing.T) {
	home := t.TempDir()
	cfgDir := filepath.Join(home, ".labproxy", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := 42424
	if err := os.WriteFile(filepath.Join(cfgDir, "labproxy.pid"), []byte("42424\n"), 0o644); err != nil {
		t.Fatalf("write pid: %v", err)
	}
	if got := readPIDFile(home); got != want {
		t.Errorf("readPIDFile: got %d want %d", got, want)
	}
}

// TestReadPIDFile_FallbackScan PID 文件缺失时回退 ps 扫描；
// 使用独特临时目录，系统真实 mihomo 不会匹配，应返回 0。
func TestReadPIDFile_FallbackScan(t *testing.T) {
	home := t.TempDir()
	// 不写 pid 文件；scanMihomoPID 找不到匹配 -d <home> 的进程 → 0
	if got := readPIDFile(home); got != 0 {
		t.Errorf("readPIDFile fallback should be 0 for unmatched home, got %d", got)
	}
}

// TestProcessAlive_CurrentProcess 当前测试进程必然存活。
func TestProcessAlive_CurrentProcess(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Fatal("current process should be alive")
	}
}

// TestProcessAlive_DeadPID 不存在的 PID 应判定为不存活。
func TestProcessAlive_DeadPID(t *testing.T) {
	// 999999 极大概率不存在；即使被回收复用，ps -p 也会因非 labproxy 判定失败
	if processAlive(999999) {
		t.Fatal("pid 999999 should not be alive")
	}
}

func init() {
	// 触发 runtime 引用避免未使用（跨平台 ps 解析路径）
	_ = runtime.GOOS
}
