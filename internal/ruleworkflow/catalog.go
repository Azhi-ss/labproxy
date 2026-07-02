package ruleworkflow

import (
	"fmt"
	"sort"

	"labproxy/internal/rules"
)

type Candidate struct {
	Name        string
	Description string
	SourceURL   string
	TargetGroup string
	Provider    rules.Provider
}

type Plan struct {
	Candidates []Candidate
	Providers  []rules.Provider
	Rules      []rules.Rule
}

func DefaultCandidates() []Candidate {
	return []Candidate{
		candidate("github", "GitHub domains and asset hosts", "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/GitHub/GitHub.yaml", "Proxies", "classical", "./rule-providers/github.yaml"),
		candidate("openai", "OpenAI and ChatGPT service domains", "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/OpenAI/OpenAI.yaml", "OpenAI", "classical", "./rule-providers/openai.yaml"),
		candidate("anthropic", "Anthropic and Claude service domains", "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Anthropic/Anthropic.yaml", "OpenAI", "classical", "./rule-providers/anthropic.yaml"),
		candidate("youtube", "YouTube service domains", "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/YouTube/YouTube.yaml", "YouTube", "classical", "./rule-providers/youtube.yaml"),
		candidate("netflix", "Netflix service domains and CIDRs", "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Netflix/Netflix.yaml", "Netflix", "classical", "./rule-providers/netflix.yaml"),
		candidate("disney", "Disney and Disney Plus service domains", "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Disney/Disney.yaml", "Disney", "classical", "./rule-providers/disney.yaml"),
		candidate("telegram", "Telegram service domains and CIDRs", "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Telegram/Telegram.yaml", "Telegram", "classical", "./rule-providers/telegram.yaml"),
	}
}

func candidate(name, description, url, target, behavior, path string) Candidate {
	return Candidate{
		Name:        name,
		Description: description,
		SourceURL:   url,
		TargetGroup: target,
		Provider: rules.Provider{
			Name:     name,
			Type:     "http",
			Behavior: behavior,
			URL:      url,
			Path:     path,
			Interval: 86400,
		},
	}
}

func SelectedCandidates(names []string) ([]Candidate, error) {
	all := DefaultCandidates()
	byName := map[string]Candidate{}
	for _, c := range all {
		byName[c.Name] = c
	}
	if len(names) == 0 {
		return all, nil
	}
	selected := make([]Candidate, 0, len(names))
	for _, name := range names {
		c, ok := byName[name]
		if !ok {
			keys := make([]string, 0, len(byName))
			for k := range byName {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return nil, fmt.Errorf("unknown candidate %q; known: %v", name, keys)
		}
		selected = append(selected, c)
	}
	return selected, nil
}
