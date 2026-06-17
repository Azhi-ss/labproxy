package rules

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed presets/*.yaml
var presetFS embed.FS

func LoadPreset(name string) ([]Rule, error) {
	data, err := presetFS.ReadFile("presets/" + name + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("preset %q not found", name)
	}
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
			return nil, fmt.Errorf("preset %q: %w", name, err)
		}
		r.Enabled = true
		rules = append(rules, r)
	}
	return rules, nil
}

func ListPresets() []string {
	entries, _ := presetFS.ReadDir("presets")
	var names []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".yaml") {
			names = append(names, strings.TrimSuffix(name, ".yaml"))
		}
	}
	return names
}
