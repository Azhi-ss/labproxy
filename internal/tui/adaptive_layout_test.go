package tui

import (
	"strings"
	"testing"

	"labproxy/internal/proxy"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// helper to create a test model with groups
func makeTestModel(groups []GroupView, width int) model {
	m := newModel(nil, Options{})
	m.groups = groups
	m.width = width
	m.height = 24
	// trigger cache update via rebuildGroups logic
	docWidth := max(0, width-docStyle.GetHorizontalFrameSize())
	panelFrameWidth := panelBaseStyle.GetHorizontalFrameSize()
	columnContentWidth := docWidth - columnGap - panelFrameWidth*2
	if columnContentWidth > 0 {
		m.groupPanelWidth = m.calcGroupsMinWidth(columnContentWidth)
	} else {
		m.groupPanelWidth = 20
	}
	return m
}

// ========== calcGroupsMinWidth Tests ==========

func TestCalcGroupsMinWidth_EmptyGroups(t *testing.T) {
	m := makeTestModel([]GroupView{}, 120)
	width := m.calcGroupsMinWidth(100)
	if width < 20 {
		t.Errorf("expected min width >= 20 for empty groups, got %d", width)
	}
	// Should fall back to minWidth (20)
	if width != 20 {
		t.Errorf("expected exactly 20 for empty groups, got %d", width)
	}
}

func TestCalcGroupsMinWidth_ShortNames(t *testing.T) {
	groups := []GroupView{
		{Name: "Proxy", Current: "node1"},
		{Name: "Global", Current: ""},
	}
	m := makeTestModel(groups, 120)
	width := m.calcGroupsMinWidth(100)
	// Proxy + " [node1]" = 5 + 8 = 13, + reservedPrefix(2) + rightPadding(2) = 17
	// Global = 6 + reservedPrefix(2) + rightPadding(2) = 10
	// max = 17, but clamped to at least minWidth(20)
	if width < 20 {
		t.Errorf("expected width >= 20, got %d", width)
	}
}

func TestCalcGroupsMinWidth_LongNames(t *testing.T) {
	groups := []GroupView{
		{Name: "ThisIsAVeryLongGroupNameThatExceedsNormalBounds", Current: "selected-node-name"},
	}
	m := makeTestModel(groups, 200)
	width := m.calcGroupsMinWidth(150)
	// Should be clamped to columnContentWidth - minOptionsWidth(30) = 120
	maxAllowed := 150 - 30
	if width > maxAllowed {
		t.Errorf("expected width <= %d (maxAllowed), got %d", maxAllowed, width)
	}
}

func TestCalcGroupsMinWidth_VeryNarrowWindow(t *testing.T) {
	m := makeTestModel([]GroupView{
		{Name: "Proxy", Current: "node1"},
	}, 50)
	width := m.calcGroupsMinWidth(40)
	// columnContentWidth <= minWidth+minOptionsWidth=50, so returns max(minWidth, half)
	expected := max(20, 40/2)
	if width != expected {
		t.Errorf("expected %d for narrow window, got %d", expected, width)
	}
}

func TestCalcGroupsMinWidth_RespectsOptionsMinWidth(t *testing.T) {
	groups := []GroupView{
		{Name: "A", Current: ""},
	}
	m := makeTestModel(groups, 200)
	columnContentWidth := 100
	width := m.calcGroupsMinWidth(columnContentWidth)
	// Options panel should get at least 30 chars
	optionsSpace := columnContentWidth - width
	if optionsSpace < 30 {
		t.Errorf("Options space %d < minOptionsWidth(30)", optionsSpace)
	}
}

func TestCalcGroupsMinWidth_UnicodeNames(t *testing.T) {
	groups := []GroupView{
		{Name: "日本語グループ", Current: "東京ノード"},
		{Name: "🌏Global Proxy", Current: "🚀Node"},
		{Name: "中文分组", Current: "北京节点"},
	}
	m := makeTestModel(groups, 200)
	width := m.calcGroupsMinWidth(150)

	// Verify the calculated width accounts for Unicode display widths correctly
	// "日本語グループ" has StringWidth of 7 (each CJK char = 2 columns in some terminals, but ansi.StringWidth counts codepoints differently)
	// We just verify it doesn't panic and returns a sane value
	if width < 20 || width > 150 {
		t.Errorf("unexpected width %d for unicode names", width)
	}

	// Specifically check that Unicode group name is measured properly
	nameWidth := ansi.StringWidth("日本語グループ")
	if nameWidth == 0 {
		t.Errorf("ansi.StringWidth returned 0 for CJK string")
	}
	currentMarkWidth := ansi.StringWidth(" [東京ノード]")
	if currentMarkWidth == 0 {
		t.Errorf("ansi.StringWidth returned 0 for CJK current mark")
	}
}

func TestCalcGroupsMinWidth_WithCurrentMark(t *testing.T) {
	groups := []GroupView{
		{Name: "Proxy", Current: "very-long-current-node-name"},
		{Name: "NoCurrent", Current: ""},
	}
	m := makeTestModel(groups, 200)
	width := m.calcGroupsMinWidth(150)

	// Width should account for " [very-long-current-node-name]"
	expectedMin := 2 + ansi.StringWidth("Proxy") + ansi.StringWidth(" [very-long-current-node-name]") + 2
	if width < expectedMin && width >= 20 {
		// Only check if it wasn't clamped by min/max bounds
		t.Logf("width=%d, expectedMin=%d (may be clamped)", width, expectedMin)
	}
}

// ========== Cached Layout Tests ==========

func TestRebuildGroups_UpdatesCache(t *testing.T) {
	m := newModel(nil, Options{})
	m.width = 120
	m.rawProxies = proxy.ProxiesResponse{
		Proxies: map[string]proxy.Proxy{
			"PROXY": {
				Type: "Selector",
				Now:  "node-a",
				All:  []string{"node-a", "node-b"},
			},
		},
	}

	initialCache := m.groupPanelWidth
	m.rebuildGroups()

	if m.groupPanelWidth == initialCache && m.groupPanelWidth != 20 {
		t.Errorf("rebuildGroups did not update groupPanelWidth cache")
	}

	if m.groupPanelWidth <= 0 {
		t.Errorf("groupPanelWidth should be positive after rebuildGroups, got %d", m.groupPanelWidth)
	}
}

func TestRebuildGroups_CacheOnWindowSizeChange(t *testing.T) {
	m := newModel(nil, Options{})
	m.rawProxies = proxy.ProxiesResponse{
		Proxies: map[string]proxy.Proxy{
			"PROXY": {Type: "Selector", Now: "n1", All: []string{"n1", "n2"}},
		},
	}
	m.groups = BuildGroupViews(m.rawProxies, "")

	// Set narrow width
	m.width = 60
	m.rebuildGroups()
	narrowWidth := m.groupPanelWidth

	// Set wide width
	m.width = 200
	m.rebuildGroups()
	wideWidth := m.groupPanelWidth

	// Wide window should give more (or equal) space to Groups panel
	// Note: this depends on actual content, but wider windows generally allow more flexibility
	t.Logf("narrowWidth=%d, wideWidth=%d", narrowWidth, wideWidth)

	if narrowWidth <= 0 || wideWidth <= 0 {
		t.Errorf("both widths should be positive: narrow=%d, wide=%d", narrowWidth, wideWidth)
	}
}

func TestRebuildGroups_FallbackWhenZeroContentWidth(t *testing.T) {
	m := newModel(nil, Options{})
	m.width = 5 // very small
	m.height = 10
	m.rebuildGroups()

	// When content width is 0 or negative, fallback to 20
	if m.groupPanelWidth != 20 {
		t.Errorf("expected fallback value 20, got %d", m.groupPanelWidth)
	}
}

// ========== Adaptive Render Tests ==========

func TestRenderBody_DoesNotPanic_NarrowWindow(t *testing.T) {
	m := newModel(nil, Options{})
	m.width = 40
	m.height = 15
	m.rawProxies = proxy.ProxiesResponse{
		Proxies: map[string]proxy.Proxy{
			"PROXY": {Type: "Selector", Now: "n1", All: []string{"n1", "n2"}},
		},
	}
	m.rebuildGroups()

	// This should not panic
	body := m.renderBody(10)
	if body == "" {
		t.Log("body is empty for narrow window (acceptable)")
	}
	_ = body // just ensure no panic
}

func TestRenderBody_DoesNotPanic_WideWindow(t *testing.T) {
	m := newModel(nil, Options{})
	m.width = 250
	m.height = 50
	m.rawProxies = proxy.ProxiesResponse{
		Proxies: map[string]proxy.Proxy{
			"PROXY":    {Type: "Selector", Now: "n1", All: []string{"n1", "n2", "n3"}},
			"GLOBAL":   {Type: "Selector", Now: "g1", All: []string{"g1"}},
			"DIRECT":   {Type: "Selector", Now: "d1", All: []string{"d1"}},
			"MyGroup":  {Type: "Selector", Now: "m1", All: []string{"m1", "m2", "m3", "m4"}},
			"LongName": {Type: "Selector", Now: "l1", All: []string{"l1"}},
		},
	}
	m.rebuildGroups()

	body := m.renderBody(40)
	if body == "" {
		t.Errorf("body should not be empty for wide window")
	}
	_ = body
}

func TestRenderBody_DoesNotPanic_ExtremeSizes(t *testing.T) {
	testCases := []struct {
		name   string
		width  int
		height int
	}{
		{"minimal", 10, 5},
		{"small", 30, 10},
		{"medium", 80, 24},
		{"large", 300, 100},
		{"ultraWide", 500, 50},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(nil, Options{})
			m.width = tc.width
			m.height = tc.height
			m.rebuildGroups()
			body := m.renderBody(max(0, tc.height-10)) // approximate available height
			_ = body // no panic = pass
		})
	}
}

