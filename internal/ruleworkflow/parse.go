package ruleworkflow

import (
	"fmt"
	"net/netip"
	"strings"

	"gopkg.in/yaml.v3"
	"labproxy/internal/rules"
)

type ProviderRule struct {
	Type    string
	Payload string
	Raw     string
}

func ParseProviderRules(data []byte) ([]ProviderRule, error) {
	lines, err := providerRuleLines(data)
	if err != nil {
		return nil, err
	}
	return parseClassicalRuleLines(lines)
}

func ParseProviderRulesForBehavior(data []byte, behavior string) ([]ProviderRule, error) {
	lines, err := providerRuleLines(data)
	if err != nil {
		return nil, err
	}

	switch behavior {
	case "classical":
		return parseClassicalRuleLines(lines)
	case "domain":
		return parseDomainRuleLines(lines)
	case "ipcidr":
		return parseIPCIDRRuleLines(lines)
	default:
		return nil, fmt.Errorf("unsupported provider behavior %q", behavior)
	}
}

func providerRuleLines(data []byte) ([]string, error) {
	var withPayload struct {
		Payload []string `yaml:"payload"`
	}
	if err := yaml.Unmarshal(data, &withPayload); err == nil && len(withPayload.Payload) > 0 {
		return withPayload.Payload, nil
	}

	var list []string
	if err := yaml.Unmarshal(data, &list); err == nil && len(list) > 0 {
		return list, nil
	}

	return strings.Split(string(data), "\n"), nil
}

func parseClassicalRuleLines(lines []string) ([]ProviderRule, error) {
	out := make([]ProviderRule, 0, len(lines))
	for _, line := range lines {
		line = normalizeProviderRuleLine(line)
		if line == "" || strings.HasPrefix(line, "#") {
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

func parseDomainRuleLines(lines []string) ([]ProviderRule, error) {
	out := make([]ProviderRule, 0, len(lines))
	for _, line := range lines {
		line = normalizeProviderRuleLine(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, ",") {
			return nil, fmt.Errorf("invalid domain provider rule %q: expected bare domain payload", line)
		}

		out = append(out, ProviderRule{
			// Mihomo domain providers carry bare payload entries, so the
			// provider behavior rather than the line format defines semantics.
			Type:    string(rules.TypeDomain),
			Payload: line,
			Raw:     line,
		})
	}
	return out, nil
}

func parseIPCIDRRuleLines(lines []string) ([]ProviderRule, error) {
	out := make([]ProviderRule, 0, len(lines))
	for _, line := range lines {
		line = normalizeProviderRuleLine(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, ",") {
			return nil, fmt.Errorf("invalid ipcidr provider rule %q: expected bare CIDR payload", line)
		}
		prefix, err := netip.ParsePrefix(line)
		if err != nil {
			return nil, fmt.Errorf("invalid ipcidr provider rule %q: %w", line, err)
		}

		ruleType := rules.TypeIPCIDR
		if prefix.Addr().Is6() {
			ruleType = rules.TypeIPCIDR6
		}

		out = append(out, ProviderRule{
			Type:    string(ruleType),
			Payload: line,
			Raw:     line,
		})
	}
	return out, nil
}

func normalizeProviderRuleLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "- ")
	return strings.TrimSpace(line)
}
