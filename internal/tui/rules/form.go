package rules

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"labproxy/internal/rules"
)

// Form is the add/edit rule form with three text inputs (Type, Payload, Proxy)
// plus a trailing no-resolve toggle field (focusIdx == len(inputs)).
type Form struct {
	inputs    []textinput.Model
	focusIdx  int
	noResolve bool
}

// NewForm builds a form with Type/Payload/Proxy inputs.
func NewForm() *Form {
	mk := func(ph string) textinput.Model {
		t := textinput.New()
		t.Placeholder = ph
		t.CharLimit = 256
		return t
	}
	f := &Form{
		inputs: []textinput.Model{
			mk("DOMAIN-SUFFIX"),
			mk("example.com"),
			mk("PROXY"),
		},
	}
	f.Focus()
	return f
}

// Focus focuses the current input field (no-op when on the toggle field).
func (f *Form) Focus() {
	if f.focusIdx >= 0 && f.focusIdx < len(f.inputs) {
		f.inputs[f.focusIdx].Focus()
	}
}

// Blur blurs all inputs.
func (f *Form) Blur() {
	for i := range f.inputs {
		f.inputs[i].Blur()
	}
}

// NextField moves focus forward, cycling through inputs and the toggle.
func (f *Form) NextField() {
	f.inputs[f.focusIdx].Blur()
	f.focusIdx = (f.focusIdx + 1) % (len(f.inputs) + 1)
	f.Focus()
}

// PrevField moves focus backward, cycling through inputs and the toggle.
func (f *Form) PrevField() {
	f.inputs[f.focusIdx].Blur()
	n := len(f.inputs) + 1
	f.focusIdx = (f.focusIdx - 1 + n) % n
	f.Focus()
}

// Update forwards key events to the focused input, or toggles no-resolve
// when the toggle field is focused.
func (f *Form) Update(msg tea.KeyMsg) {
	if f.focusIdx < len(f.inputs) {
		var cmd tea.Cmd
		f.inputs[f.focusIdx], cmd = f.inputs[f.focusIdx].Update(msg)
		_ = cmd
	} else {
		switch msg.String() {
		case " ", "x":
			f.noResolve = !f.noResolve
		}
	}
}

// ToggleNoResolve flips the no-resolve flag when the toggle field is focused.
func (f *Form) ToggleNoResolve() {
	if f.focusIdx == len(f.inputs) {
		f.noResolve = !f.noResolve
	}
}

// Build validates inputs and returns a Rule.
func (f *Form) Build() (rules.Rule, error) {
	r := rules.Rule{
		Type:      rules.RuleType(strings.TrimSpace(f.inputs[0].Value())),
		Payload:   strings.TrimSpace(f.inputs[1].Value()),
		Proxy:     strings.TrimSpace(f.inputs[2].Value()),
		NoResolve: f.noResolve,
		Enabled:   true,
	}
	if err := r.Validate(); err != nil {
		return r, err
	}
	return r, nil
}

// View renders the form with Chinese labels.
func (f *Form) View() string {
	var b strings.Builder
	b.WriteString("类型:      [")
	b.WriteString(f.inputs[0].View())
	b.WriteString("]\n")
	b.WriteString("Payload:   [")
	b.WriteString(f.inputs[1].View())
	b.WriteString("]\n")
	b.WriteString("目标:      [")
	b.WriteString(f.inputs[2].View())
	b.WriteString("]\n")
	check := "[ ]"
	if f.noResolve {
		check = "[x]"
	}
	b.WriteString(fmt.Sprintf("选项:      %s no-resolve\n", check))
	return b.String()
}
