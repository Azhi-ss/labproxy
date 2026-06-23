package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	appconfig "labproxy/internal/config"
	"labproxy/internal/proxy"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	Logs        key.Binding
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
	settingsMode  bool

	connIndex        int    // 连接面板选中行
	connConfirmClose string // 待确认关闭的目标（连接 id 或 "all"），空=无待确认

	// 日志覆盖视图
	logMode    bool               // 是否处于日志覆盖模式
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
			Logs:        key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "logs")),
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
		// 累积日志并截断；若仍在 logMode 则继续订阅下一条
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
		m.settingsMode = false
		return m, nil
	case errMsg:
		m.lastError = msg.err
		m.statusLine = msg.err.Error()
		m.settingsMode = false
		return m, nil
	case configFlagsMsg:
		m.systemProxyEnabled = msg.systemProxyEnabled
		m.allowLanEnabled = msg.allowLanEnabled
		m.tunEnabled = msg.tunEnabled
		m.statusLine = msg.status
		m.lastError = nil
		m.settingsMode = false
		return m, nil
	case tea.KeyMsg:
			// 全局视图切换：1-5 直达，Tab 循环。输入态时不拦截。
			if !m.searchMode && !m.settingsMode && !m.logMode {
				if v, ok := viewByDigit(string(msg.Runes)); ok && msg.Type == tea.KeyRunes {
					m.activeView = v
					m.statusLine = v.label()
					return m, nil
				}
				if key.Matches(msg, m.keys.Tab) {
					m.activeView = m.activeView.next()
					m.statusLine = m.activeView.label()
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

		// 日志覆盖模式：esc 退出、l 切级别，其它键交回主循环。
		if m.logMode {
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
		case m.settingsMode:
			switch {
			case key.Matches(msg, m.keys.Quit), key.Matches(msg, m.keys.Back):
				m.settingsMode = false
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
		case key.Matches(msg, m.keys.Search):
			m.searchMode = true
			m.search.Focus()
			m.statusLine = T().TypeToFilter
			return m, nil
		case key.Matches(msg, m.keys.Settings):
			m.settingsMode = true
			m.statusLine = T().SettingsOpenHint
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
		case key.Matches(msg, m.keys.TestGroup):
			return m, m.testGroupCmd()
		case key.Matches(msg, m.keys.Logs):
			m.logMode = true
			m.logActive = true
			if m.logLevel == "" {
				m.logLevel = "info"
			}
			// 在 model 上创建 ctx+cancel，确保 cancel 存入返回的 m（避免值接收者丢失）
			ctx, cancel := context.WithCancel(context.Background())
			m.logCancel = cancel
			m.logCtx = ctx
			m.statusLine = T().LogOverlayHint
			return m, m.logsCmd(ctx)
		case key.Matches(msg, m.keys.Rules):
			if m.rulesModal != nil && !m.rulesModal.IsOpen() {
				m.rulesModal.Open()
			}
			return m, nil
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

	if m.settingsMode {
		return m.renderSettingsOverlay()
	}

	if m.logMode {
		return m.renderLogOverlay()
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

	// Update cached adaptive layout width for Groups panel
	docWidth := max(0, m.width-docStyle.GetHorizontalFrameSize())
	panelFrameWidth := panelBaseStyle.GetHorizontalFrameSize()
	columnContentWidth := docWidth - columnGap - panelFrameWidth*2
	if columnContentWidth > 0 {
		m.groupPanelWidth = m.calcGroupsMinWidth(columnContentWidth)
	} else {
		m.groupPanelWidth = 20 // fallback: matches minGroupsWidth in calcGroupsMinWidth
	}
}

func (m *model) clampIndices() {
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

// handleLogKey 处理日志覆盖模式按键：esc/q 退出，l 循环切换级别。
// 切换级别会清空已累积日志并重新订阅。
func (m model) handleLogKey(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	// esc / q 退出
	if msg.Type == tea.KeyEsc {
		m.stopLogStream()
		m.logMode = false
		m.statusLine = T().LogOverlayClosed
		return true, m, nil
	}
	if msg.Type == tea.KeyRunes && string(msg.Runes) == "q" {
		m.stopLogStream()
		m.logMode = false
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
			// Allow && (shell AND) but reject standalone & (background)
			if i+1 >= len(cmd) || cmd[i+1] != '&' {
				return fmt.Errorf(T().RestartValidateErrFmt, ch)
			}
			i++ // skip second &
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

func tickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) currentGroup() *GroupView {
	if len(m.groups) == 0 || m.groupIndex >= len(m.groups) {
		return nil
	}
	return &m.groups[m.groupIndex]
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

// calcGroupsMinWidth computes the optimal width for the Groups panel
// based on actual group name lengths, with min/max boundaries.
// It also considers the Options panel's minimum width (minOptionsWidth) and
// the actual content width needed by the currently displayed options, ensuring
// both panels have enough space.
func (m model) calcGroupsMinWidth(columnContentWidth int) int {
	const (
		minGroupsWidth  = 20 // minimum usable width for Groups panel
		minOptionsWidth = 30 // minimum usable width for Options panel
		reservedPrefix  = 2  // "▸ " or "  "
		rightPadding    = 2  // right-side padding for Groups panel content
	)

	if columnContentWidth <= minGroupsWidth+minOptionsWidth {
		// Very narrow: give Groups at least minGroupsWidth (if possible) or half
		return max(minGroupsWidth, columnContentWidth/2)
	}

	// Find the longest group row width needed
	maxGroupRowWidth := 0
	for _, group := range m.groups {
		currentMarkLen := 0
		if group.Current != "" {
			currentMarkLen = ansi.StringWidth(" [" + group.Current + "]")
		}
		nameWidth := ansi.StringWidth(group.Name)
		rowWidth := reservedPrefix + nameWidth + currentMarkLen + rightPadding
		if rowWidth > maxGroupRowWidth {
			maxGroupRowWidth = rowWidth
		}
	}

	// No groups visible: fall back to reasonable default
	if maxGroupRowWidth == 0 {
		maxGroupRowWidth = minGroupsWidth
	}

	// Calculate the actual minimum width the Options panel needs
	// based on the currently selected group's option content
	optionsContentWidth := minOptionsWidth
	if group := m.currentGroup(); group != nil {
		for _, opt := range group.Options {
			// Format: " ● name delay" — marker(1) + space(1) + name + space(1) + delay
			optRowWidth := 1 + 1 + ansi.StringWidth(opt.Name) + 1 + ansi.StringWidth(plainDelayLabel(opt.DelayMS))
			if optRowWidth > optionsContentWidth {
				optionsContentWidth = optRowWidth
			}
		}
	}

	// Clamp: at least minGroupsWidth, and ensure Options gets enough space
	maxAllowed := max(minGroupsWidth, columnContentWidth-optionsContentWidth)
	if maxGroupRowWidth < minGroupsWidth {
		maxGroupRowWidth = minGroupsWidth
	} else if maxGroupRowWidth > maxAllowed {
		maxGroupRowWidth = maxAllowed
	}

	return maxGroupRowWidth
}

func (m model) renderBody(availableHeight int) string {
	docWidth := max(0, m.width-docStyle.GetHorizontalFrameSize())
	if availableHeight <= 0 || docWidth <= 0 {
		return docStyle.Width(docWidth).Render("")
	}

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

	columnContentWidth := docWidth - columnGap - panelFrameWidth*2
	if columnContentWidth < 0 {
		columnContentWidth = 0
	}

	// Dynamic adaptive width: use cached Groups panel width, but never let the
	// split exceed the terminal content area on very narrow screens.
	leftWidth := min(max(0, m.groupPanelWidth), columnContentWidth)
	middleWidth := max(0, columnContentWidth-leftWidth)
	topContentHeight := max(0, topTotalHeight-panelFrameHeight)

	top := lipgloss.NewStyle().MaxWidth(docWidth).MaxHeight(topTotalHeight).Render(
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			m.renderGroupsPanel(leftWidth, topContentHeight),
			strings.Repeat(" ", columnGap),
			m.renderOptionsPanel(middleWidth, topContentHeight),
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
		T().PanelGroups,
		T().PanelGroupsHint,
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
	title := T().PanelOptions
	subtitle := T().SelectGroupFirst
	if group != nil {
		title = fmt.Sprintf(T().OptionsTitleFmt, group.Name)
		subtitle = fmt.Sprintf(T().CurrentFmt, fallback(group.Current, "-"))
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

// renderLogOverlay 渲染日志覆盖视图：显示最近 maxLogEntries 条日志，按级别着色。
func (m model) renderLogOverlay() string {
	width := max(1, m.width)
	height := max(1, m.height)

	header := fmt.Sprintf(T().LogOverlayTitle, m.logLevel)
	body := lipgloss.JoinVertical(lipgloss.Left, header, "")
	usedHeight := lipgloss.Height(body) + 2 // 标题+空行
	avail := height - usedHeight
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
		lines = append(lines, fitLine(fmt.Sprintf("[%s] %s", e.Level, e.Payload), width))
	}
	if len(lines) == 0 {
		lines = append(lines, fitLine(mutedStyle.Render(T().LogWaiting), width))
	}

	content := strings.Join(lines, "\n")
	return lipgloss.JoinVertical(lipgloss.Left, header, "", content)
}

func (m model) renderSettingsOverlay() string {
	contentWidth := 32
	// padding(1,2)=4 + border(2)=2 → total extra 6
	totalWidth := contentWidth + 6

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1, 2).
		Width(totalWidth)

	title := titleStyle.Render(T().SettingsTitle)
	subtitle := mutedStyle.Render(T().SettingsHint)

	rows := m.visibleSettingRows(contentWidth, 5)
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		subtitle,
		"",
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	)

	modal := modalStyle.Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
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

func (m model) visibleGroupRows(width, limit int) []string {
	if limit <= 0 || width <= 0 {
		return nil
	}
	if len(m.groups) == 0 {
		return []string{fitLine(mutedStyle.Render("  "+T().NoGroupsMatchFilter), width)}
	}
	start, end := window(m.groupIndex, len(m.groups), limit)
	rows := make([]string, 0, end-start)

	for idx := start; idx < end; idx++ {
		group := m.groups[idx]
		isSelected := idx == m.groupIndex

		prefix := "  "
		if isSelected {
			prefix = "▸ "
		}

		currentMarkLen := 0
		if group.Current != "" {
			// " [Current]" = space + bracket + name + bracket, use visual width
			currentMarkLen = ansi.StringWidth(" [" + group.Current + "]")
		}

		reservedPrefix := ansi.StringWidth(prefix) // "▸ " or "  " — both are 2 cols today, but self-documenting
		nameWidth := width - reservedPrefix - currentMarkLen
		if nameWidth < 4 {
			nameWidth = 4
		}

		truncatedName := ansi.Truncate(group.Name, nameWidth, "…")

		baseStyle := lipgloss.NewStyle()
		if isSelected {
			baseStyle = selectedStyle
		} else if group.Current != "" {
			baseStyle = currentStyle
		}

		currentMark := ""
		if group.Current != "" {
			bracketStyle := mutedStyle
			curStyle := currentStyle
			if isSelected {
				bracketStyle = bracketStyle.Inherit(selectedStyle).Foreground(colorTextMuted)
				curStyle = curStyle.Inherit(selectedStyle).Foreground(colorAccent)
			}
			currentMark = baseStyle.Render(" ") + bracketStyle.Render("[") + curStyle.Render(group.Current) + bracketStyle.Render("]")
		}

		line := baseStyle.Render(prefix+truncatedName) + currentMark
		rows = append(rows, fitStyledLine(line, width, baseStyle))
	}
	return rows
}

func (m model) visibleOptionRows(width, limit int) []string {
	if limit <= 0 || width <= 0 {
		return nil
	}
	group := m.currentGroup()
	if group == nil || len(group.Options) == 0 {
		return []string{fitLine(mutedStyle.Render("  "+T().NoSelectableNodes), width)}
	}
	start, end := window(m.optionIndex, len(group.Options), limit)
	rows := make([]string, 0, end-start)
	for idx := start; idx < end; idx++ {
		option := group.Options[idx]
		isSelected := idx == m.optionIndex

		baseStyle := lipgloss.NewStyle()
		if isSelected {
			baseStyle = selectedStyle
		}

		var markerStyle lipgloss.Style
		markerChar := "○"
		if option.Selected {
			markerStyle = lipgloss.NewStyle().Foreground(colorSuccess)
			markerChar = "●"
		} else {
			markerStyle = mutedStyle
		}
		if isSelected {
			markerStyle = markerStyle.Inherit(selectedStyle).Foreground(markerStyle.GetForeground())
		}

		delayStyle := getDelayStyle(option.DelayMS)
		if isSelected {
			delayStyle = delayStyle.Inherit(selectedStyle).Foreground(delayStyle.GetForeground())
		}
		delayStrPlain := plainDelayLabel(option.DelayMS)

		reserved := 1 + 1 + 1 + 1 + ansi.StringWidth(delayStrPlain)
		nameWidth := width - reserved
		if nameWidth < 4 {
			nameWidth = 4
		}
		truncatedName := ansi.Truncate(option.Name, nameWidth, "…")

		line := baseStyle.Render(" ") +
			markerStyle.Render(markerChar) +
			baseStyle.Render(" "+truncatedName+" ") +
			delayStyle.Render(delayStrPlain)

		rows = append(rows, fitStyledLine(line, width, baseStyle))
	}
	return rows
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

func (m model) focusLabel() string {
	switch m.focus {
	case focusGroups:
		return T().FocusGroupsLabel
	default:
		return T().FocusOptionsLabel
	}
}
