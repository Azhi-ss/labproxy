package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	appconfig "labproxy/internal/config"
	"labproxy/internal/proxy"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Options struct {
	Endpoint           string
	SystemProxyEnabled bool
	MixinConfigPath    string
	RestartCommand     string
}

type App struct {
	client *proxy.Client
	opts   Options
}

func NewApp(client *proxy.Client, opts Options) *App {
	return &App{client: client, opts: opts}
}

func (a *App) Run(ctx context.Context) error {
	m := newModel(a.client, a.opts)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

type paneFocus int

const (
	focusGroups paneFocus = iota
	focusOptions
	focusSettings
)

type keyMap struct {
	Up          key.Binding
	Down        key.Binding
	Left        key.Binding
	Right       key.Binding
	Tab         key.Binding
	Select      key.Binding
	Refresh     key.Binding
	Search      key.Binding
	Settings    key.Binding
	Mode        key.Binding
	SystemProxy key.Binding
	Back        key.Binding
	Quit        key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Tab, k.Select, k.Refresh, k.Settings, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Tab, k.Select, k.Refresh, k.Search, k.Settings, k.Mode, k.SystemProxy, k.Back, k.Quit},
	}
}

type refreshMsg struct {
	version            proxy.Version
	config             proxy.Config
	traffic            proxy.Traffic
	proxies            proxy.ProxiesResponse
	connections        proxy.ConnectionsResponse
	systemProxyEnabled bool
	allowLanEnabled    bool
	tunEnabled         bool
}

type tickMsg time.Time

type statusMsg struct{ text string }

type errMsg struct{ err error }

type switchResultMsg struct {
	status string
	data   refreshMsg
}

type settingsResultMsg struct {
	status string
	data   refreshMsg
}

type model struct {
	client             *proxy.Client
	endpoint           string
	mixinConfigPath    string
	restartCommand     string
	systemProxyEnabled bool
	allowLanEnabled    bool
	tunEnabled         bool

	version string
	mode    string
	up      int64
	down    int64

	rawProxies    proxy.ProxiesResponse
	connections   proxy.ConnectionsResponse
	groups        []GroupView
	focus         paneFocus
	groupIndex    int
	optionIndex   int
	settingsIndex int

	width  int
	height int

	search     textinput.Model
	searchMode bool
	help       help.Model
	keys       keyMap
	statusLine string
	lastError  error
}

type settingAction int

const (
	settingCycleMode settingAction = iota
	settingToggleSystemProxy
	settingToggleAllowLan
	settingToggleTun
	settingRestart
)

type settingItem struct {
	Label  string
	Value  string
	Hint   string
	Action settingAction
}

