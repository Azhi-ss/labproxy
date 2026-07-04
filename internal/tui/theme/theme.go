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

// Light 是浅色终端背景主题，使用偏中性的 slate/blue palette，避免大面积淡紫选中态。
var Light = &Theme{
	Bg:            lipgloss.AdaptiveColor{Light: "#F8FAFC", Dark: "#0F172A"},
	Surface:       lipgloss.AdaptiveColor{Light: "#F1F5F9", Dark: "#111827"},
	SurfaceHigh:   lipgloss.AdaptiveColor{Light: "#DBEAFE", Dark: "#1E3A5F"},
	TextPrimary:   lipgloss.AdaptiveColor{Light: "#0F172A", Dark: "#E5E7EB"},
	TextSecondary: lipgloss.AdaptiveColor{Light: "#334155", Dark: "#CBD5E1"},
	TextMuted:     lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#94A3B8"},
	Accent:        lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#60A5FA"},
	Success:       lipgloss.AdaptiveColor{Light: "#047857", Dark: "#34D399"},
	Warning:       lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"},
	Danger:        lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"},
	Orange:        lipgloss.AdaptiveColor{Light: "#C2410C", Dark: "#FB923C"},
	Info:          lipgloss.AdaptiveColor{Light: "#0284C7", Dark: "#38BDF8"},
}

// Dark 是深色终端背景主题（显式指定时使用）。
var Dark = &Theme{
	Bg:            lipgloss.AdaptiveColor{Light: "#0F172A", Dark: "#0F172A"},
	Surface:       lipgloss.AdaptiveColor{Light: "#111827", Dark: "#111827"},
	SurfaceHigh:   lipgloss.AdaptiveColor{Light: "#1E3A5F", Dark: "#1E3A5F"},
	TextPrimary:   lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#E5E7EB"},
	TextSecondary: lipgloss.AdaptiveColor{Light: "#CBD5E1", Dark: "#CBD5E1"},
	TextMuted:     lipgloss.AdaptiveColor{Light: "#94A3B8", Dark: "#94A3B8"},
	Accent:        lipgloss.AdaptiveColor{Light: "#60A5FA", Dark: "#60A5FA"},
	Success:       lipgloss.AdaptiveColor{Light: "#34D399", Dark: "#34D399"},
	Warning:       lipgloss.AdaptiveColor{Light: "#FBBF24", Dark: "#FBBF24"},
	Danger:        lipgloss.AdaptiveColor{Light: "#F87171", Dark: "#F87171"},
	Orange:        lipgloss.AdaptiveColor{Light: "#FB923C", Dark: "#FB923C"},
	Info:          lipgloss.AdaptiveColor{Light: "#38BDF8", Dark: "#38BDF8"},
}

// Current 返回当前终端背景对应的主题。
// lipgloss.HasDarkBackground() 在 Bubble Tea init 时自动检测并缓存。
func Current() *Theme {
	if lipgloss.HasDarkBackground() {
		return Dark
	}
	return Light
}
