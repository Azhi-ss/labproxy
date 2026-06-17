package rules

import (
	"fmt"
	"strings"
)

func ParseRule(line string) (Rule, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Rule{}, fmt.Errorf("empty rule")
	}
	parts := strings.Split(line, ",")
	if len(parts) < 3 {
		return Rule{}, fmt.Errorf("rule must have at least 3 parts: %q", line)
	}
	rt := RuleType(strings.TrimSpace(parts[0]))
	if !rt.IsValid() {
		return Rule{}, fmt.Errorf("invalid type %q", rt)
	}
	r := Rule{
		Type:    rt,
		Payload: strings.TrimSpace(parts[1]),
		Proxy:   strings.TrimSpace(parts[2]),
	}
	if len(parts) > 3 && strings.TrimSpace(parts[3]) == "no-resolve" {
		r.NoResolve = true
	}
	return r, nil
}