func newModel(client *proxy.Client, opts Options) model {
	search := textinput.New()
	search.Placeholder = "Search proxies or groups"
	search.CharLimit = 64
	search.Width = 28

	return model{
		client:             client,
		endpoint:           opts.Endpoint,
		mixinConfigPath:    opts.MixinConfigPath,
		restartCommand:     opts.RestartCommand,
		systemProxyEnabled: opts.SystemProxyEnabled,
		focus:              focusGroups,
		width:              120,
		height:             32,
		search:             search,
		help:               help.New(),
		statusLine:         "connecting…",
		keys: keyMap{
			Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move up")),
			Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move down")),
			Left:        key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "focus left")),
			Right:       key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "focus right")),
			Tab:         key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane")),
			Select:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply/select")),
			Refresh:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh delay")),
			Search:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
			Settings:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "focus settings")),
			Mode:        key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "cycle mode")),
			SystemProxy: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "toggle proxy pref")),
			Back:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close search")),
			Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		},
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.refreshCmd(), tickCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(1, msg.Width)
		m.height = max(1, msg.Height)
		m.search.Width = min(28, max(12, m.width/4))
		return m, nil
	case tickMsg:
		return m, tea.Batch(m.refreshCmd(), tickCmd())
	case refreshMsg:
		m.applyState(msg)
		if m.lastError == nil {
			m.statusLine = "connected"
		}
		return m, nil
	case statusMsg:
		m.statusLine = msg.text
		m.lastError = nil
		return m, nil
	case switchResultMsg:
		m.applyState(msg.data)
		m.statusLine = msg.status
		m.lastError = nil
		return m, nil
	case settingsResultMsg:
		m.applyState(msg.data)
		m.statusLine = msg.status
		m.lastError = nil
		return m, nil
	case errMsg:
		m.lastError = msg.err
		m.statusLine = msg.err.Error()
		return m, nil
	case tea.KeyMsg:
		if m.searchMode {
			switch {
			case key.Matches(msg, m.keys.Quit), key.Matches(msg, m.keys.Back):
				m.searchMode = false
				m.search.Blur()
				m.rebuildGroups()
				m.statusLine = "search cancelled"
				return m, nil
			case key.Matches(msg, m.keys.Select):
				m.searchMode = false
				m.search.Blur()
				m.rebuildGroups()
				m.statusLine = fmt.Sprintf("filter: %s", fallback(m.search.Value(), "none"))
				return m, nil
			default:
				var cmd tea.Cmd
				m.search, cmd = m.search.Update(msg)
				m.rebuildGroups()
				return m, cmd
			}
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Search):
			m.searchMode = true
			m.search.Focus()
			m.statusLine = "type to filter groups or proxies"
			return m, nil
		case key.Matches(msg, m.keys.Settings):
			m.focus = focusSettings
			m.statusLine = "focus: settings"
			return m, nil
		case key.Matches(msg, m.keys.Mode):
			return m, m.cycleModeCmd()
		case key.Matches(msg, m.keys.SystemProxy):
			return m, m.toggleSystemProxyCmd()
		case key.Matches(msg, m.keys.Tab):
			m.toggleFocus()
			return m, nil
		case key.Matches(msg, m.keys.Left):
			m.moveFocus(-1)
			return m, nil
		case key.Matches(msg, m.keys.Right):
			m.moveFocus(1)
			return m, nil
		case key.Matches(msg, m.keys.Up):
			m.move(-1)
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.move(1)
			return m, nil
		case key.Matches(msg, m.keys.Refresh):
			return m, m.delayRefreshCmd()
		case key.Matches(msg, m.keys.Select):
			switch m.focus {
			case focusGroups:
				m.focus = focusOptions
				m.statusLine = "focus: options"
				return m, nil
			case focusSettings:
				return m, m.activateSettingCmd()
			default:
				return m, m.switchProxyCmd()
			}
		}
	}

	return m, nil
}

func (m model) View() string {
	if m.width <= 0 {
		return "loading…"
	}

	header := m.renderHeader()
	footer := m.renderFooter()
	availableBodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	body := m.renderBody(availableBodyHeight)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m *model) applyState(state refreshMsg) {
	m.version = state.version.Version
	m.mode = state.config.Mode
	m.up = state.traffic.Up
	m.down = state.traffic.Down
	m.rawProxies = state.proxies
	m.connections = state.connections
	m.systemProxyEnabled = state.systemProxyEnabled
	m.allowLanEnabled = state.allowLanEnabled
	m.tunEnabled = state.tunEnabled
	m.rebuildGroups()
	m.clampIndices()
}

func (m *model) toggleFocus() {
	m.moveFocus(1)
}

func (m *model) moveFocus(delta int) {
	order := []paneFocus{focusGroups, focusOptions, focusSettings}
	current := 0
	for idx, focus := range order {
		if m.focus == focus {
			current = idx
			break
		}
	}
	current = (current + delta + len(order)) % len(order)
	m.focus = order[current]
	m.statusLine = fmt.Sprintf("focus: %s", m.focusLabel())
}

func (m *model) move(delta int) {
	switch m.focus {
	case focusGroups:
		if len(m.groups) == 0 {
			return
		}
		m.groupIndex += delta
	case focusOptions:
		if len(m.groups) == 0 {
			return
		}
		m.optionIndex += delta
	case focusSettings:
		m.settingsIndex += delta
	}
	m.clampIndices()
}

