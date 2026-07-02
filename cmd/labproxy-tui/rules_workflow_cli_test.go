package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRulesWorkflowCandidatesPrintsDefaultBatch(t *testing.T) {
	mixin := writeWorkflowMixin(t, "mode: rule\nrules: []\n")

	code, out := runRulesWorkflowForTest(t, mixin, "candidates")

	if code != 0 {
		t.Fatalf("exit=%d output=%s", code, out)
	}
	for _, want := range []string{"github", "openai", "Proxies", "OpenAI"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected candidates output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRunRulesWorkflowInspectPrintsMixinCounts(t *testing.T) {
	mixin := writeWorkflowMixin(t, `mode: rule
rule-providers:
  github:
    type: http
    behavior: classical
    url: "https://example.test/github.yaml"
    path: "./rule-providers/github.yaml"
    interval: 86400
rules:
  - DOMAIN-SUFFIX,example.com,DIRECT
`)

	code, out := runRulesWorkflowForTest(t, mixin, "inspect")

	if code != 0 {
		t.Fatalf("exit=%d output=%s", code, out)
	}
	for _, want := range []string{"rules=1", "providers=1", "github"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected inspect output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRunRulesWorkflowFetchUsesURLOOverride(t *testing.T) {
	mixin := writeWorkflowMixin(t, "mode: rule\nrules: []\n")
	server := workflowRuleSourceServer(t, "payload:\n  - DOMAIN-SUFFIX,github.com\n")

	code, out := runRulesWorkflowForTest(t, mixin, "fetch", "--candidates=github", "--url-override=github="+server.URL)

	if code != 0 {
		t.Fatalf("exit=%d output=%s", code, out)
	}
	if !strings.Contains(out, "github rules=1") {
		t.Fatalf("expected github rule count, got:\n%s", out)
	}
}

func TestRunRulesWorkflowValidateSucceedsWithKnownTargetGroup(t *testing.T) {
	mixin := writeWorkflowMixin(t, "mode: rule\nrules: []\n")
	server := workflowRuleSourceServer(t, "payload:\n  - DOMAIN-SUFFIX,github.com\n")

	code, out := runRulesWorkflowForTest(t, mixin, "validate", "--groups=Proxies", "--candidates=github", "--url-override=github="+server.URL)

	if code != 0 {
		t.Fatalf("exit=%d output=%s", code, out)
	}
	if !strings.Contains(out, "github rules=1") {
		t.Fatalf("expected validated github count, got:\n%s", out)
	}
}

func TestRunRulesWorkflowValidateFailsWhenTargetGroupMissing(t *testing.T) {
	mixin := writeWorkflowMixin(t, "mode: rule\nrules: []\n")
	server := workflowRuleSourceServer(t, "payload:\n  - DOMAIN-SUFFIX,github.com\n")

	code, out := runRulesWorkflowForTest(t, mixin, "validate", "--groups=DIRECT", "--candidates=github", "--url-override=github="+server.URL)

	if code == 0 {
		t.Fatalf("expected failure, output=%s", out)
	}
	if !strings.Contains(out, "Proxies") {
		t.Fatalf("expected missing target group in output, got:\n%s", out)
	}
}

func TestRunRulesWorkflowValidateRequiresGroupsOrEndpoint(t *testing.T) {
	mixin := writeWorkflowMixin(t, "mode: rule\nrules: []\n")
	server := workflowRuleSourceServer(t, "payload:\n  - DOMAIN-SUFFIX,github.com\n")

	code, out := runRulesWorkflowForTest(t, mixin, "validate", "--candidates=github", "--url-override=github="+server.URL)

	if code == 0 {
		t.Fatalf("expected failure, output=%s", out)
	}
	if !strings.Contains(out, "--groups") || !strings.Contains(out, "--endpoint") {
		t.Fatalf("expected groups-or-endpoint guidance, got:\n%s", out)
	}
}

func TestRunRulesWorkflowPlanPrintsProviderRules(t *testing.T) {
	mixin := writeWorkflowMixin(t, "mode: rule\nrules: []\n")

	code, out := runRulesWorkflowForTest(t, mixin, "plan", "--candidates=github,openai")

	if code != 0 {
		t.Fatalf("exit=%d output=%s", code, out)
	}
	for _, want := range []string{"RULE-SET,github,Proxies", "RULE-SET,openai,OpenAI"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected plan output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestRunRulesWorkflowApplyValidatesBeforeWritingAndPrintsBackup(t *testing.T) {
	mixin := writeWorkflowMixin(t, "mode: rule\nrules: []\n")
	server := workflowRuleSourceServer(t, "payload:\n  - DOMAIN-SUFFIX,github.com\n")

	code, out := runRulesWorkflowForTest(t, mixin, "apply", "--groups=Proxies", "--candidates=github", "--url-override=github="+server.URL)

	if code != 0 {
		t.Fatalf("exit=%d output=%s", code, out)
	}
	for _, want := range []string{"ok", "backup="} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected apply output to contain %q, got:\n%s", want, out)
		}
	}
	data, err := os.ReadFile(mixin)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"rule-providers:", "github:", "RULE-SET,github,Proxies", server.URL} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected mixin to contain %q after apply, got:\n%s", want, text)
		}
	}
}

func TestRunRulesWorkflowApplyRefusesUnknownTargetGroupsBeforeWriting(t *testing.T) {
	mixin := writeWorkflowMixin(t, "mode: rule\nrules: []\n")
	before, err := os.ReadFile(mixin)
	if err != nil {
		t.Fatal(err)
	}
	server := workflowRuleSourceServer(t, "payload:\n  - DOMAIN-SUFFIX,github.com\n")

	code, out := runRulesWorkflowForTest(t, mixin, "apply", "--groups=DIRECT", "--candidates=github", "--url-override=github="+server.URL)

	if code == 0 {
		t.Fatalf("expected failure, output=%s", out)
	}
	after, err := os.ReadFile(mixin)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("apply wrote despite failed validation; before=%q after=%q", before, after)
	}
}

