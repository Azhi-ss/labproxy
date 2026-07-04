package tui

import (
	"github.com/charmbracelet/lipgloss"
)

func (m model) renderFooter() string {
	t := m.theme
	docWidth := m.docWidth()
	if docWidth <= 0 {
		return ""
	}
	contentWidth := chromeContentWidth(t, docWidth)
	left := statusStyle(t).Render(fallback(m.statusLine, T().StatusReady))
	if m.searchMode {
		left = lipgloss.JoinHorizontal(lipgloss.Left, left, "  ", titleStyle(t).Render(T().SearchLabel), m.search.View())
	}
	right := mutedStyle(t).Render(m.footerKeyHint())
	row := alignLine(left, right, contentWidth)
	return renderChromeLine(t, row, docWidth)
}

func (m model) footerKeyHint() string {
	if currentLang == LangZh {
		switch m.activeView {
		case viewProxies:
			return "1-5视图  tab/←→焦点  j/k移动  enter进入/切换  /搜索  T测速  ?帮助  q退出"
		case viewConnections:
			return "1-5视图  j/k移动  d关闭  D全关  ?帮助  q退出"
		case viewLogs:
			return "1-5视图  l级别  ?帮助  q退出"
		case viewRules:
			return "1-5视图  a添加  enter编辑  d删除  ?帮助  q退出"
		case viewConfig:
			return "1-5视图  j/k移动  enter切换  ?帮助  q退出"
		}
		return ""
	}

	switch m.activeView {
	case viewProxies:
		return "1-5 view  tab focus  j/k move  enter use  /  T test  ?  q"
	case viewConnections:
		return "1-5 view  j/k move  d close  D all  ?  q"
	case viewLogs:
		return "1-5 view  l level  ?  q"
	case viewRules:
		return "1-5 view  a add  enter edit  d del  ?  q"
	case viewConfig:
		return "1-5 view  j/k move  enter toggle  ?  q"
	}
	return ""
}

func (m model) renderHelpOverlay() string {
	t := m.theme
	title := titleStyle(t).Render(T().HelpTitle)

	rows := []string{title, ""}
	if currentLang == LangZh {
		rows = append(rows, mutedStyle(t).Render("1-5  切换视图"))
		rows = append(rows, mutedStyle(t).Render("Tab/←→  代理页切换焦点；其它页 Tab 切视图"))
		rows = append(rows, mutedStyle(t).Render("j/k  移动光标"))
		rows = append(rows, mutedStyle(t).Render("r    刷新延迟"))
		rows = append(rows, mutedStyle(t).Render("q    退出"))
		rows = append(rows, mutedStyle(t).Render("?    切换帮助"))
		rows = append(rows, "")
		rows = append(rows, mutedStyle(t).Render("代理: / 搜索  T 测速  enter 进入节点/切换"))
		rows = append(rows, mutedStyle(t).Render("连接: d 关闭  D 全部关闭"))
		rows = append(rows, mutedStyle(t).Render("日志: l 切换级别"))
		rows = append(rows, mutedStyle(t).Render("配置: enter 切换设置  m 模式  p 系统代理"))
	} else {
		rows = append(rows, mutedStyle(t).Render("1-5  switch view"))
		rows = append(rows, mutedStyle(t).Render("Tab/←→  focus panes on proxies; Tab changes views elsewhere"))
		rows = append(rows, mutedStyle(t).Render("j/k  move cursor"))
		rows = append(rows, mutedStyle(t).Render("r    refresh"))
		rows = append(rows, mutedStyle(t).Render("q    quit"))
		rows = append(rows, mutedStyle(t).Render("?    toggle help"))
		rows = append(rows, "")
		rows = append(rows, mutedStyle(t).Render("Proxies: / search  T test  enter focus/switch"))
		rows = append(rows, mutedStyle(t).Render("Conns:   d close  D close all"))
		rows = append(rows, mutedStyle(t).Render("Logs:    l level"))
		rows = append(rows, mutedStyle(t).Render("Config:  enter toggle  m mode  p proxy"))
	}
	rows = append(rows, "")

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		panelBaseStyle(t).Padding(1, 2).Render(content),
	)
}
