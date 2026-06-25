package tui

import (
	"github.com/charmbracelet/lipgloss"

	"labproxy/internal/tui/theme"
)

// theme.go 提供基于 Theme 的样式工厂函数，替代旧的全局 var。
// 设计原则：无边框，用背景色和间距区分层次。

// ── Layout constants ──────────────────────────────────────────────────

const columnGap = 2

var docStyle = lipgloss.NewStyle().Padding(0, 1)

// ── Style factories ───────────────────────────────────────────────────

func headerStyle(t *theme.Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Background(t.Surface).
		Padding(0, 1)
}

func panelBaseStyle(t *theme.Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Padding(0, 1).
		Background(t.Surface)
}

func titleStyle(t *theme.Theme) lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
}

func subtitleStyle(t *theme.Theme) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.TextSecondary)
}

func mutedStyle(t *theme.Theme) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.TextMuted)
}

func selectedStyle(t *theme.Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(t.TextPrimary).
		Background(t.SurfaceHigh)
}

func currentStyle(t *theme.Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Accent).Bold(true)
}

func navActiveStyle(t *theme.Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Accent).Bold(true)
}

func statusStyle(t *theme.Theme) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.TextPrimary).
		Background(t.SurfaceHigh).
		Padding(0, 1)
}

func onStyle(t *theme.Theme) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Success).Bold(true)
}

func offStyle(t *theme.Theme) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.TextMuted)
}

// ── Semantic style helpers ────────────────────────────────────────────

// getDelayStyle 按延迟返回色阶样式：<50 绿 / <150 黄 / <300 橙 / ≥300 红 / -1 红(timeout) / 其余≤0 灰。
func getDelayStyle(t *theme.Theme, ms int) lipgloss.Style {
	switch {
	case ms <= 0:
		if ms == -1 {
			return lipgloss.NewStyle().Foreground(t.Danger)
		}
		return mutedStyle(t)
	case ms < 50:
		return lipgloss.NewStyle().Foreground(t.Success)
	case ms < 150:
		return lipgloss.NewStyle().Foreground(t.Warning)
	case ms < 300:
		return lipgloss.NewStyle().Foreground(t.Orange)
	default:
		return lipgloss.NewStyle().Foreground(t.Danger)
	}
}

// getModeStyle 返回代理模式标签的颜色样式。
func getModeStyle(t *theme.Theme, mode string) lipgloss.Style {
	switch mode {
	case "rule":
		return lipgloss.NewStyle().Foreground(t.Success)
	case "global":
		return lipgloss.NewStyle().Foreground(t.Warning)
	case "direct":
		return lipgloss.NewStyle().Foreground(t.Info)
	default:
		return mutedStyle(t)
	}
}
