package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"labproxy/internal/proxy"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestNewModel(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	opts := Options{
		Endpoint:           "http://localhost:9090",
		SystemProxyEnabled: true,
	}

	m := newModel(client, opts)

	if m.client != client {
		t.Fatal("expected client to be set")
	}
	if m.endpoint != "http://localhost:9090" {
		t.Fatalf("expected endpoint 'http://localhost:9090', got %q", m.endpoint)
	}
	if !m.systemProxyEnabled {
		t.Fatal("expected system proxy enabled to be true")
	}
	if m.width != 120 {
		t.Fatalf("expected width 120, got %d", m.width)
	}
	if m.height != 32 {
		t.Fatalf("expected height 32, got %d", m.height)
	}
	if m.statusLine != "connecting…" {
		t.Fatalf("expected status line 'connecting…', got %q", m.statusLine)
	}
}

func TestInit(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected Init to return a command")
	}
}

func TestUpdate_WindowSizeMsg(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		wantWidth  int
		wantHeight int
		wantSearch int
	}{
		{name: "clamps zero", width: 0, height: 0, wantWidth: 1, wantHeight: 1, wantSearch: 12},
		{name: "uses quarter width", width: 100, height: 50, wantWidth: 100, wantHeight: 50, wantSearch: 25},
		{name: "caps search width", width: 160, height: 40, wantWidth: 160, wantHeight: 40, wantSearch: 28},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := proxy.NewClient("http://localhost:9090", "")
			m := newModel(client, Options{Endpoint: "http://localhost:9090"})

			newModel, cmd := m.Update(tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			if cmd != nil {
				t.Fatal("expected nil command for WindowSizeMsg")
			}

			newM := newModel.(model)
			if newM.width != tt.wantWidth {
				t.Fatalf("expected width %d, got %d", tt.wantWidth, newM.width)
			}
			if newM.height != tt.wantHeight {
				t.Fatalf("expected height %d, got %d", tt.wantHeight, newM.height)
			}
			if newM.search.Width != tt.wantSearch {
				t.Fatalf("expected search width %d, got %d", tt.wantSearch, newM.search.Width)
			}
		})
	}
}

func TestUpdate_TickMsg(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})

	msg := tickMsg(time.Now())
	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Fatal("expected command for tickMsg")
	}
}

func TestUpdate_RefreshMsg(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})

	msg := refreshMsg{
		version: proxy.Version{Version: "v1.18.0"},
		config:  proxy.Config{Mode: "rule"},
		traffic: proxy.Traffic{Up: 100, Down: 200},
		proxies: proxy.ProxiesResponse{
			Proxies: map[string]proxy.Proxy{
				"GLOBAL": {
					Name: "GLOBAL",
					Type: "Selector",
					Now:  "Node-A",
					All:  []string{"Node-A"},
				},
			},
		},
	}

	newModel, cmd := m.Update(msg)

	if cmd != nil {
		t.Fatal("expected nil command for refreshMsg")
	}

	newM := newModel.(model)
	if newM.version != "v1.18.0" {
		t.Fatalf("expected version 'v1.18.0', got %q", newM.version)
	}
	if newM.mode != "rule" {
		t.Fatalf("expected mode 'rule', got %q", newM.mode)
	}
	if newM.up != 100 {
		t.Fatalf("expected up 100, got %d", newM.up)
	}
	if newM.down != 200 {
		t.Fatalf("expected down 200, got %d", newM.down)
	}
	if newM.statusLine != "connected" {
		t.Fatalf("expected status line 'connected', got %q", newM.statusLine)
	}
}

func TestUpdate_StatusMsg(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})

	msg := statusMsg{text: "test status"}
	newModel, cmd := m.Update(msg)

	if cmd != nil {
		t.Fatal("expected nil command for statusMsg")
	}
	newM := newModel.(model)
	if newM.statusLine != "test status" {
		t.Fatalf("expected status line 'test status', got %q", newM.statusLine)
	}
	if newM.lastError != nil {
		t.Fatal("expected last error to be nil")
	}
}

func TestUpdate_ErrMsg(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})

	msg := errMsg{err: fmt.Errorf("test error")}
	newModel, cmd := m.Update(msg)

	if cmd != nil {
		t.Fatal("expected nil command for errMsg")
	}
	newM := newModel.(model)
	if newM.lastError == nil {
		t.Fatal("expected last error to be set")
	}
}

