package tui

import (
	"context"
	"fmt"
	"sync"
	"time"

	appconfig "labproxy/internal/config"
	"labproxy/internal/proxy"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RulesModal is the contract for the rules modal — implemented in internal/tui/rules.
type RulesModal interface {
	IsOpen() bool
	Open()
	Update(tea.KeyMsg) bool
	View() string
}

type Options struct {
	Endpoint           string
	SystemProxyEnabled bool
	MixinConfigPath    string
	RestartCommand     string
	RulesModal         RulesModal
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
	focusConnections
)

// maxLogEntries 是日志覆盖视图保留的最大行数，超出自动丢弃最旧条目。
const maxLogEntries = 500

// logLevels 是 l 键循环切换的日志级别顺序。
var logLevels = []string{"info", "warning", "error", "debug"}

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
	Rules       key.Binding
	TestGroup   key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Tab, k.Select, k.Refresh, k.Settings, k.Rules, k.Quit}
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

// configFlagsMsg carries only config-related boolean flags,
// used by toggle-system-proxy / allow-lan / tun to avoid
// overwriting traffic / proxy / connection state with stale snapshots.
type configFlagsMsg struct {
	status             string
	systemProxyEnabled bool
	allowLanEnabled    bool
	tunEnabled         bool
}

type switchResultMsg struct {
	status string
	data   refreshMsg
}

type settingsResultMsg struct {
	status string
	data   refreshMsg
}

// testGroupResultMsg 携带批量测速结果（map[name]int，-1=失败）。
// 直接写入 OptionView.DelayMS，不依赖 mihomo history，支持 timeout 显示。
type testGroupResultMsg struct {
	groupName string
	results   map[string]int
}

// logEntryMsg 携带从 mihomo /logs 流收到的单条日志。
type logEntryMsg struct {
	entry proxy.LogEntry
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
	activeView    viewID
	groupIndex    int
	optionIndex   int
		settingsIndex int

	connIndex        int    // 连接面板选中行
	connConfirmClose string // 待确认关闭的目标（连接 id 或 "all"），空=无待确认

	// 日志视图
	logEntries []proxy.LogEntry   // 累积的日志条目（截断到 maxLogEntries）
	logLevel   string             // 当前订阅级别（debug/info/warning/error）
	logActive  bool               // 日志流是否正在订阅
	logCancel  context.CancelFunc // 停止当前日志订阅
	logCtx     context.Context    // 当前日志订阅的 ctx（与 logCancel 配对）

	width  int
	height int

	// Cached adaptive layout values (updated by rebuildGroups)
	groupPanelWidth int

	search     textinput.Model
	searchMode bool
	help       help.Model
	keys       keyMap
	statusLine string
	lastError  error
	rulesModal RulesModal
}

