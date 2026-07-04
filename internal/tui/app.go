package tui

import (
	"context"
	"fmt"
	"sync"
	"time"

	appconfig "labproxy/internal/config"
	"labproxy/internal/proxy"
	"labproxy/internal/tui/theme"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RulesModal is the contract for the rules modal — implemented in internal/tui/rules.
type RulesModal interface {
	IsOpen() bool
	Open()
	Close()
	Update(tea.KeyMsg) bool
	View() string
	SetTheme(t *theme.Theme)
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

	theme *theme.Theme

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

	search     textinput.Model
	searchMode bool
	helpMode   bool
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

	t := theme.Current()
	if opts.RulesModal != nil {
		opts.RulesModal.SetTheme(t)
	}

	return model{
		client:             client,
		endpoint:           opts.Endpoint,
		mixinConfigPath:    opts.MixinConfigPath,
		restartCommand:     opts.RestartCommand,
		systemProxyEnabled: opts.SystemProxyEnabled,
		theme:              t,
		focus:              focusGroups,
		activeView:         viewProxies,
		width:              120,
		height:             32,
		search:             search,
		statusLine:         T().StatusConnecting,
		rulesModal:         opts.RulesModal,
		keys:               defaultKeyMap(),
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
		m.ensureFocusForView()
		return m, nil
	case errMsg:
		m.lastError = msg.err
		m.statusLine = msg.err.Error()
		m.activeView = viewProxies
		m.ensureFocusForView()
		return m, nil
	case configFlagsMsg:
		m.systemProxyEnabled = msg.systemProxyEnabled
		m.allowLanEnabled = msg.allowLanEnabled
		m.tunEnabled = msg.tunEnabled
		m.statusLine = msg.status
		m.lastError = nil
		m.activeView = viewProxies
		m.ensureFocusForView()
		return m, nil
	case tea.KeyMsg:
		// ? 帮助浮层（最高优先级，输入态时不拦截）
		if !m.searchMode && msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == '?' {
			m.helpMode = !m.helpMode
			return m, nil
		}

		// 代理页内 Tab 只切换 pane 焦点；视图切换保留给 1-5，避免和面板提示冲突。
		if !m.searchMode && m.activeView == viewProxies && key.Matches(msg, m.keys.Tab) {
			m.moveFocus(1)
			return m, nil
		}

		// 全局视图切换：1-5 直达；非代理页 Tab 循环。输入态时不拦截。
		if !m.searchMode && m.activeView != viewConfig {
			if v, ok := viewByDigit(string(msg.Runes)); ok && msg.Type == tea.KeyRunes {
				return m.switchView(v)
			}
			if key.Matches(msg, m.keys.Tab) {
				return m.switchView(m.activeView.next())
			}
		}

		if m.activeView == viewRules && m.rulesModal != nil && m.rulesModal.IsOpen() {
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
		if m.activeView == viewConnections && m.focus == focusConnections {
			if handled, mm, mcmd := m.handleConnectionCloseKey(msg); handled {
				return mm, mcmd
			}
		}

		switch {
		case m.activeView == viewConfig:
			switch {
			case key.Matches(msg, m.keys.Quit), key.Matches(msg, m.keys.Back):
				m.activeView = viewProxies
				m.ensureFocusForView()
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
			case key.Matches(msg, m.keys.Mode):
				return m, m.cycleModeCmd()
			case key.Matches(msg, m.keys.SystemProxy):
				return m, m.toggleSystemProxyCmd()
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
		case m.activeView == viewProxies && key.Matches(msg, m.keys.Select):
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

func (m model) switchView(v viewID) (tea.Model, tea.Cmd) {
	if m.activeView == viewLogs && v != viewLogs && m.logCancel != nil {
		m.logCancel()
		m.logActive = false
	}
	if m.activeView == viewRules && v != viewRules && m.rulesModal != nil {
		m.rulesModal.Close()
	}

	m.activeView = v
	m.ensureFocusForView()
	m.statusLine = v.label()

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
	if v == viewRules && m.rulesModal != nil && !m.rulesModal.IsOpen() {
		m.rulesModal.Open()
	}
	return m, nil
}

func (m *model) ensureFocusForView() {
	switch m.activeView {
	case viewConnections:
		m.focus = focusConnections
	default:
		if m.focus == focusConnections {
			m.focus = focusGroups
		}
	}
}

func (m model) View() string {
	if m.width <= 0 {
		return T().Loading
	}

	if m.helpMode {
		return m.renderHelpOverlay()
	}

	header := m.renderHeader()
	tabs := m.renderTabs()
	footer := m.renderFooter()
	availableBodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(tabs) - lipgloss.Height(footer)
	body := m.renderBody(availableBodyHeight)
	return lipgloss.JoinVertical(lipgloss.Left, header, tabs, body, footer)
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

func (m *model) moveFocus(delta int) {
	order := []paneFocus{focusGroups, focusOptions}
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

func (m model) renderBody(availableHeight int) string {
	docWidth := m.docWidth()
	if availableHeight <= 0 || docWidth <= 0 {
		return docStyle.Width(docWidth).Render("")
	}

	switch m.activeView {
	case viewProxies:
		return m.renderProxiesView(docWidth, availableHeight)
	case viewConnections:
		return m.renderConnectionsView(docWidth, availableHeight)
	case viewLogs:
		return m.renderLogsView(docWidth, availableHeight)
	case viewRules:
		return m.renderRulesView(docWidth, availableHeight)
	case viewConfig:
		return m.renderConfigView(docWidth, availableHeight)
	default:
		return m.renderProxiesView(docWidth, availableHeight)
	}
}
