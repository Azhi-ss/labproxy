package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"clash-for-lab/internal/mihomo"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Options struct {
	Endpoint           string
	SystemProxyEnabled bool
}

type App struct {
	client *mihomo.Client
	opts   Options
}

func NewApp(client *mihomo.Client, opts Options) *App {
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
)

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Tab     key.Binding
	Select  key.Binding
	Refresh key.Binding
	Search  key.Binding
	Back    key.Binding
	Quit    key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Tab, k.Select, k.Refresh, k.Search, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Tab, k.Select, k.Refresh, k.Search, k.Back, k.Quit},
	}
}

type refreshMsg struct {
	version mihomo.Version
	config  mihomo.Config
	traffic mihomo.Traffic
	proxies mihomo.ProxiesResponse
}

type tickMsg time.Time

type statusMsg struct{ text string }

type errMsg struct{ err error }

type switchResultMsg struct {
	status string
	data   refreshMsg
}

type model struct {
	client             *mihomo.Client
	endpoint           string
	systemProxyEnabled bool

	version string
	mode    string
	up      int64
	down    int64

	rawProxies  mihomo.ProxiesResponse
	groups      []GroupView
	focus       paneFocus
	groupIndex  int
	optionIndex int

	width  int
	height int

	search     textinput.Model
	searchMode bool
	help       help.Model
	keys       keyMap
	statusLine string
	lastError  error
}

func newModel(client *mihomo.Client, opts Options) model {
	search := textinput.New()
	search.Placeholder = "Search proxies or groups"
	search.CharLimit = 64
	search.Width = 28

	return model{
		client:             client,
		endpoint:           opts.Endpoint,
		systemProxyEnabled: opts.SystemProxyEnabled,
		focus:              focusGroups,
		width:              120,
		height:             32,
		search:             search,
		help:               help.New(),
		statusLine:         "connecting…",
		keys: keyMap{
			Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move up")),
			Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move down")),
			Left:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "focus groups")),
			Right:   key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "focus options")),
			Tab:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "switch pane")),
			Select:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "focus/switch")),
			Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh delay")),
			Search:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
			Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close search")),
			Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		},
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.refreshCmd(), tickCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		return m, tea.Batch(m.refreshCmd(), tickCmd())
	case refreshMsg:
		m.version = msg.version.Version
		m.mode = msg.config.Mode
		m.up = msg.traffic.Up
		m.down = msg.traffic.Down
		m.rawProxies = msg.proxies
		m.rebuildGroups()
		if m.lastError == nil {
			m.statusLine = "connected"
		}
		return m, nil
	case statusMsg:
		m.statusLine = msg.text
		m.lastError = nil
		return m, nil
	case switchResultMsg:
		m.version = msg.data.version.Version
		m.mode = msg.data.config.Mode
		m.up = msg.data.traffic.Up
		m.down = msg.data.traffic.Down
		m.rawProxies = msg.data.proxies
		m.rebuildGroups()
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
		case key.Matches(msg, m.keys.Tab):
			m.toggleFocus()
			return m, nil
		case key.Matches(msg, m.keys.Left):
			m.focus = focusGroups
			m.statusLine = "focus: groups"
			return m, nil
		case key.Matches(msg, m.keys.Right):
			m.focus = focusOptions
			m.statusLine = "focus: options"
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
			if m.focus == focusGroups {
				m.focus = focusOptions
				m.statusLine = "focus: options"
				return m, nil
			}
			return m, m.switchProxyCmd()
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

func (m *model) toggleFocus() {
	if m.focus == focusGroups {
		m.focus = focusOptions
		m.statusLine = "focus: options"
		return
	}
	m.focus = focusGroups
	m.statusLine = "focus: groups"
}

func (m *model) move(delta int) {
	if len(m.groups) == 0 {
		return
	}
	if m.focus == focusGroups {
		m.groupIndex += delta
	} else {
		m.optionIndex += delta
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

		version, err := m.client.Version(ctx)
		if err != nil {
			return errMsg{err}
		}
		config, err := m.client.Config(ctx)
		if err != nil {
			return errMsg{err}
		}
		traffic, err := m.client.Traffic(ctx)
		if err != nil {
			return errMsg{err}
		}
		proxies, err := m.client.Proxies(ctx)
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{version: version, config: config, traffic: traffic, proxies: proxies}
	}
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

		proxies, err := m.client.Proxies(ctx)
		if err != nil {
			return errMsg{err}
		}
		return refreshMsg{
			version: mihomo.Version{Version: m.version},
			config:  mihomo.Config{Mode: m.mode},
			traffic: mihomo.Traffic{Up: m.up, Down: m.down},
			proxies: proxies,
		}
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
		version, err := m.client.Version(ctx)
		if err != nil {
			return errMsg{err}
		}
		config, err := m.client.Config(ctx)
		if err != nil {
			return errMsg{err}
		}
		traffic, err := m.client.Traffic(ctx)
		if err != nil {
			return errMsg{err}
		}
		proxies, err := m.client.Proxies(ctx)
		if err != nil {
			return errMsg{err}
		}
		return switchResultMsg{
			status: fmt.Sprintf("switched %s → %s", groupName, optionName),
			data: refreshMsg{
				version: version,
				config:  config,
				traffic: traffic,
				proxies: proxies,
			},
		}
	}
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

	activePanelStyle   = panelBaseStyle.Copy().BorderForeground(lipgloss.Color("86"))
	inactivePanelStyle = panelBaseStyle.Copy().BorderForeground(lipgloss.Color("240"))

	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("62")).Padding(0, 1)
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62"))
	currentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("78")).Bold(true)
)