func newModel(client *proxy.Client, opts Options) model {
	search := textinput.New()
	search.Placeholder = T().SearchPlaceholder
	search.CharLimit = 64
	search.Width = 28

	return model{
		client:             client,
		endpoint:           opts.Endpoint,
		mixinConfigPath:    opts.MixinConfigPath,
		restartCommand:     opts.RestartCommand,
		systemProxyEnabled: opts.SystemProxyEnabled,
		focus:              focusGroups,
		activeView:         viewProxies,
		width:              120,
		height:             32,
		search:             search,
		help: func() help.Model {
			h := help.New()
			h.Width = max(0, 120-docStyle.GetHorizontalFrameSize()-headerStyle.GetHorizontalFrameSize())
			return h
		}(),
		statusLine: T().StatusConnecting,
		rulesModal: opts.RulesModal,
		keys: keyMap{
			Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", T().HelpMoveUp)),
			Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", T().HelpMoveDown)),
			Left:        key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", T().HelpFocusLeft)),
			Right:       key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", T().HelpFocusRight)),
			Tab:         key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", T().HelpSwitchPane)),
			Select:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", T().HelpApplySelect)),
			Refresh:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", T().HelpRefreshDelay)),
			Search:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", T().HelpSearch)),
			Settings:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", T().HelpSettings)),
			Mode:        key.NewBinding(key.WithKeys("m"), key.WithHelp("m", T().HelpCycleMode)),
			SystemProxy: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", T().HelpToggleProxyPref)),
			Back:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", T().HelpCloseBack)),
			Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", T().HelpQuit)),
			Rules:       key.NewBinding(key.WithKeys("R"), key.WithHelp("R", T().RulesHelpOpen)),
			TestGroup:   key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "test group")),
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
		m.help.Width = max(0, m.width-docStyle.GetHorizontalFrameSize()-headerStyle.GetHorizontalFrameSize())
		m.rebuildGroups()
		return m, nil
	case tickMsg:
		return m, tea.Batch(m.refreshCmd(), tickCmd())
	case refreshMsg:
		m.applyState(msg)
		if m.lastError == nil {
			m.statusLine = T().StatusConnected
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
	case testGroupResultMsg:
		m.applyTestGroupResult(msg)
		return m, nil
	case logEntryMsg:
		// 累积日志并截断；若仍在订阅则继续订阅下一条
		m.logEntries = append(m.logEntries, msg.entry)
		if len(m.logEntries) > maxLogEntries {
			m.logEntries = m.logEntries[len(m.logEntries)-maxLogEntries:]
		}
		if m.logActive && m.logCtx != nil {
			return m, m.logsCmd(m.logCtx)
		}
		return m, nil
	case settingsResultMsg:
		m.applyState(msg.data)
		m.statusLine = msg.status
		m.lastError = nil
		m.activeView = viewProxies
		return m, nil
	case errMsg:
		m.lastError = msg.err
		m.statusLine = msg.err.Error()
		m.activeView = viewProxies
		return m, nil
	case configFlagsMsg:
		m.systemProxyEnabled = msg.systemProxyEnabled
		m.allowLanEnabled = msg.allowLanEnabled
		m.tunEnabled = msg.tunEnabled
		m.statusLine = msg.status
		m.lastError = nil
		m.activeView = viewProxies
		return m, nil
	case tea.KeyMsg:
		// 全局视图切换：1-5 直达，Tab 循环。输入态时不拦截。
		if !m.searchMode && m.activeView != viewConfig {
			if v, ok := viewByDigit(string(msg.Runes)); ok && msg.Type == tea.KeyRunes {
				// 切离 viewLogs：停止订阅
				if m.activeView == viewLogs && v != viewLogs && m.logCancel != nil {
					m.logCancel()
					m.logActive = false
				}
				m.activeView = v
				m.statusLine = v.label()
				// 切到 viewLogs：启动订阅
				if v == viewLogs && !m.logActive {
					m.logActive = true
					if m.logLevel == "" {
						m.logLevel = "info"
					}
					ctx, cancel := context.WithCancel(context.Background())
					m.logCancel = cancel
					m.logCtx = ctx
					return m, tea.Batch(m.logsCmd(ctx))
				}
				return m, nil
			}
			if key.Matches(msg, m.keys.Tab) {
				next := m.activeView.next()
				if m.activeView == viewLogs && next != viewLogs && m.logCancel != nil {
					m.logCancel()
					m.logActive = false
				}
				m.activeView = next
				m.statusLine = next.label()
				if next == viewLogs && !m.logActive {
					m.logActive = true
					if m.logLevel == "" {
						m.logLevel = "info"
					}
					ctx, cancel := context.WithCancel(context.Background())
					m.logCancel = cancel
					m.logCtx = ctx
					return m, tea.Batch(m.logsCmd(ctx))
				}
				return m, nil
			}
		}

		if m.rulesModal != nil && m.rulesModal.IsOpen() {
			if m.rulesModal.Update(msg) {
				return m, nil
			}
		}
		if m.searchMode {
			switch {
			case key.Matches(msg, m.keys.Quit), key.Matches(msg, m.keys.Back):
				m.searchMode = false
				m.search.Blur()
				m.rebuildGroups()
				m.statusLine = T().SearchCancelled
				return m, nil
			case key.Matches(msg, m.keys.Select):
				m.searchMode = false
				m.search.Blur()
				m.rebuildGroups()
				m.statusLine = fmt.Sprintf(T().FilterLabelFmt, fallback(m.search.Value(), T().FilterNone))
				return m, nil
			default:
				var cmd tea.Cmd
				m.search, cmd = m.search.Update(msg)
				m.rebuildGroups()
				return m, cmd
			}
		}

		// 日志视图按键：esc 返回代理视图，l 切级别。
		if m.activeView == viewLogs {
			if handled, mm, mcmd := m.handleLogKey(msg); handled {
				return mm, mcmd
			}
		}

		// 连接面板断连快捷键：仅当焦点在连接面板时生效。
		// d → 关闭当前选中连接（首次进入待确认，再次确认）；D → 关闭全部（同理）。
		if m.focus == focusConnections {
			if handled, mm, mcmd := m.handleConnectionCloseKey(msg); handled {
				return mm, mcmd
			}
		}

		switch {
		case m.activeView == viewConfig:
			switch {
			case key.Matches(msg, m.keys.Quit), key.Matches(msg, m.keys.Back):
				m.activeView = viewProxies
				m.statusLine = T().SettingsClosed
				return m, nil
			case key.Matches(msg, m.keys.Up):
				m.settingsIndex--
				if m.settingsIndex < 0 {
					m.settingsIndex = len(m.settingsItems()) - 1
				}
				return m, nil
			case key.Matches(msg, m.keys.Down):
				m.settingsIndex++
				items := m.settingsItems()
				if m.settingsIndex >= len(items) {
					m.settingsIndex = 0
				}
				return m, nil
			case key.Matches(msg, m.keys.Select):
				return m, m.activateSettingCmd()
			default:
				return m, nil
			}
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case m.activeView == viewProxies && key.Matches(msg, m.keys.Search):
			m.searchMode = true
			m.search.Focus()
			m.statusLine = T().TypeToFilter
			return m, nil
		case key.Matches(msg, m.keys.Mode):
			return m, m.cycleModeCmd()
		case key.Matches(msg, m.keys.SystemProxy):
			return m, m.toggleSystemProxyCmd()
		case key.Matches(msg, m.keys.Tab):
			m.toggleFocus()
			return m, nil
		case m.activeView == viewProxies && key.Matches(msg, m.keys.Left):
			m.moveFocus(-1)
			return m, nil
		case m.activeView == viewProxies && key.Matches(msg, m.keys.Right):
			m.moveFocus(1)
			return m, nil
		case m.activeView == viewConnections && key.Matches(msg, m.keys.Up):
			if len(m.connections.Connections) > 0 {
				m.connIndex = (m.connIndex - 1 + len(m.connections.Connections)) % len(m.connections.Connections)
			}
			return m, nil
		case m.activeView == viewConnections && key.Matches(msg, m.keys.Down):
			if len(m.connections.Connections) > 0 {
				m.connIndex = (m.connIndex + 1) % len(m.connections.Connections)
			}
			return m, nil
		case key.Matches(msg, m.keys.Up):
			m.move(-1)
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.move(1)
			return m, nil
		case key.Matches(msg, m.keys.Refresh):
			return m, m.delayRefreshCmd()
		case m.activeView == viewProxies && key.Matches(msg, m.keys.TestGroup):
			return m, m.testGroupCmd()
		case key.Matches(msg, m.keys.Select):
			switch m.focus {
			case focusGroups:
				m.focus = focusOptions
				m.statusLine = T().FocusOptions
				return m, nil
			default:
				return m, m.switchProxyCmd()
			}
		}
	}

	return m, nil
}

