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

func TestApplyPlanReturnedBackupRestoresInitialContentAfterProviderAndRuleWrites(t *testing.T) {
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

	result, err := ApplyPlan(store, BuildPlan(candidates))
	if err != nil {
		t.Fatalf("ApplyPlan: %v", err)
	}
	if result.BackupPath == "" {
		t.Fatal("expected backup path")
	}

	backupData, err := os.ReadFile(result.BackupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(backupData) != original {
		t.Fatalf("expected returned backup to keep initial content, got %q", string(backupData))
	}

	applied, err := os.ReadFile(mixin)
	if err != nil {
		t.Fatal(err)
	}
	appliedText := string(applied)
	if !strings.Contains(appliedText, "rule-providers:") || !strings.Contains(appliedText, "RULE-SET,github,Proxies") {
		t.Fatalf("expected provider and rule writes before rollback, got %s", appliedText)
	}

	if err := RollbackMixin(mixin, result.BackupPath); err != nil {
		t.Fatalf("RollbackMixin: %v", err)
	}
	restored, err := os.ReadFile(mixin)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != original {
		t.Fatalf("expected returned backup to restore original content, got %q", string(restored))
	}
}

func TestApplyPlanReportsRollbackErrorWhenSavingRulesFails(t *testing.T) {
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

	missingBackup := filepath.Join(dir, "missing-backup.yaml")
	saveRulesErr := errors.New("save rules boom")
	restoreBackup := backupFn
	restoreSaveRules := saveRulesFn
	backupFn = func(*rules.Store) (string, error) {
		return missingBackup, nil
	}
	saveRulesFn = func(*rules.Store, []rules.Rule) error {
		return saveRulesErr
	}
	defer func() {
		backupFn = restoreBackup
		saveRulesFn = restoreSaveRules
	}()

	_, err = ApplyPlan(store, BuildPlan(candidates))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, saveRulesErr) {
		t.Fatalf("expected error to wrap save rules failure, got %v", err)
	}
	if !strings.Contains(err.Error(), "rollback") || !strings.Contains(err.Error(), "missing-backup.yaml") {
		t.Fatalf("expected combined rollback error, got %v", err)
	}
}

func TestMergeRulesPreservesExistingDuplicates(t *testing.T) {
	duplicate := rules.Rule{Type: rules.TypeDomainSuffix, Payload: "example.com", Proxy: "DIRECT", Enabled: true}
	incomingNew := rules.Rule{Type: rules.TypeRuleSet, Payload: "github", Proxy: "Proxies", Enabled: true}

	merged := mergeRules(
		[]rules.Rule{duplicate, duplicate},
		[]rules.Rule{duplicate, incomingNew, incomingNew},
	)

	if len(merged) != 3 {
		t.Fatalf("expected two existing duplicates plus one incoming rule, got %#v", merged)
	}
	if merged[0] != duplicate || merged[1] != duplicate || merged[2] != incomingNew {
		t.Fatalf("unexpected merged rules: %#v", merged)
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