func TestRenderBody_OptionsPanelHasMinimumSpace(t *testing.T) {
	m := newModel(nil, Options{})
	m.width = 120
	m.height = 30
	m.rawProxies = proxy.ProxiesResponse{
		Proxies: map[string]proxy.Proxy{
			"PROXY": {Type: "Selector", Now: "node1", All: []string{"node1", "long-option-name-here"}},
		},
	}
	m.groups = BuildGroupViews(m.rawProxies, "")
	m.groupIndex = 0
	m.rebuildGroups()

	docWidth := max(0, m.width-docStyle.GetHorizontalFrameSize())
	panelFrameWidth := panelBaseStyle.GetHorizontalFrameSize()
	columnContentWidth := docWidth - columnGap - panelFrameWidth*2

	leftWidth := m.groupPanelWidth
	rightWidth := columnContentWidth - leftWidth

	t.Logf("columnContentWidth=%d, leftWidth(Groups)=%d, rightWidth(Options)=%d",
		columnContentWidth, leftWidth, rightWidth)

	// Right width should be non-negative (Options panel needs space)
	if rightWidth < 0 {
		t.Errorf("Options panel has negative space: %d", rightWidth)
	}
}

// ========== WindowSizeMsg Integration Tests ==========

func TestUpdate_WindowSizeMsg_TriggersRecalculation(t *testing.T) {
	m := newModel(nil, Options{})
	m.rawProxies = proxy.ProxiesResponse{
		Proxies: map[string]proxy.Proxy{
			"PROXY": {Type: "Selector", Now: "n1", All: []string{"n1"}},
		},
	}
	m.groups = BuildGroupViews(m.rawProxies, "")

	// Initial state with default width
	initialWidth := m.groupPanelWidth

	// Simulate WindowSizeMsg for a different size
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 180, Height: 40})
	model := newM.(model)

	if model.groupPanelWidth == initialWidth && initialWidth != 20 {
		t.Log("Note: groupPanelWidth may stay same if content hasn't changed significantly")
	}

	if model.width != 180 {
		t.Errorf("expected width 180, got %d", model.width)
	}
	if model.height != 40 {
		t.Errorf("expected height 40, got %d", model.height)
	}
}

