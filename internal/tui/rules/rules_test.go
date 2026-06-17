package rules

import "testing"

func TestModal_Open(t *testing.T) {
	m := NewModal("/tmp/mixin.yaml")
	m.Open()
	if !m.IsOpen() {
		t.Error("modal should be open after Open()")
	}
	if m.View() == "" {
		t.Error("modal should produce non-empty view")
	}
}

func TestModal_Close(t *testing.T) {
	m := NewModal("/tmp/mixin.yaml")
	m.Open()
	m.Close()
	if m.IsOpen() {
		t.Error("modal should be closed after Close()")
	}
}