func (m model) View() string {
	if m.width <= 0 {
		return T().Loading
	}

	// 按 activeView 分派 body。
	_ = m.activeView

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

// applyTestGroupResult 将批量测速结果写入对应组的 OptionView.DelayMS。
// 失败节点记 -1（渲染为 timeout）；未测节点保持原值。
func (m *model) applyTestGroupResult(msg testGroupResultMsg) {
	for i := range m.groups {
		if m.groups[i].Name != msg.groupName {
			continue
		}
		for j := range m.groups[i].Options {
			name := m.groups[i].Options[j].Name
			if delay, ok := msg.results[name]; ok {
				m.groups[i].Options[j].DelayMS = delay
			}
		}
		break
	}
	m.statusLine = fmt.Sprintf(T().TestGroupDoneFmt, msg.groupName)
}

func (m *model) toggleFocus() {
	m.moveFocus(1)
}

func (m *model) moveFocus(delta int) {
	order := []paneFocus{focusGroups, focusOptions, focusConnections}
	current := 0
	for idx, focus := range order {
		if m.focus == focus {
			current = idx
			break
		}
	}
	current = (current + delta + len(order)) % len(order)
	m.focus = order[current]
	switch m.focus {
	case focusGroups:
		m.statusLine = T().FocusGroups
	case focusOptions:
		m.statusLine = T().FocusOptions
	case focusConnections:
		m.statusLine = T().FocusConnections
	}
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
	case focusConnections:
		if len(m.connections.Connections) == 0 {
			return
		}
		m.connIndex += delta
		m.clampConnIndex()
	}
	m.clampIndices()
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

		// Concurrent delay testing: fire up to 4 goroutines at once so
		// many nodes don't exhaust the 12s context timeout sequentially.
		const concurrency = 4
		sem := make(chan struct{}, concurrency)
		var wg sync.WaitGroup

		for _, optionName := range optionNames {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				_, _ = m.client.Delay(ctx, name, 5*time.Second)
			}(optionName)
		}
		wg.Wait()

		state, err := m.fetchState(ctx)
		if err != nil {
			return errMsg{err}
		}
		return state
	}
}

