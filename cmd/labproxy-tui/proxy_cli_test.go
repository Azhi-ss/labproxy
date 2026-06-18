package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"labproxy/internal/profile"
)

func TestRunProxiesCLI_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxies" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"proxies":{"GLOBAL":{"name":"GLOBAL","type":"Selector","now":"DIRECT","all":["DIRECT","REJECT"]}}}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	code := runProxiesCLI(&out, &out, []string{"--json"}, srv.URL, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Proxies map[string]struct {
				Name string `json:"name"`
				Type string `json:"type"`
				Now  string `json:"now"`
			} `json:"proxies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if !env.OK {
		t.Fatalf("ok=false: %s", out.String())
	}
	g, ok := env.Data.Proxies["GLOBAL"]
	if !ok {
		t.Fatalf("GLOBAL missing: %+v", env.Data.Proxies)
	}
	if g.Type != "Selector" || g.Now != "DIRECT" {
		t.Errorf("GLOBAL wrong: %+v", g)
	}
}

func TestRunConnectionsCLI_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connections" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"downloadTotal":100,"uploadTotal":200,"connections":[{"id":"abc","upload":10,"download":20,"rule":"DOMAIN-SUFFIX","chains":["DIRECT"],"metadata":{"network":"tcp","host":"example.com"}}]}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	code := runConnectionsCLI(&out, &out, []string{"--json"}, srv.URL, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			DownloadTotal int64 `json:"downloadTotal"`
			Connections   []struct {
				ID string `json:"id"`
			} `json:"connections"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if env.Data.DownloadTotal != 100 {
		t.Errorf("downloadTotal=%d want 100", env.Data.DownloadTotal)
	}
	if len(env.Data.Connections) != 1 || env.Data.Connections[0].ID != "abc" {
		t.Errorf("connections wrong: %+v", env.Data.Connections)
	}
}

func TestRunDelayCLI_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/proxies/MYNODE/delay") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("url") == "" || r.URL.Query().Get("timeout") == "" {
			http.Error(w, "missing params", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"delay":42}`))
	}))
	defer srv.Close()

	var out bytes.Buffer
	code := runDelayCLI(&out, &out, []string{"MYNODE", "--json"}, srv.URL, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Name  string `json:"name"`
			Delay int    `json:"delay"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if env.Data.Delay != 42 || env.Data.Name != "MYNODE" {
		t.Errorf("delay result wrong: %+v", env.Data)
	}
}

func TestRunDelayCLI_MissingName(t *testing.T) {
	var out bytes.Buffer
	code := runDelayCLI(&out, &out, []string{"--json"}, "http://x", "")
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing name")
	}
	if !strings.Contains(out.String(), "name") && !strings.Contains(out.String(), "usage") {
		t.Errorf("error should mention name/usage: %s", out.String())
	}
}

func TestRunProxiesCLI_HumanReadable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"proxies":{"GLOBAL":{"name":"GLOBAL","type":"Selector","now":"DIRECT","all":["DIRECT"]}}}`)
	}))
	defer srv.Close()

	var out bytes.Buffer
	code := runProxiesCLI(&out, &out, []string{}, srv.URL, "")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	s := out.String()
	if !strings.Contains(s, "GLOBAL") {
		t.Errorf("human output should contain GLOBAL: %s", s)
	}
}

func TestRunConnectionsCLI_CloseOne(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	var out bytes.Buffer
	code := runConnectionsCLI(&out, &out, []string{"close", "conn-xyz", "--json"}, srv.URL, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method=%s want DELETE", gotMethod)
	}
	if gotPath != "/connections/conn-xyz" {
		t.Errorf("path=%s want /connections/conn-xyz", gotPath)
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Closed string `json:"closed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if !env.OK || env.Data.Closed != "conn-xyz" {
		t.Errorf("result wrong: %+v", env)
	}
}

