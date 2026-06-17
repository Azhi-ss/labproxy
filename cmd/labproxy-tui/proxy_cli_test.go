package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
		OK   bool   `json:"ok"`
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
		OK   bool   `json:"ok"`
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
