package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// viewID 标识五个常驻视图，对应 Clash dashboard 的核心页。
type viewID int

const (
	viewProxies viewID = iota
	viewConnections
	viewLogs
	viewRules
	viewConfig
)

var viewOrder = []viewID{viewProxies, viewConnections, viewLogs, viewRules, viewConfig}

// next 返回下一个视图（循环）。
func (v viewID) next() viewID {
	for i, x := range viewOrder {
		if x == v {
			return viewOrder[(i+1)%len(viewOrder)]
		}
	}
	return viewOrder[0]
}

// label 返回视图在导航栏与标题中的显示名（走 i18n）。
func (v viewID) label() string {
	switch v {
	case viewProxies:
		return T().NavProxies
	case viewConnections:
		return T().NavConnections
	case viewLogs:
		return T().NavLogs
	case viewRules:
		return T().NavRules
	case viewConfig:
		return T().NavConfig
	}
	return ""
}

// shortKey 是导航栏显示的单字符快捷键。
func (v viewID) shortKey() string {
	switch v {
	case viewProxies:
		return "1"
	case viewConnections:
		return "2"
	case viewLogs:
		return "3"
	case viewRules:
		return "4"
	case viewConfig:
		return "5"
	}
	return ""
}

// viewByDigit 将 "1".."5" 映射到视图，超出范围返回 ok=false。
func viewByDigit(digit string) (viewID, bool) {
	for _, v := range viewOrder {
		if v.shortKey() == digit {
			return v, true
		}
	}
	return viewProxies, false
}

// renderNav 渲染左侧窄导航栏：每项显示 [digit] label，当前项加 ▌ 前缀与高亮。
func renderNav(active viewID, height int) string {
	rows := make([]string, 0, len(viewOrder))
	for _, v := range viewOrder {
		marker := " "
		style := mutedStyle
		if v == active {
			marker = "▌"
			style = navActiveStyle
		}
		rows = append(rows, style.Render(fmt.Sprintf("%s%s %s", marker, v.shortKey(), v.label())))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return panelBaseStyle.
		BorderForeground(colorPrimary).
		Width(14).
		Height(height).
		Render(content)
}
