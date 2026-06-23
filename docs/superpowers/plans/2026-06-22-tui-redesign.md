# TUI 重构实现计划：Clash 式导航 + 视图标签

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `internal/tui/` 从「三栏同屏 + overlay」重构为「侧边导航 + 五视图标签」的 Clash 式架构，消除所有 overlay，拆分 `app.go`(1916 行) 为按视图组织的小文件。

**Architecture:** 顶层 `model` 持有全局状态与 `activeView` 枚举；`Update` 按当前视图分派；每个视图是独立文件，自包含子状态、Update 分支、View 渲染。overlay（settings/log/rules）全部改为常驻视图。复用现有 theme palette、消息类型、`viewmodel`、`rules/` 子包、`i18n`，不改后端 `internal/proxy`。

**Tech Stack:** Go 1.21+、Bubble Tea (`github.com/charmbracelet/bubbletea`)、Lip Gloss (`github.com/charmbracelet/lipgloss`)、Bubbles (`textinput`/`help`)。

**Spec:** `docs/superpowers/specs/2026-06-22-tui-redesign-design.md`

---

## 文件结构总览

重构后目录（任务逐步创建/迁移）：

```
internal/tui/
├── app.go            App/Run/顶层 model/Update 路由/Init  (~200行)
├── keys.go           keyMap 定义 + 各视图 binding
├── theme.go          从 app.go 迁出的色彩/样式 token
├── header.go         顶部状态条 renderHeader + statusPill
├── footer.go         底部键位栏 renderFooter + 动态 help
├── nav.go            左侧导航栏 + viewID 枚举 + 视图切换
├── view_proxies.go   Proxies 视图（group 列表 + 节点 list + 搜索 + 测速）
├── view_conns.go     Connections 视图（表格 + 关闭）
├── view_logs.go      Logs 视图（流 + 过滤 + 级别切换）
├── view_rules.go     Rules 视图（复用 rules/ 子包，常驻化）
├── view_config.go    Config 视图（复用现有 settings 逻辑，常驻化）
├── viewmodel.go      保留不动
├── i18n.go           保留，新增视图标签 key
├── helpers.go        fitLine/window/formatBytes/delayLabel 等纯函数
├── app_test.go       保留顶层路由测试（瘦身后）
├── view_proxies_test.go / view_conns_test.go / view_logs_test.go / ...
└── rules/            子包，保留不动
```

**迁移策略**：先建新骨架（theme/keys/nav/helpers + app 路由），再逐视图从旧 `app.go` 抽取迁移，每视图一个任务、独立可测、独立提交。旧代码在所有视图迁移完成后删除。

---

## 阶段 0：准备

### Task 0.1: 确认基线构建与测试通过

**Files:**
- 无修改，仅验证

- [ ] **Step 1: 运行现有测试**

Run: `go test ./internal/tui/... 2>&1 | tail -20`
Expected: PASS（记录通过数作为基线）

- [ ] **Step 2: 确认构建**

Run: `go build ./... 2>&1`
Expected: 无输出（成功）

- [ ] **Step 3: 记录基线**

Run: `go test ./internal/tui/... -v 2>&1 | grep -c "^--- PASS"`
Expected: 记录数字（迁移后不应减少）

---

## 阶段 1：新骨架（抽取共用部分，保持编译通过）

> 本阶段新增文件并从 app.go 删除对应定义，保持编译通过。

### Task 1.1: 创建 theme.go，迁移样式定义

**Files:**
- Create: `internal/tui/theme.go`
- Modify: `internal/tui/app.go`（删除 1138-1207 行的 `var(...)` palette 块）

- [ ] **Step 1: 创建 theme.go，将 app.go 1138-1207 行的整个 `var ( ... )` 块（colorPrimary 到 fitLineStyle）原样移入**

```go
package tui

import "github.com/charmbracelet/lipgloss"

// theme.go 集中管理所有色彩 token 与样式定义。
// 详见 docs/superpowers/specs/2026-06-22-tui-redesign-design.md §7。

var (
	// ── Theme palette ──────────────────────────────────────────────────
	colorPrimary      = lipgloss.Color("39")     // bright blue — identity & structure
	colorAccent       = lipgloss.Color("86")     // bright cyan-green — focus & active
	colorSurfaceHigh  = lipgloss.Color("62")     // deep indigo — selection bg

	colorTextPrimary   = lipgloss.Color("252")   // near-white
	colorTextSecondary = lipgloss.Color("246")   // mid-gray
	colorTextMuted     = lipgloss.Color("243")   // dim gray

	// Semantic: state & delay colors
	colorSuccess = lipgloss.Color("42")  // green  — low delay / on
	colorWarning = lipgloss.Color("220") // yellow — mid delay
	colorInfo    = lipgloss.Color("117") // light blue
	colorDanger  = lipgloss.Color("203") // red — high delay / off / error
	colorOrange  = lipgloss.Color("215") // orange — mid-high delay

	// ── Layout constants ──────────────────────────────────────────────
	columnGap = 2

	docStyle = lipgloss.NewStyle().Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1)

	panelBaseStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)

	activePanelStyle   = panelBaseStyle.BorderForeground(colorAccent)
	inactivePanelStyle = panelBaseStyle.BorderForeground(lipgloss.Color("237"))

	navActiveStyle = lipgloss.NewStyle().
			Foreground(colorAccent).Bold(true)

	// ── Typography ──────────────────────────────────────────────────────
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	subtitleStyle = lipgloss.NewStyle().Foreground(colorTextSecondary)

	// ── Status & feedback ──────────────────────────────────────────────
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(colorSurfaceHigh).
			Padding(0, 1)
	mutedStyle    = lipgloss.NewStyle().Foreground(colorTextMuted)
	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorTextPrimary).
			Background(colorSurfaceHigh)
	currentStyle = lipgloss.NewStyle().
			Foreground(colorAccent).Bold(true)

	onStyle  = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	offStyle = lipgloss.NewStyle().Foreground(colorTextMuted)

	fitLineStyle = lipgloss.NewStyle()
)
```

