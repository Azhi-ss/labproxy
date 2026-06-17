package rules

import (
	"os"
	"path/filepath"
	"strings"
)

func Export(s *Store, path string, includeDisabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rules, err := s.loadRules()
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Exported by labproxy-tui rules export\n")
	for _, r := range rules {
		if !r.Enabled && !includeDisabled {
			continue
		}
		b.WriteString("- ")
		b.WriteString(r.String())
		b.WriteString("\n")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