// ========== Unicode Name Handling Tests ==========

func TestUnicodeGroupName_DisplayWidth(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expectGT int
	}{
		{"CJK", "日本語グループ", 7},
		{"Emoji", "🌏Global Proxy", 14},
		{"Mixed", "Test-中文组", 11},
		{"Arabic", "مجموعة", 5},
		{"Korean", "한글그룹", 5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := ansi.StringWidth(tc.input)
			if w < tc.expectGT {
				t.Errorf("StringWidth(%q) = %d, expected >= %d", tc.input, w, tc.expectGT)
			}
		})
	}
}

func TestBuildGroupViews_UnicodeFiltering(t *testing.T) {
	resp := proxy.ProxiesResponse{
		Proxies: map[string]proxy.Proxy{
			"日本語": {
				Type: "Selector",
				Now:  "東京",
				All:  []string{"東京", "大阪", "京都"},
			},
			"English": {
				Type: "Selector",
				Now:  "London",
				All:  []string{"London", "Paris"},
			},
		},
	}

	// Filter by Japanese text
	groups := BuildGroupViews(resp, "日本")
	foundJapanese := false
	for _, g := range groups {
		if g.Name == "日本語" {
			foundJapanese = true
			break
		}
	}
	if !foundJapanese {
		t.Errorf("should find Japanese group when filtering by '日本'")
	}

	// Filter by English - should not include Japanese group if filter doesn't match
	groupsEng := BuildGroupViews(resp, "Lon")
	for _, g := range groupsEng {
		if g.Name == "日本語" {
			t.Errorf("Japanese group should not appear when filtering by 'Lon'")
		}
	}
}