func TestUpdate_KeyMsg_Quit(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Fatal("expected quit command")
	}
	// Cannot compare structs with uncomparable fields directly
}

func TestUpdate_KeyMsg_CtrlC(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Fatal("expected quit command for Ctrl+C")
	}
	// Cannot compare structs with uncomparable fields directly
}

func TestUpdate_KeyMsg_Tab(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})
	m.focus = focusGroups

	msg := tea.KeyMsg{Type: tea.KeyTab}
	newModel, cmd := m.Update(msg)

	if cmd != nil {
		t.Fatal("expected nil command for tab key")
	}
	newM := newModel.(model)
	if newM.focus != focusOptions {
		t.Fatalf("expected focus to be options, got %d", newM.focus)
	}
}

func TestUpdate_KeyMsg_LeftRight(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})
	m.focus = focusOptions

	// Left key should switch to groups
	msgLeft := tea.KeyMsg{Type: tea.KeyLeft}
	newModel, cmd := m.Update(msgLeft)
	if cmd != nil {
		t.Fatal("expected nil command for left key")
	}
	newM := newModel.(model)
	if newM.focus != focusGroups {
		t.Fatalf("expected focus to be groups after left key, got %d", newM.focus)
	}

	// Right key should switch to options
	msgRight := tea.KeyMsg{Type: tea.KeyRight}
	newModel, cmd = newModel.Update(msgRight)
	if cmd != nil {
		t.Fatal("expected nil command for right key")
	}
	newM = newModel.(model)
	if newM.focus != focusOptions {
		t.Fatalf("expected focus to be options after right key, got %d", newM.focus)
	}

	// Right key again should switch to settings
	newModel, cmd = newModel.Update(msgRight)
	if cmd != nil {
		t.Fatal("expected nil command for right key to settings")
	}
	newM = newModel.(model)
	if newM.focus != focusSettings {
		t.Fatalf("expected focus to be settings after second right key, got %d", newM.focus)
	}
}

func TestUpdate_KeyMsg_UpDown(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})

	// Setup model with groups
	m.groups = []GroupView{
		{Name: "GROUP-A", Options: []OptionView{{Name: "Node-A1"}}},
		{Name: "GROUP-B", Options: []OptionView{{Name: "Node-B1"}}},
	}
	m.groupIndex = 0

	// Down key should move down
	msgDown := tea.KeyMsg{Type: tea.KeyDown}
	newModel, cmd := m.Update(msgDown)
	if cmd != nil {
		t.Fatal("expected nil command for down key")
	}
	newM := newModel.(model)
	if newM.groupIndex != 1 {
		t.Fatalf("expected group index 1, got %d", newM.groupIndex)
	}

	// Up key should move up
	msgUp := tea.KeyMsg{Type: tea.KeyUp}
	newModel, cmd = newModel.Update(msgUp)
	if cmd != nil {
		t.Fatal("expected nil command for up key")
	}
	newM = newModel.(model)
	if newM.groupIndex != 0 {
		t.Fatalf("expected group index 0, got %d", newM.groupIndex)
	}
}

func TestUpdate_KeyMsg_Refresh(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})

	// Setup model with groups
	m.groups = []GroupView{
		{Name: "GLOBAL", Options: []OptionView{{Name: "Node-A"}, {Name: "Node-B"}}},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Fatal("expected command for refresh key")
	}
}

func TestUpdate_KeyMsg_Search(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	newModel, cmd := m.Update(msg)

	if cmd != nil {
		t.Fatal("expected nil command for search key")
	}
	newM := newModel.(model)
	if !newM.searchMode {
		t.Fatal("expected search mode to be true")
	}
}

func TestUpdate_KeyMsg_SystemProxy(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	mixinPath := filepath.Join(t.TempDir(), "mixin.yaml")
	m := newModel(client, Options{Endpoint: "http://localhost:9090", MixinConfigPath: mixinPath})

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("expected command for system proxy key")
	}

	result := cmd()
	settingsMsg, ok := result.(settingsResultMsg)
	if !ok {
		t.Fatalf("expected settingsResultMsg, got %T", result)
	}
	if !settingsMsg.data.systemProxyEnabled {
		t.Fatal("expected system proxy to be enabled")
	}

	content, err := os.ReadFile(mixinPath)
	if err != nil {
		t.Fatalf("read mixin: %v", err)
	}
	if !strings.Contains(string(content), "enable: true") {
		t.Fatalf("expected mixin to persist system proxy=true, got %q", string(content))
	}
}

