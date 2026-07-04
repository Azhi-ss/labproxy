package rules

import (
	"embed"
	"fmt"
)

//go:embed presets/*.yaml
var presetFS embed.FS

func LoadPreset(name string) ([]Rule, error) {
	data, err := presetFS.ReadFile("presets/" + name + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("preset %q not found", name)
	}
	rules, err := parseRuleList(data)
	if err != nil {
		return nil, fmt.Errorf("preset %q: %w", name, err)
	}
	for i := range rules {
		rules[i].Enabled = true
	}
	return rules, nil
}
