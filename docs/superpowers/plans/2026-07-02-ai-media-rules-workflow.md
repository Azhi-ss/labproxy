# AI and Media Rules Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a repeatable inspect/fetch/validate/plan/apply/verify/rollback workflow for adding AI/developer and media `rule-providers` to labproxy without destabilizing the current proxy setup.

**Architecture:** Add a focused `internal/ruleworkflow` package that owns candidate registry, source fetching, validation, plan rendering, and backup metadata. Expose it through a `labproxy rules workflow ...` CLI surface that reuses existing `internal/rules.Store` and `internal/proxy.Client` instead of modifying TUI view code.

**Tech Stack:** Go 1.24, standard library HTTP/YAML/file APIs, existing `internal/rules`, existing `internal/proxy`, shell integration tests.

## Global Constraints

- Preserve existing labproxy TUN, DNS, system proxy, node subscriptions, and proxy-group membership.
- Keep user-maintained local override rules at the top of `~/.labproxy/mixin.yaml`.
- Do not replace the existing 4237-rule subscription-generated base.
- Use `rule-providers` plus `RULE-SET` entries for external rules.
- First batch provider targets: `github -> Proxies`, `openai -> OpenAI`, `anthropic -> OpenAI`, `youtube -> YouTube`, `netflix -> Netflix`, `disney -> Disney`, `telegram -> Telegram`.
- Keep existing Hugging Face inline overrides (`DOMAIN-SUFFIX,huggingface.co,US` and `DOMAIN-SUFFIX,hf-mirror.com,DIRECT`) instead of importing a provider until a dedicated non-404 source is found.
- Source check on 2026-07-02: the planned blackmatrix7 Hugging Face Clash URL returned 404, so it is intentionally excluded from first-batch provider imports.
- Apple rules are outside the first batch.
- No new dependencies.

---

## File Structure

- Create `internal/ruleworkflow/catalog.go`: built-in provider candidates and target mappings.
- Create `internal/ruleworkflow/fetch.go`: fetch external rule source bytes with size and HTTP checks.
- Create `internal/ruleworkflow/validate.go`: parse source rules, verify provider behavior, target groups, names, and rule counts.
- Create `internal/ruleworkflow/plan.go`: construct provider additions, `RULE-SET` rules, and human-readable plan output.
- Create `internal/ruleworkflow/apply.go`: backup, apply provider/rule changes through `rules.Store`, and rollback by backup path.
- Create `internal/ruleworkflow/verify.go`: runtime API verification helpers.
- Create `internal/ruleworkflow/*_test.go`: unit tests for each component.
- Modify `cmd/labproxy-tui/rules_cli.go`: dispatch `rules workflow <subcommand>`.
- Create `cmd/labproxy-tui/rules_workflow_cli.go`: CLI argument parsing and output.
- Modify `internal/proxy/client.go`: add `ReloadConfig(ctx, path string) error` for `PUT /configs?force=true`.
- Test `tests/rules_workflow_cli_test.sh`: CLI integration against temporary mixin files and a local HTTP fixture.
- Modify `README.md`: document the workflow commands and first-batch provider policy.

---

### Task 1: Candidate Registry and Plan Model

**Files:**
- Create: `internal/ruleworkflow/catalog.go`
- Create: `internal/ruleworkflow/catalog_test.go`

**Interfaces:**
- Produces: `Candidate`, `Plan`, `DefaultCandidates() []Candidate`, `SelectedCandidates(names []string) ([]Candidate, error)`
- Consumes: no local project state

- [ ] **Step 1: Write failing tests for default candidates**

Create `internal/ruleworkflow/catalog_test.go`:

```go
package ruleworkflow

import "testing"

func TestDefaultCandidatesContainFirstBatch(t *testing.T) {
	candidates := DefaultCandidates()
	got := map[string]Candidate{}
	for _, c := range candidates {
		got[c.Name] = c
	}
	for _, name := range []string{"github", "openai", "anthropic", "youtube", "netflix", "disney", "telegram"} {
		c, ok := got[name]
		if !ok {
			t.Fatalf("missing candidate %s", name)
		}
		if c.Provider.Name != name {
			t.Fatalf("candidate %s provider name mismatch: %q", name, c.Provider.Name)
		}
		if c.TargetGroup == "" {
			t.Fatalf("candidate %s has empty target group", name)
		}
		if c.Provider.Type != "http" {
			t.Fatalf("candidate %s provider type = %q, want http", name, c.Provider.Type)
		}
	}
}

func TestSelectedCandidatesRejectUnknownName(t *testing.T) {
	_, err := SelectedCandidates([]string{"github", "missing"})
	if err == nil {
		t.Fatal("expected error for unknown candidate")
	}
}

func TestSelectedCandidatesDefaultAll(t *testing.T) {
	got, err := SelectedCandidates(nil)
	if err != nil {
		t.Fatalf("SelectedCandidates nil: %v", err)
	}
	if len(got) != len(DefaultCandidates()) {
		t.Fatalf("len = %d, want %d", len(got), len(DefaultCandidates()))
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run: `go test ./internal/ruleworkflow -run 'TestDefaultCandidates|TestSelectedCandidates' -v`

Expected: FAIL because `internal/ruleworkflow` does not exist.

- [ ] **Step 3: Implement candidate registry**

Create `internal/ruleworkflow/catalog.go`:

```go
package ruleworkflow

import (
	"fmt"
	"sort"

	"labproxy/internal/rules"
)

type Candidate struct {
	Name        string
	Description string
	SourceURL   string
	TargetGroup string
	Provider    rules.Provider
}

type Plan struct {
	Candidates []Candidate
	Providers  []rules.Provider
	Rules      []rules.Rule
}

