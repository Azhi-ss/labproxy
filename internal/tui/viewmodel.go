package tui

import (
	"sort"
	"strings"

	"clash-for-lab/internal/mihomo"
)

type GroupView struct {
	Name    string
	Type    string
	Current string
	Options []OptionView
}

type OptionView struct {
	Name     string
	Selected bool
	DelayMS  int
}

func BuildGroupViews(resp mihomo.ProxiesResponse, filter string) []GroupView {
	filter = strings.ToLower(strings.TrimSpace(filter))
	groups := make([]GroupView, 0)

	for name, proxy := range resp.Proxies {
		if len(proxy.All) == 0 {
			continue
		}

		options := make([]OptionView, 0, len(proxy.All))
		for _, optionName := range proxy.All {
			if isMetaOptionName(optionName) {
				continue
			}
			optionProxy := resp.Proxies[optionName]
			if filter != "" && !strings.Contains(strings.ToLower(optionName), filter) && !strings.Contains(strings.ToLower(name), filter) {
				continue
			}
			options = append(options, OptionView{
				Name:     optionName,
				Selected: proxy.Now == optionName,
				DelayMS:  latestDelay(optionProxy),
			})
		}

		// Skip groups with no options (all filtered out or only meta options)
		if len(options) == 0 {
			continue
		}

		groups = append(groups, GroupView{
			Name:    name,
			Type:    proxy.Type,
			Current: proxy.Now,
			Options: options,
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		if priority(groups[i].Name) != priority(groups[j].Name) {
			return priority(groups[i].Name) < priority(groups[j].Name)
		}
		return groups[i].Name < groups[j].Name
	})

	return groups
}

func latestDelay(proxy mihomo.Proxy) int {
	if len(proxy.History) == 0 {
		return 0
	}
	return proxy.History[len(proxy.History)-1].Delay
}

func priority(name string) int {
	switch name = strings.ToUpper(name); name {
	case "GLOBAL":
		return 0
	case "PROXY":
		return 1
	default:
		return 10
	}
}

func isMetaOptionName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(lower, "traffic:") || strings.HasPrefix(lower, "expire:")
}
