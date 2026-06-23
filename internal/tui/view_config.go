package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	appconfig "labproxy/internal/config"
	"labproxy/internal/proxy"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type settingAction int

const (
	settingCycleMode settingAction = iota
	settingToggleSystemProxy
	settingToggleAllowLan
	settingToggleTun
	settingRestart
)

type settingItem struct {
	Label    string
	Value    string // styled display value for UI rendering
	RawValue bool   // plain bool for data comparison (toggle items only)
	Hint     string
	Action   settingAction
}

// renderConfigView 渲染 Config 视图：复用 settingsItems + visibleSettingRows。
func (m model) renderConfigView(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	panelStyle := panelBaseStyle.Width(width).Height(height)
	innerWidth := max(0, width-panelStyle.GetHorizontalFrameSize()-4)

	title := titleStyle.Render(T().SettingsTitle)
	subtitle := mutedStyle.Render(T().SettingsHint)

	rows := m.visibleSettingRows(innerWidth, 5)
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		subtitle,
		"",
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	)

	return panelStyle.Render(content)
}

func (m model) settingsItems() []settingItem {
	restartHint := T().HintRestartShell
	if strings.TrimSpace(m.restartCommand) != "" {
		restartHint = T().HintRestartMixin
	}
	return []settingItem{
		{Label: T().SettingLabelMode, Value: fallback(m.mode, "rule"), RawValue: false, Hint: T().HintCycle, Action: settingCycleMode},
		{Label: T().SettingLabelSysProxy, Value: boolLabel(m.systemProxyEnabled), RawValue: m.systemProxyEnabled, Hint: T().HintNewShells, Action: settingToggleSystemProxy},
		{Label: T().SettingLabelAllowLan, Value: boolLabel(m.allowLanEnabled), RawValue: m.allowLanEnabled, Hint: T().HintRestart, Action: settingToggleAllowLan},
		{Label: T().SettingLabelTun, Value: boolLabel(m.tunEnabled), RawValue: m.tunEnabled, Hint: T().HintRestart, Action: settingToggleTun},
		{Label: T().SettingLabelRestart, Value: "", RawValue: false, Hint: restartHint, Action: settingRestart},
	}
}

func (m model) visibleSettingRows(width, limit int) []string {
	if limit <= 0 || width <= 0 {
		return nil
	}
	items := m.settingsItems()
	if len(items) == 0 {
		return []string{fitLine(mutedStyle.Render("  "+T().NoSettingsAvailable), width)}
	}
	start, end := window(m.settingsIndex, len(items), limit)
	rows := make([]string, 0, end-start)
	for idx := start; idx < end; idx++ {
		item := items[idx]
		isSelected := idx == m.settingsIndex
		prefix := "  "
		if isSelected {
			prefix = "▸ "
		}

		baseStyle := lipgloss.NewStyle()
		if isSelected {
			baseStyle = selectedStyle
		}

		var valueStyle lipgloss.Style
		var valueStrPlain string
		switch item.Action {
		case settingCycleMode:
			valueStrPlain = strings.ToLower(strings.TrimSpace(item.Value))
			valueStyle = getModeStyle(valueStrPlain)
		case settingToggleSystemProxy, settingToggleAllowLan, settingToggleTun:
			isOn := item.RawValue
			valueStrPlain = T().BoolOff
			valueStyle = offStyle
			if isOn {
				valueStrPlain = T().BoolOn
				valueStyle = onStyle
			}
		case settingRestart:
			valueStrPlain = T().ValueRestart
			valueStyle = lipgloss.NewStyle().Foreground(colorInfo).Bold(true)
		default:
			valueStrPlain = item.Value
			valueStyle = mutedStyle
		}

		if isSelected {
			valueStyle = valueStyle.Inherit(selectedStyle).Foreground(valueStyle.GetForeground())
		}

		hintPart := ""
		hintWidth := 0
		if isSelected && item.Hint != "" {
			hintPart = "  " + item.Hint
			hintWidth = ansi.StringWidth(hintPart)
		}

		reserved := ansi.StringWidth(prefix) + 2 + ansi.StringWidth(valueStrPlain) + hintWidth
		labelWidth := width - reserved
		if labelWidth < 4 {
			labelWidth = 4
		}
		truncatedLabel := ansi.Truncate(item.Label, labelWidth, "…")

		line := baseStyle.Render(prefix+truncatedLabel+"  ") +
			valueStyle.Render(valueStrPlain)

		if hintPart != "" {
			hintStyle := mutedStyle
			if isSelected {
				hintStyle = hintStyle.Inherit(selectedStyle).Foreground(colorTextMuted)
			}
			line += hintStyle.Render(hintPart)
		}

		rows = append(rows, fitStyledLine(line, width, baseStyle))
	}
	return rows
}

