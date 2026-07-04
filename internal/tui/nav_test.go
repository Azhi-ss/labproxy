package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"labproxy/internal/tui/theme"
)

func TestViewID_Next(t *testing.T) {
	cases := []struct{ in, want viewID }{
		{viewProxies, viewConnections},
		{viewConnections, viewLogs},
		{viewLogs, viewRules},
		{viewRules, viewConfig},
		{viewConfig, viewProxies}, // wrap
	}
	for _, c := range cases {
		if got := c.in.next(); got != c.want {
			t.Errorf("%v.next() = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestViewID_Label(t *testing.T) {
	if viewProxies.label() == "" {
		t.Error("proxies label empty")
	}
}

func TestViewID_ByDigit(t *testing.T) {
	cases := []struct {
		digit string
		want  viewID
		ok    bool
	}{
		{"1", viewProxies, true},
		{"2", viewConnections, true},
		{"3", viewLogs, true},
		{"4", viewRules, true},
		{"5", viewConfig, true},
		{"6", viewProxies, false},
		{"0", viewProxies, false},
	}
	for _, c := range cases {
		got, ok := viewByDigit(c.digit)
		if ok != c.ok {
			t.Errorf("viewByDigit(%q) ok=%v want %v", c.digit, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("viewByDigit(%q) = %v, want %v", c.digit, got, c.want)
		}
	}
}

func TestRenderTopTabsCompactUsesDigitsOnly(t *testing.T) {
	tabs := ansi.Strip(renderTopTabs(theme.Light, viewProxies, true, 40))
	if !strings.Contains(tabs, "[1]") || !strings.Contains(tabs, "[2]") {
		t.Fatalf("expected compact tabs to include digit tabs, got %q", tabs)
	}
	if strings.Contains(tabs, T().NavConnections) {
		t.Fatalf("expected compact tabs to hide labels, got %q", tabs)
	}
}

func TestRenderTopTabsStandardShowsLabels(t *testing.T) {
	tabs := ansi.Strip(renderTopTabs(theme.Light, viewProxies, false, 120))
	if !strings.Contains(tabs, T().NavConnections) {
		t.Fatalf("expected standard tabs to include labels, got %q", tabs)
	}
}