// testGroupCmd 批量测当前组全部节点延迟，返回精确结果（含 -1 失败标记）。
// 与 delayRefreshCmd 的区别：直接拿 DelayGroup 结果渲染，不依赖 mihomo history，
// 从而能显示 timeout（而非 --）。
func (m model) testGroupCmd() tea.Cmd {
	group := m.currentGroup()
	if group == nil || len(group.Options) == 0 {
		return nil
	}
	// 取原始 Proxy（含 All）以调用 DelayGroup
	proxyGroup, ok := m.rawProxies.Proxies[group.Name]
	if !ok {
		return nil
	}
	groupName := group.Name

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		results, err := m.client.DelayGroup(ctx, proxyGroup, 5*time.Second)
		if err != nil {
			return errMsg{err}
		}
		return testGroupResultMsg{groupName: groupName, results: results}
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
			status: fmt.Sprintf(T().SwitchedFmt, groupName, optionName),
			data:   state,
		}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) renderHeader() string {
	docWidth := max(0, m.width-docStyle.GetHorizontalFrameSize())
	if docWidth <= 0 {
		return ""
	}
	innerWidth := max(0, docWidth-headerStyle.GetHorizontalFrameSize())
	titleRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		titleStyle.Render(T().AppTitle),
		"  ",
		subtitleStyle.Render(T().PressSForSettings),
	)

	metaRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		statusPill(T().PillEndpoint, fallback(m.endpoint, "-")),
		statusPill(T().PillMode, modeLabel(m.mode)),
		statusPill(T().PillProxy, boolLabel(m.systemProxyEnabled)),
		statusPill(T().PillLan, boolLabel(m.allowLanEnabled)),
		statusPill(T().PillTun, boolLabel(m.tunEnabled)),
		statusPill("↑", formatBytes(m.up)),
		statusPill("↓", formatBytes(m.down)),
		statusPill(T().PillFocus, m.focusLabel()),
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
		BorderForeground(lipgloss.Color("237")).
		Padding(0, 1)
	return pill.Render(fmt.Sprintf("%s %s", mutedStyle.Render(label), value))
}

func (m model) renderBody(availableHeight int) string {
	docWidth := max(0, m.width-docStyle.GetHorizontalFrameSize())
	if availableHeight <= 0 || docWidth <= 0 {
		return docStyle.Width(docWidth).Render("")
	}

	navWidth := 14 + panelBaseStyle.GetHorizontalFrameSize()
	nav := renderNav(m.activeView, availableHeight)
	contentWidth := max(0, docWidth-navWidth-1)

	var rest string
	switch m.activeView {
	case viewProxies:
		rest = m.renderProxiesView(contentWidth, availableHeight)
	case viewConnections:
		rest = m.renderConnectionsView(contentWidth, availableHeight)
	case viewLogs:
		rest = m.renderLogsView(contentWidth, availableHeight)
	case viewRules:
		rest = m.renderRulesView(contentWidth, availableHeight)
	case viewConfig:
		rest = m.renderConfigView(contentWidth, availableHeight)
	default:
		rest = m.renderProxiesView(contentWidth, availableHeight)
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, nav, " ", rest)
}

func (m model) renderFooter() string {
	docWidth := max(0, m.width-docStyle.GetHorizontalFrameSize())
	if docWidth <= 0 {
		return ""
	}
	innerWidth := max(0, docWidth-headerStyle.GetHorizontalFrameSize())
	helpView := fitLine(mutedStyle.Render(m.help.View(m.keys)), innerWidth)
	left := statusStyle.Render(fallback(m.statusLine, T().StatusReady))
	if m.searchMode {
		left = lipgloss.JoinHorizontal(lipgloss.Left, left, "  ", titleStyle.Render(T().SearchLabel), m.search.View())
	}
	row := lipgloss.JoinVertical(lipgloss.Left, fitLine(left, innerWidth), helpView)
	return docStyle.Width(docWidth).Render(headerStyle.Width(innerWidth).MaxWidth(docWidth).Render(row))
}

func (m model) focusLabel() string {
	switch m.focus {
	case focusGroups:
		return T().FocusGroupsLabel
	default:
		return T().FocusOptionsLabel
	}
}
