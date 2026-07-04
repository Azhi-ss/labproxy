package tui

import (
	"strings"
)

func (m model) renderHeader() string {
	t := m.theme
	docWidth := m.docWidth()
	if docWidth <= 0 {
		return ""
	}
	contentWidth := chromeContentWidth(t, docWidth)

	// 左：应用标题
	left := titleStyle(t).Render(T().AppTitle)

	// 右：紧凑状态行
	parts := []string{
		mutedStyle(t).Render(T().PillEndpoint+" ") + fallback(m.endpoint, "-"),
		mutedStyle(t).Render(T().PillMode+" ") + modeLabel(t, m.mode),
		mutedStyle(t).Render(T().PillProxy+" ") + boolLabel(t, m.systemProxyEnabled),
		mutedStyle(t).Render(T().PillLan+" ") + boolLabel(t, m.allowLanEnabled),
		mutedStyle(t).Render(T().PillTun+" ") + boolLabel(t, m.tunEnabled),
		mutedStyle(t).Render("↑ ") + formatBytes(m.up),
		mutedStyle(t).Render("↓ ") + formatBytes(m.down),
	}
	right := strings.Join(parts, mutedStyle(t).Render("  ·  "))

	row := alignLine(left, right, contentWidth)

	return renderChromeLine(t, row, docWidth)
}

func (m model) renderTabs() string {
	t := m.theme
	docWidth := m.docWidth()
	if docWidth <= 0 {
		return ""
	}
	contentWidth := chromeContentWidth(t, docWidth)
	row := renderTopTabs(t, m.activeView, docWidth < 90, contentWidth)
	return renderChromeLine(t, row, docWidth)
}
