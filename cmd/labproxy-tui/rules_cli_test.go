package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunRulesCLI_RequiresMixinPath(t *testing.T) {
	var buf bytes.Buffer
	code := runRulesCLI(&buf, &buf, []string{"list"}, "")
	if code != 2 {
		t.Errorf("expected exit 2, got %d", code)
	}
	if !strings.Contains(buf.String(), "mixin") {
		t.Errorf("expected error to mention mixin, got %s", buf.String())
	}
}