func DefaultCandidates() []Candidate {
	return []Candidate{
		candidate("github", "GitHub domains and asset hosts", "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/GitHub/GitHub.yaml", "Proxies", "classical", "./rule-providers/github.yaml"),
		candidate("openai", "OpenAI and ChatGPT service domains", "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/OpenAI/OpenAI.yaml", "OpenAI", "classical", "./rule-providers/openai.yaml"),
		candidate("anthropic", "Anthropic and Claude service domains", "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Anthropic/Anthropic.yaml", "OpenAI", "classical", "./rule-providers/anthropic.yaml"),
		candidate("youtube", "YouTube service domains", "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/YouTube/YouTube.yaml", "YouTube", "classical", "./rule-providers/youtube.yaml"),
		candidate("netflix", "Netflix service domains and CIDRs", "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Netflix/Netflix.yaml", "Netflix", "classical", "./rule-providers/netflix.yaml"),
		candidate("disney", "Disney and Disney Plus service domains", "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Disney/Disney.yaml", "Disney", "classical", "./rule-providers/disney.yaml"),
		candidate("telegram", "Telegram service domains and CIDRs", "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Telegram/Telegram.yaml", "Telegram", "classical", "./rule-providers/telegram.yaml"),
	}
}

func candidate(name, description, url, target, behavior, path string) Candidate {
	return Candidate{
		Name:        name,
		Description: description,
		SourceURL:   url,
		TargetGroup: target,
		Provider: rules.Provider{
			Name:     name,
			Type:     "http",
			Behavior: behavior,
			URL:      url,
			Path:     path,
			Interval: 86400,
		},
	}
}

func SelectedCandidates(names []string) ([]Candidate, error) {
	all := DefaultCandidates()
	byName := map[string]Candidate{}
	for _, c := range all {
		byName[c.Name] = c
	}
	if len(names) == 0 {
		return all, nil
	}
	selected := make([]Candidate, 0, len(names))
	for _, name := range names {
		c, ok := byName[name]
		if !ok {
			keys := make([]string, 0, len(byName))
			for k := range byName {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return nil, fmt.Errorf("unknown candidate %q; known: %v", name, keys)
		}
		selected = append(selected, c)
	}
	return selected, nil
}
```

- [ ] **Step 4: Run tests and verify they pass**

Run: `go test ./internal/ruleworkflow -run 'TestDefaultCandidates|TestSelectedCandidates' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ruleworkflow/catalog.go internal/ruleworkflow/catalog_test.go
git commit -m "Add AI and media rule candidate registry" \
  -m "The rule workflow needs a fixed first-batch catalog so validation and planning can run before touching live config." \
  -m "Constraint: First batch excludes Apple and targets existing labproxy strategy groups only." \
  -m "Confidence: high" \
  -m "Scope-risk: narrow" \
  -m "Tested: go test ./internal/ruleworkflow -run 'TestDefaultCandidates|TestSelectedCandidates' -v"
```

### Task 2: Fetch and Parse Candidate Rule Sources

**Files:**
- Create: `internal/ruleworkflow/fetch.go`
- Create: `internal/ruleworkflow/fetch_test.go`
- Create: `internal/ruleworkflow/parse.go`
- Create: `internal/ruleworkflow/parse_test.go`

**Interfaces:**
- Consumes: `Candidate`
- Produces: `FetchedSource`, `ProviderRule`, `FetchCandidate(ctx context.Context, client *http.Client, c Candidate) (FetchedSource, error)`, `ParseProviderRules(data []byte) ([]ProviderRule, error)`

- [ ] **Step 1: Write failing fetch and parse tests**

Create `internal/ruleworkflow/fetch_test.go`:

```go
package ruleworkflow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchCandidateSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("payload: [DOMAIN-SUFFIX,github.com]\n"))
	}))
	defer server.Close()

	c := candidate("github", "GitHub", server.URL, "Proxies", "classical", "./rule-providers/github.yaml")
	got, err := FetchCandidate(context.Background(), server.Client(), c)
	if err != nil {
		t.Fatalf("FetchCandidate: %v", err)
	}
	if got.Candidate.Name != "github" {
		t.Fatalf("candidate name = %q", got.Candidate.Name)
	}
	if string(got.Data) == "" {
		t.Fatal("expected non-empty data")
	}
}

func TestFetchCandidateRejectsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()

	c := candidate("github", "GitHub", server.URL, "Proxies", "classical", "./rule-providers/github.yaml")
	_, err := FetchCandidate(context.Background(), server.Client(), c)
	if err == nil {
		t.Fatal("expected http error")
	}
}
```

Create `internal/ruleworkflow/parse_test.go`:

```go
package ruleworkflow

import "testing"

