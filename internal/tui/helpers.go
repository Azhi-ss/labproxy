package tui

import (
	"fmt"
	"strings"

	"labproxy/internal/proxy"
	"labproxy/internal/tui/theme"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func fitLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(line, width, "…")
}

func fitStyledLine(line string, width int, padStyle lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	line = ansi.Truncate(line, width, "…")
	if visLen := ansi.StringWidth(line); visLen < width {
		line += padStyle.Render(strings.Repeat(" ", width-visLen))
	}
	return line
}

func alignLine(left, right string, width int) string {
	if width <= 0 {
		return ""
	}
	left = ansi.Truncate(left, width, "…")
	leftWidth := ansi.StringWidth(left)
	if strings.TrimSpace(right) == "" || leftWidth >= width {
		return fitStyledLine(left, width, lipgloss.NewStyle())
	}
	rightWidth := ansi.StringWidth(right)
	if leftWidth+1+rightWidth > width {
		remaining := width - leftWidth - 1
		if remaining <= 0 {
			return fitStyledLine(left, width, lipgloss.NewStyle())
		}
		right = ansi.Truncate(right, remaining, "…")
		rightWidth = ansi.StringWidth(right)
	}
	gap := width - leftWidth - rightWidth
	if gap < 1 {
		gap = 1
	}
	return fitStyledLine(left+strings.Repeat(" ", gap)+right, width, lipgloss.NewStyle())
}

func (m model) docWidth() int {
	return max(0, m.width-docStyle.GetHorizontalFrameSize())
}

func chromeContentWidth(t *theme.Theme, docWidth int) int {
	return max(0, docWidth-headerStyle(t).GetHorizontalFrameSize()-4)
}

func renderChromeLine(t *theme.Theme, row string, docWidth int) string {
	if docWidth <= 0 {
		return ""
	}
	contentWidth := chromeContentWidth(t, docWidth)
	return docStyle.Width(docWidth).Render(
		headerStyle(t).Render(fitLine(row, contentWidth)),
	)
}

func renderPanelContent(t *theme.Theme, title, subtitle string, rows []string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	lines := make([]string, 0, height)
	lines = append(lines, fitLine(titleStyle(t).Render(ansi.Truncate(title, width, "…")), width))

	if height >= 3 && strings.TrimSpace(subtitle) != "" {
		lines = append(lines, "")
		lines = append(lines, fitLine(subtitleStyle(t).Render(ansi.Truncate(subtitle, width, "…")), width))
	}

	// 标题/副标题与行之间留空行
	if len(lines) > 0 && len(rows) > 0 && height > len(lines) {
		lines = append(lines, "")
	}

	remaining := height - len(lines)
	if remaining > 0 && len(rows) > 0 {
		if remaining > len(rows) {
			remaining = len(rows)
		}
		lines = append(lines, rows[:remaining]...)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func renderPanel(style lipgloss.Style, width, height int, content string) string {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	frameW := style.GetHorizontalFrameSize()
	return style.
		Width(width + frameW).
		Height(height).
		MaxWidth(width + frameW).
		MaxHeight(height).
		Render(content)
}

func plainDelayLabel(ms int) string {
	if ms == -1 {
		return "timeout"
	}
	if ms <= 0 {
		return "--"
	}
	return fmt.Sprintf("%dms", ms)
}

func fallback(value, alt string) string {
	if strings.TrimSpace(value) == "" {
		return alt
	}
	return value
}

func truncate(value string, width int) string {
	return ansi.Truncate(value, width, "…")
}

func delayLabel(t *theme.Theme, ms int) string {
	if ms <= 0 {
		return mutedStyle(t).Render("--")
	}
	return getDelayStyle(t, ms).Render(fmt.Sprintf("%dms", ms))
}

func window(selected, total, limit int) (int, int) {
	if total <= limit {
		return 0, total
	}
	start := selected - limit/2
	if start < 0 {
		start = 0
	}
	end := start + limit
	if end > total {
		end = total
		start = end - limit
	}
	return start, end
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func boolLabel(t *theme.Theme, value bool) string {
	if value {
		return onStyle(t).Render(T().BoolOn)
	}
	return offStyle(t).Render(T().BoolOff)
}

func boolLabelPlain(value bool) string {
	if value {
		return T().BoolOn
	}
	return T().BoolOff
}

func modeLabel(t *theme.Theme, mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "rule" && mode != "global" && mode != "direct" {
		mode = fallback(mode, "unknown")
	}
	return getModeStyle(t, mode).Render(mode)
}

func modeLabelPlain(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "rule" && mode != "global" && mode != "direct" {
		mode = fallback(mode, "unknown")
	}
	return mode
}

func nextMode(current string) string {
	switch strings.ToLower(strings.TrimSpace(current)) {
	case "global":
		return "direct"
	case "direct":
		return "rule"
	default:
		return "global"
	}
}

func connectionTarget(conn proxy.Connection) string {
	if host := strings.TrimSpace(conn.Metadata.Host); host != "" {
		return host
	}
	if destination := strings.TrimSpace(conn.Metadata.Destination); destination != "" {
		return destination
	}
	return conn.ID
}

func formatBytes(value int64) string {
	units := []string{"B/s", "KB/s", "MB/s", "GB/s"}
	size := float64(value)
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}
	return fmt.Sprintf("%.1f%s", size, units[unit])
}

func formatSize(value int64) string {
	units := []string{"B", "KB", "MB", "GB"}
	size := float64(value)
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}
	return fmt.Sprintf("%.1f%s", size, units[unit])
}