- [ ] **Step 2: 从 app.go 删除被迁移的 `var(...)` 块（1138 行 `var (` 到 1207 行 `)`）**

- [ ] **Step 3: 扩展延迟色阶函数 getDelayStyle（替换 app.go:1794 旧实现）**

先在 app.go 删除旧 `getDelayStyle`，再在 theme.go 加入：

```go
// getDelayStyle 按延迟返回色阶样式：<50 绿 / <150 黄 / <300 橙 / ≥300 红 / -1 暗红 / 0 灰。
func getDelayStyle(ms int) lipgloss.Style {
	switch {
	case ms <= 0:
		if ms == -1 {
			return lipgloss.NewStyle().Foreground(colorDanger)
		}
		return mutedStyle
	case ms < 50:
		return lipgloss.NewStyle().Foreground(colorSuccess)
	case ms < 150:
		return lipgloss.NewStyle().Foreground(colorWarning)
	case ms < 300:
		return lipgloss.NewStyle().Foreground(colorOrange)
	default:
		return lipgloss.NewStyle().Foreground(colorDanger)
	}
}
```

- [ ] **Step 4: 编译验证**

Run: `go build ./internal/tui/... 2>&1`
Expected: 无输出

- [ ] **Step 5: 测试验证**

Run: `go test ./internal/tui/... 2>&1 | tail -5`
Expected: PASS（与基线一致）

- [ ] **Step 6: Commit**

```bash
git add internal/tui/theme.go internal/tui/app.go
git commit -m "refactor(tui): extract theme tokens to theme.go with delay color scale"
```

---

### Task 1.2: 创建 helpers.go，迁移纯函数

**Files:**
- Create: `internal/tui/helpers.go`
- Modify: `internal/tui/app.go`（删除迁移的函数）

- [ ] **Step 1: 创建 helpers.go，迁移以下纯函数（原样从 app.go 移入）**

迁移的函数（按 app.go 行号）：`fitLine`(1718)、`fitStyledLine`(1725)、`renderPanelContent`(1736)、`renderPanel`(1758)、`plainDelayLabel`(1773)、`fallback`(1783)、`truncate`(1790)、`delayLabel`(1808)、`window`(1815)、`max`(1831)、`boolLabel`(1838)、`modeLabel`(1858)、`nextMode`(1866)、`connectionTarget`(1877)、`formatBytes`(1896)、`formatSize`(1907)。

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"labproxy/internal/proxy"
)

// helpers.go 存放无状态的渲染辅助纯函数。

// （各函数原样从 app.go 迁移，函数体不变）
```

> 实施：逐个函数从 app.go 剪切粘贴到 helpers.go，函数体保持不变。`import` 按实际用到的包填写。

- [ ] **Step 2: 从 app.go 删除上述函数定义**

- [ ] **Step 3: 编译验证**

Run: `go build ./internal/tui/... 2>&1`
Expected: 无输出

- [ ] **Step 4: 测试验证**

Run: `go test ./internal/tui/... 2>&1 | tail -5`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/helpers.go internal/tui/app.go
git commit -m "refactor(tui): extract pure helpers to helpers.go"
```

---

### Task 1.3: 定义 viewID 枚举与导航状态

**Files:**
- Create: `internal/tui/nav.go`、`internal/tui/nav_test.go`

- [ ] **Step 1: 写 nav_test.go**

```go
package tui

import "testing"

func TestViewID_Next(t *testing.T) {
	cases := []struct{ in, want viewID }{
		{viewProxies, viewConnections},
		{viewConnections, viewLogs},
		{viewLogs, viewRules},
		{viewRules, viewConfig},
		{viewConfig, viewProxies}, // wrap
	}
	for _, c := range cases {
		if got := c.in.next(); got != c.want {
			t.Errorf("%v.next() = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestViewID_Label(t *testing.T) {
	if viewProxies.label() == "" {
		t.Error("proxies label empty")
	}
}

func TestViewID_ByDigit(t *testing.T) {
	cases := []struct {
		digit string
		want  viewID
		ok    bool
	}{
		{"1", viewProxies, true},
		{"2", viewConnections, true},
		{"3", viewLogs, true},
		{"4", viewRules, true},
		{"5", viewConfig, true},
		{"6", viewProxies, false},
		{"0", viewProxies, false},
	}
	for _, c := range cases {
		got, ok := viewByDigit(c.digit)
		if ok != c.ok {
			t.Errorf("viewByDigit(%q) ok=%v want %v", c.digit, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("viewByDigit(%q) = %v, want %v", c.digit, got, c.want)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/tui/ -run TestViewID 2>&1 | tail -10`
Expected: FAIL（`viewID` 未定义）

- [ ] **Step 3: 实现 nav.go**

```go
package tui

// viewID 标识五个常驻视图，对应 Clash dashboard 的核心页。
type viewID int

const (
	viewProxies viewID = iota
	viewConnections
	viewLogs
	viewRules
	viewConfig
)

var viewOrder = []viewID{viewProxies, viewConnections, viewLogs, viewRules, viewConfig}

// next 返回下一个视图（循环）。
func (v viewID) next() viewID {
	for i, x := range viewOrder {
		if x == v {
			return viewOrder[(i+1)%len(viewOrder)]
		}
	}
	return viewOrder[0]
}

// label 返回视图在导航栏与标题中的显示名（走 i18n）。
func (v viewID) label() string {
	switch v {
	case viewProxies:
		return T().NavProxies
	case viewConnections:
		return T().NavConnections
	case viewLogs:
		return T().NavLogs
	case viewRules:
		return T().NavRules
	case viewConfig:
		return T().NavConfig
	}
	return ""
}

// shortKey 是导航栏显示的单字符快捷键。
func (v viewID) shortKey() string {
	switch v {
	case viewProxies:
		return "1"
	case viewConnections:
		return "2"
	case viewLogs:
		return "3"
	case viewRules:
		return "4"
	case viewConfig:
		return "5"
	}
	return ""
}

// viewByDigit 将 "1".."5" 映射到视图，超出范围返回 ok=false。
func viewByDigit(digit string) (viewID, bool) {
	for _, v := range viewOrder {
		if v.shortKey() == digit {
			return v, true
		}
	}
	return viewProxies, false
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/tui/ -run TestViewID 2>&1 | tail -10`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/nav.go internal/tui/nav_test.go
git commit -m "feat(tui): add viewID enum and navigation state"
```

---

### Task 1.4: i18n 新增视图标签 key

**Files:**
- Modify: `internal/tui/i18n.go`

- [ ] **Step 1: 在 Dict 结构体新增字段**

```go
	// Navigation bar
	NavProxies      string
	NavConnections  string
	NavLogs         string
	NavRules        string
	NavConfig       string
	AppSubtitle     string // 顶部副标题

	// Help overlay
	HelpTitle string