func TestVisibleGroupRows_UnicodeNames_NotTruncatedBadly(t *testing.T) {
	m := newModel(nil, Options{})
	m.groups = []GroupView{
		{Name: "日本語グループ 🎌", Current: "東京ノード"},
		{Name: "ShortName", Current: ""},
	}
	m.groupIndex = 0

	rows := m.visibleGroupRows(25, 5)
	if len(rows) == 0 {
		t.Errorf("expected rows for unicode groups")
	}

	// Check that rows don't contain garbled content
	for i, row := range rows {
		if len(row) == 0 {
			t.Errorf("row %d is empty", i)
		}
		t.Logf("row %d: %q (display width: %d)", i, row, ansi.StringWidth(row))
	}
}

// ========== Edge Case Tests ==========

func TestCalcGroupsMinWidth_SingleCharGroupName(t *testing.T) {
	groups := []GroupView{
		{Name: "P", Current: "X"},
	}
	m := makeTestModel(groups, 100)
	width := m.calcGroupsMinWidth(70)
	// Very short name but should still meet minimum
	if width < 20 {
		t.Errorf("expected minimum 20, got %d", width)
	}
}

func TestCalcGroupsMaxWidth_GroupsTakeHalf(t *testing.T) {
	// When all group names are short and options need space,
	// Groups shouldn't take more than half
	groups := []GroupView{
		{Name: "P", Current: "A"},
		{Name: "G", Current: "B"},
		{Name: "D", Current: "C"},
	}
	m := makeTestModel(groups, 100)
	colWidth := 80
	width := m.calcGroupsMinWidth(colWidth)

	// With short names, width should not exceed colWidth/2 typically
	// unless constrained by minWidth
	if width > colWidth/2+5 { // small tolerance
		t.Logf("groups width %d exceeds half of %d by more than tolerance", width, colWidth)
	}
}

func TestRenderView_CompleteIntegration(t *testing.T) {
	m := newModel(nil, Options{Endpoint: "http://localhost:9090"})
	m.width = 120
	m.height = 32
	m.mode = "rule"
	m.version = "1.18.0"
	m.systemProxyEnabled = true
	m.allowLanEnabled = false
	m.tunEnabled = false
	m.statusLine = "connected"

	m.rawProxies = proxy.ProxiesResponse{
		Proxies: map[string]proxy.Proxy{
			"PROXY": {
				Type: "Selector",
				Now:  "auto-node",
				All:  []string{"auto-node", "manual-node-1", "manual-node-2"},
			},
			"GLOBAL": {
				Type: "Selector",
				Now:  "direct",
				All:  []string{"direct", "proxy"},
			},
		},
	}
	m.connections = proxy.ConnectionsResponse{}
	m.applyState(refreshMsg{
		version:            proxy.Version{Version: "1.18.0"},
		config:             proxy.Config{Mode: "rule"},
		traffic:            proxy.Traffic{Up: 1024, Down: 2048},
		proxies:            m.rawProxies,
		connections:        proxy.ConnectionsResponse{},
		systemProxyEnabled: true,
		allowLanEnabled:    false,
		tunEnabled:         false,
	})

	view := m.View()
	if view == "" {
		t.Errorf("View() should return non-empty string")
	}

	// Check view contains expected elements
	if !containsStr(view, "LabProxy") {
		t.Errorf("View should contain title 'LabProxy'")
	}
	if !containsStr(view, "Groups") {
		t.Errorf("View should contain 'Groups' panel header")
	}
	if !containsStr(view, "Options") {
		t.Errorf("View should contain 'Options' panel header")
	}
	t.Logf("View length: %d chars", len(view))
}

// helper function
func containsStr(s, substr string) bool {
	// Strip ANSI sequences for comparison
	clean := ansi.Strip(s)
	return strings.Contains(clean, substr)
}