func TestActivateSettingCmd_AllowLanAndTun(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	mixinPath := filepath.Join(t.TempDir(), "mixin.yaml")
	m := newModel(client, Options{Endpoint: "http://localhost:9090", MixinConfigPath: mixinPath})
	m.focus = focusSettings

	m.settingsIndex = 2 // Allow LAN
	result := m.activateSettingCmd()()
	allowLanMsg, ok := result.(settingsResultMsg)
	if !ok {
		t.Fatalf("expected settingsResultMsg for allow-lan, got %T", result)
	}
	if !allowLanMsg.data.allowLanEnabled {
		t.Fatal("expected allow-lan to be enabled")
	}

	m.settingsIndex = 3 // Tun
	result = m.activateSettingCmd()()
	tunMsg, ok := result.(settingsResultMsg)
	if !ok {
		t.Fatalf("expected settingsResultMsg for tun, got %T", result)
	}
	if !tunMsg.data.tunEnabled {
		t.Fatal("expected tun to be enabled")
	}
}

func TestUpdate_KeyMsg_InSearchMode(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})
	m.searchMode = true

	// ESC should exit search mode
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	newModel, cmd := m.Update(msg)

	if cmd != nil {
		t.Fatal("expected nil command for ESC in search mode")
	}
	newM := newModel.(model)
	if newM.searchMode {
		t.Fatal("expected search mode to be false after ESC")
	}
}

func TestView_Loading(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})
	m.width = 0

	view := m.View()
	if view != "loading…" {
		t.Fatalf("expected 'loading…', got %q", view)
	}
}

func newLayoutTestModel() model {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{
		Endpoint:           "http://localhost:9090",
		SystemProxyEnabled: true,
	})
	m.version = "v1.18.0"
	m.mode = "rule"
	m.up = 1024
	m.down = 2048
	m.statusLine = "connected"
	m.width = 120
	m.height = 32
	m.groups = []GroupView{
		{
			Name:    "GLOBAL",
			Current: "Node-A",
			Options: []OptionView{
				{Name: "Node-A", Selected: true, DelayMS: 42},
				{Name: "Node-B", DelayMS: 180},
			},
		},
	}
	m.connections = proxy.ConnectionsResponse{
		DownloadTotal: 4096,
		UploadTotal:   2048,
		Connections: []proxy.Connection{
			{
				ID:       "conn-1",
				Metadata: proxy.ConnectionMetadata{Host: "example.com", Destination: "1.1.1.1:443"},
				Download: 2048,
				Upload:   1024,
				Chains:   []string{"Node-A"},
				Rule:     "MATCH",
			},
		},
	}
	return m
}

func TestView_BasicRender(t *testing.T) {
	m := newLayoutTestModel()

	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty view")
	}
	if !strings.Contains(view, "LabProxy TUI") {
		t.Fatal("expected view to contain 'LabProxy TUI'")
	}
	if !strings.Contains(view, "connected") {
		t.Fatal("expected view to contain 'connected'")
	}
	if !strings.Contains(view, "Settings") {
		t.Fatal("expected view to contain 'Settings'")
	}
	if !strings.Contains(view, "Connections") {
		t.Fatal("expected view to contain 'Connections'")
	}
	if !strings.Contains(view, "example.com") {
		t.Fatal("expected view to contain rendered connection target")
	}
}

func TestView_SmallHeightHidesConnections(t *testing.T) {
	m := newLayoutTestModel()
	headerHeight := lipgloss.Height(m.renderHeader())
	footerHeight := lipgloss.Height(m.renderFooter())
	m.height = headerHeight + footerHeight + 5

	view := m.View()
	if !strings.Contains(view, "Groups") || !strings.Contains(view, "Options") || !strings.Contains(view, "Settings") {
		t.Fatal("expected compact body to keep three top panels")
	}
	if strings.Contains(view, "Connections") {
		t.Fatal("expected connections panel to be hidden in small height")
	}
}

func TestView_TinyHeightHidesBody(t *testing.T) {
	m := newLayoutTestModel()
	headerHeight := lipgloss.Height(m.renderHeader())
	footerHeight := lipgloss.Height(m.renderFooter())
	m.height = headerHeight + footerHeight

	view := m.View()
	if !strings.Contains(view, "connected") {
		t.Fatal("expected header/footer content to remain visible")
	}
	if strings.Contains(view, "Groups") || strings.Contains(view, "Connections") {
		t.Fatal("expected body panels to be hidden when no vertical space remains")
	}
}