func TestRunRulesWorkflowVerifyReloadsThenInspectsRuntime(t *testing.T) {
	mixin := writeWorkflowMixin(t, "mode: rule\nrules: []\n")
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.String())
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/configs" && r.URL.Query().Get("force") == "true":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/proxies":
			_, _ = w.Write([]byte(`{"proxies":{"Proxies":{"name":"Proxies","type":"Selector"},"Node-A":{"name":"Node-A","type":"SS"}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/connections":
			_, _ = w.Write([]byte(`{"connections":[{"id":"1"},{"id":"2"}]}`))
		default:
			t.Fatalf("unexpected runtime request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	code, out := runRulesWorkflowForTest(t, mixin, "verify", "--endpoint="+server.URL, "--reload-config=/tmp/runtime.yaml")

	if code != 0 {
		t.Fatalf("exit=%d output=%s", code, out)
	}
	for _, want := range []string{"reloaded=/tmp/runtime.yaml", "strategy-groups=Proxies", "connections=2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected verify output to contain %q, got:\n%s", want, out)
		}
	}
	wantCalls := []string{"PUT /configs?force=true", "GET /proxies", "GET /connections"}
	if strings.Join(calls, "|") != strings.Join(wantCalls, "|") {
		t.Fatalf("unexpected call order: got %v want %v", calls, wantCalls)
	}
}

func TestRunRulesWorkflowRollbackRestoresBackup(t *testing.T) {
	original := "mode: rule\nrules:\n  - DOMAIN-SUFFIX,original.test,DIRECT\n"
	mixin := writeWorkflowMixin(t, original)
	backup := filepath.Join(filepath.Dir(mixin), "mixin.yaml.backup")
	if err := os.WriteFile(backup, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mixin, []byte("mode: rule\nrules:\n  - DOMAIN-SUFFIX,changed.test,DIRECT\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out := runRulesWorkflowForTest(t, mixin, "rollback", "--backup="+backup)

	if code != 0 {
		t.Fatalf("exit=%d output=%s", code, out)
	}
	data, err := os.ReadFile(mixin)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("rollback did not restore backup; got %q", data)
	}
	if !strings.Contains(out, "ok") {
		t.Fatalf("expected ok output, got:\n%s", out)
	}
}

func runRulesWorkflowForTest(t *testing.T, mixin string, args ...string) (int, string) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	allArgs := append([]string{"workflow"}, args...)
	code := runRulesCLI(&stdout, &stderr, allArgs, mixin)
	return code, stdout.String() + stderr.String()
}

func writeWorkflowMixin(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mixin.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func workflowRuleSourceServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}
