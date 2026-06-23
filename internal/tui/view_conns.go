package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
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
		m.statusLine = T().ConnCloseAllLabel + " — press D again to confirm"
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
		m.statusLine = target + " — press d again to confirm"
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
	subtitle := fmt.Sprintf(T().ConnectionStatsFmt, len(m.connections.Connections), formatSize(m.connections.DownloadTotal), formatSize(m.connections.UploadTotal))
	content := renderPanelContent(
		T().PanelConnections,
		subtitle,
		m.visibleConnectionRows(width, max(0, height)),
		width,
		height,
	)
	return renderPanel(inactivePanelStyle, width, height, content)
}

func (m model) visibleConnectionRows(width, limit int) []string {
	if limit <= 0 || width <= 0 {
		return nil
	}
	connections := m.connections.Connections
	if len(connections) == 0 {
		return []string{fitLine(mutedStyle.Render("  "+T().NoActiveConnections), width)}
	}
	if len(connections) > limit {
		connections = connections[:limit]
	}
	rows := make([]string, 0, len(connections))
	for _, conn := range connections {
		line := fmt.Sprintf(" %s  %s  %s  ↓%s ↑%s", connectionTarget(conn), mutedStyle.Render(conn.Rule), strings.Join(conn.Chains, " → "), formatSize(conn.Download), formatSize(conn.Upload))
		line = ansi.Truncate(line, width, "…")
		rows = append(rows, fitLine(line, width))
	}
	return rows
}
