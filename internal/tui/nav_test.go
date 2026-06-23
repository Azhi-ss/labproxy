package tui

import "testing"

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
