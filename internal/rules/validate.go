package rules

import (
	"fmt"
	"strings"
)

func (r Rule) Validate() error {
	if !r.Type.IsValid() {
		return fmt.Errorf("rule type %q is invalid", r.Type)
	}
	if strings.TrimSpace(r.Proxy) == "" {
		return fmt.Errorf("rule proxy is empty")
	}
	if r.Type != TypeMatch && r.Type != TypeMatchSrc {
		if strings.TrimSpace(r.Payload) == "" {
			return fmt.Errorf("rule payload is empty for type %s", r.Type)
		}
	}
	return nil
}

func ValidateProvider(p Provider) error {
	if p.Name == "" {
		return fmt.Errorf("provider name is empty")
	}
	// Guard against YAML injection: a name used as a mapping key must not
	// contain characters that would break rendering (":", "#") or carry
	// leading/trailing whitespace.
	if strings.ContainsAny(p.Name, ":#") || p.Name != strings.TrimSpace(p.Name) {
		return fmt.Errorf("provider name %q contains invalid characters (':', '#', or surrounding spaces)", p.Name)
	}
	if p.Type != "http" && p.Type != "file" {
		return fmt.Errorf("provider type must be http or file, got %q", p.Type)
	}
	if p.Type == "http" && p.URL == "" {
		return fmt.Errorf("http provider requires url")
	}
	if p.Path == "" {
		return fmt.Errorf("provider path is empty")
	}
	switch p.Behavior {
	case "domain", "ipcidr", "classical":
	default:
		return fmt.Errorf("provider behavior must be domain|ipcidr|classical, got %q", p.Behavior)
	}
	if p.Interval < 0 {
		return fmt.Errorf("provider interval must be >= 0")
	}
	return nil
}
