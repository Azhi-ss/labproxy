package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRunDoctorCLI_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/version":
			fmt.Fprint(w, `{"version":"v1.18","meta":true}`)
		case "/configs":
			fmt.Fprint(w, `{"mode":"rule","mixed-port":7890}`)
		case "/connections":
			fmt.Fprint(w, `{"downloadTotal":0,"uploadTotal":0,"connections":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	home := t.TempDir()
	labDir := filepath.Join(home, ".labproxy")
	os.MkdirAll(labDir+"/config", 0o755)
	os.WriteFile(labDir+"/mixin.yaml", []byte("system-proxy:\n  enable: true\n"), 0o644)

	var out bytes.Buffer
	code := runDoctorCLI(&out, &out, []string{"--json"}, home, srv.URL, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Checks []struct {
				Name   string `json:"name"`
				OK     bool   `json:"ok"`
				Detail string `json:"detail"`
			} `json:"checks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if !env.OK {
		t.Fatal("expected ok=true")
	}
	if len(env.Data.Checks) == 0 {
		t.Fatal("expected checks")
	}
	// 应含 kernel/profile/mixin 检查项
	names := map[string]bool{}
	for _, c := range env.Data.Checks {
		names[c.Name] = true
	}
	if !names["kernel_api"] {
		t.Errorf("missing kernel_api check: %+v", names)
	}
}

func TestRunDoctorCLI_KernelUnreachable(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(home+"/.labproxy/config", 0o755)

	var out bytes.Buffer
	code := runDoctorCLI(&out, &out, []string{"--json"}, home, "http://127.0.0.1:1", "")
	// kernel 不可达不应让整个 doctor 退出非零，而是标记该检查失败
	if code != 0 {
		t.Fatalf("doctor should exit 0 even with failed checks: %d", code)
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Checks []struct {
				Name string `json:"name"`
				OK   bool  `json:"ok"`
			} `json:"checks"`
		} `json:"data"`
	}
	json.Unmarshal(out.Bytes(), &env)
	if !env.OK {
		t.Fatal("envelope ok should be true")
	}
	// kernel_api 检查应为 false
	for _, c := range env.Data.Checks {
		if c.Name == "kernel_api" && c.OK {
			t.Error("kernel_api should be false when unreachable")
		}
	}
}

func TestRunDoctorCLI_ProfileCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"version":"t","meta":true}`)
	}))
	defer srv.Close()

	home := t.TempDir()
	labDir := filepath.Join(home, ".labproxy")
	os.MkdirAll(labDir+"/profiles/work", 0o755)

	var out bytes.Buffer
	code := runDoctorCLI(&out, &out, []string{"--json"}, home, srv.URL, "")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	var env struct {
		Data struct {
			Checks []struct {
				Name   string `json:"name"`
				Detail string `json:"detail"`
			} `json:"checks"`
		} `json:"data"`
	}
	json.Unmarshal(out.Bytes(), &env)
	found := false
	for _, c := range env.Data.Checks {
		if c.Name == "profiles" {
			found = true
		}
	}
	if !found {
		t.Error("missing profiles check")
	}
}
