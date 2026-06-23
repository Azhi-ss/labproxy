package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
)

// logsCmd 订阅 mihomo /logs 流，阻塞读一条返回 logEntryMsg。
// ctx 由调用方创建并存入 model.logCancel，确保退出时可取消。
// 持续流靠 Update 收到 logEntryMsg 后再次调度 logsCmd（复用同一 ctx，不新建流）。
func (m model) logsCmd(ctx context.Context) tea.Cmd {
	level := m.logLevel
	if level == "" {
		level = "info"
	}
	ch := m.client.Logs(ctx, level)

	return func() tea.Msg {
		entry, ok := <-ch
		if !ok {
			return nil
		}
		return logEntryMsg{entry: entry}
	}
}

// handleLogKey 处理日志视图按键：esc 返回代理视图，l 循环切换级别。
// 切换级别会清空已累积日志并重新订阅。
func (m model) handleLogKey(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	// esc 返回代理视图
	if msg.Type == tea.KeyEsc {
		m.stopLogStream()
		m.activeView = viewProxies
		m.statusLine = T().LogOverlayClosed
		return true, m, nil
	}
	// l 切换级别
	if msg.Type == tea.KeyRunes && string(msg.Runes) == "l" {
		m.stopLogStream() // 停止旧流
		m.logLevel = nextLogLevel(m.logLevel)
		m.logEntries = nil
		// 新建 ctx+cancel 存入 model
		ctx, cancel := context.WithCancel(context.Background())
		m.logCancel = cancel
		m.logCtx = ctx
		m.logActive = true
		m.statusLine = fmt.Sprintf(T().LogLevelFmt, m.logLevel)
		return true, m, m.logsCmd(ctx)
	}
	return false, m, nil
}

// stopLogStream 停止当前日志订阅并重置 active 标志。
func (m *model) stopLogStream() {
	m.logActive = false
	if m.logCancel != nil {
		m.logCancel()
		m.logCancel = nil
	}
}

// nextLogLevel 在 logLevels 中循环到下一个级别。
func nextLogLevel(current string) string {
	if current == "" {
		current = "info"
	}
	for i, l := range logLevels {
		if l == current {
			if i+1 < len(logLevels) {
				return logLevels[i+1]
			}
			return logLevels[0]
		}
	}
	return logLevels[0]
}

// renderLogsView 渲染 Logs 视图：全屏日志列表 + 级别指示。
// 复用原 renderLogOverlay 的着色与截断逻辑。
func (m model) renderLogsView(width, height int) string {
	w := max(1, width)
	h := max(1, height)

	header := fmt.Sprintf(T().LogOverlayTitle, m.logLevel)
	body := lipgloss.JoinVertical(lipgloss.Left, header, "")
	usedHeight := lipgloss.Height(body) + 2 // 标题+空行
	avail := h - usedHeight
	if avail < 1 {
		avail = 1
	}

	// 取最近 avail 条
	entries := m.logEntries
	if len(entries) > avail {
		entries = entries[len(entries)-avail:]
	}

	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		lines = append(lines, fitLine(fmt.Sprintf("[%s] %s", e.Level, e.Payload), w))
	}
	if len(lines) == 0 {
		lines = append(lines, fitLine(mutedStyle.Render(T().LogWaiting), w))
	}

	content := strings.Join(lines, "\n")
	return lipgloss.JoinVertical(lipgloss.Left, header, "", content)
}