func (m *model) rebuildGroups() {
	currentGroup := ""
	currentOption := ""
	if group := m.currentGroup(); group != nil {
		currentGroup = group.Name
		if len(group.Options) > 0 && m.optionIndex < len(group.Options) {
			currentOption = group.Options[m.optionIndex].Name
		}
	}
	m.groups = BuildGroupViews(m.rawProxies, m.search.Value())
	if currentGroup != "" {
		for idx, group := range m.groups {
			if group.Name == currentGroup {
				m.groupIndex = idx
				break
			}
		}
	}
	m.clampIndices()
	if currentOption != "" {
		if group := m.currentGroup(); group != nil {
			for idx, option := range group.Options {
				if option.Name == currentOption {
					m.optionIndex = idx
					break
				}
			}
		}
	}
}

func (m *model) clampIndices() {
	settingsCount := len(m.settingsItems())
	if settingsCount == 0 {
		m.settingsIndex = 0
	} else {
		if m.settingsIndex < 0 {
			m.settingsIndex = 0
		}
		if m.settingsIndex >= settingsCount {
			m.settingsIndex = settingsCount - 1
		}
	}

	if len(m.groups) == 0 {
		m.groupIndex = 0
		m.optionIndex = 0
		return
	}
	if m.groupIndex < 0 {
		m.groupIndex = 0
	}
	if m.groupIndex >= len(m.groups) {
		m.groupIndex = len(m.groups) - 1
	}
	options := m.groups[m.groupIndex].Options
	if len(options) == 0 {
		m.optionIndex = 0
		return
	}
	if m.optionIndex < 0 {
		m.optionIndex = 0
	}
	if m.optionIndex >= len(options) {
		m.optionIndex = len(options) - 1
	}
}

func (m model) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		state, err := m.fetchState(ctx)
		if err != nil {
			return errMsg{err}
		}
		return state
	}
}

func (m model) fetchState(ctx context.Context) (refreshMsg, error) {
	version, err := m.client.Version(ctx)
	if err != nil {
		return refreshMsg{}, err
	}
	config, err := m.client.Config(ctx)
	if err != nil {
		return refreshMsg{}, err
	}
	traffic, err := m.client.Traffic(ctx)
	if err != nil {
		return refreshMsg{}, err
	}
	proxies, err := m.client.Proxies(ctx)
	if err != nil {
		return refreshMsg{}, err
	}
	connections, err := m.client.Connections(ctx)
	if err != nil {
		connections = proxy.ConnectionsResponse{}
	}
	systemProxyEnabled, err := appconfig.ReadSystemProxyEnabled(m.mixinConfigPath)
	if err != nil {
		return refreshMsg{}, err
	}
	allowLanEnabled, err := appconfig.ReadAllowLanEnabled(m.mixinConfigPath)
	if err != nil {
		return refreshMsg{}, err
	}
	tunEnabled, err := appconfig.ReadTunEnabled(m.mixinConfigPath)
	if err != nil {
		return refreshMsg{}, err
	}
	return refreshMsg{
		version:            version,
		config:             config,
		traffic:            traffic,
		proxies:            proxies,
		connections:        connections,
		systemProxyEnabled: systemProxyEnabled,
		allowLanEnabled:    allowLanEnabled,
		tunEnabled:         tunEnabled,
	}, nil
}

func (m model) delayRefreshCmd() tea.Cmd {
	group := m.currentGroup()
	if group == nil {
		return nil
	}
	optionNames := make([]string, 0, len(group.Options))
	for _, option := range group.Options {
		optionNames = append(optionNames, option.Name)
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		for _, optionName := range optionNames {
			_, _ = m.client.Delay(ctx, optionName, 5*time.Second)
		}

		state, err := m.fetchState(ctx)
		if err != nil {
			return errMsg{err}
		}
		return state
	}
}

