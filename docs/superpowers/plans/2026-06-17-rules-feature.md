# Rules Feature Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add clash-verge-rev-equivalent rules management (view/CRUD/toggle/reorder/import-export/rule-providers/reset) to labproxy, exposed via TUI modal and CLI subcommand on the same Go binary.

**Architecture:** New `internal/rules` package owns all business logic. TUI modal in `internal/tui/rules/` and CLI subcommand in `cmd/labproxy-tui/main.go` both depend only on `internal/rules`. Shell `proxyctl.sh` forwards to the Go binary.

**Tech Stack:** Go 1.24, Bubble Tea, lipgloss, gopkg.in/yaml.v3 (already in go.sum), bash for shell tests.

**Spec:** `docs/superpowers/specs/2026-06-17-rules-feature-design.md`

---

## Phase 1: Data Model + Validation + Parsing

### Task 1: Rule type and struct
**Files:** Create `internal/rules/types.go`, `internal/rules/types_test.go`
- Test: `TestRuleType_IsValid` (13 valid + 1 invalid), `TestRule_String` (3 cases).
- Implement: `RuleType` with 13 const + `IsValid()`, `Rule` struct + `String()`, `Provider`, `Diff`, `ImportSource`.
- Commit: `feat(rules): add core types and string serialization`

### Task 2: ParseRule
**Files:** Create `internal/rules/parse.go`, `parse_test.go`
- Test: `TestParseRule` table — 3 valid + 3 invalid.
- Implement: split by `,`, validate type, optional `no-resolve`.
- Commit: `feat(rules): add ParseRule for single-line clash format`

### Task 3: Validate
**Files:** Create `internal/rules/validate.go`, `validate_test.go`
- Test: 7 cases (valid, MATCH-no-payload, empty type, invalid type, empty proxy, empty payload non-MATCH, whitespace proxy).
- Implement: `Rule.Validate()` + `ValidateProvider(Provider)`.
- Commit: `feat(rules): add Validate for Rule and Provider`

---

## Phase 2: Store (atomic read/write with backup)

### Task 4: Store skeleton + LoadRules
**Files:** Create `internal/rules/store.go`, `store_test.go`
- Test: `TestStore_LoadRules_Empty` + `TestStore_LoadRules_PreservesEnabled` (3 rules incl commented).
- Implement: `Store{Path, sync.Mutex}`, `NewStore`, `LoadRules` (line scanner respecting `rules:` block + comment prefix), `Backup()`.
- Commit: `feat(rules): add Store with LoadRules preserving enabled state`

### Task 5: SaveRules atomic
**Files:** Modify `store.go` + append test
- Test: `TestStore_SaveRules_AtomicAndBackup`.
- Implement: `SaveRules` (backup → read original → replace `rules:` block → write `.tmp` → `os.Rename` → `rotateBackups(5)`). Helpers: `renderRulesBlock`, `replaceRulesBlock`, `rotateBackups`.
- Commit: `feat(rules): add SaveRules with atomic write and backup rotation`

### Task 6: Provider Load/Save
**Files:** Modify `store.go` + append test
- Test: `TestStore_Providers_Roundtrip` (1 provider).
- Implement: `LoadProviders` (`gopkg.in/yaml.v3`), `SaveProviders`, `replaceProviderBlock`.
- Commit: `feat(rules): add provider load/save with YAML roundtrip`

---

## Phase 3: Rule CRUD

### Task 7: AddRule / UpdateRule / DeleteRule
**Files:** Create `internal/rules/crud.go`, `crud_test.go`
- Test: helper `newTestStore` + 4 tests (add, add-validation-fail, delete, update).
- Implement: each validates, locks, loads, mutates, saves, returns Diff.
- Commit: `feat(rules): add AddRule/UpdateRule/DeleteRule`

