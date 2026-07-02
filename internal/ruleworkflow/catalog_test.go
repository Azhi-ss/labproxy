package ruleworkflow

import "testing"

func TestDefaultCandidatesContainFirstBatch(t *testing.T) {
	candidates := DefaultCandidates()
	got := map[string]Candidate{}

	want := map[string]struct {
		targetGroup string
		sourceURL   string
		behavior    string
		path        string
		interval    int
	}{
		"github": {
			targetGroup: "Proxies",
			sourceURL:   "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/GitHub/GitHub.yaml",
			behavior:    "classical",
			path:        "./rule-providers/github.yaml",
			interval:    86400,
		},
		"openai": {
			targetGroup: "OpenAI",
			sourceURL:   "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/OpenAI/OpenAI.yaml",
			behavior:    "classical",
			path:        "./rule-providers/openai.yaml",
			interval:    86400,
		},
		"anthropic": {
			targetGroup: "OpenAI",
			sourceURL:   "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Anthropic/Anthropic.yaml",
			behavior:    "classical",
			path:        "./rule-providers/anthropic.yaml",
			interval:    86400,
		},
		"youtube": {
			targetGroup: "YouTube",
			sourceURL:   "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/YouTube/YouTube.yaml",
			behavior:    "classical",
			path:        "./rule-providers/youtube.yaml",
			interval:    86400,
		},
		"netflix": {
			targetGroup: "Netflix",
			sourceURL:   "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Netflix/Netflix.yaml",
			behavior:    "classical",
			path:        "./rule-providers/netflix.yaml",
			interval:    86400,
		},
		"disney": {
			targetGroup: "Disney",
			sourceURL:   "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Disney/Disney.yaml",
			behavior:    "classical",
			path:        "./rule-providers/disney.yaml",
			interval:    86400,
		},
		"telegram": {
			targetGroup: "Telegram",
			sourceURL:   "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/Telegram/Telegram.yaml",
			behavior:    "classical",
			path:        "./rule-providers/telegram.yaml",
			interval:    86400,
		},
	}

	if len(candidates) != len(want) {
		t.Fatalf("len(candidates) = %d, want %d", len(candidates), len(want))
	}

	for _, c := range candidates {
		if _, ok := want[c.Name]; !ok {
			t.Fatalf("unexpected candidate %s present in DefaultCandidates", c.Name)
		}
		got[c.Name] = c
	}

	for name, expected := range want {
		c, ok := got[name]
		if !ok {
			t.Fatalf("missing candidate %s", name)
		}
		if c.Provider.Name != name {
			t.Fatalf("candidate %s provider name mismatch: %q", name, c.Provider.Name)
		}
		if c.Provider.Type != "http" {
			t.Fatalf("candidate %s provider type = %q, want http", name, c.Provider.Type)
		}
		if c.TargetGroup != expected.targetGroup {
			t.Fatalf("candidate %s target group = %q, want %q", name, c.TargetGroup, expected.targetGroup)
		}
		if c.SourceURL != expected.sourceURL {
			t.Fatalf("candidate %s source url = %q, want %q", name, c.SourceURL, expected.sourceURL)
		}
		if c.Provider.URL != expected.sourceURL {
			t.Fatalf("candidate %s provider url = %q, want %q", name, c.Provider.URL, expected.sourceURL)
		}
		if c.Provider.Behavior != expected.behavior {
			t.Fatalf("candidate %s provider behavior = %q, want %q", name, c.Provider.Behavior, expected.behavior)
		}
		if c.Provider.Path != expected.path {
			t.Fatalf("candidate %s provider path = %q, want %q", name, c.Provider.Path, expected.path)
		}
		if c.Provider.Interval != expected.interval {
			t.Fatalf("candidate %s provider interval = %d, want %d", name, c.Provider.Interval, expected.interval)
		}
	}

	for _, name := range []string{"huggingface", "apple"} {
		if _, ok := got[name]; ok {
			t.Fatalf("unexpected candidate %s present in DefaultCandidates", name)
		}
	}
}

func TestSelectedCandidatesRejectUnknownName(t *testing.T) {
	_, err := SelectedCandidates([]string{"github", "missing"})
	if err == nil {
		t.Fatal("expected error for unknown candidate")
	}
}

func TestSelectedCandidatesDefaultAll(t *testing.T) {
	got, err := SelectedCandidates(nil)
	if err != nil {
		t.Fatalf("SelectedCandidates nil: %v", err)
	}
	if len(got) != len(DefaultCandidates()) {
		t.Fatalf("len = %d, want %d", len(got), len(DefaultCandidates()))
	}
}