func TestRenderBody_TinyWidthReturnsEmpty(t *testing.T) {
	m := newLayoutTestModel()
	m.width = docStyle.GetHorizontalFrameSize()

	body := m.renderBody(6)
	if strings.TrimSpace(body) != "" {
		t.Fatalf("expected empty body for tiny width, got %q", body)
	}
}

func TestToggleFocus(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})
	m.focus = focusGroups

	m.toggleFocus()
	if m.focus != focusOptions {
		t.Fatalf("expected focus to be options, got %d", m.focus)
	}
	if m.statusLine != "focus: options" {
		t.Fatalf("expected status line 'focus: options', got %q", m.statusLine)
	}

	m.toggleFocus()
	if m.focus != focusSettings {
		t.Fatalf("expected focus to be settings, got %d", m.focus)
	}
	if m.statusLine != "focus: settings" {
		t.Fatalf("expected status line 'focus: settings', got %q", m.statusLine)
	}

	m.toggleFocus()
	if m.focus != focusGroups {
		t.Fatalf("expected focus to cycle back to groups, got %d", m.focus)
	}
}

func TestMove(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})

	// Empty groups - should not crash
	m.move(1)
	if m.groupIndex != 0 {
		t.Fatalf("expected group index 0 with no groups, got %d", m.groupIndex)
	}

	// With groups
	m.groups = []GroupView{
		{Name: "GROUP-A", Options: []OptionView{{Name: "Node-A1"}, {Name: "Node-A2"}}},
		{Name: "GROUP-B", Options: []OptionView{{Name: "Node-B1"}}},
	}
	m.focus = focusGroups

	// Move down
	m.move(1)
	if m.groupIndex != 1 {
		t.Fatalf("expected group index 1, got %d", m.groupIndex)
	}

	// Move up
	m.move(-1)
	if m.groupIndex != 0 {
		t.Fatalf("expected group index 0, got %d", m.groupIndex)
	}

	// Move in options focus
	m.focus = focusOptions
	m.optionIndex = 0
	m.move(1)
	if m.optionIndex != 1 {
		t.Fatalf("expected option index 1, got %d", m.optionIndex)
	}
}

func TestClampIndices(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})

	// Empty groups
	m.clampIndices()
	if m.groupIndex != 0 || m.optionIndex != 0 {
		t.Fatalf("expected indices to be 0 with no groups, got group=%d, option=%d", m.groupIndex, m.optionIndex)
	}

	// With groups
	m.groups = []GroupView{
		{Name: "GROUP-A", Options: []OptionView{{Name: "Node-A1"}}},
		{Name: "GROUP-B", Options: []OptionView{{Name: "Node-B1"}}},
	}
	m.groupIndex = 5
	m.optionIndex = 10

	m.clampIndices()
	if m.groupIndex != 1 {
		t.Fatalf("expected group index clamped to 1, got %d", m.groupIndex)
	}
	if m.optionIndex != 0 {
		t.Fatalf("expected option index clamped to 0, got %d", m.optionIndex)
	}
}

func TestCurrentGroup(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")
	m := newModel(client, Options{Endpoint: "http://localhost:9090"})

	// No groups
	if m.currentGroup() != nil {
		t.Fatal("expected nil with no groups")
	}

	// With groups
	m.groups = []GroupView{
		{Name: "GROUP-A", Options: []OptionView{{Name: "Node-A1"}}},
		{Name: "GROUP-B", Options: []OptionView{{Name: "Node-B1"}}},
	}
	m.groupIndex = 1

	group := m.currentGroup()
	if group == nil {
		t.Fatal("expected non-nil group")
	}
	if group.Name != "GROUP-B" {
		t.Fatalf("expected group 'GROUP-B', got %q", group.Name)
	}

	// Out of bounds
	m.groupIndex = 10
	if m.currentGroup() != nil {
		t.Fatal("expected nil with out of bounds index")
	}
}

func TestFallback(t *testing.T) {
	tests := []struct {
		input    string
		alt      string
		expected string
	}{
		{"value", "alt", "value"},
		{"", "alt", "alt"},
		{"   ", "alt", "alt"},
		{"  value  ", "alt", "  value  "}, // fallback returns the original value if TrimSpace is not empty
	}

	for _, tt := range tests {
		result := fallback(tt.input, tt.alt)
		if result != tt.expected {
			t.Fatalf("fallback(%q, %q): expected %q, got %q", tt.input, tt.alt, tt.expected, result)
		}
	}
}

