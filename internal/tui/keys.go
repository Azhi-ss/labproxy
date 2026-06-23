package tui

import (
	"github.com/charmbracelet/bubbles/key"
)

type keyMap struct {
	Up          key.Binding
	Down        key.Binding
	Left        key.Binding
	Right       key.Binding
	Tab         key.Binding
	Select      key.Binding
	Refresh     key.Binding
	Search      key.Binding
	Mode        key.Binding
	SystemProxy key.Binding
	Back        key.Binding
	Quit        key.Binding
	TestGroup   key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Tab, k.Select, k.Refresh, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Tab, k.Select, k.Refresh, k.Search, k.Mode, k.SystemProxy, k.Back, k.Quit},
	}
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", T().HelpMoveUp)),
		Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", T().HelpMoveDown)),
		Left:        key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", T().HelpFocusLeft)),
		Right:       key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", T().HelpFocusRight)),
		Tab:         key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", T().HelpSwitchPane)),
		Select:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", T().HelpApplySelect)),
		Refresh:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", T().HelpRefreshDelay)),
		Search:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", T().HelpSearch)),
		Mode:        key.NewBinding(key.WithKeys("m"), key.WithHelp("m", T().HelpCycleMode)),
		SystemProxy: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", T().HelpToggleProxyPref)),
		Back:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", T().HelpCloseBack)),
		Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", T().HelpQuit)),
		TestGroup:   key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "test group")),
	}
}
