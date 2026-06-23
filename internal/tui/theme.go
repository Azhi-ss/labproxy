package tui

import "github.com/charmbracelet/lipgloss"

// theme.go 集中管理所有色彩 token 与样式定义。
// 详见 docs/superpowers/specs/2026-06-22-tui-redesign-design.md §7。

var (
	// ── Theme palette ──────────────────────────────────────────────────
	colorPrimary     = lipgloss.Color("39") // bright blue — identity & structure
	colorAccent      = lipgloss.Color("86") // bright cyan-green — focus & active
	colorSurfaceHigh = lipgloss.Color("62") // deep indigo — selection bg

	colorTextPrimary   = lipgloss.Color("252") // near-white
	colorTextSecondary = lipgloss.Color("246") // mid-gray
	colorTextMuted     = lipgloss.Color("243") // dim gray

	// Semantic: state & delay colors
	colorSuccess = lipgloss.Color("42")  // green  — low delay / on
	colorWarning = lipgloss.Color("220") // yellow — mid delay
	colorInfo    = lipgloss.Color("117") // light blue
	colorDanger  = lipgloss.Color("203") // red — high delay / off / error
	colorOrange  = lipgloss.Color("215") // orange — mid-high delay

	// ── Layout constants ──────────────────────────────────────────────
	columnGap = 2

	docStyle = lipgloss.NewStyle().Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1)

	panelBaseStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)

	activePanelStyle   = panelBaseStyle.BorderForeground(colorAccent)
	inactivePanelStyle = panelBaseStyle.BorderForeground(lipgloss.Color("237"))

	navActiveStyle = lipgloss.NewStyle().
			Foreground(colorAccent).Bold(true)

	// ── Typography ──────────────────────────────────────────────────────
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	subtitleStyle = lipgloss.NewStyle().Foreground(colorTextSecondary)

	// ── Status & feedback ──────────────────────────────────────────────
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(colorSurfaceHigh).
			Padding(0, 1)
	mutedStyle    = lipgloss.NewStyle().Foreground(colorTextMuted)
	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorTextPrimary).
			Background(colorSurfaceHigh)
	currentStyle = lipgloss.NewStyle().
			Foreground(colorAccent).Bold(true)

	onStyle  = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	offStyle = lipgloss.NewStyle().Foreground(colorTextMuted)

	fitLineStyle = lipgloss.NewStyle()
)

// getDelayStyle 按延迟返回色阶样式：<50 绿 / <150 黄 / <300 橙 / ≥300 红 / -1 暗红 / 0 灰。
func getDelayStyle(ms int) lipgloss.Style {
	switch {
	case ms <= 0:
		if ms == -1 {
			return lipgloss.NewStyle().Foreground(colorDanger)
		}
		return mutedStyle
	case ms < 50:
		return lipgloss.NewStyle().Foreground(colorSuccess)
	case ms < 150:
		return lipgloss.NewStyle().Foreground(colorWarning)
	case ms < 300:
		return lipgloss.NewStyle().Foreground(colorOrange)
	default:
		return lipgloss.NewStyle().Foreground(colorDanger)
	}
}
