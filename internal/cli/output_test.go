package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestIsJSONFlag(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"absent", []string{"status"}, false},
		{"trailing", []string{"status", "--json"}, true},
		{"leading", []string{"--json", "status"}, true},
		{"middle", []string{"proxies", "--json", "group"}, true},
		{"equals form", []string{"status", "--json=true"}, true},
		{"equals false", []string{"status", "--json=false"}, false},
		{"short not recognized", []string{"status", "-j"}, false},
		{"no args", []string{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsJSONFlag(c.args); got != c.want {
				t.Fatalf("IsJSONFlag(%v) = %v, want %v", c.args, got, c.want)
			}
		})
	}
}

func TestEnvelopeSuccessJSON(t *testing.T) {
	var buf bytes.Buffer
	PrintJSON(&buf, Envelope{OK: true, Data: map[string]int{"port": 7890}})

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid json output: %v\n%s", err, buf.String())
	}
	if got["ok"] != true {
		t.Errorf("ok field = %v, want true", got["ok"])
		data, _ := got["data"].(map[string]any)
		if data["port"] != float64(7890) {
			t.Errorf("data.port = %v, want 7890", data["port"])
		}
	}
	if _, ok := got["error"]; !ok {
		t.Errorf("error field missing")
	}
}

func TestEnvelopeErrorJSON(t *testing.T) {
	var buf bytes.Buffer
	PrintJSON(&buf, Envelope{OK: false, Error: "kernel not running"})

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}
	if got["ok"] != false {
		t.Errorf("ok = %v, want false", got["ok"])
	}
	if got["error"] != "kernel not running" {
		t.Errorf("error = %v, want 'kernel not running'", got["error"])
	}
	// error envelope data should be null, not omitted
	if !reflect.DeepEqual(got["data"], nil) {
		t.Errorf("data = %v, want nil", got["data"])
	}
}

func TestPrintJSONEndsWithNewline(t *testing.T) {
	var buf bytes.Buffer
	PrintJSON(&buf, Envelope{OK: true, Data: nil})
	if buf.Len() == 0 || buf.Bytes()[buf.Len()-1] != '\n' {
		t.Fatalf("output should end with newline, got %q", buf.String())
	}
}