func TestParseProviderRulesFromPayloadYAML(t *testing.T) {
	data := []byte(`
payload:
  - DOMAIN-SUFFIX,github.com
  - DOMAIN,raw.githubusercontent.com
`)
	got, err := ParseProviderRules(data)
	if err != nil {
		t.Fatalf("ParseProviderRules: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Payload != "github.com" || got[0].Type != "DOMAIN-SUFFIX" {
		t.Fatalf("first rule = %+v", got[0])
	}
}

func TestParseProviderRulesFromPlainList(t *testing.T) {
	data := []byte(`
# comment
- DOMAIN-SUFFIX,youtube.com
DOMAIN-SUFFIX,googlevideo.com
`)
	got, err := ParseProviderRules(data)
	if err != nil {
		t.Fatalf("ParseProviderRules: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run: `go test ./internal/ruleworkflow -run 'TestFetchCandidate|TestParseProviderRules' -v`

Expected: FAIL because fetch and parse functions do not exist.

- [ ] **Step 3: Implement fetch**

Create `internal/ruleworkflow/fetch.go`:

```go
package ruleworkflow

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

const MaxRuleSourceBytes = 5 * 1024 * 1024

type FetchedSource struct {
	Candidate Candidate
	Data      []byte
}

func FetchCandidate(ctx context.Context, client *http.Client, c Candidate) (FetchedSource, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.SourceURL, nil)
	if err != nil {
		return FetchedSource{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return FetchedSource{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return FetchedSource{}, fmt.Errorf("fetch %s: http %d", c.Name, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxRuleSourceBytes+1))
	if err != nil {
		return FetchedSource{}, err
	}
	if len(data) > MaxRuleSourceBytes {
		return FetchedSource{}, fmt.Errorf("fetch %s: source exceeds %d bytes", c.Name, MaxRuleSourceBytes)
	}
	return FetchedSource{Candidate: c, Data: data}, nil
}
```

- [ ] **Step 4: Implement parser**

Create `internal/ruleworkflow/parse.go`:

```go
package ruleworkflow

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type ProviderRule struct {
	Type    string
	Payload string
	Raw     string
}

func ParseProviderRules(data []byte) ([]ProviderRule, error) {
	var withPayload struct {
		Payload []string `yaml:"payload"`
	}
	if err := yaml.Unmarshal(data, &withPayload); err == nil && len(withPayload.Payload) > 0 {
		return parseRuleLines(withPayload.Payload)
	}

	var list []string
	if err := yaml.Unmarshal(data, &list); err == nil && len(list) > 0 {
		return parseRuleLines(list)
	}

	lines := strings.Split(string(data), "\n")
	return parseRuleLines(lines)
}

func parseRuleLines(lines []string) ([]ProviderRule, error) {
	out := []ProviderRule{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid provider rule %q", line)
		}
		out = append(out, ProviderRule{
			Type:    strings.TrimSpace(parts[0]),
			Payload: strings.TrimSpace(parts[1]),
			Raw:     line,
		})
	}
	return out, nil
}
```

- [ ] **Step 5: Run tests and verify they pass**

Run: `go test ./internal/ruleworkflow -run 'TestFetchCandidate|TestParseProviderRules' -v`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ruleworkflow/fetch.go internal/ruleworkflow/fetch_test.go internal/ruleworkflow/parse.go internal/ruleworkflow/parse_test.go
git commit -m "Add rule source fetch and parse helpers" \
  -m "Candidate validation needs to inspect external rule files before adding providers to mixin.yaml." \
  -m "Constraint: Rule source reads are capped at 5 MiB." \
  -m "Confidence: high" \
  -m "Scope-risk: narrow" \
  -m "Tested: go test ./internal/ruleworkflow -run 'TestFetchCandidate|TestParseProviderRules' -v"
```

### Task 3: Validation and Plan Rendering

**Files:**
- Create: `internal/ruleworkflow/validate.go`
- Create: `internal/ruleworkflow/validate_test.go`
- Create: `internal/ruleworkflow/plan.go`
- Create: `internal/ruleworkflow/plan_test.go`

**Interfaces:**
- Consumes: `FetchedSource`, `rules.Store`, strategy group names
- Produces: `ValidationResult`, `ValidateSources(...)`, `BuildPlan(...)`, `RenderPlan(...)`

- [ ] **Step 1: Write failing validation and plan tests**

Create `internal/ruleworkflow/validate_test.go`:

```go
package ruleworkflow

import "testing"

func TestValidateSourcesRejectsMissingTargetGroup(t *testing.T) {
	src := FetchedSource{
		Candidate: candidate("github", "GitHub", "https://example.test/github.yaml", "Proxies", "classical", "./rule-providers/github.yaml"),
		Data:      []byte("payload:\n  - DOMAIN-SUFFIX,github.com\n"),
	}
	_, err := ValidateSources([]FetchedSource{src}, map[string]bool{"DIRECT": true})
	if err == nil {
		t.Fatal("expected missing target group error")
	}
}

func TestValidateSourcesSuccess(t *testing.T) {
	src := FetchedSource{
		Candidate: candidate("github", "GitHub", "https://example.test/github.yaml", "Proxies", "classical", "./rule-providers/github.yaml"),
		Data:      []byte("payload:\n  - DOMAIN-SUFFIX,github.com\n"),
	}
	results, err := ValidateSources([]FetchedSource{src}, map[string]bool{"Proxies": true})
	if err != nil {
		t.Fatalf("ValidateSources: %v", err)
	}
	if len(results) != 1 || results[0].RuleCount != 1 {
		t.Fatalf("unexpected results: %+v", results)
	}
}
```

Create `internal/ruleworkflow/plan_test.go`:

```go
package ruleworkflow

import (
	"strings"
	"testing"
)

func TestBuildPlanCreatesRuleSetRules(t *testing.T) {
	candidates, err := SelectedCandidates([]string{"github", "openai"})
	if err != nil {
		t.Fatal(err)
	}
	plan := BuildPlan(candidates)
	if len(plan.Providers) != 2 || len(plan.Rules) != 2 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if plan.Rules[0].Type != "RULE-SET" || plan.Rules[0].Payload != "github" || plan.Rules[0].Proxy != "Proxies" {
		t.Fatalf("unexpected first rule: %+v", plan.Rules[0])
	}
}

func TestRenderPlanIncludesCandidateNames(t *testing.T) {
	candidates, _ := SelectedCandidates([]string{"github"})
	text := RenderPlan(BuildPlan(candidates))
	if !strings.Contains(text, "github") || !strings.Contains(text, "RULE-SET,github,Proxies") {
		t.Fatalf("rendered plan missing github details: %s", text)
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run: `go test ./internal/ruleworkflow -run 'TestValidateSources|TestBuildPlan|TestRenderPlan' -v`

Expected: FAIL because validation and plan functions do not exist.

- [ ] **Step 3: Implement validation**

Create `internal/ruleworkflow/validate.go`:

```go
package ruleworkflow

import (
	"fmt"
	"strings"

	"labproxy/internal/rules"
)

type ValidationResult struct {
	Candidate Candidate
	RuleCount int
	Rules     []ProviderRule
}

func ValidateSources(sources []FetchedSource, strategyGroups map[string]bool) ([]ValidationResult, error) {
	results := make([]ValidationResult, 0, len(sources))
	seenProviders := map[string]bool{}
	for _, src := range sources {
		c := src.Candidate
		if seenProviders[c.Provider.Name] {
			return nil, fmt.Errorf("duplicate provider %q", c.Provider.Name)
		}
		seenProviders[c.Provider.Name] = true
		if !strategyGroups[c.TargetGroup] {
			return nil, fmt.Errorf("candidate %s targets missing strategy group %q", c.Name, c.TargetGroup)
		}
		if err := rules.ValidateProvider(c.Provider); err != nil {
			return nil, fmt.Errorf("candidate %s provider invalid: %w", c.Name, err)
		}
		parsed, err := ParseProviderRules(src.Data)
		if err != nil {
			return nil, fmt.Errorf("candidate %s parse failed: %w", c.Name, err)
		}
		if len(parsed) == 0 {
			return nil, fmt.Errorf("candidate %s has no rules", c.Name)
		}
		for _, r := range parsed {
			if err := validateProviderRuleForBehavior(r, c.Provider.Behavior); err != nil {
				return nil, fmt.Errorf("candidate %s invalid rule %q: %w", c.Name, r.Raw, err)
			}
		}
		results = append(results, ValidationResult{Candidate: c, RuleCount: len(parsed), Rules: parsed})
	}
	return results, nil
}

func validateProviderRuleForBehavior(rule ProviderRule, behavior string) error {
	ruleType := strings.ToUpper(rule.Type)
	switch behavior {
	case "classical":
		return nil
	case "domain":
		if ruleType == "DOMAIN" || ruleType == "DOMAIN-SUFFIX" || ruleType == "DOMAIN-KEYWORD" {
			return nil
		}
	case "ipcidr":
		if ruleType == "IP-CIDR" || ruleType == "IP-CIDR6" {
			return nil
		}
	}
	return fmt.Errorf("rule type %s does not match behavior %s", rule.Type, behavior)
}
```

- [ ] **Step 4: Implement plan rendering**

Create `internal/ruleworkflow/plan.go`:

```go
package ruleworkflow

import (
	"fmt"
	"strings"

	"labproxy/internal/rules"
)

func BuildPlan(candidates []Candidate) Plan {
	plan := Plan{Candidates: candidates}
	for _, c := range candidates {
		plan.Providers = append(plan.Providers, c.Provider)
		plan.Rules = append(plan.Rules, rules.Rule{
			Type:    rules.TypeRuleSet,
			Payload: c.Provider.Name,
			Proxy:   c.TargetGroup,
			Enabled: true,
		})
	}
	return plan
}

func RenderPlan(plan Plan) string {
	var b strings.Builder
	b.WriteString("Rule workflow plan\n\n")
	b.WriteString("Providers:\n")
	for _, p := range plan.Providers {
		fmt.Fprintf(&b, "- %s behavior=%s path=%s url=%s\n", p.Name, p.Behavior, p.Path, p.URL)
	}
	b.WriteString("\nRules:\n")
	for _, r := range plan.Rules {
		fmt.Fprintf(&b, "- %s\n", r.String())
	}
	return b.String()
}
```

- [ ] **Step 5: Run tests and verify they pass**

Run: `go test ./internal/ruleworkflow -run 'TestValidateSources|TestBuildPlan|TestRenderPlan' -v`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ruleworkflow/validate.go internal/ruleworkflow/validate_test.go internal/ruleworkflow/plan.go internal/ruleworkflow/plan_test.go
git commit -m "Validate and render external rule import plans" \
  -m "Rule providers must be parseable and target existing strategy groups before the workflow can write mixin changes." \
  -m "Constraint: Missing target groups fail validation." \
  -m "Confidence: high" \
  -m "Scope-risk: narrow" \
  -m "Tested: go test ./internal/ruleworkflow -run 'TestValidateSources|TestBuildPlan|TestRenderPlan' -v"
```

### Task 4: Apply and Rollback Changes Safely

**Files:**
- Create: `internal/ruleworkflow/apply.go`
- Create: `internal/ruleworkflow/apply_test.go`

**Interfaces:**
- Consumes: `rules.Store`, `Plan`
- Produces: `ApplyPlan(store *rules.Store, plan Plan) (ApplyResult, error)`, `RollbackMixin(mixinPath, backupPath string) error`

- [ ] **Step 1: Write failing apply and rollback tests**

Create `internal/ruleworkflow/apply_test.go`:

```go
package ruleworkflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"labproxy/internal/rules"
)

func TestApplyPlanAddsProvidersAndRules(t *testing.T) {
	dir := t.TempDir()
	mixin := filepath.Join(dir, "mixin.yaml")
	if err := os.WriteFile(mixin, []byte("mode: rule\nrules:\n  - DOMAIN-SUFFIX,hf-mirror.com,DIRECT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := rules.NewStore(mixin)
	if err != nil {
		t.Fatal(err)
	}
	candidates, _ := SelectedCandidates([]string{"github"})
	result, err := ApplyPlan(store, BuildPlan(candidates))
	if err != nil {
		t.Fatalf("ApplyPlan: %v", err)
	}
	if result.BackupPath == "" {
		t.Fatal("expected backup path")
	}
	data, _ := os.ReadFile(mixin)
	text := string(data)
	if !strings.Contains(text, "rule-providers:") {
		t.Fatalf("missing providers block: %s", text)
	}
	if !strings.Contains(text, "RULE-SET,github,Proxies") {
		t.Fatalf("missing ruleset rule: %s", text)
	}
}

func TestRollbackMixinRestoresBackup(t *testing.T) {
	dir := t.TempDir()
	mixin := filepath.Join(dir, "mixin.yaml")
	backup := filepath.Join(dir, "mixin.yaml.bak.test")
	if err := os.WriteFile(mixin, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RollbackMixin(mixin, backup); err != nil {
		t.Fatalf("RollbackMixin: %v", err)
	}
	data, _ := os.ReadFile(mixin)
	if string(data) != "original\n" {
		t.Fatalf("got %q", string(data))
	}
}
```

- [ ] **Step 2: Run tests and verify they fail**

Run: `go test ./internal/ruleworkflow -run 'TestApplyPlan|TestRollbackMixin' -v`

Expected: FAIL because apply functions do not exist.

- [ ] **Step 3: Implement apply and rollback**

Create `internal/ruleworkflow/apply.go`:

```go
package ruleworkflow

import (
	"fmt"
	"os"

	"labproxy/internal/rules"
)

type ApplyResult struct {
	BackupPath string
}

func ApplyPlan(store *rules.Store, plan Plan) (ApplyResult, error) {
	backupPath, err := store.Backup()
	if err != nil {
		return ApplyResult{}, err
	}
	existingProviders, err := store.LoadProviders()
	if err != nil {
		return ApplyResult{}, err
	}
	existingRules, err := store.LoadRules()
	if err != nil {
		return ApplyResult{}, err
	}

	providers := mergeProviders(existingProviders, plan.Providers)
	ruleList := mergeRules(existingRules, plan.Rules)
	if err := store.SaveProviders(providers); err != nil {
		return ApplyResult{}, fmt.Errorf("save providers: %w", err)
	}
	if err := store.SaveRules(ruleList); err != nil {
		_ = RollbackMixin(store.Path, backupPath)
		return ApplyResult{}, fmt.Errorf("save rules: %w", err)
	}
	return ApplyResult{BackupPath: backupPath}, nil
}

func mergeProviders(existing, incoming []rules.Provider) []rules.Provider {
	byName := map[string]rules.Provider{}
	order := []string{}
	for _, p := range existing {
		if _, ok := byName[p.Name]; !ok {
			order = append(order, p.Name)
		}
		byName[p.Name] = p
	}
	for _, p := range incoming {
		if _, ok := byName[p.Name]; !ok {
			order = append(order, p.Name)
		}
		byName[p.Name] = p
	}
	out := make([]rules.Provider, 0, len(order))
	for _, name := range order {
		out = append(out, byName[name])
	}
	return out
}

func mergeRules(existing, incoming []rules.Rule) []rules.Rule {
	seen := map[string]bool{}
	out := make([]rules.Rule, 0, len(existing)+len(incoming))
	for _, r := range existing {
		seen[r.String()] = true
		out = append(out, r)
	}
	for _, r := range incoming {
		if seen[r.String()] {
			continue
		}
		seen[r.String()] = true
		out = append(out, r)
	}
	return out
}

func RollbackMixin(mixinPath, backupPath string) error {
	if mixinPath == "" || backupPath == "" {
		return fmt.Errorf("mixin path and backup path are required")
	}
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return err
	}
	tmp := mixinPath + ".rollback.tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, mixinPath)
}
```

- [ ] **Step 4: Run tests and verify they pass**

Run: `go test ./internal/ruleworkflow -run 'TestApplyPlan|TestRollbackMixin' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ruleworkflow/apply.go internal/ruleworkflow/apply_test.go
git commit -m "Apply and roll back rule workflow plans" \
  -m "Provider imports need a reversible write path that preserves local override rules and prints a backup path." \
  -m "Constraint: Rollback restores a named backup file exactly." \
  -m "Confidence: medium" \
  -m "Scope-risk: moderate" \
  -m "Tested: go test ./internal/ruleworkflow -run 'TestApplyPlan|TestRollbackMixin' -v"
```

### Task 5: Runtime Reload and Verification Helpers

**Files:**
- Modify: `internal/proxy/client.go`
- Modify: `internal/proxy/client_test.go`
- Create: `internal/ruleworkflow/verify.go`
- Create: `internal/ruleworkflow/verify_test.go`

**Interfaces:**
- Consumes: `proxy.Client`
- Produces: `(*proxy.Client).ReloadConfig(ctx context.Context, configPath string) error`, `RuntimeSummary`, `InspectRuntime(ctx context.Context, client RuntimeClient) (RuntimeSummary, error)`

- [ ] **Step 1: Write failing proxy reload test**

Append to `internal/proxy/client_test.go`:

```go
func TestClientReloadConfig(t *testing.T) {
	var gotMethod, gotPath string
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "")
	err := client.ReloadConfig(context.Background(), "/tmp/runtime.yaml")
	if err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != "/configs" || gotQuery != "force=true" {
		t.Fatalf("request = %s %s?%s", gotMethod, gotPath, gotQuery)
	}
}
```

- [ ] **Step 2: Write failing runtime inspect test**

Create `internal/ruleworkflow/verify_test.go`:

```go
package ruleworkflow

import (
	"context"
	"testing"

	"labproxy/internal/proxy"
)

type fakeRuntimeClient struct{}

func (fakeRuntimeClient) Proxies(context.Context) (proxy.ProxiesResponse, error) {
	return proxy.ProxiesResponse{Proxies: map[string]proxy.Proxy{
		"OpenAI": {Name: "OpenAI", Type: "Selector"},
		"Proxies": {Name: "Proxies", Type: "Selector"},
	}}, nil
}

func (fakeRuntimeClient) Connections(context.Context) (proxy.ConnectionsResponse, error) {
	return proxy.ConnectionsResponse{Connections: []proxy.Connection{{ID: "1", Chains: []string{"JP", "OpenAI"}}}}, nil
}

func TestInspectRuntime(t *testing.T) {
	got, err := InspectRuntime(context.Background(), fakeRuntimeClient{})
	if err != nil {
		t.Fatalf("InspectRuntime: %v", err)
	}
	if !got.StrategyGroups["OpenAI"] || got.ConnectionCount != 1 {
		t.Fatalf("unexpected summary: %+v", got)
	}
}
```

- [ ] **Step 3: Run tests and verify they fail**

Run: `go test ./internal/proxy ./internal/ruleworkflow -run 'TestClientReloadConfig|TestInspectRuntime' -v`

Expected: FAIL because reload and inspect helpers do not exist.

- [ ] **Step 4: Implement `ReloadConfig`**

Append to `internal/proxy/client.go` near other config methods:

```go
func (c *Client) ReloadConfig(ctx context.Context, configPath string) error {
	payload, err := json.Marshal(map[string]string{"path": configPath})
	if err != nil {
		return fmt.Errorf("marshal reload payload: %w", err)
	}
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("parse base url: %w", err)
	}
	endpoint.Path = path.Join(endpoint.Path, "/configs")
	q := endpoint.Query()
	q.Set("force", "true")
	endpoint.RawQuery = q.Encode()

	req, err := c.newRequest(ctx, http.MethodPut, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reload config failed: %s", strings.TrimSpace(string(body)))
	}
	return nil
}
```

- [ ] **Step 5: Implement runtime inspect**

Create `internal/ruleworkflow/verify.go`:

```go
package ruleworkflow

import (
	"context"

	"labproxy/internal/proxy"
)

type RuntimeClient interface {
	Proxies(context.Context) (proxy.ProxiesResponse, error)
	Connections(context.Context) (proxy.ConnectionsResponse, error)
}

type RuntimeSummary struct {
	StrategyGroups  map[string]bool
	ConnectionCount int
}

func InspectRuntime(ctx context.Context, client RuntimeClient) (RuntimeSummary, error) {
	proxies, err := client.Proxies(ctx)
	if err != nil {
		return RuntimeSummary{}, err
	}
	conns, err := client.Connections(ctx)
	if err != nil {
		return RuntimeSummary{}, err
	}
	groups := map[string]bool{}
	for name, p := range proxies.Proxies {
		switch p.Type {
		case "Selector", "URLTest", "Fallback", "LoadBalance":
			groups[name] = true
		}
	}
	return RuntimeSummary{StrategyGroups: groups, ConnectionCount: len(conns.Connections)}, nil
}
```

- [ ] **Step 6: Run tests and verify they pass**

Run: `go test ./internal/proxy ./internal/ruleworkflow -run 'TestClientReloadConfig|TestInspectRuntime' -v`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/proxy/client.go internal/proxy/client_test.go internal/ruleworkflow/verify.go internal/ruleworkflow/verify_test.go
git commit -m "Add runtime reload and inspection helpers" \
  -m "The rule workflow needs to hot reload mihomo and confirm available strategy groups through the existing controller client." \
  -m "Constraint: Reload uses PUT /configs?force=true." \
  -m "Confidence: high" \
  -m "Scope-risk: moderate" \
  -m "Tested: go test ./internal/proxy ./internal/ruleworkflow -run 'TestClientReloadConfig|TestInspectRuntime' -v"
```

### Task 6: CLI Workflow Subcommands

**Files:**
- Modify: `cmd/labproxy-tui/rules_cli.go`
- Create: `cmd/labproxy-tui/rules_workflow_cli.go`
- Create: `cmd/labproxy-tui/rules_workflow_cli_test.go`

**Interfaces:**
- Consumes: `internal/ruleworkflow`, optional Mihomo controller endpoint
- Produces: `labproxy rules workflow candidates|inspect|fetch|validate|plan|apply|verify|rollback`

- [ ] **Step 1: Write failing CLI tests**

Create `cmd/labproxy-tui/rules_workflow_cli_test.go` with coverage for:

- `workflow candidates` prints first-batch names.
- `workflow inspect` prints current mixin rule/provider counts.
- `workflow fetch --candidates=github --url-override=github=<httptest URL>` fetches a local fixture and prints `github rules=1`.
- `workflow validate --groups=Proxies --candidates=github --url-override=github=<httptest URL>` succeeds for `payload: [DOMAIN-SUFFIX,github.com]`.
- `workflow validate --groups=DIRECT --candidates=github --url-override=github=<httptest URL>` fails because `Proxies` is missing.
- `workflow plan --candidates=github,openai` prints `RULE-SET,github,Proxies` and `RULE-SET,openai,OpenAI`.
- `workflow apply --groups=Proxies --candidates=github --url-override=github=<httptest URL>` validates first, writes provider/rule entries, and prints `backup=...`.
- `workflow rollback --backup=<path>` restores the original mixin.

- [ ] **Step 2: Run tests and verify they fail**

Run: `go test ./cmd/labproxy-tui -run 'TestRunRulesWorkflow' -v`

Expected: FAIL because workflow subcommand does not exist.

- [ ] **Step 3: Dispatch workflow subcommand**

Modify `cmd/labproxy-tui/rules_cli.go` inside `runRulesCLI` switch:

```go
	case "workflow":
		return cliWorkflow(stdout, stderr, store, rest)
```

- [ ] **Step 4: Implement workflow CLI**

Create `cmd/labproxy-tui/rules_workflow_cli.go`.

Command behavior:

- `candidates`: print name, target group, behavior, URL, description.
- `inspect`: read `store.LoadRules()` and `store.LoadProviders()` and print counts plus existing provider names.
- `fetch`: select candidates, fetch each source, parse provider payload, print per-candidate rule counts; do not write files.
- `validate`: select candidates, fetch each source, parse provider payload, validate provider definitions, rule payload format, duplicate names, and required target groups.
- `plan`: render the provider/rule changes only; do not fetch or write.
- `apply`: select candidates, fetch and validate first, then call `ApplyPlan`; default behavior must refuse to write when validation fails.
- `verify`: call the Mihomo controller when `--endpoint=` is provided, print strategy groups and connection count, and optionally call `ReloadConfig` with `--reload-config=`.
- `rollback`: restore a named backup path.

Supported flags:

- `--candidates=github,openai`
- `--groups=OpenAI,Proxies,US,YouTube,Netflix,Disney,Telegram` for offline validation and tests
- `--endpoint=http://127.0.0.1:9090` for runtime verification
- `--secret=<mihomo-secret>` when the controller requires authentication
- `--reload-config=/Users/azhi/.labproxy/runtime.yaml` for hot reload verification
- `--url-override=name=url` for tests and emergency source override
- `--backup=/path/to/mixin.yaml.bak.20260702-120000`

Implementation notes:

- Build selected candidates once with `SelectedCandidates`.
- Apply `--url-override` before fetch/validate so tests never depend on GitHub availability.
- For `validate` and `apply`, if `--groups` is omitted but `--endpoint` is present, call `InspectRuntime` and use runtime strategy groups.
- If neither `--groups` nor `--endpoint` is provided for `validate` or `apply`, fail with a clear message. This prevents writing rules against unknown groups.
- `apply` should print both `ok` and `backup=<path>`.
- `verify --reload-config=...` should call `ReloadConfig` and then `InspectRuntime`.

- [ ] **Step 5: Run tests and verify they pass**

Run: `go test ./cmd/labproxy-tui -run 'TestRunRulesWorkflow' -v`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/labproxy-tui/rules_cli.go cmd/labproxy-tui/rules_workflow_cli.go cmd/labproxy-tui/rules_workflow_cli_test.go
git commit -m "Expose validated rule provider workflow commands" \
  -m "The rules CLI now has a workflow namespace that fetches and validates external providers before any mixin write." \
  -m "Constraint: Apply refuses to write unless target groups are known from flags or the runtime controller." \
  -m "Confidence: medium" \
  -m "Scope-risk: moderate" \
  -m "Tested: go test ./cmd/labproxy-tui -run 'TestRunRulesWorkflow' -v"
```

### Task 7: Integration Test and Documentation

**Files:**
- Create: `tests/rules_workflow_cli_test.sh`
- Modify: `README.md`

**Interfaces:**
- Consumes: compiled `cmd/labproxy-tui`
- Produces: repeatable shell integration coverage and user-facing workflow docs

- [ ] **Step 1: Write integration test**

Create `tests/rules_workflow_cli_test.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

LABPROXY_HOME="$(mktemp -d)"
export LABPROXY_HOME
mkdir -p "$LABPROXY_HOME/config" "$LABPROXY_HOME/bin"
FIXTURES="$LABPROXY_HOME/fixtures"
mkdir -p "$FIXTURES"

cat > "$FIXTURES/github.yaml" <<'YAML'
payload:
  - DOMAIN-SUFFIX,github.com
YAML

python3 - "$FIXTURES" "$LABPROXY_HOME/port" <<'PY' &
import http.server
import os
import socketserver
import sys

directory, port_file = sys.argv[1], sys.argv[2]
os.chdir(directory)
with socketserver.TCPServer(("127.0.0.1", 0), http.server.SimpleHTTPRequestHandler) as httpd:
    with open(port_file, "w", encoding="utf-8") as f:
        f.write(str(httpd.server_address[1]))
    httpd.serve_forever()
PY
server_pid=$!
trap 'kill "$server_pid" 2>/dev/null || true' EXIT
for _ in $(seq 1 50); do
  port="$(cat "$LABPROXY_HOME/port" 2>/dev/null || true)"
  if [ -n "$port" ]; then
    break
  fi
  sleep 0.1
done
if [ -z "${port:-}" ]; then
  echo "fixture server did not start"
  exit 1
fi
github_url="http://127.0.0.1:$port/github.yaml"

go build -o "$LABPROXY_HOME/bin/labproxy-tui" ./cmd/labproxy-tui

MIXIN="$LABPROXY_HOME/config/mixin.yaml"
cat > "$MIXIN" <<'YAML'
mode: rule
rules:
  - DOMAIN-SUFFIX,hf-mirror.com,DIRECT
YAML

BIN="$LABPROXY_HOME/bin/labproxy-tui"

"$BIN" rules --mixin-config "$MIXIN" workflow candidates | grep -q "github"
"$BIN" rules --mixin-config "$MIXIN" workflow inspect | grep -q "rules=1 providers=0"
"$BIN" rules --mixin-config "$MIXIN" workflow fetch --candidates=github --url-override=github="$github_url" | grep -q "github rules=1"
"$BIN" rules --mixin-config "$MIXIN" workflow validate --groups=Proxies --candidates=github --url-override=github="$github_url" | grep -q "ok"
"$BIN" rules --mixin-config "$MIXIN" workflow plan --candidates=github,openai | grep -q "RULE-SET,github,Proxies"

apply_out=$("$BIN" rules --mixin-config "$MIXIN" workflow apply --groups=Proxies --candidates=github --url-override=github="$github_url")
echo "$apply_out" | grep -q "backup="
grep -q "rule-providers:" "$MIXIN"
grep -q "RULE-SET,github,Proxies" "$MIXIN"
grep -q "DOMAIN-SUFFIX,hf-mirror.com,DIRECT" "$MIXIN"

backup="${apply_out#*backup=}"
"$BIN" rules --mixin-config "$MIXIN" workflow rollback --backup="$backup"
grep -q "DOMAIN-SUFFIX,hf-mirror.com,DIRECT" "$MIXIN"
if grep -q "RULE-SET,github,Proxies" "$MIXIN"; then
  echo "rollback did not remove github ruleset"
  exit 1
fi

echo "OK: rules workflow CLI"
```

- [ ] **Step 2: Run integration test and verify it passes**

Run: `bash tests/rules_workflow_cli_test.sh`

Expected: PASS with `OK: rules workflow CLI`.

- [ ] **Step 3: Update README**

Add this section under `## Rules Management` in `README.md`:

```markdown
### AI and media rule workflow

The workflow commands help test and apply a cautious first batch of external rule providers for AI/developer and media traffic.

```bash
labproxy rules workflow candidates
labproxy rules workflow inspect
labproxy rules workflow fetch --candidates=github,openai
labproxy rules workflow validate --endpoint=http://127.0.0.1:9090 --candidates=github,openai
labproxy rules workflow plan --candidates=github,openai
labproxy rules workflow apply --endpoint=http://127.0.0.1:9090 --candidates=github,openai
labproxy rules workflow verify --endpoint=http://127.0.0.1:9090 --reload-config=/Users/azhi/.labproxy/runtime.yaml
labproxy rules workflow rollback --backup=/path/to/mixin.yaml.bak.20260702-120000
```

The first batch maps providers to existing strategy groups:

```text
github -> Proxies
openai -> OpenAI
anthropic -> OpenAI
youtube -> YouTube
netflix -> Netflix
disney -> Disney
telegram -> Telegram
```

Run `validate` and `plan` before `apply`. `apply` validates again and refuses to write if the target groups are unknown. Keep local override rules at the top of `mixin.yaml`.

Hugging Face stays as inline local rules for now because the previously proposed blackmatrix7 HuggingFace provider URL currently returns 404.
```

- [ ] **Step 4: Run focused tests**

Run:

```bash
go test ./internal/ruleworkflow ./cmd/labproxy-tui ./internal/proxy
bash tests/rules_workflow_cli_test.sh
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tests/rules_workflow_cli_test.sh README.md
git commit -m "Document and test the AI media rule workflow" \
  -m "The workflow needs a shell-level regression so future changes do not silently break backup, apply, or rollback behavior." \
  -m "Constraint: Documentation tells users to plan before apply." \
  -m "Confidence: high" \
  -m "Scope-risk: narrow" \
  -m "Tested: go test ./internal/ruleworkflow ./cmd/labproxy-tui ./internal/proxy; bash tests/rules_workflow_cli_test.sh"
```

### Task 8: End-to-End Live Verification Without Applying New Providers

**Files:**
- No source edits.

**Interfaces:**
- Consumes: installed labproxy runtime
- Produces: verification notes for the final report

- [ ] **Step 1: Verify current runtime is still healthy**

Run:

```bash
curl -sS --max-time 5 http://127.0.0.1:9090/configs | jq '{mode, mixedPort: ."mixed-port", tun: .tun.enable}'
curl -sS --max-time 12 -x http://127.0.0.1:7893 https://www.cloudflare.com/cdn-cgi/trace | sed -n '1,12p'
```

Expected:

- First command prints `mode: "rule"`, `mixedPort: 7893`, `tun: true`.
- Second command returns Cloudflare trace text with an `ip=` line.

- [ ] **Step 2: Run read-only workflow commands against the real mixin**

Run:

```bash
/Users/azhi/.labproxy/bin/labproxy rules workflow candidates
/Users/azhi/.labproxy/bin/labproxy rules workflow inspect
/Users/azhi/.labproxy/bin/labproxy rules workflow fetch --candidates=github,openai
/Users/azhi/.labproxy/bin/labproxy rules workflow validate --endpoint=http://127.0.0.1:9090 --candidates=github,openai
/Users/azhi/.labproxy/bin/labproxy rules workflow plan --candidates=github,openai
/Users/azhi/.labproxy/bin/labproxy rules workflow verify --endpoint=http://127.0.0.1:9090
```

Expected:

- `candidates` lists `github` and `openai`.
- `inspect` prints rule and provider counts.
- `fetch` prints nonzero source rule counts.
- `validate` confirms target groups exist and provider sources parse.
- `plan` prints `RULE-SET,github,Proxies` and `RULE-SET,openai,OpenAI`.
- `verify` prints runtime strategy groups and connection count.

- [ ] **Step 3: Do not apply live providers in this task**

Stop after read-only verification. Live `apply` requires explicit execution approval or the implementation phase's apply task.

- [ ] **Step 4: Final status**

Run:

```bash
git status --short
```

Expected: only intended implementation files are modified or committed. Existing unrelated `.gitignore` changes remain untouched unless the user separately asks to handle them.
