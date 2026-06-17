package rules

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Store struct {
	Path string
	mu   sync.Mutex
}

func NewStore(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("store path is empty")
	}
	return &Store{Path: path}, nil
}

// --- Rules ---

func (s *Store) LoadRules() ([]Rule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadRules()
}

func (s *Store) loadRules() ([]Rule, error) {
	f, err := os.Open(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var rules []Rule
	var inRules bool
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "rules:") {
			inRules = true
			continue
		}
		if !inRules {
			continue
		}
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(raw, " ") && !strings.HasPrefix(raw, "\t") && strings.Contains(trimmed, ":") {
			inRules = false
			continue
		}
		enabled := true
		line := trimmed
		if strings.HasPrefix(line, "- ") {
			line = strings.TrimPrefix(line, "- ")
		} else if strings.HasPrefix(line, "#") && strings.Contains(line, "- ") {
			enabled = false
			idx := strings.Index(line, "- ")
			line = line[idx+2:]
		} else {
			continue
		}
		r, err := ParseRule(line)
		if err != nil {
			continue
		}
		r.Enabled = enabled
		rules = append(rules, r)
	}
	return rules, scanner.Err()
}

func (s *Store) Backup() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.backup()
}

func (s *Store) backup() (string, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	ts := time.Now().Format("20060102-150405")
	dst := fmt.Sprintf("%s.bak.%s", s.Path, ts)
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return "", err
	}
	return dst, nil
}

func (s *Store) SaveRules(rules []Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveRules(rules)
}

func (s *Store) saveRules(rules []Rule) error {
	if _, err := s.backup(); err != nil {
		return fmt.Errorf("backup: %w", err)
	}
	original, err := os.ReadFile(s.Path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(original)
	newBlock := renderRulesBlock(rules)
	content = replaceRulesBlock(content, newBlock)

	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.Path); err != nil {
		return err
	}
	return s.rotateBackups(5)
}

func (s *Store) rotateBackups(keep int) error {
	matches, err := filepath.Glob(s.Path + ".bak.*")
	if err != nil {
		return err
	}
	if len(matches) <= keep {
		return nil
	}
	type kv struct {
		path string
		ts   string
	}
	var backups []kv
	for _, m := range matches {
		base := filepath.Base(m)
		if idx := strings.Index(base, ".bak."); idx >= 0 {
			backups = append(backups, kv{m, base[idx+5:]})
		}
	}
	for i := 0; i < len(backups)-keep; i++ {
		_ = os.Remove(backups[i].path)
	}
	return nil
}

func renderRulesBlock(rules []Rule) string {
	if len(rules) == 0 {
		return "rules: []\n"
	}
	var b strings.Builder
	b.WriteString("rules:\n")
	for _, r := range rules {
		line := r.String()
		if !r.Enabled {
			b.WriteString("  # - ")
			b.WriteString(line)
			b.WriteString("\n")
		} else {
			b.WriteString("  - ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func replaceRulesBlock(content, newBlock string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	inRules := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inRules && trimmed == "rules:" {
			out = append(out, strings.Split(newBlock, "\n")...)
			inRules = true
			continue
		}
		if inRules {
			if trimmed != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.Contains(trimmed, ":") {
				inRules = false
			} else {
				continue
			}
		}
		out = append(out, line)
	}
	if !inRules {
		out = append(out, strings.Split(newBlock, "\n")...)
	}
	return strings.Join(out, "\n")
}

// --- Providers ---

func (s *Store) LoadProviders() ([]Provider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadProviders()
}

func (s *Store) loadProviders() ([]Provider, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var raw struct {
		RuleProviders map[string]struct {
			Type     string `yaml:"type"`
			Behavior string `yaml:"behavior"`
			URL      string `yaml:"url"`
			Path     string `yaml:"path"`
			Interval int    `yaml:"interval"`
		} `yaml:"rule-providers"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	var providers []Provider
	for name, p := range raw.RuleProviders {
		providers = append(providers, Provider{
			Name: name, Type: p.Type, Behavior: p.Behavior,
			URL: p.URL, Path: p.Path, Interval: p.Interval,
		})
	}
	return providers, nil
}

func (s *Store) SaveProviders(providers []Provider) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveProviders(providers)
}

// saveProviders performs the write without acquiring s.mu, so it can be called
// from methods that already hold the lock (mirrors saveRules/SaveRules).
func (s *Store) saveProviders(providers []Provider) error {
	if _, err := s.backup(); err != nil {
		return err
	}
	data, err := os.ReadFile(s.Path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(data)
	rpMap := make(map[string]Provider)
	for _, p := range providers {
		rpMap[p.Name] = p
	}
	var rendered strings.Builder
	rendered.WriteString("rule-providers:\n")
	for name, p := range rpMap {
		rendered.WriteString(fmt.Sprintf("  %s:\n", name))
		rendered.WriteString(fmt.Sprintf("    type: %s\n", p.Type))
		rendered.WriteString(fmt.Sprintf("    behavior: %s\n", p.Behavior))
		if p.URL != "" {
			rendered.WriteString(fmt.Sprintf("    url: %q\n", p.URL))
		}
		rendered.WriteString(fmt.Sprintf("    path: %q\n", p.Path))
		if p.Interval > 0 {
			rendered.WriteString(fmt.Sprintf("    interval: %d\n", p.Interval))
		}
	}
	content = replaceProviderBlock(content, rendered.String())
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}

func replaceProviderBlock(content, newBlock string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	inBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inBlock && trimmed == "rule-providers:" {
			out = append(out, strings.Split(newBlock, "\n")...)
			inBlock = true
			continue
		}
		if inBlock {
			if trimmed != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				inBlock = false
			} else {
				continue
			}
		}
		out = append(out, line)
	}
	if !inBlock {
		out = append(out, strings.Split(newBlock, "\n")...)
	}
	return strings.Join(out, "\n")
}