func (m model) activateSettingCmd() tea.Cmd {
	items := m.settingsItems()
	if len(items) == 0 || m.settingsIndex >= len(items) {
		return nil
	}

	switch items[m.settingsIndex].Action {
	case settingCycleMode:
		return m.cycleModeCmd()
	case settingToggleSystemProxy:
		return m.toggleSystemProxyCmd()
	case settingToggleAllowLan:
		return m.toggleAllowLanCmd()
	case settingToggleTun:
		return m.toggleTunCmd()
	case settingRestart:
		return m.restartRuntimeCmd()
	default:
		return nil
	}
}

func (m model) cycleModeCmd() tea.Cmd {
	next := nextMode(m.mode)
	return func() tea.Msg {
		persistErr := appconfig.WriteMode(m.mixinConfigPath, next)

		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		liveErr := m.client.UpdateMode(ctx, next)
		state, err := m.fetchState(ctx)
		if err != nil {
			if liveErr != nil {
				return errMsg{fmt.Errorf("update mode failed: %v; refresh failed: %w", liveErr, err)}
			}
			return errMsg{err}
		}

		status := fmt.Sprintf(T().ModeToFmt, fallback(state.config.Mode, next))
		switch {
		case persistErr == nil && liveErr == nil:
		case persistErr != nil && liveErr == nil:
			status += fmt.Sprintf(T().ModeSaveFailedFmt, persistErr)
		case persistErr == nil && liveErr != nil:
			status = fmt.Sprintf(T().ModeApplyFailedFmt, next, liveErr)
		default:
			return errMsg{fmt.Errorf("save mode: %v; live apply: %v", persistErr, liveErr)}
		}

		return settingsResultMsg{status: status, data: state}
	}
}

func (m model) toggleSystemProxyCmd() tea.Cmd {
	next := !m.systemProxyEnabled
	return func() tea.Msg {
		if err := appconfig.WriteSystemProxyEnabled(m.mixinConfigPath, next); err != nil {
			return errMsg{err}
		}
		sysEnabled, err := appconfig.ReadSystemProxyEnabled(m.mixinConfigPath)
		if err != nil {
			return errMsg{err}
		}
		return configFlagsMsg{
			status:             fmt.Sprintf(T().SysProxyPrefFmt, boolLabel(next)),
			systemProxyEnabled: sysEnabled,
			allowLanEnabled:    m.allowLanEnabled,
			tunEnabled:         m.tunEnabled,
		}
	}
}

func (m model) toggleAllowLanCmd() tea.Cmd {
	next := !m.allowLanEnabled
	return func() tea.Msg {
		if err := appconfig.WriteAllowLanEnabled(m.mixinConfigPath, next); err != nil {
			return errMsg{err}
		}
		lanEnabled, err := appconfig.ReadAllowLanEnabled(m.mixinConfigPath)
		if err != nil {
			return errMsg{err}
		}
		return configFlagsMsg{
			status:             fmt.Sprintf(T().AllowLanPrefFmt, boolLabel(next)),
			systemProxyEnabled: m.systemProxyEnabled,
			allowLanEnabled:    lanEnabled,
			tunEnabled:         m.tunEnabled,
		}
	}
}

