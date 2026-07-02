package ruleworkflow

import (
	"errors"
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

	candidates, err := SelectedCandidates([]string{"github"})
	if err != nil {
		t.Fatalf("SelectedCandidates: %v", err)
	}

	result, err := ApplyPlan(store, BuildPlan(candidates))
	if err != nil {
		t.Fatalf("ApplyPlan: %v", err)
	}
	if result.BackupPath == "" {
		t.Fatal("expected backup path")
	}

	data, err := os.ReadFile(mixin)
	if err != nil {
		t.Fatal(err)
	}

	text := string(data)
	if !strings.Contains(text, "rule-providers:") {
		t.Fatalf("missing providers block: %s", text)
	}
	if !strings.Contains(text, "RULE-SET,github,Proxies") {
		t.Fatalf("missing ruleset rule: %s", text)
	}
}

func TestApplyPlanRollsBackToInitialBackupWhenSavingRulesFails(t *testing.T) {
	dir := t.TempDir()
	mixin := filepath.Join(dir, "mixin.yaml")
	original := "mode: rule\nrules:\n  - DOMAIN-SUFFIX,hf-mirror.com,DIRECT\n"
	if err := os.WriteFile(mixin, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := rules.NewStore(mixin)
	if err != nil {
		t.Fatal(err)
	}

	candidates, err := SelectedCandidates([]string{"github"})
	if err != nil {
		t.Fatalf("SelectedCandidates: %v", err)
	}

	restoreSaveRules := saveRulesFn
	saveRulesFn = func(*rules.Store, []rules.Rule) error {
		return errors.New("boom")
	}
	defer func() {
		saveRulesFn = restoreSaveRules
	}()

	result, err := ApplyPlan(store, BuildPlan(candidates))
	if err == nil {
		t.Fatal("expected error")
	}
	if result.BackupPath != "" {
		t.Fatalf("expected empty result on failure, got %+v", result)
	}

	data, readErr := os.ReadFile(mixin)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != original {
		t.Fatalf("expected rollback to restore original content, got %q", string(data))
	}
	if !strings.Contains(err.Error(), "save rules") {
		t.Fatalf("expected save rules error, got %v", err)
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

	data, err := os.ReadFile(mixin)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original\n" {
		t.Fatalf("got %q", string(data))
	}
}