func TestRunConnectionsCLI_CloseAll(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	var out bytes.Buffer
	code := runConnectionsCLI(&out, &out, []string{"close", "all", "--json"}, srv.URL, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method=%s want DELETE", gotMethod)
	}
	if gotPath != "/connections" {
		t.Errorf("path=%s want /connections (all)", gotPath)
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Closed string `json:"closed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if !env.OK || env.Data.Closed != "all" {
		t.Errorf("result wrong: %+v", env)
	}
}

func TestRunConnectionsCLI_CloseMissingTarget(t *testing.T) {
	var out bytes.Buffer
	code := runConnectionsCLI(&out, &out, []string{"close", "--json"}, "http://x", "")
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing close target")
	}
	s := out.String()
	if !strings.Contains(s, "usage") && !strings.Contains(s, "id") && !strings.Contains(s, "all") {
		t.Errorf("error should mention usage/id/all: %s", s)
	}
}

func TestRunConnectionsCLI_CloseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	var out bytes.Buffer
	code := runConnectionsCLI(&out, &out, []string{"close", "conn-1", "--json"}, srv.URL, "")
	if code == 0 {
		t.Fatalf("expected non-zero exit on server error")
	}
	var env struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if env.OK {
		t.Errorf("should be ok=false")
	}
	if !strings.Contains(env.Error, "close") {
		t.Errorf("error should mention close: %q", env.Error)
	}
}

func TestRunConnectionsCLI_CloseHuman(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	var out bytes.Buffer
	code := runConnectionsCLI(&out, &out, []string{"close", "conn-1"}, srv.URL, "")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "conn-1") {
		t.Errorf("human output should mention conn-1: %s", out.String())
	}
}

func TestRunTestCLI_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/proxies" && r.Method == http.MethodGet:
			fmt.Fprint(w, `{"proxies":{"GLOBAL":{"name":"GLOBAL","type":"Selector","all":["Node-A","Node-B","Node-C"]},"Node-A":{"name":"Node-A","type":"Shadowsocks"},"Node-B":{"name":"Node-B","type":"Shadowsocks"},"Node-C":{"name":"Node-C","type":"Shadowsocks"}}}`)
		case strings.HasSuffix(r.URL.Path, "/delay"):
			parts := strings.Split(r.URL.Path, "/")
			name := parts[2]
			delay := 0
			switch name {
			case "Node-A":
				delay = 120
			case "Node-B":
				delay = 50
			case "Node-C":
				http.Error(w, "timeout", http.StatusGatewayTimeout)
				return
			}
			fmt.Fprintf(w, `{"delay":%d}`, delay)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	var out bytes.Buffer
	code := runTestCLI(&out, &out, []string{"GLOBAL", "--json"}, srv.URL, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Group   string         `json:"group"`
			Results map[string]int `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if !env.OK || env.Data.Group != "GLOBAL" {
		t.Errorf("result wrong: %+v", env)
	}
	if env.Data.Results["Node-A"] != 120 || env.Data.Results["Node-B"] != 50 {
		t.Errorf("delays wrong: %+v", env.Data.Results)
	}
	if env.Data.Results["Node-C"] != -1 {
		t.Errorf("Node-C should be -1, got %d", env.Data.Results["Node-C"])
	}
}

func TestRunTestCLI_HumanSorted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/proxies":
			fmt.Fprint(w, `{"proxies":{"GLOBAL":{"name":"GLOBAL","type":"Selector","all":["Node-A","Node-B"]},"Node-A":{"name":"Node-A","type":"Shadowsocks"},"Node-B":{"name":"Node-B","type":"Shadowsocks"}}}`)
		case strings.HasSuffix(r.URL.Path, "/delay"):
			parts := strings.Split(r.URL.Path, "/")
			name := parts[2]
			delay := 0
			if name == "Node-A" {
				delay = 200
			} else {
				delay = 30
			}
			fmt.Fprintf(w, `{"delay":%d}`, delay)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	var out bytes.Buffer
	code := runTestCLI(&out, &out, []string{"GLOBAL"}, srv.URL, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}
	s := out.String()
	// Node-B (30ms) 应排在 Node-A (200ms) 之前（按延迟升序）
	idxB := strings.Index(s, "Node-B")
	idxA := strings.Index(s, "Node-A")
	if idxB < 0 || idxA < 0 {
		t.Fatalf("output missing nodes: %s", s)
	}
	if idxB > idxA {
		t.Errorf("expected Node-B (lower delay) before Node-A: %s", s)
	}
}