func (m model) switchProxyCmd() tea.Cmd {
	group := m.currentGroup()
	if group == nil || len(group.Options) == 0 || m.optionIndex >= len(group.Options) {
		return nil
	}
	groupName := group.Name
	optionName := group.Options[m.optionIndex].Name

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := m.client.SwitchProxy(ctx, groupName, optionName); err != nil {
			return errMsg{err}
		}
		state, err := m.fetchState(ctx)
		if err != nil {
			return errMsg{err}
		}
		return switchResultMsg{
			status: fmt.Sprintf("switched %s → %s", groupName, optionName),
			data:   state,
		}
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

		status := fmt.Sprintf("mode → %s", fallback(state.config.Mode, next))
		switch {
		case persistErr == nil && liveErr == nil:
		case persistErr != nil && liveErr == nil:
			status += fmt.Sprintf(" (live ok, mixin save failed: %v)", persistErr)
		case persistErr == nil && liveErr != nil:
			status = fmt.Sprintf("mode saved as %s; live apply failed: %v", next, liveErr)
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
		state, err := m.refreshSettingsOnly()
		if err != nil {
			return errMsg{err}
		}
		return settingsResultMsg{
			status: fmt.Sprintf("system proxy pref → %s (applies to new shells / next start)", boolLabel(next)),
			data:   state,
		}
	}
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

func (m model) toggleAllowLanCmd() tea.Cmd {
	next := !m.allowLanEnabled
	return func() tea.Msg {
		if err := appconfig.WriteAllowLanEnabled(m.mixinConfigPath, next); err != nil {
			return errMsg{err}
		}
		state, err := m.refreshSettingsOnly()
		if err != nil {
			return errMsg{err}
		}
		return settingsResultMsg{
			status: fmt.Sprintf("allow-lan pref → %s (saved, restart to apply)", boolLabel(next)),
			data:   state,
		}
	}
}

func (m model) toggleTunCmd() tea.Cmd {
	next := !m.tunEnabled
	return func() tea.Msg {
		if err := appconfig.WriteTunEnabled(m.mixinConfigPath, next); err != nil {
			return errMsg{err}
		}
		state, err := m.refreshSettingsOnly()
		if err != nil {
			return errMsg{err}
		}
		return settingsResultMsg{
			status: fmt.Sprintf("tun pref → %s (saved, restart to apply)", boolLabel(next)),
			data:   state,
		}
	}
}

func (m model) restartRuntimeCmd() tea.Cmd {
	if strings.TrimSpace(m.restartCommand) == "" {
		return func() tea.Msg {
			state, err := m.refreshSettingsOnly()
			if err != nil {
				return errMsg{err}
			}
			return settingsResultMsg{
				status: "restart command unavailable; run labproxy restart in shell",
				data:   state,
			}
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
		return settingsResultMsg{status: "runtime restarted and settings reapplied", data: state}
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

func tickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) currentGroup() *GroupView {
	if len(m.groups) == 0 || m.groupIndex >= len(m.groups) {
		return nil
	}
	return &m.groups[m.groupIndex]
}

var (
	docStyle = lipgloss.NewStyle().
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("39")).
			Padding(0, 1)

	panelBaseStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)

	activePanelStyle   = panelBaseStyle.BorderForeground(lipgloss.Color("86"))
	inactivePanelStyle = panelBaseStyle.BorderForeground(lipgloss.Color("240"))

	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("62")).Padding(0, 1)
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62"))
	currentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("78")).Bold(true)
)

func (m model) renderHeader() string {
	docWidth := max(0, m.width-docStyle.GetHorizontalFrameSize())
	if docWidth <= 0 {
		return ""
	}
	innerWidth := max(0, docWidth-headerStyle.GetHorizontalFrameSize())
	titleRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		titleStyle.Render("Clash TUI"),
		"  ",
		subtitleStyle.Render("3-pane dashboard · s settings · enter apply"),
	)

	metaRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		statusPill("endpoint", fallback(m.endpoint, "-")),
		statusPill("mode", fallback(m.mode, "unknown")),
		statusPill("system proxy", boolLabel(m.systemProxyEnabled)),
		statusPill("allow-lan", boolLabel(m.allowLanEnabled)),
		statusPill("tun", boolLabel(m.tunEnabled)),
		statusPill("traffic ↑", formatBytes(m.up)),
		statusPill("traffic ↓", formatBytes(m.down)),
		statusPill("focus", m.focusLabel()),
	)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		fitLine(titleRow, innerWidth),
		"",
		fitLine(metaRow, innerWidth),
	)
	return docStyle.Width(docWidth).Render(headerStyle.Width(innerWidth).MaxWidth(docWidth).Render(content))
}

func statusPill(label, value string) string {
	pill := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1)
	return pill.Render(fmt.Sprintf("%s %s", mutedStyle.Render(label), value))
}

