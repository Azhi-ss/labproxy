package tui

import (
	"testing"

	"clash-for-lab/internal/mihomo"
)

func TestBuildGroupViews(t *testing.T) {
	resp := mihomo.ProxiesResponse{
		Proxies: map[string]mihomo.Proxy{
			"GLOBAL": {Name: "GLOBAL", Type: "Selector", Now: "Node-A", All: []string{"Traffic: 1GB", "Node-A", "Node-B", "Expire: 2027-01-01"}},
			"Node-A": {Name: "Node-A", Type: "SS", History: []mihomo.DelayHistory{{Delay: 42}}},
			"Node-B": {Name: "Node-B", Type: "SS", History: []mihomo.DelayHistory{{Delay: 84}}},
			"DIRECT": {Name: "DIRECT", Type: "Direct"},
		},
	}

	groups := BuildGroupViews(resp, "")
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Name != "GLOBAL" {
		t.Fatalf("expected GLOBAL group, got %s", groups[0].Name)
	}
	if !groups[0].Options[0].Selected {
		t.Fatalf("expected first option selected")
	}
	if groups[0].Options[0].DelayMS != 42 {
		t.Fatalf("expected latest delay 42, got %d", groups[0].Options[0].DelayMS)
	}
	if len(groups[0].Options) != 2 {
		t.Fatalf("expected metadata options filtered out, got %d options", len(groups[0].Options))
	}
}

func TestBuildGroupViews_EmptyResponse(t *testing.T) {
	resp := mihomo.ProxiesResponse{
		Proxies: map[string]mihomo.Proxy{},
	}

	groups := BuildGroupViews(resp, "")
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups, got %d", len(groups))
	}
}

func TestBuildGroupViews_NoOptions(t *testing.T) {
	resp := mihomo.ProxiesResponse{
		Proxies: map[string]mihomo.Proxy{
			"DIRECT": {Name: "DIRECT", Type: "Direct", All: []string{}},
		},
	}

	groups := BuildGroupViews(resp, "")
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups (no options), got %d", len(groups))
	}
}

func TestBuildGroupViews_OnlyMetaOptions(t *testing.T) {
	resp := mihomo.ProxiesResponse{
		Proxies: map[string]mihomo.Proxy{
			"GROUP": {Name: "GROUP", Type: "Selector", Now: "Node-A", All: []string{"Traffic: 1GB", "Expire: 2027-01-01"}},
		},
	}

	groups := BuildGroupViews(resp, "")
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups (only meta options), got %d", len(groups))
	}
}

func TestBuildGroupViews_PrioritySorting(t *testing.T) {
	resp := mihomo.ProxiesResponse{
		Proxies: map[string]mihomo.Proxy{
			"PROXY":   {Name: "PROXY", Type: "Selector", Now: "Node-C", All: []string{"Node-C"}},
			"GLOBAL":  {Name: "GLOBAL", Type: "Selector", Now: "Node-A", All: []string{"Node-A"}},
			"CUSTOM":  {Name: "CUSTOM", Type: "Selector", Now: "Node-B", All: []string{"Node-B"}},
			"Node-A":  {Name: "Node-A", Type: "SS", History: []mihomo.DelayHistory{{Delay: 42}}},
			"Node-B":  {Name: "Node-B", Type: "SS", History: []mihomo.DelayHistory{{Delay: 84}}},
			"Node-C":  {Name: "Node-C", Type: "SS", History: []mihomo.DelayHistory{{Delay: 120}}},
		},
	}

	groups := BuildGroupViews(resp, "")
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	expectedOrder := []string{"GLOBAL", "PROXY", "CUSTOM"}
	for i, expected := range expectedOrder {
		if groups[i].Name != expected {
			t.Fatalf("expected group %d to be %s, got %s", i, expected, groups[i].Name)
		}
	}
}

