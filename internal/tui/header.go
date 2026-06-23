package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m model) renderHeader() string {
	docWidth := max(0, m.width-docStyle.GetHorizontalFrameSize())
	if docWidth <= 0 {
		return ""
	}
	innerWidth := max(0, docWidth-headerStyle.GetHorizontalFrameSize())
	titleRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		titleStyle.Render(T().AppTitle),
		"  ",
		subtitleStyle.Render(T().PressSForSettings),
	)

	metaRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		statusPill(T().PillEndpoint, fallback(m.endpoint, "-")),
		statusPill(T().PillMode, modeLabel(m.mode)),
		statusPill(T().PillProxy, boolLabel(m.systemProxyEnabled)),
		statusPill(T().PillLan, boolLabel(m.allowLanEnabled)),
		statusPill(T().PillTun, boolLabel(m.tunEnabled)),
		statusPill("↑", formatBytes(m.up)),
		statusPill("↓", formatBytes(m.down)),
		statusPill(T().PillFocus, m.focusLabel()),
	)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		fitLine(titleRow, innerWidth),
		"",
		fitLine(metaRow, innerWidth),
	)
	return docStyle.Width(docWidth).Render(headerStyle.Width(innerWidth).MaxWidth(docWidth).Render(content))
}

func statusPill(label, value string) string {
	pill := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("237")).
		Padding(0, 1)
	return pill.Render(fmt.Sprintf("%s %s", mutedStyle.Render(label), value))
}
