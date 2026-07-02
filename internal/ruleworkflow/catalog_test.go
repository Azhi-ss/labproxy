package ruleworkflow

import "testing"

func TestDefaultCandidatesContainFirstBatch(t *testing.T) {
	candidates := DefaultCandidates()
	got := map[string]Candidate{}
	for _, c := range candidates {
		got[c.Name] = c
	}
	for _, name := range []string{"github", "openai", "anthropic", "youtube", "netflix", "disney", "telegram"} {
		c, ok := got[name]
		if !ok {
			t.Fatalf("missing candidate %s", name)
		}
		if c.Provider.Name != name {
			t.Fatalf("candidate %s provider name mismatch: %q", name, c.Provider.Name)
		}
		if c.TargetGroup == "" {
			t.Fatalf("candidate %s has empty target group", name)
		}
		if c.Provider.Type != "http" {
			t.Fatalf("candidate %s provider type = %q, want http", name, c.Provider.Type)
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
