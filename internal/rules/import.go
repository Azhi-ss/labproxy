package rules

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	maxImportBytes = 5 * 1024 * 1024
	importTimeout  = 10 * time.Second
)

func (s *Store) Import(src ImportSource, mode string) (Diff, error) {
	var newRules []Rule
	var err error
	switch src.Kind {
	case "preset":
		newRules, err = LoadPreset(src.Ref)
	case "file":
		newRules, err = loadRulesFromFile(src.Ref)
	case "url":
		newRules, err = loadRulesFromURL(src.Ref)
	default:
		return Diff{}, fmt.Errorf("unknown import kind %q", src.Kind)
	}
	if err != nil {
		return Diff{}, err
	}
	for i := range newRules {
		newRules[i].Enabled = true
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	existing, err := s.loadRules()
	if err != nil {
		return Diff{}, err
	}
	merged, _ := mergeUnique(existing, newRules, mode == "replace")
	if err := s.saveRules(merged); err != nil {
		return Diff{}, err
	}
	if mode == "replace" {
		return Diff{Added: newRules, Removed: existing}, nil
	}
	return Diff{Added: newRules}, nil
}

func loadRulesFromFile(path string) ([]Rule, error) {
	if strings.Contains(path, "..") {
		return nil, fmt.Errorf("path contains '..'")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseRuleList(data)
}

func loadRulesFromURL(rawURL string) ([]Rule, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("url scheme must be http or https, got %q", u.Scheme)
	}
	client := &http.Client{Timeout: importTimeout}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, maxImportBytes)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	return parseRuleList(data)
}

func parseRuleList(data []byte) ([]Rule, error) {
	var rules []Rule
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		r, err := ParseRule(line)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

func mergeUnique(existing, incoming []Rule, replace bool) ([]Rule, int) {
	if replace {
		return incoming, 0
	}
	seen := make(map[string]bool)
	for _, r := range existing {
		seen[string(r.Type)+"|"+r.Payload] = true
	}
	merged := append([]Rule{}, existing...)
	skipped := 0
	for _, r := range incoming {
		key := string(r.Type) + "|" + r.Payload
		if seen[key] {
			skipped++
			continue
		}
		seen[key] = true
		merged = append(merged, r)
	}
	return merged, skipped
}
