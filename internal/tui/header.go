package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m model) renderHeader() string {
	t := m.theme
	docWidth := max(0, m.width-docStyle.GetHorizontalFrameSize())
	if docWidth <= 0 {
		return ""
	}
	innerWidth := max(0, docWidth-headerStyle(t).GetHorizontalFrameSize())

	// 左：应用标题
	left := titleStyle(t).Render(T().AppTitle)

	// 右：紧凑状态行
	parts := []string{
		mutedStyle(t).Render(T().PillEndpoint) + fallback(m.endpoint, "-"),
		mutedStyle(t).Render(T().PillMode) + modeLabel(t, m.mode),
		mutedStyle(t).Render(T().PillProxy) + boolLabel(t, m.systemProxyEnabled),
		mutedStyle(t).Render(T().PillLan) + boolLabel(t, m.allowLanEnabled),
		mutedStyle(t).Render(T().PillTun) + boolLabel(t, m.tunEnabled),
		mutedStyle(t).Render("↑") + formatBytes(m.up),
		mutedStyle(t).Render("↓") + formatBytes(m.down),
	}
	right := lipgloss.JoinHorizontal(lipgloss.Left, parts...)

	// 左右之间填充空格
	gap := innerWidth - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		gap = 1
	}
	row := left + strings.Repeat(" ", gap) + right

	return docStyle.Width(docWidth).Render(
		headerStyle(t).Width(innerWidth).MaxWidth(docWidth).Render(
			fitLine(row, innerWidth),
		),
	)
}
