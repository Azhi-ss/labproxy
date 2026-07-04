package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// 连接表格列宽（视觉列，非字节）。
const (
	connColTarget = 28 // 主机/目标
	connColRule   = 14 // 规则名
	connColChain  = 18 // 代理链
	connColDown   = 10 // 下载（右对齐）
	connColUp     = 10 // 上传（右对齐）
	connColGap    = 2  // 列间距
)

// renderConnectionsView 渲染 Connections 视图：占满主区的连接表。
func (m model) renderConnectionsView(width, height int) string {
	m2 := m
	m2.focus = focusConnections
	return m2.renderConnectionsPanel(width, height)
}

// clampConnIndex 限制连接选中行在有效范围。
func (m *model) clampConnIndex() {
	n := len(m.connections.Connections)
	if n == 0 {
		m.connIndex = 0
		return
	}
	if m.connIndex < 0 {
		m.connIndex = 0
	}
	if m.connIndex >= n {
		m.connIndex = n - 1
	}
}

// handleConnectionCloseKey 处理连接面板的 d/D 断连按键。
// 返回 (handled, model, cmd)；handled=false 表示未处理，交回主 Update。
// 交互：按 d/D 设待确认态并提示，再次按相同键确认执行，其它键取消。
func (m model) handleConnectionCloseKey(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	if msg.Type != tea.KeyRunes {
		return false, m, nil
	}
	runes := string(msg.Runes)
	isClose := runes == "d"
	isCloseAll := runes == "D"

	// 已有待确认态
	if m.connConfirmClose != "" {
		// 再次按对应键确认
		confirmMatch := (m.connConfirmClose == "all" && isCloseAll) ||
			(m.connConfirmClose != "all" && isClose)
		if confirmMatch {
			target := m.connConfirmClose
			m.connConfirmClose = ""
			return true, m, m.closeConnectionCmd(target)
		}
		// 其它任意键取消
		m.connConfirmClose = ""
		m.statusLine = T().FocusConnections
		return true, m, nil
	}

	// 首次按 d/D 进入待确认
	if isCloseAll {
		m.connConfirmClose = "all"
		m.statusLine = T().ConnCloseAllConfirm
		return true, m, nil
	}
	if isClose {
		conns := m.connections.Connections
		if len(conns) == 0 {
			return true, m, nil
		}
		m.clampConnIndex()
		target := conns[m.connIndex].ID
		m.connConfirmClose = target
		m.statusLine = fmt.Sprintf(T().ConnCloseConfirmFmt, target)
		return true, m, nil
	}
	return false, m, nil
}

func (m model) closeConnectionCmd(target string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		var closeErr error
		if target == "all" {
			closeErr = m.client.CloseAllConnections(ctx)
		} else {
			closeErr = m.client.CloseConnection(ctx, target)
		}
		if closeErr != nil {
			return errMsg{fmt.Errorf("close connection %s: %w", target, closeErr)}
		}

		state, err := m.fetchState(ctx)
		if err != nil {
			return errMsg{err}
		}
		label := target
		if target == "all" {
			label = T().ConnCloseAllLabel
		}
		return switchResultMsg{
			status: fmt.Sprintf(T().ConnClosedFmt, label),
			data:   state,
		}
	}
}

func (m model) renderConnectionsPanel(width, height int) string {
	t := m.theme
	subtitle := fmt.Sprintf(T().ConnectionStatsFmt, len(m.connections.Connections), formatSize(m.connections.DownloadTotal), formatSize(m.connections.UploadTotal))
	content := renderPanelContent(
		t,
		T().PanelConnections,
		subtitle,
		m.visibleConnectionRows(width, max(0, height)),
		width,
		height,
	)
	return renderPanel(panelBaseStyle(t), width, height, content)
}

func (m model) visibleConnectionRows(width, limit int) []string {
	if limit <= 0 || width <= 0 {
		return nil
	}
	t := m.theme
	connections := m.connections.Connections
	if len(connections) == 0 {
		return []string{fitLine(mutedStyle(t).Render("  "+T().NoActiveConnections), width)}
	}

	// 表头
	header := connColumn(
		mutedStyle(t), "Host", connColTarget,
		mutedStyle(t), "Rule", connColRule,
		mutedStyle(t), "Chain", connColChain,
		mutedStyle(t), "↓Down", connColDown,
		mutedStyle(t), "↑Up", connColUp,
	)

	rows := make([]string, 0, len(connections)+1)
	rows = append(rows, fitLine(header, width))

	for i, conn := range connections {
		if i >= limit-1 {
			break
		}
		isSelected := i == m.connIndex
		target := connectionTarget(conn)
		rule := conn.Rule
		chain := strings.Join(conn.Chains, " → ")
		down := formatSize(conn.Download)
		up := formatSize(conn.Upload)

		baseStyle := lipgloss.NewStyle().Foreground(t.TextPrimary)
		if isSelected {
			baseStyle = selectedStyle(t)
		}

		line := connColumnStyled(
			baseStyle, target, connColTarget,
			mutedStyle(t), rule, connColRule,
			mutedStyle(t), chain, connColChain,
			mutedStyle(t), down, connColDown,
			mutedStyle(t), up, connColUp,
		)
		rows = append(rows, fitStyledLine(line, width, baseStyle))
	}
	return rows
}

// connColumn 构建固定宽度列行。
// 参数格式: style1, text1, width1, style2, text2, width2, ...
func connColumn(args ...any) string {
	var parts []string
	for i := 0; i < len(args); i += 3 {
		style := args[i].(lipgloss.Style)
		text := args[i+1].(string)
		colWidth := args[i+2].(int)
		truncated := ansi.Truncate(text, colWidth, "…")
		visLen := ansi.StringWidth(truncated)
		padding := colWidth - visLen
		if padding < 0 {
			padding = 0
		}
		parts = append(parts, style.Render(truncated+strings.Repeat(" ", padding)))
	}
	return strings.Join(parts, strings.Repeat(" ", connColGap))
}

// connColumnStyled 类似 connColumn，但第一列使用自定义样式。
func connColumnStyled(firstStyle lipgloss.Style, firstText string, firstWidth int, args ...any) string {
	truncated := ansi.Truncate(firstText, firstWidth, "…")
	visLen := ansi.StringWidth(truncated)
	padding := firstWidth - visLen
	if padding < 0 {
		padding = 0
	}
	first := firstStyle.Render(truncated + strings.Repeat(" ", padding))

	var rest []string
	for i := 0; i < len(args); i += 3 {
		style := args[i].(lipgloss.Style)
		text := args[i+1].(string)
		colWidth := args[i+2].(int)
		truncated := ansi.Truncate(text, colWidth, "…")
		visLen := ansi.StringWidth(truncated)
		padding := colWidth - visLen
		if padding < 0 {
			padding = 0
		}
		rest = append(rest, style.Render(truncated+strings.Repeat(" ", padding)))
	}
	return first + strings.Repeat(" ", connColGap) + strings.Join(rest, strings.Repeat(" ", connColGap))
}