func TestBuildGroupViews_WithFilter(t *testing.T) {
	resp := mihomo.ProxiesResponse{
		Proxies: map[string]mihomo.Proxy{
			"GLOBAL":    {Name: "GLOBAL", Type: "Selector", Now: "Node-A", All: []string{"Node-A", "Node-B", "Node-C"}},
			"PROXY":     {Name: "PROXY", Type: "Selector", Now: "Node-C", All: []string{"Node-C", "Node-D"}},
			"Node-A":    {Name: "Node-A", Type: "SS", History: []mihomo.DelayHistory{{Delay: 42}}},
			"Node-B":    {Name: "Node-B", Type: "SS", History: []mihomo.DelayHistory{{Delay: 84}}},
			"Node-C":    {Name: "Node-C", Type: "SS", History: []mihomo.DelayHistory{{Delay: 120}}},
			"Node-D":    {Name: "Node-D", Type: "SS", History: []mihomo.DelayHistory{{Delay: 150}}},
		},
	}

	groups := BuildGroupViews(resp, "node-a")
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Name != "GLOBAL" {
		t.Fatalf("expected GLOBAL group, got %s", groups[0].Name)
	}
	if len(groups[0].Options) != 1 {
		t.Fatalf("expected 1 option, got %d", len(groups[0].Options))
	}
	if groups[0].Options[0].Name != "Node-A" {
		t.Fatalf("expected Node-A option, got %s", groups[0].Options[0].Name)
	}
}

func TestBuildGroupViews_FilterMatchesGroupName(t *testing.T) {
	resp := mihomo.ProxiesResponse{
		Proxies: map[string]mihomo.Proxy{
			"GLOBAL":      {Name: "GLOBAL", Type: "Selector", Now: "Node-A", All: []string{"Node-A"}},
			"PROXY":       {Name: "PROXY", Type: "Selector", Now: "Node-B", All: []string{"Node-B"}},
			"MY-GROUP":    {Name: "MY-GROUP", Type: "Selector", Now: "Node-C", All: []string{"Node-C"}},
			"Node-A":      {Name: "Node-A", Type: "SS", History: []mihomo.DelayHistory{{Delay: 42}}},
			"Node-B":      {Name: "Node-B", Type: "SS", History: []mihomo.DelayHistory{{Delay: 84}}},
			"Node-C":      {Name: "Node-C", Type: "SS", History: []mihomo.DelayHistory{{Delay: 120}}},
		},
	}

	groups := BuildGroupViews(resp, "my-group")
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Name != "MY-GROUP" {
		t.Fatalf("expected MY-GROUP group, got %s", groups[0].Name)
	}
}

func TestBuildGroupViews_FilterHidesGroupWithNoMatchingOptions(t *testing.T) {
	resp := mihomo.ProxiesResponse{
		Proxies: map[string]mihomo.Proxy{
			"GLOBAL":      {Name: "GLOBAL", Type: "Selector", Now: "Node-A", All: []string{"Node-A"}},
			"PROXY":       {Name: "PROXY", Type: "Selector", Now: "Node-B", All: []string{"Node-B"}},
			"Node-A":      {Name: "Node-A", Type: "SS", History: []mihomo.DelayHistory{{Delay: 42}}},
			"Node-B":      {Name: "Node-B", Type: "SS", History: []mihomo.DelayHistory{{Delay: 84}}},
		},
	}

	groups := BuildGroupViews(resp, "node-c")
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups, got %d", len(groups))
	}
}

func TestBuildGroupViews_CaseInsensitiveFilter(t *testing.T) {
	resp := mihomo.ProxiesResponse{
		Proxies: map[string]mihomo.Proxy{
			"GLOBAL": {Name: "GLOBAL", Type: "Selector", Now: "Node-A", All: []string{"Node-A", "Node-B"}},
			"Node-A": {Name: "Node-A", Type: "SS", History: []mihomo.DelayHistory{{Delay: 42}}},
			"Node-B": {Name: "Node-B", Type: "SS", History: []mihomo.DelayHistory{{Delay: 84}}},
		},
	}

	tests := []struct {
		filter   string
		expected int
	}{
		{"node-a", 1},
		{"NODE-A", 1},
		{"NoDe-a", 1},
		{"NO-A", 0}, // "NO-A" is not a substring of "Node-A"
	}

	for _, tt := range tests {
		groups := BuildGroupViews(resp, tt.filter)
		if len(groups) != tt.expected {
			t.Fatalf("filter %q: expected %d group, got %d", tt.filter, tt.expected, len(groups))
		}
	}
}

func TestBuildGroupViews_FilterWithWhitespace(t *testing.T) {
	resp := mihomo.ProxiesResponse{
		Proxies: map[string]mihomo.Proxy{
			"GLOBAL": {Name: "GLOBAL", Type: "Selector", Now: "Node-A", All: []string{"Node-A"}},
			"Node-A": {Name: "Node-A", Type: "SS", History: []mihomo.DelayHistory{{Delay: 42}}},
		},
	}

	groups := BuildGroupViews(resp, "  node-a  ")
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
}