func (m model) renderBody(availableHeight int) string {
	docWidth := max(0, m.width-docStyle.GetHorizontalFrameSize())
	if availableHeight <= 0 || docWidth <= 0 {
		return docStyle.Width(docWidth).Render("")
	}

	const columnGap = 2
	const rowGap = 1

	panelFrameWidth := panelBaseStyle.GetHorizontalFrameSize()
	panelFrameHeight := panelBaseStyle.GetVerticalFrameSize()
	minTopTotalHeight := panelFrameHeight + 2
	minConnectionTotalHeight := panelFrameHeight + 2

	topTotalHeight := availableHeight
	connectionTotalHeight := 0
	if availableHeight >= minTopTotalHeight+rowGap+minConnectionTotalHeight {
		connectionTotalHeight = min(10, availableHeight/3)
		if connectionTotalHeight < minConnectionTotalHeight {
			connectionTotalHeight = minConnectionTotalHeight
		}
		candidateTopHeight := availableHeight - connectionTotalHeight - rowGap
		if candidateTopHeight >= minTopTotalHeight {
			topTotalHeight = candidateTopHeight
		} else {
			connectionTotalHeight = 0
		}
	}

	columnContentWidth := docWidth - columnGap*2 - panelFrameWidth*3
	if columnContentWidth < 0 {
		columnContentWidth = 0
	}
	leftWidth := columnContentWidth / 3
	middleWidth := columnContentWidth / 3
	rightWidth := columnContentWidth - leftWidth - middleWidth
	topContentHeight := max(0, topTotalHeight-panelFrameHeight)

	top := lipgloss.NewStyle().MaxWidth(docWidth).MaxHeight(topTotalHeight).Render(
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.renderGroupsPanel(leftWidth, topContentHeight),
			strings.Repeat(" ", columnGap),
			m.renderOptionsPanel(middleWidth, topContentHeight),
			strings.Repeat(" ", columnGap),
			m.renderSettingsPanel(rightWidth, topContentHeight),
		),
	)
	if connectionTotalHeight == 0 {
		return docStyle.Width(docWidth).Render(top)
	}

	connectionContentWidth := max(0, docWidth-panelFrameWidth)
	connectionContentHeight := max(0, connectionTotalHeight-panelFrameHeight)
	connections := m.renderConnectionsPanel(connectionContentWidth, connectionContentHeight)
	body := lipgloss.NewStyle().MaxWidth(docWidth).MaxHeight(availableHeight).Render(
		lipgloss.JoinVertical(lipgloss.Left, top, "", connections),
	)
	return docStyle.Width(docWidth).Render(body)
}

func (m model) renderGroupsPanel(width, height int) string {
	style := inactivePanelStyle
	if m.focus == focusGroups {
		style = activePanelStyle
	}
	content := renderPanelContent(
		"Groups",
		"Tab / ←→ to switch focus",
		m.visibleGroupRows(width, max(0, height)),
		width,
		height,
	)
	return renderPanel(style, width, height, content)
}

func (m model) renderOptionsPanel(width, height int) string {
	style := inactivePanelStyle
	if m.focus == focusOptions {
		style = activePanelStyle
	}
	group := m.currentGroup()
	title := "Options"
	subtitle := "Select a group first"
	if group != nil {
		title = fmt.Sprintf("Options · %s", group.Name)
		subtitle = fmt.Sprintf("current %s", fallback(group.Current, "-"))
	}
	content := renderPanelContent(
		title,
		subtitle,
		m.visibleOptionRows(width, max(0, height)),
		width,
		height,
	)
	return renderPanel(style, width, height, content)
}

func (m model) renderSettingsPanel(width, height int) string {
	style := inactivePanelStyle
	if m.focus == focusSettings {
		style = activePanelStyle
	}
	content := renderPanelContent(
		"Settings",
		"Enter applies selected action",
		m.visibleSettingRows(width, max(0, height)),
		width,
		height,
	)
	return renderPanel(style, width, height, content)
}

func (m model) renderConnectionsPanel(width, height int) string {
	subtitle := fmt.Sprintf("%d active · ↓ %s · ↑ %s", len(m.connections.Connections), formatSize(m.connections.DownloadTotal), formatSize(m.connections.UploadTotal))
	content := renderPanelContent(
		"Connections",
		subtitle,
		m.visibleConnectionRows(width, max(0, height)),
		width,
		height,
	)
	return renderPanel(inactivePanelStyle, width, height, content)
}

