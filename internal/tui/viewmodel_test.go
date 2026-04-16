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