func (m model) renderHeader() string {
	innerWidth := max(60, m.width-4)
	titleRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		titleStyle.Render("Clash TUI"),
		"  ",
		subtitleStyle.Render("first-party Bubble Tea UI"),
	)

	metaRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		statusPill("endpoint", fallback(m.endpoint, "-")),
		statusPill("mode", fallback(m.mode, "unknown")),
		statusPill("system proxy", boolLabel(m.systemProxyEnabled)),
		statusPill("traffic ↑", formatBytes(m.up)),
		statusPill("traffic ↓", formatBytes(m.down)),
		statusPill("focus", m.focusLabel()),
	)

	content := lipgloss.JoinVertical(lipgloss.Left, titleRow, "", metaRow)
	return docStyle.Width(m.width).Render(headerStyle.Width(innerWidth).Render(content))
}

func statusPill(label, value string) string {
	pill := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1)
	return pill.Render(fmt.Sprintf("%s %s", mutedStyle.Render(label), value))
}

func (m model) renderBody(availableHeight int) string {
	innerWidth := max(60, m.width-4)
	leftWidth := max(26, innerWidth/3)
	rightWidth := max(36, innerWidth-leftWidth-2)
	panelHeight := max(8, availableHeight)

	groups := m.renderGroupsPanel(leftWidth, panelHeight)
	options := m.renderOptionsPanel(rightWidth, panelHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, groups, "  ", options)
	return docStyle.Width(m.width).Render(body)
}

func (m model) renderGroupsPanel(width, height int) string {
	style := inactivePanelStyle
	if m.focus == focusGroups {
		style = activePanelStyle
	}
	header := titleStyle.Render("Groups") + "\n" + subtitleStyle.Render("Tab / ←→ to switch focus")
	rows := m.visibleGroupRows(max(6, height-4))
	content := lipgloss.JoinVertical(lipgloss.Left, append([]string{header, ""}, rows...)...)
	return style.Width(width).Height(height).Render(content)
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
	rows := m.visibleOptionRows(max(6, height-4))
	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(title),
		subtitleStyle.Render(subtitle),
		"",
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	)
	return style.Width(width).Height(height).Render(content)
}

func (m model) visibleGroupRows(limit int) []string {
	if len(m.groups) == 0 {
		return []string{mutedStyle.Render("No groups match the current filter.")}
	}
	start, end := window(m.groupIndex, len(m.groups), limit)
	rows := make([]string, 0, end-start)
	for idx := start; idx < end; idx++ {
		group := m.groups[idx]
		prefix := "  "
		if idx == m.groupIndex {
			prefix = "▶ "
		}
		line := fmt.Sprintf("%s%-18s %s", prefix, truncate(group.Name, 18), mutedStyle.Render("["+fallback(group.Current, "-")+"]"))
		if idx == m.groupIndex {
			line = selectedStyle.Render(line)
		} else if group.Current != "" {
			line = currentStyle.Render(prefix+truncate(group.Name, 18)) + " " + mutedStyle.Render("["+group.Current+"]")
		}
		rows = append(rows, line)
	}
	return rows
}

func (m model) visibleOptionRows(limit int) []string {
	group := m.currentGroup()
	if group == nil || len(group.Options) == 0 {
		return []string{mutedStyle.Render("No selectable nodes in this group.")}
	}
	start, end := window(m.optionIndex, len(group.Options), limit)
	rows := make([]string, 0, end-start)
	for idx := start; idx < end; idx++ {
		option := group.Options[idx]
		marker := " "
		if option.Selected {
			marker = "✓"
		}
		delay := delayLabel(option.DelayMS)
		line := fmt.Sprintf("%s %-32s %s", marker, truncate(option.Name, 32), delay)
		if idx == m.optionIndex {
			line = selectedStyle.Render(line)
		}
		rows = append(rows, line)
	}
	return rows
}

func (m model) renderFooter() string {
	innerWidth := max(60, m.width-4)
	helpView := m.help.View(m.keys)
	left := statusStyle.Render(fallback(m.statusLine, "ready"))
	if m.searchMode {
		left = lipgloss.JoinHorizontal(lipgloss.Left, left, "  ", titleStyle.Render("Search:"), m.search.View())
	}
	row := lipgloss.JoinVertical(lipgloss.Left, left, mutedStyle.Render(helpView))
	return docStyle.Width(m.width).Render(headerStyle.Width(innerWidth).Render(row))
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

func (m model) focusLabel() string {
	if m.focus == focusGroups {
		return "groups"
	}
	return "options"
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
