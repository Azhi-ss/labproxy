package rules

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
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