### Task 8: ToggleRule + MoveRule
**Files:** Modify `crud.go` + append tests
- Test: `TestStore_ToggleRule` (toggle twice), `TestStore_MoveRule` (move 0→2).
- Implement: ToggleRule flips Enabled; MoveRule splices.
- Commit: `feat(rules): add ToggleRule and MoveRule`

### Task 9: ResetRules
**Files:** Modify `crud.go` + append test
- Test: `TestStore_ResetRules` — after reset, LoadRules returns 0.
- Implement: capture old, saveRules(nil), return Diff.
- Commit: `feat(rules): add ResetRules to clear all user rules`

---

## Phase 4: Import / Export / Presets

### Task 10: Built-in presets
**Files:** Create `internal/rules/presets/{direct,private,gfw,tld-not-cn}.yaml`, `presets/README.md`, `preset.go`, `preset_test.go`
- Create 4 YAML preset files (1-5 rules each).
- Test: `TestLoadPreset` (4 known + 1 unknown).
- Implement: `//go:embed presets/*.yaml` FS, `LoadPreset`, `ListPresets`.
- Commit: `feat(rules): add built-in presets and loader`

### Task 11: Import (url/file/preset)
**Files:** Create `internal/rules/import.go`, `import_test.go`
- Test: 4 cases (preset, file, url via httptest, bad scheme rejected).
- Implement: `Import(src, mode)` dispatch; `loadRulesFromFile` (reject `..`); `loadRulesFromURL` (http/https only, 10s timeout, 5MB cap); `parseRuleList`; `mergeUnique` (key=`type|payload`).
- Commit: `feat(rules): add Import supporting url/file/preset with dedup`

### Task 12: Export
**Files:** Create `internal/rules/export.go`, `export_test.go`
- Test: `TestExport` — 1 enabled + 1 commented, default excludes disabled.
- Implement: `Export(s, path, includeDisabled)`.
- Commit: `feat(rules): add Export to standard clash rule format`

---

## Phase 5: Rule Providers CRUD

### Task 13: Provider CRUD + refresh
**Files:** Create `internal/rules/providers.go`, `providers_test.go`
- Test: `TestStore_AddProvider` + `TestStore_RefreshProvider` (httptest).
- Implement: Add/Update/Delete/Refresh (GET URL → write to p.Path relative to mixin dir, 30s timeout, 5MB cap).
- Commit: `feat(rules): add provider CRUD with http refresh`

---

## Phase 6: CLI Subcommand

### Task 14: CLI dispatch
**Files:** Create `cmd/labproxy-tui/rules_cli.go`, `rules_cli_test.go`; modify `cmd/labproxy-tui/main.go`
- Test: `TestRunRulesCLI_RequiresMixinPath` — exit 2 + "mixin" in output.
- Implement: `runRulesCLI(stdout, stderr io.Writer, args []string, mixinPath string) int` with subcommand switch: list, add, delete/rm, enable, disable, move, edit, import, export, providers (list/add/delete/refresh), reset.
- Wire into main.go: at top of `main()`, `if os.Args[1] == "rules" { os.Exit(runRulesCLI(os.Stdout, os.Stderr, os.Args[2:], *mixinConfig)) }`.
- Commit: `feat(rules): add CLI subcommand dispatch and handlers`

### Task 15: Shell wrapper
**Files:** Modify `scripts/proxyctl.sh`
- Locate case statement; add `rules)` case forwarding to Go binary.
- Run existing tests to confirm no regression.
- Commit: `feat(rules): forward labproxy rules to Go binary`

### Task 16: .gitignore for backups
**Files:** Modify `.gitignore`
- Append `mixin.yaml.bak.*`.
- Commit: `chore: ignore mixin.yaml backup files`

---

## Phase 7: TUI Modal

### Task 17: Modal skeleton
**Files:** Create `internal/tui/rules/modal.go`, `rules_test.go`
- Test: `TestModal_Open` + `TestModal_Close`.
- Implement: `View` enum, `Modal{path, store, mu, open, view, cursor}`, `NewModal` (constructs `rules.NewStore`), `Open`/`Close`/`IsOpen`/`View`.
- Commit: `feat(rules): add TUI modal skeleton`

