package tui

import (
	"fmt"
	"strings"

	"labproxy/internal/proxy"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func fitLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	return fitLineStyle.MaxWidth(width).Render(line)
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

func renderPanelContent(title, subtitle string, rows []string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	lines := make([]string, 0, height)
	lines = append(lines, fitLine(titleStyle.Render(ansi.Truncate(title, width, "…")), width))
	if height >= 2 && strings.TrimSpace(subtitle) != "" {
		lines = append(lines, fitLine(subtitleStyle.Render(ansi.Truncate(subtitle, width, "…")), width))
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
	return style.
		Width(width).
		Height(height).
		MaxWidth(width).
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

func delayLabel(ms int) string {
	if ms <= 0 {
		return mutedStyle.Render("--")
	}
	return getDelayStyle(ms).Render(fmt.Sprintf("%dms", ms))
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

func boolLabel(value bool) string {
	if value {
		return onStyle.Render(T().BoolOn)
	}
	return offStyle.Render(T().BoolOff)
}

func modeLabel(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "rule" && mode != "global" && mode != "direct" {
		mode = fallback(mode, "unknown")
	}
	return getModeStyle(mode).Render(mode)
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
