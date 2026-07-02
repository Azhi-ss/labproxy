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

	return parseRuleLines(strings.Split(string(data), "\n"))
}

func parseRuleLines(lines []string) ([]ProviderRule, error) {
	out := make([]ProviderRule, 0, len(lines))
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
