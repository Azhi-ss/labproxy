package tui

import (
	"github.com/charmbracelet/lipgloss"
)

func (m model) renderFooter() string {
	t := m.theme
	docWidth := max(0, m.width-docStyle.GetHorizontalFrameSize())
	if docWidth <= 0 {
		return ""
	}
	innerWidth := max(0, docWidth-headerStyle(t).GetHorizontalFrameSize())
	helpView := fitLine(mutedStyle(t).Render(m.footerKeyHint()), innerWidth)
	left := statusStyle(t).Render(fallback(m.statusLine, T().StatusReady))
	if m.searchMode {
		left = lipgloss.JoinHorizontal(lipgloss.Left, left, "  ", titleStyle(t).Render(T().SearchLabel), m.search.View())
	}
	row := lipgloss.JoinVertical(lipgloss.Left, fitLine(left, innerWidth), helpView)
	return docStyle.Width(docWidth).Render(headerStyle(t).Width(innerWidth).MaxWidth(docWidth).Render(row))
}

func (m model) focusLabel() string {
	switch m.focus {
	case focusGroups:
		return T().FocusGroupsLabel
	default:
		return T().FocusOptionsLabel
	}
}

func (m model) footerKeyHint() string {
	switch m.activeView {
	case viewProxies:
		return "1-5 view  / search  t test  h/l focus  j/k move  enter switch  ? help  q quit"
	case viewConnections:
		return "1-5 view  j/k move  d close  D close all  ? help  q quit"
	case viewLogs:
		return "1-5 view  / filter  l level  ? help  q quit"
	case viewRules:
		return "1-5 view  a add  enter edit  d delete  ? help  q quit"
	case viewConfig:
		return "1-5 view  j/k move  enter toggle  ? help  q quit"
	}
	return ""
}

func (m model) renderHelpOverlay() string {
	t := m.theme
	title := titleStyle(t).Render(T().HelpTitle)

	rows := []string{title, ""}
	rows = append(rows, mutedStyle(t).Render("1-5  switch view"))
	rows = append(rows, mutedStyle(t).Render("Tab  next view"))
	rows = append(rows, mutedStyle(t).Render("j/k  move cursor"))
	rows = append(rows, mutedStyle(t).Render("/    search/filter"))
	rows = append(rows, mutedStyle(t).Render("r    refresh"))
	rows = append(rows, mutedStyle(t).Render("q    quit"))
	rows = append(rows, mutedStyle(t).Render("?    toggle help"))
	rows = append(rows, "")
	rows = append(rows, mutedStyle(t).Render("Proxies: t test  h/l focus  enter switch"))
	rows = append(rows, mutedStyle(t).Render("Conns:   d close  D close all"))
	rows = append(rows, mutedStyle(t).Render("Logs:    l level"))
	rows = append(rows, mutedStyle(t).Render("Config:  enter toggle"))

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		panelBaseStyle(t).Padding(1, 2).Render(content),
	)
}