func (m model) toggleTunCmd() tea.Cmd {
	next := !m.tunEnabled
	return func() tea.Msg {
		if err := appconfig.WriteTunEnabled(m.mixinConfigPath, next); err != nil {
			return errMsg{err}
		}
		tunEnabled, err := appconfig.ReadTunEnabled(m.mixinConfigPath)
		if err != nil {
			return errMsg{err}
		}
		return configFlagsMsg{
			status:             fmt.Sprintf(T().TunPrefFmt, boolLabel(next)),
			systemProxyEnabled: m.systemProxyEnabled,
			allowLanEnabled:    m.allowLanEnabled,
			tunEnabled:         tunEnabled,
		}
	}
}

// validateRestartCommand rejects shell metacharacters that could enable
// command injection through the user-supplied restart command. It allows
// && (logical AND used for chaining source + command) but rejects
// standalone & (background operator).
func validateRestartCommand(cmd string) error {
	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]
		switch ch {
		case ';', '|', '$', '`', '(', ')', '<', '>', '\n', '\r':
			return fmt.Errorf(T().RestartValidateErrFmt, ch)
		case '&':
			if i+1 < len(cmd) && cmd[i+1] == '&' {
				i++ // skip second &
				continue
			}
			return fmt.Errorf(T().RestartValidateErrFmt, ch)
		}
	}
	return nil
}

func (m model) restartRuntimeCmd() tea.Cmd {
	if strings.TrimSpace(m.restartCommand) == "" {
		return func() tea.Msg {
			state, err := m.refreshSettingsOnly()
			if err != nil {
				return errMsg{err}
			}
			return settingsResultMsg{
				status: T().RestartUnavailable,
				data:   state,
			}
		}
	}

	if err := validateRestartCommand(m.restartCommand); err != nil {
		return func() tea.Msg {
			return errMsg{err}
		}
	}

	return func() tea.Msg {
		cmd := exec.Command("bash", "-lc", m.restartCommand)
		output, err := cmd.CombinedOutput()
		if err != nil {
			message := strings.TrimSpace(string(output))
			if message != "" {
				return errMsg{fmt.Errorf("restart failed: %w: %s", err, message)}
			}
			return errMsg{fmt.Errorf("restart failed: %w", err)}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		state, refreshErr := m.fetchState(ctx)
		if refreshErr != nil {
			return errMsg{fmt.Errorf("restart succeeded but refresh failed: %w", refreshErr)}
		}
		return settingsResultMsg{status: T().RuntimeRestarted, data: state}
	}
}

func (m model) refreshSettingsOnly() (refreshMsg, error) {
	state := refreshMsg{
		version:     proxy.Version{Version: m.version},
		config:      proxy.Config{Mode: m.mode},
		traffic:     proxy.Traffic{Up: m.up, Down: m.down},
		proxies:     m.rawProxies,
		connections: m.connections,
	}
	var err error
	state.systemProxyEnabled, err = appconfig.ReadSystemProxyEnabled(m.mixinConfigPath)
	if err != nil {
		return refreshMsg{}, err
	}
	state.allowLanEnabled, err = appconfig.ReadAllowLanEnabled(m.mixinConfigPath)
	if err != nil {
		return refreshMsg{}, err
	}
	state.tunEnabled, err = appconfig.ReadTunEnabled(m.mixinConfigPath)
	if err != nil {
		return refreshMsg{}, err
	}
	return state, nil
}

func getModeStyle(mode string) lipgloss.Style {
	switch mode {
	case "rule":
		return lipgloss.NewStyle().Foreground(colorSuccess)
	case "global":
		return lipgloss.NewStyle().Foreground(colorWarning)
	case "direct":
		return lipgloss.NewStyle().Foreground(colorInfo)
	default:
		return mutedStyle
	}
}