```

- [ ] **Step 2: 在 en 字典填充值**

```go
		NavProxies:     "Proxies",
		NavConnections: "Connections",
		NavLogs:        "Logs",
		NavRules:       "Rules",
		NavConfig:      "Config",
		AppSubtitle:    "labproxy",
		HelpTitle:      "Keybindings",
```

- [ ] **Step 3: 在 zh 字典填充值**

```go
		NavProxies:     "代理",
		NavConnections: "连接",
		NavLogs:        "日志",
		NavRules:       "规则",
		NavConfig:      "配置",
		AppSubtitle:    "labproxy",
		HelpTitle:      "快捷键",
```

- [ ] **Step 4: 编译验证**

Run: `go build ./internal/tui/... 2>&1`
Expected: 无输出

- [ ] **Step 5: Commit**

```bash
git add internal/tui/i18n.go
git commit -m "feat(tui/i18n): add nav bar and help labels"
```

---

## 阶段 2：视图骨架与路由切换

> 本阶段改造 `app.go` 的 `model`/`Update`/`View`，引入 `activeView`，建立路由。旧三栏渲染暂保留为 Proxies 视图占位，后续阶段逐个替换。

### Task 2.1: model 引入 activeView 字段

**Files:**
- Modify: `internal/tui/app.go`（model struct, newModel）

- [ ] **Step 1: 在 model struct 新增字段（focus paneFocus 之后）**

```go
	activeView viewID
```

保留 `focus`/`settingsMode`/`logMode` 等旧字段，迁移期间共存。

- [ ] **Step 2: newModel 初始化**

在返回的 `model{...}` 中新增 `activeView: viewProxies,`。

- [ ] **Step 3: 编译验证**

Run: `go build ./internal/tui/... 2>&1`
Expected: 无输出

- [ ] **Step 4: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(tui): add activeView field to model"
```

---

### Task 2.2: Update 中加入全局视图切换键（1-5/Tab）

**Files:**
- Modify: `internal/tui/app.go`（Update 函数 KeyMsg 分支）
- Modify: `internal/tui/app_test.go`

- [ ] **Step 1: 写路由测试（若 newTestModel helper 不存在则先加，参考现有 app_test.go 构造方式）**

```go
func TestUpdate_SwitchViewByDigit(t *testing.T) {
	m := newTestModel()
	for digit, want := range map[string]viewID{
		"2": viewConnections, "3": viewLogs, "4": viewRules, "5": viewConfig,
	} {
		mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(digit)})
		m2 := mm.(model)
		if m2.activeView != want {
			t.Errorf("digit %q: activeView=%v want %v", digit, m2.activeView, want)
		}
		mm, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
		m = mm.(model)
		if m.activeView != viewProxies {
			t.Errorf("digit 1: activeView=%v want %v", m.activeView, viewProxies)
		}
	}
}

func TestUpdate_TabCyclesView(t *testing.T) {
	m := newTestModel()
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if mm.(model).activeView != viewConnections {
		t.Errorf("tab: activeView=%v want %v", mm.(model).activeView, viewConnections)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/tui/ -run TestUpdate_SwitchView -v 2>&1 | tail -15`
Expected: FAIL

- [ ] **Step 3: 在 Update 的 KeyMsg 分支开头（searchMode/settingsMode 判断之前）插入全局视图切换**

```go
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
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/tui/ -run TestUpdate_SwitchView -v 2>&1 | tail -15`
Expected: PASS

- [ ] **Step 5: 全量回归并修复受影响的旧 Tab 测试**

Run: `go test ./internal/tui/... 2>&1 | tail -10`