func TestRunTestCLI_GroupNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"proxies":{"GLOBAL":{"name":"GLOBAL","type":"Selector","all":["A"]}}}`)
	}))
	defer srv.Close()

	var out bytes.Buffer
	code := runTestCLI(&out, &out, []string{"NoExist", "--json"}, srv.URL, "")
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing group")
	}
	var env struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if env.OK {
		t.Errorf("should be ok=false")
	}
}

func TestRunTestCLI_DefaultGroup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/proxies":
			fmt.Fprint(w, `{"proxies":{"GLOBAL":{"name":"GLOBAL","type":"Selector","all":["A"]},"A":{"name":"A","type":"Shadowsocks"}}}`)
		case strings.HasSuffix(r.URL.Path, "/delay"):
			fmt.Fprint(w, `{"delay":10}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	var out bytes.Buffer
	// 不指定组名，应默认取 GLOBAL
	code := runTestCLI(&out, &out, []string{"--json"}, srv.URL, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}
	var env struct {
		Data struct {
			Group string `json:"group"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if env.Data.Group != "GLOBAL" {
		t.Errorf("default group should be GLOBAL, got %q", env.Data.Group)
	}
}

func TestRunLogsCLI_NoFollow(t *testing.T) {
	home := t.TempDir()
	logDir := home + "/.labproxy/logs"
	os.MkdirAll(logDir, 0o755)
	content := ""
	for i := 0; i < 60; i++ {
		content += fmt.Sprintf("line %d\n", i)
	}
	os.WriteFile(logDir+"/labproxy.log", []byte(content), 0o644)

	var out bytes.Buffer
	// 无 -f：输出最近 50 行
	code := runLogsCLI(&out, &out, []string{}, home, "http://x", "")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}
	s := out.String()
	// 应含末尾 line 59，不应含 line 0（被截断到 50 行）
	if !strings.Contains(s, "line 59") {
		t.Errorf("expected recent line 59: %s", s)
	}
	if strings.Contains(s, "line 0\n") {
		t.Errorf("should not contain line 0 (truncated): %s", s)
	}
}

func TestRunLogsCLI_FollowStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/logs" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		lines := []string{
			`{"type":"info","payload":"hello"}`,
			`{"type":"error","payload":"oops"}`,
		}
		for _, l := range lines {
			fmt.Fprintln(w, l)
			if flusher != nil {
				flusher.Flush()
			}
		}
		// 写完即返回，关闭连接让客户端 EOF 退出
		return
	}))
	defer srv.Close()

	var out bytes.Buffer
	// -f 流式；ctx 在测试内控制取消
	code := runLogsCLIFollow(&out, &out, []string{"-f"}, srv.URL, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "hello") || !strings.Contains(s, "oops") {
		t.Errorf("expected hello+oops in output: %s", s)
	}
}

func TestRunLogsCLI_FollowJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		fmt.Fprintln(w, `{"type":"info","payload":"msg1"}`)
		if flusher != nil {
			flusher.Flush()
		}
		return
	}))
	defer srv.Close()

	var out bytes.Buffer
	code := runLogsCLIFollow(&out, &out, []string{"-f", "--json"}, srv.URL, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}
	// 每行应为合法 JSON envelope
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) == 0 {
		t.Fatal("expected at least one json line")
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Level   string `json:"level"`
			Payload string `json:"payload"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &env); err != nil {
		t.Fatalf("invalid json line: %v\n%s", err, lines[0])
	}
	if env.Data.Payload != "msg1" {
		t.Errorf("payload=%s want msg1", env.Data.Payload)
	}
}

