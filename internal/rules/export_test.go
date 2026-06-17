package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExport(t *testing.T) {
	s := newTestStore(t, "rules:\n  - DOMAIN,a.com,DIRECT\n  # - DOMAIN,b.com,DIRECT\n")
	out := filepath.Join(t.TempDir(), "out.yaml")
	if err := Export(s, out, false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	content := string(data)
	if !strings.Contains(content, "DOMAIN,a.com,DIRECT") {
		t.Error("expected enabled rule in export")
	}
	if strings.Contains(content, "DOMAIN,b.com,DIRECT") {
		t.Error("disabled rule should not be exported by default")
	}
}