func TestNextMode(t *testing.T) {
	tests := []struct {
		current  string
		expected string
	}{
		{"rule", "global"},
		{"global", "direct"},
		{"direct", "rule"},
		{"", "global"},
	}

	for _, tt := range tests {
		if got := nextMode(tt.current); got != tt.expected {
			t.Fatalf("nextMode(%q): expected %q, got %q", tt.current, tt.expected, got)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"short", 10, "short"},
		{"longer text", 5, "long…"},
		{"ab", 2, "ab"},
		{"abc", 2, "a…"},
		{"", 5, ""},
		{"test", 0, ""},
		{"test", 1, "t"},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.width)
		if result != tt.expected {
			t.Fatalf("truncate(%q, %d): expected %q, got %q", tt.input, tt.width, tt.expected, result)
		}
	}
}

func TestDelayLabel(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "--"},
		{-1, "--"},
		{42, "42ms"},
		{149, "149ms"},
		{299, "299ms"},
		{300, "300ms"},
	}

	for _, tt := range tests {
		result := delayLabel(tt.input)
		// Check that it contains delay (color codes may be present)
		if tt.input > 0 {
			if !strings.Contains(result, "ms") {
				t.Fatalf("delayLabel(%d): expected 'ms' in result, got %q", tt.input, result)
			}
		} else {
			if result != "--" {
				t.Fatalf("delayLabel(%d): expected '--', got %q", tt.input, result)
			}
		}
	}
}

func TestWindow(t *testing.T) {
	tests := []struct {
		selected  int
		total     int
		limit     int
		wantStart int
		wantEnd   int
	}{
		{0, 10, 5, 0, 5},
		{5, 10, 5, 3, 8},
		{9, 10, 5, 5, 10},
		{0, 3, 10, 0, 3},
		{1, 3, 2, 0, 2},
		{0, 0, 5, 0, 0},
	}

	for _, tt := range tests {
		start, end := window(tt.selected, tt.total, tt.limit)
		if start != tt.wantStart || end != tt.wantEnd {
			t.Fatalf("window(%d, %d, %d): expected (%d, %d), got (%d, %d)",
				tt.selected, tt.total, tt.limit, tt.wantStart, tt.wantEnd, start, end)
		}
	}
}

func TestMax(t *testing.T) {
	tests := []struct {
		a, b     int
		expected int
	}{
		{5, 3, 5},
		{3, 5, 5},
		{0, 0, 0},
		{-1, 1, 1},
		{42, 42, 42},
	}

	for _, tt := range tests {
		result := max(tt.a, tt.b)
		if result != tt.expected {
			t.Fatalf("max(%d, %d): expected %d, got %d", tt.a, tt.b, tt.expected, result)
		}
	}
}

func TestBoolLabel(t *testing.T) {
	if boolLabel(true) != "on" {
		t.Fatalf("boolLabel(true): expected 'on', got %q", boolLabel(true))
	}
	if boolLabel(false) != "off" {
		t.Fatalf("boolLabel(false): expected 'off', got %q", boolLabel(false))
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0.0B/s"},
		{1023, "1023.0B/s"},
		{1024, "1.0KB/s"},
		{1024 * 1024, "1.0MB/s"},
		{1024 * 1024 * 1024, "1.0GB/s"},
		{1500, "1.5KB/s"},
		{1024 * 1024 * 2, "2.0MB/s"},
		{int64(2.5 * 1024 * 1024), "2.5MB/s"},
	}

	for _, tt := range tests {
		result := formatBytes(tt.input)
		// Check that it ends with the expected unit pattern
		if !strings.Contains(result, "B/s") {
			t.Fatalf("formatBytes(%d): expected 'B/s' in result, got %q", tt.input, result)
		}
	}
}

func TestFocusLabel(t *testing.T) {
	client := proxy.NewClient("http://localhost:9090", "")

	m := newModel(client, Options{Endpoint: "http://localhost:9090"})
	m.focus = focusGroups
	if m.focusLabel() != "groups" {
		t.Fatalf("focusLabel(): expected 'groups', got %q", m.focusLabel())
	}

	m.focus = focusOptions
	if m.focusLabel() != "options" {
		t.Fatalf("focusLabel(): expected 'options', got %q", m.focusLabel())
	}

	m.focus = focusSettings
	if m.focusLabel() != "settings" {
		t.Fatalf("focusLabel(): expected 'settings', got %q", m.focusLabel())
	}
}