### Task 18: Wire 'R' key in main TUI
**Files:** Modify `internal/tui/app.go`
- Define `type RulesModal interface { IsOpen() bool; Open(); Update(tea.KeyMsg) bool; View() string }` at top.
- Add `RulesModal` to `Options`; `rulesModal` to `model`; init in `newModel`.
- Add `Rules key.Binding` to `keyMap` with `key.WithKeys("R")`; add to `ShortHelp`.
- In `Update`: if modal open, route to `m.rulesModal.Update(msg)`. Add case for `R` key to Open.
- Build to confirm compile.
- Commit: `feat(tui): wire R key to open rules modal`

### Task 19: List view rendering
**Files:** Create `internal/tui/rules/list.go`; append test
- Test: `TestList_FormatRow` checks row contains type+payload.
- Implement: `formatRow(idx, r, width) string` with `●`/`○` marker.
- Commit: `feat(rules): add list view row formatting`

### Task 20: Form view
**Files:** Create `internal/tui/rules/form.go`
- Implement: `Form` with 3 `textinput.Model` (Type, Payload, Proxy) + noResolve flag. `Build()` validates. `View()` renders Chinese labels.
- Build to confirm.
- Commit: `feat(rules): add form view for add/edit`

### Task 21: Modal Update loop
**Files:** Modify `internal/tui/rules/modal.go`
- Add `Update(msg tea.KeyMsg) bool` — esc/R toggles Menu/close; `1`→List; `2`→Providers; `3`→Import; `4`→ResetRules.
- Commit: `feat(tui): wire modal Update to handle keys`

### Task 22: i18n keys
**Files:** Modify `internal/tui/i18n.go`; create `internal/tui/i18n_test.go`
- Add 6 fields: `RulesTitle`, `RulesMenuList`, `RulesMenuProviders`, `RulesMenuImport`, `RulesMenuReset`, `RulesHelpOpen` (EN + ZH).
- Test: `TestRulesI18nKeys_NotEmpty`.
- Commit: `feat(tui): add i18n keys for rules manager`

---

## Phase 8: Integration & Smoke Tests

### Task 23: CLI integration test
**Files:** Create `tests/rules_cli_test.sh`
- Build binary in temp `LABPROXY_HOME`; init mixin with 1 rule; assert: list shows it; add→grep; disable→grep `^  # -`; enable→grep `^  -`; import preset:private→grep IP-CIDR; reset -y→grep `^rules: \[\]`; backup exists.
- Commit: `test: add CLI integration test for rules subcommand`

### Task 24: Persistence test
**Files:** Create `tests/rules_persistence_test.sh`
- Add 6 rules; assert: backup count ≤5; total=7; mixin.yaml valid YAML.
- Commit: `test: add persistence and rotation test`

---

## Phase 9: Documentation & Final Wiring

### Task 25: Update README
**Files:** Modify `README.md` + `README.en.md`
- Add Rules section after "核心命令" / equivalent — covers all CLI subcommands + supported types.
- Commit: `docs: add Rules management section to README`

### Task 26: Wire Modal into main.go
**Files:** Modify `cmd/labproxy-tui/main.go`
- Import `labproxy/internal/tui/rules`; construct `rulesModal := rules.NewModal(*mixinConfig)`; pass to `tui.Options{RulesModal: ...}`.
- Build + smoke test.
- Commit: `feat: wire rules modal into main TUI app`

### Task 27: Full regression
- `go test ./... -race -count=1` — PASS
- `for t in tests/*.sh; do bash "$t" || exit 1; done` — PASS
- `go vet ./...` — clean

---

## Completion Checklist
- [ ] All Go tests pass with `-race`
- [ ] All shell tests pass
- [ ] `go vet ./...` clean
- [ ] README updated (zh + en)
- [ ] 9 phases of spec implemented
- [ ] No regressions in existing TUI/CLI