func (m model) visibleGroupRows(width, limit int) []string {
	if limit <= 0 || width <= 0 {
		return nil
	}
	if len(m.groups) == 0 {
		return []string{fitLine(mutedStyle.Render("No groups match the current filter."), width)}
	}
	start, end := window(m.groupIndex, len(m.groups), limit)
	rows := make([]string, 0, end-start)
	for idx := start; idx < end; idx++ {
		group := m.groups[idx]
		prefix := "  "
		if idx == m.groupIndex {
			prefix = "▶ "
		}
		line := truncate(fmt.Sprintf("%s%s [%s]", prefix, group.Name, fallback(group.Current, "-")), width)
		switch {
		case idx == m.groupIndex:
			line = fitLine(selectedStyle.Render(line), width)
		case group.Current != "":
			line = fitLine(currentStyle.Render(line), width)
		default:
			line = fitLine(line, width)
		}
		rows = append(rows, line)
	}
	return rows
}

func (m model) visibleOptionRows(width, limit int) []string {
	if limit <= 0 || width <= 0 {
		return nil
	}
	group := m.currentGroup()
	if group == nil || len(group.Options) == 0 {
		return []string{fitLine(mutedStyle.Render("No selectable nodes in this group."), width)}
	}
	start, end := window(m.optionIndex, len(group.Options), limit)
	rows := make([]string, 0, end-start)
	for idx := start; idx < end; idx++ {
		option := group.Options[idx]
		marker := " "
		if option.Selected {
			marker = "✓"
		}
		line := truncate(fmt.Sprintf("%s %s %s", marker, option.Name, plainDelayLabel(option.DelayMS)), width)
		if idx == m.optionIndex {
			line = fitLine(selectedStyle.Render(line), width)
		} else {
			line = fitLine(line, width)
		}
		rows = append(rows, line)
	}
	return rows
}

func (m model) settingsItems() []settingItem {
	restartHint := "run labproxy restart in shell"
	if strings.TrimSpace(m.restartCommand) != "" {
		restartHint = "apply saved mixin changes"
	}
	return []settingItem{
		{Label: "Mode", Value: fallback(m.mode, "rule"), Hint: "live + persisted", Action: settingCycleMode},
		{Label: "System Proxy", Value: boolLabel(m.systemProxyEnabled), Hint: "new shells / next start", Action: settingToggleSystemProxy},
		{Label: "Allow LAN", Value: boolLabel(m.allowLanEnabled), Hint: "saved; restart required", Action: settingToggleAllowLan},
		{Label: "Tun", Value: boolLabel(m.tunEnabled), Hint: "saved; restart required", Action: settingToggleTun},
		{Label: "Apply / Restart", Value: "run", Hint: restartHint, Action: settingRestart},
	}
}

func (m model) visibleSettingRows(width, limit int) []string {
	if limit <= 0 || width <= 0 {
		return nil
	}
	items := m.settingsItems()
	if len(items) == 0 {
		return []string{fitLine(mutedStyle.Render("No settings available."), width)}
	}
	start, end := window(m.settingsIndex, len(items), limit)
	rows := make([]string, 0, end-start)
	for idx := start; idx < end; idx++ {
		item := items[idx]
		prefix := "  "
		if idx == m.settingsIndex {
			prefix = "▶ "
		}
		line := truncate(fmt.Sprintf("%s%s %s %s", prefix, item.Label, item.Value, item.Hint), width)
		if idx == m.settingsIndex {
			line = fitLine(selectedStyle.Render(line), width)
		} else {
			line = fitLine(line, width)
		}
		rows = append(rows, line)
	}
	return rows
}

