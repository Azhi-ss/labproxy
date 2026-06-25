// Package theme 提供双模式（浅色/深色）颜色 token，通过 lipgloss.AdaptiveColor 自动适配终端背景。
// lipgloss.HasDarkBackground() 在 Bubble Tea init 时自动检测并缓存，AdaptiveColor 在渲染时解析。
package theme

import "github.com/charmbracelet/lipgloss"

// Theme 包含所有语义颜色 token。两个预构建实例：Light 和 Dark。
type Theme struct {
	// 背景色
	Bg          lipgloss.AdaptiveColor
	Surface     lipgloss.AdaptiveColor
	SurfaceHigh lipgloss.AdaptiveColor

	// 文字色
	TextPrimary   lipgloss.AdaptiveColor
	TextSecondary lipgloss.AdaptiveColor
	TextMuted     lipgloss.AdaptiveColor

	// 语义色
	Accent  lipgloss.AdaptiveColor
	Success lipgloss.AdaptiveColor
	Warning lipgloss.AdaptiveColor
	Danger  lipgloss.AdaptiveColor
	Orange  lipgloss.AdaptiveColor
	Info    lipgloss.AdaptiveColor
}

// Light 是浅色终端背景主题。
// 所有文字色在 #EEEEEE 背景上 WCAG AA 对比度 ≥ 4.5:1（TextMuted ≥ 4.5:1）。
var Light = &Theme{
	Bg:            lipgloss.AdaptiveColor{Light: "255", Dark: "234"},
	Surface:       lipgloss.AdaptiveColor{Light: "254", Dark: "235"},
	SurfaceHigh:   lipgloss.AdaptiveColor{Light: "189", Dark: "236"},
	TextPrimary:   lipgloss.AdaptiveColor{Light: "234", Dark: "255"},
	TextSecondary: lipgloss.AdaptiveColor{Light: "239", Dark: "145"},
	TextMuted:     lipgloss.AdaptiveColor{Light: "240", Dark: "242"},
	Accent:        lipgloss.AdaptiveColor{Light: "25", Dark: "75"},
	Success:       lipgloss.AdaptiveColor{Light: "22", Dark: "42"},
	Warning:       lipgloss.AdaptiveColor{Light: "130", Dark: "220"},
	Danger:        lipgloss.AdaptiveColor{Light: "160", Dark: "203"},
	Orange:        lipgloss.AdaptiveColor{Light: "166", Dark: "214"},
	Info:          lipgloss.AdaptiveColor{Light: "25", Dark: "75"},
}

// Dark 是深色终端背景主题（显式指定时使用）。
var Dark = &Theme{
	Bg:            lipgloss.AdaptiveColor{Light: "234", Dark: "234"},
	Surface:       lipgloss.AdaptiveColor{Light: "235", Dark: "235"},
	SurfaceHigh:   lipgloss.AdaptiveColor{Light: "236", Dark: "236"},
	TextPrimary:   lipgloss.AdaptiveColor{Light: "255", Dark: "255"},
	TextSecondary: lipgloss.AdaptiveColor{Light: "145", Dark: "145"},
	TextMuted:     lipgloss.AdaptiveColor{Light: "242", Dark: "242"},
	Accent:        lipgloss.AdaptiveColor{Light: "75", Dark: "75"},
	Success:       lipgloss.AdaptiveColor{Light: "42", Dark: "42"},
	Warning:       lipgloss.AdaptiveColor{Light: "220", Dark: "220"},
	Danger:        lipgloss.AdaptiveColor{Light: "203", Dark: "203"},
	Orange:        lipgloss.AdaptiveColor{Light: "214", Dark: "214"},
	Info:          lipgloss.AdaptiveColor{Light: "75", Dark: "75"},
}

// Current 返回当前终端背景对应的主题。
// lipgloss.HasDarkBackground() 在 Bubble Tea init 时自动检测并缓存。
func Current() *Theme {
	if lipgloss.HasDarkBackground() {
		return Dark
	}
	return Light
}