func TestBuildGroupViews_MultipleGroups(t *testing.T) {
	resp := mihomo.ProxiesResponse{
		Proxies: map[string]mihomo.Proxy{
			"GROUP-A": {Name: "GROUP-A", Type: "Selector", Now: "Node-A1", All: []string{"Node-A1", "Node-A2"}},
			"GROUP-B": {Name: "GROUP-B", Type: "Selector", Now: "Node-B1", All: []string{"Node-B1", "Node-B2"}},
			"GROUP-C": {Name: "GROUP-C", Type: "Selector", Now: "Node-C1", All: []string{"Node-C1", "Node-C2"}},
			"Node-A1": {Name: "Node-A1", Type: "SS", History: []mihomo.DelayHistory{{Delay: 42}}},
			"Node-A2": {Name: "Node-A2", Type: "SS", History: []mihomo.DelayHistory{{Delay: 84}}},
			"Node-B1": {Name: "Node-B1", Type: "SS", History: []mihomo.DelayHistory{{Delay: 120}}},
			"Node-B2": {Name: "Node-B2", Type: "SS", History: []mihomo.DelayHistory{{Delay: 150}}},
			"Node-C1": {Name: "Node-C1", Type: "SS", History: []mihomo.DelayHistory{{Delay: 200}}},
			"Node-C2": {Name: "Node-C2", Type: "SS", History: []mihomo.DelayHistory{{Delay: 250}}},
		},
	}

	groups := BuildGroupViews(resp, "")
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	for _, group := range groups {
		if len(group.Options) != 2 {
			t.Fatalf("group %s: expected 2 options, got %d", group.Name, len(group.Options))
		}
	}
}

func TestLatestDelay(t *testing.T) {
	tests := []struct {
		name     string
		proxy    mihomo.Proxy
		expected int
	}{
		{
			name:     "empty history",
			proxy:    mihomo.Proxy{History: []mihomo.DelayHistory{}},
			expected: 0,
		},
		{
			name:     "single entry",
			proxy:    mihomo.Proxy{History: []mihomo.DelayHistory{{Delay: 42}}},
			expected: 42,
		},
		{
			name:     "multiple entries",
			proxy:    mihomo.Proxy{History: []mihomo.DelayHistory{{Delay: 42}, {Delay: 84}, {Delay: 120}}},
			expected: 120,
		},
		{
			name:     "zero delay",
			proxy:    mihomo.Proxy{History: []mihomo.DelayHistory{{Delay: 0}}},
			expected: 0,
		},
		{
			name:     "negative delay",
			proxy:    mihomo.Proxy{History: []mihomo.DelayHistory{{Delay: -1}}},
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := latestDelay(tt.proxy)
			if result != tt.expected {
				t.Fatalf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestPriority(t *testing.T) {
	tests := []struct {
		name     string
		group    string
		expected int
	}{
		{"global", "GLOBAL", 0},
		{"global lowercase", "global", 0},
		{"proxy", "Proxy", 1},
		{"proxy uppercase", "PROXY", 1},
		{"proxy lowercase", "proxy", 1},
		{"custom", "CUSTOM", 10},
		{"random", "Random", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := priority(tt.group)
			if result != tt.expected {
				t.Fatalf("priority(%q): expected %d, got %d", tt.group, tt.expected, result)
			}
		})
	}
}

func TestIsMetaOptionName(t *testing.T) {
	tests := []struct {
		name     string
		option   string
		expected bool
	}{
		{"traffic prefix", "traffic: 1GB", true},
		{"Traffic prefix", "Traffic: 1GB", true},
		{"TRAFFIC prefix", "TRAFFIC: 1GB", true},
		{"expire prefix", "expire: 2027-01-01", true},
		{"Expire prefix", "Expire: 2027-01-01", true},
		{"EXPIRE prefix", "EXPIRE: 2027-01-01", true},
		{"normal option", "Node-A", false},
		{"with whitespace", "  traffic: 1GB  ", true},
		{"empty string", "", false},
		{"only whitespace", "   ", false},
		{"partial match", "prefix-traffic: 1GB", false},
		{"suffix match", "traffic-: 1GB", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMetaOptionName(tt.option)
			if result != tt.expected {
				t.Fatalf("isMetaOptionName(%q): expected %v, got %v", tt.option, tt.expected, result)
			}
		})
	}
}