func (m model) visibleConnectionRows(width, limit int) []string {
	if limit <= 0 || width <= 0 {
		return nil
	}
	connections := m.connections.Connections
	if len(connections) == 0 {
		return []string{fitLine(mutedStyle.Render("No active connections."), width)}
	}
	if len(connections) > limit {
		connections = connections[:limit]
	}
	rows := make([]string, 0, len(connections))
	for _, conn := range connections {
		line := truncate(
			fmt.Sprintf("%s %s %s ↓%s ↑%s", connectionTarget(conn), conn.Rule, strings.Join(conn.Chains, " → "), formatSize(conn.Download), formatSize(conn.Upload)),
			width,
		)
		rows = append(rows, fitLine(line, width))
	}
	return rows
}

func (m model) renderFooter() string {
	docWidth := max(0, m.width-docStyle.GetHorizontalFrameSize())
	if docWidth <= 0 {
		return ""
	}
	innerWidth := max(0, docWidth-headerStyle.GetHorizontalFrameSize())
	helpView := fitLine(mutedStyle.Render(m.help.View(m.keys)), innerWidth)
	left := statusStyle.Render(fallback(m.statusLine, "ready"))
	if m.searchMode {
		left = lipgloss.JoinHorizontal(lipgloss.Left, left, "  ", titleStyle.Render("Search:"), m.search.View())
	}
	row := lipgloss.JoinVertical(lipgloss.Left, fitLine(left, innerWidth), helpView)
	return docStyle.Width(docWidth).Render(headerStyle.Width(innerWidth).MaxWidth(docWidth).Render(row))
}

func fitLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(line)
}

func renderPanelContent(title, subtitle string, rows []string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	lines := make([]string, 0, height)
	lines = append(lines, fitLine(titleStyle.Render(truncate(title, width)), width))
	if height >= 2 && strings.TrimSpace(subtitle) != "" {
		lines = append(lines, fitLine(subtitleStyle.Render(truncate(subtitle, width)), width))
	}

	remaining := height - len(lines)
	if remaining > 0 && len(rows) > 0 {
		if remaining > len(rows) {
			remaining = len(rows)
		}
		lines = append(lines, rows[:remaining]...)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func renderPanel(style lipgloss.Style, width, height int, content string) string {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	return style.
		Width(width).
		Height(height).
		MaxWidth(width + style.GetHorizontalFrameSize()).
		MaxHeight(height + style.GetVerticalFrameSize()).
		Render(content)
}

func plainDelayLabel(ms int) string {
	if ms <= 0 {
		return "--"
	}
	return fmt.Sprintf("%dms", ms)
}

func fallback(value, alt string) string {
	if strings.TrimSpace(value) == "" {
		return alt
	}
	return value
}

func truncate(value string, width int) string {
	r := []rune(value)
	if len(r) <= width {
		return value
	}
	if width <= 1 {
		return string(r[:width])
	}
	return string(r[:width-1]) + "…"
}

func delayLabel(ms int) string {
	if ms <= 0 {
		return mutedStyle.Render("--")
	}
	label := fmt.Sprintf("%dms", ms)
	switch {
	case ms < 150:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(label)
	case ms < 300:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render(label)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render(label)
	}
}

func window(selected, total, limit int) (int, int) {
	if total <= limit {
		return 0, total
	}
	start := selected - limit/2
	if start < 0 {
		start = 0
	}
	end := start + limit
	if end > total {
		end = total
		start = end - limit
	}
	return start, end
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func boolLabel(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

func nextMode(current string) string {
	switch strings.ToLower(strings.TrimSpace(current)) {
	case "global":
		return "direct"
	case "direct":
		return "rule"
	default:
		return "global"
	}
}

func connectionTarget(conn proxy.Connection) string {
	if host := strings.TrimSpace(conn.Metadata.Host); host != "" {
		return host
	}
	if destination := strings.TrimSpace(conn.Metadata.Destination); destination != "" {
		return destination
	}
	return conn.ID
}

func (m model) focusLabel() string {
	switch m.focus {
	case focusGroups:
		return "groups"
	case focusSettings:
		return "settings"
	default:
		return "options"
	}
}

func formatBytes(value int64) string {
	units := []string{"B/s", "KB/s", "MB/s", "GB/s"}
	size := float64(value)
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}
	return fmt.Sprintf("%.1f%s", size, units[unit])
}

func formatSize(value int64) string {
	units := []string{"B", "KB", "MB", "GB"}
	size := float64(value)
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}
	return fmt.Sprintf("%.1f%s", size, units[unit])
}