旧 Tab 测试（断言 paneFocus 切换）需改为断言 activeView 或改用 h/l 测 pane 焦点。逐个修正语义。

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat(tui): global view switching via 1-5 and Tab"
```

---

### Task 2.3: View() 按 activeView 路由（过渡期保留旧渲染）

**Files:**
- Modify: `internal/tui/app.go`（View 函数）

- [ ] **Step 1: View() 增加注释分支（零行为变化，保留所有现有 return）**

```go
func (m model) View() string {
	if m.width <= 0 {
		return T().Loading
	}

	// 阶段过渡：后续按 activeView 分派 body，当前仍走旧三栏。
	_ = m.activeView

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
```

- [ ] **Step 2: 编译+测试**

Run: `go build ./internal/tui/... && go test ./internal/tui/... 2>&1 | tail -5`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/tui/app.go
git commit -m "refactor(tui): View routes by activeView (transitional)"
```

---

### Task 2.4: nav.go 渲染 + 接入左侧导航栏

**Files:**
- Modify: `internal/tui/nav.go`（新增 renderNav）
- Modify: `internal/tui/app.go`（renderBody 接入 nav）

- [ ] **Step 1: nav.go 新增 renderNav（顶部补 `import "fmt"`）**

```go
// renderNav 渲染左侧窄导航栏：每项显示 [digit] label，当前项加 ▌ 前缀与高亮。
func renderNav(active viewID, height int) string {
	rows := make([]string, 0, len(viewOrder))
	for _, v := range viewOrder {
		marker := " "
		style := mutedStyle
		if v == active {
			marker = "▌"
			style = navActiveStyle
		}
		rows = append(rows, style.Render(fmt.Sprintf("%s%s %s", marker, v.shortKey(), v.label())))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return panelBaseStyle.
		BorderForeground(colorPrimary).
		Width(14).
		Height(height).
		Render(content)
}
```

- [ ] **Step 2: app.go 将旧三栏拼接逻辑重命名为 renderOldThreePane，新 renderBody 负责 nav + 调用它**

```go
func (m model) renderBody(availableHeight int) string {
	docWidth := max(0, m.width-docStyle.GetHorizontalFrameSize())
	if availableHeight <= 0 || docWidth <= 0 {
		return docStyle.Width(docWidth).Render("")
	}

	navWidth := 14 + panelBaseStyle.GetHorizontalFrameSize()
	nav := renderNav(m.activeView, availableHeight)
	rest := m.renderOldThreePane(max(0, docWidth-navWidth-1), availableHeight)
	return lipgloss.JoinHorizontal(lipgloss.Left, nav, " ", rest)
}
```

`renderOldThreePane` 内容 = 原 `renderBody` 三栏拼接（groups+options+connections）。

- [ ] **Step 3: 编译验证**

Run: `go build ./internal/tui/... 2>&1`
Expected: 无输出

- [ ] **Step 4: 构建 TUI 验证产物生成**

Run: `VERSION=dev bash scripts/build-tui.sh 2>&1 | tail -3`
Expected: 成功生成 `bin/labproxy-tui`

- [ ] **Step 5: 测试**

Run: `go test ./internal/tui/... 2>&1 | tail -5`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tui/nav.go internal/tui/app.go
git commit -m "feat(tui): render left nav bar with active view highlight"
```

---

## 阶段 3：Proxies 视图（替换旧三栏中的 groups+options）

### Task 3.1: 创建 view_proxies.go，迁移 group/option 渲染

**Files:**
- Create: `internal/tui/view_proxies.go`
- Modify: `internal/tui/app.go`（移除迁出的函数）

- [ ] **Step 1: 迁移以下函数（原样）到 view_proxies.go**

`renderGroupsPanel`(1370)、`renderOptionsPanel`(1385)、`visibleGroupRows`、`visibleOptionRows`、`rebuildGroups`(570)、`clampIndices`(611)、`currentGroup`(1148)、`calcGroupsMinWidth`(1240)。

```go
package tui

// view_proxies.go: Proxies 视图（group 列表 + 节点 list）。
// 迁移自旧 app.go 的 groups/options 面板逻辑。

// （函数体原样迁入，签名不变）
```

- [ ] **Step 2: 从 app.go 删除迁出的函数**

- [ ] **Step 3: 新增 renderProxiesView**

```go
// renderProxiesView 渲染 Proxies 视图主体：左 group 列表 + 右节点 list。
func (m model) renderProxiesView(width, height int) string {
	groupsWidth := m.calcGroupsMinWidth(width - columnGap)
	optionsWidth := max(0, width-groupsWidth-columnGap)
	groups := m.renderGroupsPanel(groupsWidth, height)
	options := m.renderOptionsPanel(optionsWidth, height)
	return lipgloss.JoinHorizontal(lipgloss.Left, groups, strings.Repeat(" ", columnGap), options)
}
```

- [ ] **Step 4: renderBody 路由：Proxies 视图调用 renderProxiesView（不再含 Connections 列）**

`renderOldThreePane` 改为按 `activeView` 分派：

```go
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
```

> 本步引入了对其它视图渲染函数的调用——这些函数在后续任务才实现。为保持编译通过，本步**只**接入 `viewProxies` 分支，其余分支用占位 `rest = mutedStyle.Render("TODO")`，后续任务逐个替换为真实渲染。删除旧 `renderOldThreePane`。

- [ ] **Step 5: 编译+测试**

Run: `go build ./internal/tui/... && go test ./internal/tui/... 2>&1 | tail -5`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tui/view_proxies.go internal/tui/app.go
git commit -m "refactor(tui): extract proxies view, route body by activeView"
```

---

### Task 3.2: Proxies 视图键位：h/l 切 group/option 焦点

**Files:**
- Modify: `internal/tui/app.go`（Update KeyMsg）
- Create: `internal/tui/view_proxies_test.go`

- [ ] **Step 1: 写测试**

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestProxiesFocus_RightMovesToOptions(t *testing.T) {
	m := newTestModel()
	m.activeView = viewProxies
	m.focus = focusGroups
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if mm.(model).focus != focusOptions {
		t.Errorf("l: focus=%v want %v", mm.(model).focus, focusOptions)
	}
}

func TestProxiesFocus_LeftBackToGroups(t *testing.T) {
	m := newTestModel()
	m.activeView = viewProxies
	m.focus = focusOptions
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if mm.(model).focus != focusGroups {
		t.Errorf("h: focus=%v want %v", mm.(model).focus, focusGroups)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/tui/ -run TestProxiesFocus -v 2>&1 | tail -10`
Expected: FAIL 或已通过（旧 h/l 全局移动）——需确保仅在 viewProxies 生效

- [ ] **Step 3: Update 中 h/l 加视图前置**

```go
		case m.activeView == viewProxies && key.Matches(msg, m.keys.Left):
			m.moveFocus(-1)
			return m, nil
		case m.activeView == viewProxies && key.Matches(msg, m.keys.Right):
			m.moveFocus(1)
			return m, nil
```

旧 h/l 分支加 `m.activeView == viewProxies` 前置。

- [ ] **Step 4: 测试通过**

Run: `go test ./internal/tui/ -run TestProxiesFocus -v 2>&1 | tail -10`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/view_proxies_test.go
git commit -m "feat(tui): h/l focus switch in proxies view"
```

---

### Task 3.3: Proxies 视图搜索与测速（绑定到视图）

**Files:**
- Modify: `internal/tui/app.go`

- [ ] **Step 1: 写测试：非 Proxies 视图按 / 不进入搜索**

```go
func TestSearchOnlyInProxiesView(t *testing.T) {
	m := newTestModel()
	m.activeView = viewConnections
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if mm.(model).searchMode {
		t.Error("searchMode should not activate outside proxies view")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/tui/ -run TestSearchOnly -v 2>&1 | tail -10`
Expected: FAIL

- [ ] **Step 3: Search/TestGroup 分支加视图前置**

```go
		case m.activeView == viewProxies && key.Matches(msg, m.keys.Search):
			m.searchMode = true
			m.search.Focus()
			m.statusLine = T().TypeToFilter
			return m, nil
		...
		case m.activeView == viewProxies && key.Matches(msg, m.keys.TestGroup):
			return m, m.testGroupCmd()
```

- [ ] **Step 4: 全量回归**

Run: `go test ./internal/tui/... 2>&1 | tail -5`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/view_proxies_test.go
git commit -m "feat(tui): scope search and test-group to proxies view"
```

---

## 阶段 4：Connections 视图

### Task 4.1: 创建 view_conns.go，迁移连接渲染

**Files:**
- Create: `internal/tui/view_conns.go`
- Modify: `internal/tui/app.go`（移除 renderConnectionsPanel/visibleConnectionRows/clampConnIndex/handleConnectionCloseKey）

- [ ] **Step 1: 迁移函数到 view_conns.go**

迁移：`renderConnectionsPanel`(1467)、`visibleConnectionRows`(1683)、`clampConnIndex`(556)、`handleConnectionCloseKey`、连接关闭相关 cmd。

- [ ] **Step 2: 新增 renderConnectionsView**

```go
// renderConnectionsView 渲染 Connections 视图：占满主区的连接表。
func (m model) renderConnectionsView(width, height int) string {
	m2 := m
	m2.focus = focusConnections // 复用旧 focus 高亮逻辑
	return m2.renderConnectionsPanel(width, height)
}
```

- [ ] **Step 3: renderBody 中 viewConnections 分支替换占位**

```go
	case viewConnections:
		rest = m.renderConnectionsView(contentWidth, availableHeight)
```

- [ ] **Step 4: 编译+测试**

Run: `go build ./internal/tui/... && go test ./internal/tui/... 2>&1 | tail -5`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/view_conns.go internal/tui/app.go
git commit -m "refactor(tui): extract connections view to view_conns.go"
```

---

### Task 4.2: Connections 视图键位（j/k 移动、d/D 关闭）

**Files:**
- Modify: `internal/tui/app.go`、`internal/tui/view_conns.go`
- Create: `internal/tui/view_conns_test.go`

- [ ] **Step 1: 写测试**

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"labproxy/internal/proxy"
)

func testConnections(n int) proxy.ConnectionsResponse {
	resp := proxy.ConnectionsResponse{}
	for i := 0; i < n; i++ {
		resp.Connections = append(resp.Connections, proxy.Connection{ID: string(rune('a' + i))})
	}
	return resp
}

func TestConnections_JKMovesCursor(t *testing.T) {
	m := newTestModel()
	m.activeView = viewConnections
	m.connections = testConnections(3)
	m.connIndex = 0
	down, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if down.(model).connIndex != 1 {
		t.Errorf("j: connIndex=%d want 1", down.(model).connIndex)
	}
	up, _ := down.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if up.(model).connIndex != 0 {
		t.Errorf("k: connIndex=%d want 0", up.(model).connIndex)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/tui/ -run TestConnections_JK -v 2>&1 | tail -10`
Expected: FAIL

- [ ] **Step 3: Update 中 j/k/↑↓ 在 viewConnections 移动 connIndex（复用现有 m.keys.Down/Up，已含 j/k）**

```go
		case m.activeView == viewConnections && key.Matches(msg, m.keys.Down):
			if len(m.connections.Connections) > 0 {
				m.connIndex = (m.connIndex + 1) % len(m.connections.Connections)
			}
			return m, nil
		case m.activeView == viewConnections && key.Matches(msg, m.keys.Up):
			if len(m.connections.Connections) > 0 {
				m.connIndex = (m.connIndex - 1 + len(m.connections.Connections)) % len(m.connections.Connections)
			}
			return m, nil
```

- [ ] **Step 4: 测试通过**

Run: `go test ./internal/tui/ -run TestConnections_JK -v 2>&1 | tail -10`
Expected: PASS

- [ ] **Step 5: 确保 d/D 关闭连接在 viewConnections 生效（旧 handleConnectionCloseKey 加视图前置）**

Run: `go test ./internal/tui/... 2>&1 | tail -5`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go internal/tui/view_conns_test.go internal/tui/view_conns.go
git commit -m "feat(tui): j/k cursor in connections view"
```

---

## 阶段 5：Logs 视图（常驻化，去掉 overlay）

### Task 5.1: 创建 view_logs.go，迁移日志渲染

**Files:**
- Create: `internal/tui/view_logs.go`
- Modify: `internal/tui/app.go`（移除 renderLogOverlay、handleLogKey）

- [ ] **Step 1: 迁移 renderLogOverlay(1408) 与 handleLogKey(805) 到 view_logs.go，改造为 renderLogsView**

```go
// renderLogsView 渲染 Logs 视图：全屏日志列表 + 级别指示。
// 复用原 renderLogOverlay 的着色与截断逻辑，去掉 overlay 全屏遮罩。
func (m model) renderLogsView(width, height int) string {
	// 迁移原 renderLogOverlay 主体，去掉 logMode 判断
}
```

- [ ] **Step 2: 进入 Logs 视图自动开始订阅；切离时 cancel**

在 Task 2.2 的视图切换分支扩展（viewByDigit 与 Tab 共用）：

```go
		// 切到 viewLogs：启动订阅
		if v == viewLogs && !m.logActive {
			m.logActive = true
			if m.logLevel == "" {
				m.logLevel = "info"
			}
			ctx, cancel := context.WithCancel(context.Background())
			m.logCancel = cancel
			m.logCtx = ctx
			m.activeView = v
			m.statusLine = v.label()
			return m, tea.Batch(m.logsCmd(ctx))
		}
		// 切离 viewLogs：停止订阅
		if m.activeView == viewLogs && v != viewLogs && m.logCancel != nil {
			m.logCancel()
			m.logActive = false
		}
```

> 注意：需重构视图切换逻辑为：先计算目标 v，再处理订阅副作用，最后设置 activeView。把 Task 2.2 的两个分支（viewByDigit 与 Tab）合并为「计算目标视图 → 副作用 → 设置」。

- [ ] **Step 3: renderBody 中 viewLogs 分支替换占位**

```go
	case viewLogs:
		rest = m.renderLogsView(contentWidth, availableHeight)
```

- [ ] **Step 4: 删除 View() 中 `if m.logMode { return m.renderLogOverlay() }` 顶层短路**

- [ ] **Step 5: 编译+测试（旧 logMode 测试改为 viewLogs 断言）**

Run: `go build ./internal/tui/... && go test ./internal/tui/... 2>&1 | tail -5`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tui/view_logs.go internal/tui/app.go
git commit -m "refactor(tui): promote logs overlay to persistent view"
```

---

### Task 5.2: Logs 视图过滤 (/) 与级别切换 (l)

**Files:**
- Modify: `internal/tui/view_logs.go`、`internal/tui/app.go`
- Create: `internal/tui/view_logs_test.go`

- [ ] **Step 1: model 新增 `logFilter string`**

- [ ] **Step 2: 写测试**

```go
package tui

import (
	"testing"

	"labproxy/internal/proxy"
)

func TestLogsFilter(t *testing.T) {
	m := newTestModel()
	m.activeView = viewLogs
	m.logEntries = []proxy.LogEntry{
		{Level: "info", Message: "started"},
		{Level: "error", Message: "boom"},
	}
	m.logFilter = "boom"
	rows := m.filteredLogEntries()
	if len(rows) != 1 || rows[0].Message != "boom" {
		t.Errorf("filter=%v rows=%+v", m.logFilter, rows)
	}
}
```

- [ ] **Step 3: 运行确认失败**

Run: `go test ./internal/tui/ -run TestLogsFilter -v 2>&1 | tail -10`
Expected: FAIL

- [ ] **Step 4: 实现 filteredLogEntries**

```go
func (m model) filteredLogEntries() []proxy.LogEntry {
	if m.logFilter == "" {
		return m.logEntries
	}
	f := strings.ToLower(m.logFilter)
	out := make([]proxy.LogEntry, 0, len(m.logEntries))
	for _, e := range m.logEntries {
		if strings.Contains(strings.ToLower(e.Message), f) {
			out = append(out, e)
		}
	}
	return out
}
```

- [ ] **Step 5: renderLogsView 使用 filteredLogEntries；search 确认分支按视图分派**

当 `activeView==viewLogs` 时，search 确认将 `search.Value()` 写入 `logFilter`：

```go
		case key.Matches(msg, m.keys.Select) && m.searchMode:
			m.searchMode = false
			m.search.Blur()
			if m.activeView == viewLogs {
				m.logFilter = m.search.Value()
				m.statusLine = fmt.Sprintf(T().FilterLabelFmt, fallback(m.logFilter, T().FilterNone))
			} else {
				m.rebuildGroups()
				m.statusLine = fmt.Sprintf(T().FilterLabelFmt, fallback(m.search.Value(), T().FilterNone))
			}
			return m, nil
```

- [ ] **Step 6: 测试通过**

Run: `go test ./internal/tui/ -run TestLogsFilter -v 2>&1 | tail -10`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/tui/view_logs.go internal/tui/app.go internal/tui/view_logs_test.go
git commit -m "feat(tui): log filtering in logs view"
```

---

## 阶段 6：Config 视图（常驻化 settings）

### Task 6.1: 创建 view_config.go，迁移 settings 渲染

**Files:**
- Create: `internal/tui/view_config.go`
- Modify: `internal/tui/app.go`（移除 renderSettingsOverlay/settingsItems/visibleSettingRows/activateSettingCmd/cycleModeCmd/toggleSystemProxyCmd/settingAction/settingItem）

- [ ] **Step 1: 迁移函数与类型到 view_config.go**

迁移：`renderSettingsOverlay`(1438)、`settingsItems`(1590)、`visibleSettingRows`(1604)、`activateSettingCmd`、`cycleModeCmd`、`toggleSystemProxyCmd`、`settingAction`/`settingItem` 类型。

- [ ] **Step 2: 改造为 renderConfigView**

```go
// renderConfigView 渲染 Config 视图：复用 settingsItems + visibleSettingRows。
func (m model) renderConfigView(width, height int) string {
	// 迁移原 renderSettingsOverlay 主体，去掉 overlay 全屏遮罩
}
```

- [ ] **Step 3: renderBody 中 viewConfig 分支替换占位**

```go
	case viewConfig:
		rest = m.renderConfigView(contentWidth, availableHeight)
```

- [ ] **Step 4: Update 中 settingsMode 逻辑改为 activeView==viewConfig**

将 `if m.settingsMode { ... }` 分支的条件改为 `if m.activeView == viewConfig { ... }`，键位逻辑不变（j/k 移动 settingsIndex，Enter 激活）。

- [ ] **Step 5: 删除 View() 中 `if m.settingsMode { return m.renderSettingsOverlay() }` 顶层短路；移除 settingsMode 字段及全部引用**

Run: `rg -n "settingsMode" internal/tui/`
逐个替换为 `activeView == viewConfig` 判断后删除字段。

- [ ] **Step 6: 编译+测试**

Run: `go build ./internal/tui/... && go test ./internal/tui/... 2>&1 | tail -5`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/tui/view_config.go internal/tui/app.go
git commit -m "refactor(tui): promote settings overlay to persistent config view"
```

---

## 阶段 7：Rules 视图（常驻化 rules modal）

### Task 7.1: 创建 view_rules.go，将 rules modal 接入为常驻视图

**Files:**
- Create: `internal/tui/view_rules.go`
- Modify: `internal/tui/app.go`

- [ ] **Step 1: 新增 renderRulesView**

```go
package tui

// renderRulesView 渲染 Rules 视图：调用 rulesModal.View() 放入主区。
func (m model) renderRulesView(width, height int) string {
	if m.rulesModal == nil {
		return mutedStyle.Render("rules unavailable")
	}
	return m.rulesModal.View()
}
```

- [ ] **Step 2: Update 切换到 viewRules 时 modal.Open()，切离时 modal.Close()**

在视图切换副作用中：

```go
		if v == viewRules && m.rulesModal != nil && !m.rulesModal.IsOpen() {
			m.rulesModal.Open()
		}
		if m.activeView == viewRules && v != viewRules && m.rulesModal != nil {
			m.rulesModal.Close()
		}
```

- [ ] **Step 3: renderBody 中 viewRules 分支替换占位**

```go
	case viewRules:
		rest = m.renderRulesView(contentWidth, availableHeight)
```

- [ ] **Step 4: KeyMsg 中 modal 处理加视图前置**

```go
		if m.activeView == viewRules && m.rulesModal != nil && m.rulesModal.IsOpen() {
			if m.rulesModal.Update(msg) {
				return m, nil
			}
		}
```

- [ ] **Step 5: 删除旧 R 键 binding（m.keys.Rules）与对应 case**

- [ ] **Step 6: 编译+测试**

Run: `go build ./internal/tui/... && go test ./internal/tui/... 2>&1 | tail -5`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/tui/view_rules.go internal/tui/app.go
git commit -m "refactor(tui): promote rules modal to persistent rules view"
```

---

## 阶段 8：header/footer/keys 抽离与帮助浮层

### Task 8.1: 创建 header.go 与 footer.go

**Files:**
- Create: `internal/tui/header.go`、`internal/tui/footer.go`
- Modify: `internal/tui/app.go`（移除 renderHeader/renderFooter/statusPill/focusLabel）

- [ ] **Step 1: 迁移 renderHeader(1207)/statusPill(1228) 到 header.go**

- [ ] **Step 2: 迁移 renderFooter(1703)/focusLabel(1887) 到 footer.go**

- [ ] **Step 3: footer 随 activeView 动态显示键位**

```go
func (m model) footerKeyHint() string {
	switch m.activeView {
	case viewProxies:
		return "1-5 view  / search  t test  h/l focus  j/k move  enter switch  ? help  q quit"
	case viewConnections:
		return "1-5 view  j/k move  d close  D close all  ? help  q quit"
	case viewLogs:
		return "1-5 view  / filter  l level  ? help  q quit"
	case viewRules:
		return "1-5 view  a add  enter edit  d delete  ? help  q quit"
	case viewConfig:
		return "1-5 view  j/k move  enter toggle  ? help  q quit"
	}
	return ""
}
```

renderFooter 使用 `m.footerKeyHint()` 替代旧 `m.help.View(m.keys)`。

- [ ] **Step 4: 编译+测试**

Run: `go build ./internal/tui/... && go test ./internal/tui/... 2>&1 | tail -5`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/header.go internal/tui/footer.go internal/tui/app.go
git commit -m "refactor(tui): extract header and footer, dynamic key hints"
```

---

### Task 8.2: 创建 keys.go，迁移 keyMap

**Files:**
- Create: `internal/tui/keys.go`
- Modify: `internal/tui/app.go`（移除 keyMap 类型与 newModel 中的 keys 初始化）

- [ ] **Step 1: 迁移 keyMap struct(70)、ShortHelp(89)、FullHelp(93) 到 keys.go**

- [ ] **Step 2: 新增 defaultKeyMap()（从 newModel 抽出 keys 初始化）**

```go
func defaultKeyMap() keyMap {
	return keyMap{
		Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", T().HelpMoveUp)),
		Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", T().HelpMoveDown)),
		Left:        key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", T().HelpFocusLeft)),
		Right:       key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", T().HelpFocusRight)),
		Tab:         key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", T().HelpSwitchPane)),
		Select:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", T().HelpApplySelect)),
		Refresh:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", T().HelpRefreshDelay)),
		Search:      key.NewBinding(key.WithKeys("/"), key.WithHelp("/", T().HelpSearch)),
		Mode:        key.NewBinding(key.WithKeys("m"), key.WithHelp("m", T().HelpCycleMode)),
		SystemProxy: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", T().HelpToggleProxyPref)),
		Back:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", T().HelpCloseBack)),
		Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", T().HelpQuit)),
		TestGroup:   key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "test group")),
	}
}
```

> 移除旧 `Settings`/`Rules`/`Logs` binding（已常驻化）。

- [ ] **Step 3: newModel 调用 `keys: defaultKeyMap(),`**

- [ ] **Step 4: 编译+测试**

Run: `go build ./internal/tui/... && go test ./internal/tui/... 2>&1 | tail -5`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/keys.go internal/tui/app.go
git commit -m "refactor(tui): extract keyMap to keys.go"
```

---

### Task 8.3: 帮助浮层（?）

**Files:**
- Modify: `internal/tui/app.go`

- [ ] **Step 1: model 新增 `helpMode bool`**

- [ ] **Step 2: 写测试**

```go
func TestHelpOverlay_Toggle(t *testing.T) {
	m := newTestModel()
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !mm.(model).helpMode {
		t.Fatal("? did not open help")
	}
	mm2, _ := mm.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if mm2.(model).helpMode {
		t.Fatal("? did not close help")
	}
}
```

- [ ] **Step 3: 运行确认失败**

Run: `go test ./internal/tui/ -run TestHelpOverlay -v 2>&1 | tail -10`
Expected: FAIL

- [ ] **Step 4: Update 中 `?` 切换 helpMode（最高优先级）**

```go
		if !m.searchMode && msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == '?' {
			m.helpMode = !m.helpMode
			return m, nil
		}
```

- [ ] **Step 5: View 中 helpMode 时渲染帮助浮层**

```go
	if m.helpMode {
		return m.renderHelpOverlay()
	}
```

新增 `renderHelpOverlay`（放 footer.go 或新建 help.go）：显示全局键位 + 当前视图键位，`?`/`esc` 关闭。

- [ ] **Step 6: 测试通过**

Run: `go test ./internal/tui/ -run TestHelpOverlay -v 2>&1 | tail -10`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat(tui): add ? help overlay"
```

---

## 阶段 9：清理与验收

### Task 9.1: 删除遗留死代码

**Files:**
- Modify: `internal/tui/app.go`

- [ ] **Step 1: 搜索并删除残留**

Run: `rg -n "settingsMode|logMode|m.keys.Rules|m.keys.Logs|toggleFocus" internal/tui/`
Expected: 无输出（全部已迁移）

- [ ] **Step 2: 清理 toggleFocus 中 Connections 相关分支（若 pane 焦点仅 Proxies 保留）**

- [ ] **Step 3: 编译+测试**

Run: `go build ./internal/tui/... && go test ./internal/tui/... 2>&1 | tail -5`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/tui/app.go
git commit -m "refactor(tui): remove dead overlay code"
```

---

### Task 9.2: app.go 瘦身校验与文件行数检查

**Files:**
- 无修改，仅验证

- [ ] **Step 1: 检查行数**

Run: `wc -l internal/tui/*.go | sort -n`
Expected: `app.go` < 250 行，所有文件 < 800 行

- [ ] **Step 2: 若 app.go 仍过大，继续抽取剩余渲染函数到对应 view_*.go**

- [ ] **Step 3: 全量测试**

Run: `go test ./internal/tui/... -v 2>&1 | grep -E "^(--- PASS|--- FAIL|PASS|FAIL)" | tail -20`
Expected: 全 PASS，通过数 ≥ 基线

- [ ] **Step 4: go vet**

Run: `go vet ./internal/tui/... 2>&1`
Expected: 无输出

- [ ] **Step 5: Commit（如有改动）**

```bash
git add internal/tui/
git commit -m "refactor(tui): final slim-down of app.go"
```

---

### Task 9.3: 更新 README 键位表

**Files:**
- Modify: `README.md`

- [ ] **Step 1: 定位 TUI Interface 段落**

Run: `rg -n "## TUI Interface" README.md`

- [ ] **Step 2: 替换键位说明**

```markdown
## TUI Interface

\`\`\`
labproxy tui
\`\`\`

The TUI uses a Clash-style left nav with five persistent views:

| Key | View        | Actions                              |
|-----|-------------|--------------------------------------|
| 1   | Proxies     | / search, t test, h/l focus, enter switch |
| 2   | Connections | j/k move, d close, D close all       |
| 3   | Logs        | / filter, l level                    |
| 4   | Rules       | a add, enter edit, d delete          |
| 5   | Config      | j/k move, enter toggle               |

Global: `1-5` switch view, `Tab` cycle, `r` refresh, `?` help, `q` quit.
```

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: update TUI keybinding table for new nav design"
```

---

### Task 9.4: 构建预编译二进制并最终验证

**Files:**
- 无修改

- [ ] **Step 1: 构建 TUI**

Run: `VERSION=dev bash scripts/build-tui.sh 2>&1 | tail -5`
Expected: 成功生成 `bin/labproxy-tui`

- [ ] **Step 2: 全量测试**

Run: `go test ./... 2>&1 | tail -10`
Expected: PASS

- [ ] **Step 3: 覆盖率检查**

Run: `go test ./internal/tui/... -cover 2>&1 | tail -3`
Expected: 记录覆盖率（目标 ≥ 80%）

- [ ] **Step 4: 最终提交（如有未提交改动）**

```bash
git status
# 若有改动：
git add -A && git commit -m "chore(tui): rebuild precompiled binaries"
```

---

## 自检结果

**1. Spec 覆盖：**
- §4 架构（导航+视图状态机）→ Task 2.1-2.4
- §5 五视图 → Task 3.x / 4.x / 5.x / 6.x / 7.x
- §6 键位 → Task 2.2 / 3.2 / 4.2 / 5.2 / 8.2
- §7 视觉（token 化+色阶+圆角+▌）→ Task 1.1 / 2.4
- §8 文件拆分 → 阶段 1-8 全部
- §9 数据流（复用消息）→ 全程未改消息类型，Task 5.1 复用 logsCmd
- §10 测试 → 每个功能 Task 都含 TDD 步骤
- §11 迁移风险 → 分阶段 + 每任务独立提交 + 基线对比（Task 0.1/9.2）

**2. 占位符扫描：** 无 TBD/TODO（`renderBody` 中的占位是过渡期明确设计，后续任务替换）；所有代码步骤含完整代码或明确迁移指令。

**3. 类型一致性：** `viewID`/`viewProxies` 等在 Task 1.3 定义，后续全程一致；`renderProxiesView`/`renderConnectionsView` 等命名统一；`activeView` 字段在 2.1 引入后全程一致。

---

## 执行交接

计划已保存到 `docs/superpowers/plans/2026-06-22-tui-redesign.md`。两种执行方式：

**1. Subagent-Driven（推荐）** — 每个 Task 派发新 subagent，任务间 review，迭代快
**2. Inline Execution** — 在当前会话用 executing-plans 批量执行，带检查点

选哪种？