func TestRunLogsCLI_LevelFlag(t *testing.T) {
	var gotLevel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLevel = r.URL.Query().Get("level")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"type":"warning","payload":"w"}`)
		w.(http.Flusher).Flush()
		return
	}))
	defer srv.Close()

	var out bytes.Buffer
	code := runLogsCLIFollow(&out, &out, []string{"-f", "--level", "warning"}, srv.URL, "")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if gotLevel != "warning" {
		t.Errorf("level=%s want warning", gotLevel)
	}
}

func TestRunProfileCLI_CreateFromCurrent(t *testing.T) {
	home := t.TempDir()
	labDir := home + "/.labproxy"
	os.MkdirAll(labDir, 0o755)
	// 当前 mixin.yaml + rules
	os.WriteFile(labDir+"/mixin.yaml", []byte("system-proxy:\n  enable: true\n"), 0o644)
	os.WriteFile(labDir+"/rules.yaml", []byte("rules:\n  - MATCH,DIRECT\n"), 0o644)

	var out bytes.Buffer
	code := runProfileCLI(&out, &out, []string{"create", "snap1", "--json"}, home)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if !env.OK || env.Data.Name != "snap1" {
		t.Errorf("result wrong: %+v", env)
	}
	// profile 应存在
	if _, err := os.Stat(labDir + "/profiles/snap1/mixin.yaml"); err != nil {
		t.Errorf("profile not created: %v", err)
	}
}

func TestRunProfileCLI_List(t *testing.T) {
	home := t.TempDir()
	s, _ := profile.NewStore(home + "/.labproxy")
	s.Create(profile.Profile{Name: "a", Mixin: []byte("x\n"), Rules: []byte("y\n")})
	s.Create(profile.Profile{Name: "b", Mixin: []byte("x\n"), Rules: []byte("y\n")})

	var out bytes.Buffer
	code := runProfileCLI(&out, &out, []string{"list", "--json"}, home)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}
	var env struct {
		OK   bool     `json:"ok"`
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if !env.OK {
		t.Fatal("ok=false")
	}
	want := map[string]bool{"a": true, "b": true}
	if len(env.Data) != 2 {
		t.Fatalf("expected 2 profiles, got %v", env.Data)
	}
	for _, n := range env.Data {
		if !want[n] {
			t.Errorf("unexpected %q", n)
		}
	}
}

func TestRunProfileCLI_Delete(t *testing.T) {
	home := t.TempDir()
	s, _ := profile.NewStore(home + "/.labproxy")
	s.Create(profile.Profile{Name: "tmp", Mixin: []byte("x\n"), Rules: []byte("y\n")})

	var out bytes.Buffer
	code := runProfileCLI(&out, &out, []string{"delete", "tmp", "--json"}, home)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}
	var env struct {
		OK bool `json:"ok"`
	}
	json.Unmarshal(out.Bytes(), &env)
	if !env.OK {
		t.Error("expected ok=true")
	}
	if _, err := os.Stat(home + "/.labproxy/profiles/tmp"); !os.IsNotExist(err) {
		t.Errorf("profile should be removed: %v", err)
	}
}

func TestRunProfileCLI_UseAppliesMixin(t *testing.T) {
	home := t.TempDir()
	labDir := home + "/.labproxy"
	os.MkdirAll(labDir, 0o755)
	// 先建一个 profile，mixin 含特定内容
	s, _ := profile.NewStore(labDir)
	s.Create(profile.Profile{Name: "work", Mixin: []byte("system-proxy:\n  enable: false\n"), Rules: []byte("rules: []\n")})
	// 当前 mixin 是另一份
	os.WriteFile(labDir+"/mixin.yaml", []byte("system-proxy:\n  enable: true\n"), 0o644)

	var out bytes.Buffer
	code := runProfileCLI(&out, &out, []string{"use", "work", "--json"}, home)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, out.String())
	}
	// mixin.yaml 应被覆写为 profile 的内容
	got, _ := os.ReadFile(labDir + "/mixin.yaml")
	if string(got) != "system-proxy:\n  enable: false\n" {
		t.Errorf("mixin not applied: %q", got)
	}
}

func TestRunProfileCLI_UseNotFound(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(home+"/.labproxy", 0o755)
	var out bytes.Buffer
	code := runProfileCLI(&out, &out, []string{"use", "nope", "--json"}, home)
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	var env struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	json.Unmarshal(out.Bytes(), &env)
	if env.OK {
		t.Error("expected ok=false")
	}
	if !strings.Contains(env.Error, "nope") {
		t.Errorf("error should mention nope: %q", env.Error)
	}
}
