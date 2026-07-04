package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// renderProxiesView 渲染 Proxies 视图主体：左 group 列表 + 右节点 list。
func (m model) renderProxiesView(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if width < proxyStackWidth {
		return m.renderProxiesStack(width, height)
	}

	t := m.theme
	layout := m.proxyLayout(width, panelBaseStyle(t).GetHorizontalFrameSize())
	groups := m.renderGroupsPanel(layout.groupsWidth, height)
	options := m.renderOptionsPanel(layout.optionsWidth, height)
	if !layout.showDetails {
		return lipgloss.JoinHorizontal(lipgloss.Left, groups, strings.Repeat(" ", columnGap), options)
	}
	details := m.renderDetailsPanel(layout.detailsWidth, height)
	return lipgloss.JoinHorizontal(lipgloss.Left, groups, strings.Repeat(" ", columnGap), options, strings.Repeat(" ", columnGap), details)
}

const (
	proxyStackWidth      = 72
	proxyThreeColumnMin  = 126
	proxyMinGroupsWidth  = 20
	proxyMaxGroupsWidth  = 34
	proxyMinOptionsWidth = 36
	proxyMinDetailsWidth = 30
	proxyMaxDetailsWidth = 42
)

type proxyViewLayout struct {
	groupsWidth  int
	optionsWidth int
	detailsWidth int
	showDetails  bool
}

func (m model) renderProxiesStack(width, height int) string {
	frameWidth := panelBaseStyle(m.theme).GetHorizontalFrameSize()
	panelWidth := max(0, width-frameWidth)
	if panelWidth <= 0 {
		return ""
	}
	if height < 7 {
		return m.renderGroupsPanel(panelWidth, height)
	}
	groupsHeight := max(3, min(height/2, height-3))
	optionsHeight := max(0, height-groupsHeight)
	groups := m.renderGroupsPanel(panelWidth, groupsHeight)
	options := m.renderOptionsPanel(panelWidth, optionsHeight)
	return lipgloss.JoinVertical(lipgloss.Left, groups, options)
}

func (m model) proxyLayout(width, frameWidth int) proxyViewLayout {
	if width >= proxyThreeColumnMin {
		if layout, ok := m.proxyThreeColumnLayout(width, frameWidth); ok {
			return layout
		}
	}

	columnContentWidth := max(0, width-columnGap-frameWidth*2)
	groupsWidth := min(m.calcGroupsMinWidth(columnContentWidth), columnContentWidth)
	optionsWidth := max(0, columnContentWidth-groupsWidth)
	return proxyViewLayout{
		groupsWidth:  groupsWidth,
		optionsWidth: optionsWidth,
	}
}

func (m model) proxyThreeColumnLayout(width, frameWidth int) (proxyViewLayout, bool) {
	columnContentWidth := max(0, width-columnGap*2-frameWidth*3)
	if columnContentWidth < proxyMinGroupsWidth+proxyMinOptionsWidth+proxyMinDetailsWidth {
		return proxyViewLayout{}, false
	}

	groupsWidth := clamp(m.calcGroupsMinWidth(columnContentWidth), proxyMinGroupsWidth, proxyMaxGroupsWidth)
	detailsWidth := clamp(columnContentWidth/4, proxyMinDetailsWidth, proxyMaxDetailsWidth)
	optionsWidth := columnContentWidth - groupsWidth - detailsWidth

	if optionsWidth < proxyMinOptionsWidth {
		shortfall := proxyMinOptionsWidth - optionsWidth
		detailsShrink := min(shortfall, max(0, detailsWidth-proxyMinDetailsWidth))
		detailsWidth -= detailsShrink
		shortfall -= detailsShrink
		groupsShrink := min(shortfall, max(0, groupsWidth-proxyMinGroupsWidth))
		groupsWidth -= groupsShrink
		optionsWidth = columnContentWidth - groupsWidth - detailsWidth
	}

	if optionsWidth < proxyMinOptionsWidth || detailsWidth < proxyMinDetailsWidth {
		return proxyViewLayout{}, false
	}

	return proxyViewLayout{
		groupsWidth:  groupsWidth,
		optionsWidth: optionsWidth,
		detailsWidth: detailsWidth,
		showDetails:  true,
	}, true
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

func (m model) currentGroup() *GroupView {
	if len(m.groups) == 0 || m.groupIndex >= len(m.groups) {
		return nil
	}
	return &m.groups[m.groupIndex]
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

func (m model) renderGroupsPanel(width, height int) string {
	t := m.theme
	style := panelBaseStyle(t)
	content := renderPanelContent(
		t,
		focusPanelTitle(T().PanelGroups, m.focus == focusGroups),
		T().PanelGroupsHint,
		m.visibleGroupRows(width, max(0, height)),
		width,
		height,
	)
	return renderPanel(style, width, height, content)
}

func (m model) renderOptionsPanel(width, height int) string {
	t := m.theme
	style := panelBaseStyle(t)
	group := m.currentGroup()
	title := T().PanelOptions
	subtitle := T().SelectGroupFirst
	if group != nil {
		title = fmt.Sprintf(T().OptionsTitleFmt, group.Name)
		subtitle = fmt.Sprintf(T().CurrentFmt, fallback(group.Current, "-"))
	}
	content := renderPanelContent(
		t,
		focusPanelTitle(title, m.focus == focusOptions),
		subtitle,
		m.visibleOptionRows(width, max(0, height)),
		width,
		height,
	)
	return renderPanel(style, width, height, content)
}

func (m model) renderDetailsPanel(width, height int) string {
	t := m.theme
	style := panelBaseStyle(t)
	rows := m.visibleDetailsRows(width)
	content := renderPanelContent(
		t,
		focusPanelTitle(T().PanelDetails, false),
		"",
		rows,
		width,
		height,
	)
	return renderPanel(style, width, height, content)
}

func (m model) visibleDetailsRows(width int) []string {
	if width <= 0 {
		return nil
	}
	t := m.theme
	group := m.currentGroup()
	if group == nil {
		return []string{fitLine(mutedStyle(t).Render("  "+T().SelectGroupFirst), width)}
	}

	rows := []string{
		m.detailRow(T().DetailsGroupLabel, group.Name, width),
		m.detailRow(T().DetailsTypeLabel, fallback(group.Type, "-"), width),
		m.detailRow(T().DetailsCurrentLabel, fallback(group.Current, "-"), width),
		m.detailRow(T().PanelOptions, fmt.Sprintf("%d", len(group.Options)), width),
	}

	if option := m.currentOption(); option != nil {
		rows = append(rows, "")
		rows = append(rows, m.detailRow(T().DetailsNodeLabel, option.Name, width))
		rows = append(rows, m.detailRow(T().DetailsDelayLabel, plainDelayLabel(option.DelayMS), width))
		action := T().DetailsActionApply
		actionStyle := mutedStyle(t)
		if option.Selected {
			action = "● " + T().DetailsActionCurrent
			actionStyle = lipgloss.NewStyle().Foreground(t.Success).Bold(true)
		} else {
			action = "○ " + action
		}
		rows = append(rows, "")
		rows = append(rows, fitLine(actionStyle.Render(action), width))
	}

	return rows
}

func (m model) detailRow(label, value string, width int) string {
	t := m.theme
	prefix := mutedStyle(t).Render(label + " ")
	valueWidth := width - ansi.StringWidth(label+" ")
	if valueWidth <= 0 {
		return fitLine(prefix, width)
	}
	return fitLine(prefix+subtitleStyle(t).Render(ansi.Truncate(value, valueWidth, "…")), width)
}

func focusPanelTitle(title string, focused bool) string {
	if focused {
		return "▸ " + title
	}
	return "  " + title
}

func (m model) currentOption() *OptionView {
	group := m.currentGroup()
	if group == nil || len(group.Options) == 0 || m.optionIndex < 0 || m.optionIndex >= len(group.Options) {
		return nil
	}
	return &group.Options[m.optionIndex]
}

func (m model) visibleGroupRows(width, limit int) []string {
	if limit <= 0 || width <= 0 {
		return nil
	}
	t := m.theme
	if len(m.groups) == 0 {
		return []string{fitLine(mutedStyle(t).Render("  "+T().NoGroupsMatchFilter), width)}
	}
	start, end := window(m.groupIndex, len(m.groups), limit)
	rows := make([]string, 0, end-start)

	for idx := start; idx < end; idx++ {
		group := m.groups[idx]
		isSelected := idx == m.groupIndex

		prefix := "  "
		focusedRow := isSelected && m.focus == focusGroups
		if focusedRow {
			prefix = "▸ "
		} else if isSelected {
			prefix = "• "
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
		if focusedRow {
			baseStyle = selectedStyle(t)
		} else if isSelected {
			baseStyle = inactiveSelectionStyle(t)
		} else if group.Current != "" {
			baseStyle = currentStyle(t)
		}

		currentMark := ""
		if group.Current != "" {
			bracketStyle := mutedStyle(t)
			curStyle := currentStyle(t)
			if focusedRow {
				bracketStyle = bracketStyle.Inherit(selectedStyle(t)).Foreground(t.TextMuted)
				curStyle = curStyle.Inherit(selectedStyle(t)).Foreground(t.Accent)
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
	t := m.theme
	group := m.currentGroup()
	if group == nil || len(group.Options) == 0 {
		return []string{fitLine(mutedStyle(t).Render("  "+T().NoSelectableNodes), width)}
	}
	start, end := window(m.optionIndex, len(group.Options), limit)
	rows := make([]string, 0, end-start)
	for idx := start; idx < end; idx++ {
		option := group.Options[idx]
		isSelected := idx == m.optionIndex
		focusedRow := isSelected && m.focus == focusOptions

		baseStyle := lipgloss.NewStyle()
		if focusedRow {
			baseStyle = selectedStyle(t)
		} else if isSelected {
			baseStyle = inactiveSelectionStyle(t)
		}

		var markerStyle lipgloss.Style
		markerChar := "○"
		if option.Selected {
			markerStyle = lipgloss.NewStyle().Foreground(t.Success)
			markerChar = "●"
		} else {
			markerStyle = mutedStyle(t)
		}
		if focusedRow {
			markerStyle = markerStyle.Inherit(selectedStyle(t)).Foreground(markerStyle.GetForeground())
		}

		delayStyle := getDelayStyle(t, option.DelayMS)
		if focusedRow {
			delayStyle = delayStyle.Inherit(selectedStyle(t)).Foreground(delayStyle.GetForeground())
		}
		delayStrPlain := plainDelayLabel(option.DelayMS)

		markerWidth := ansi.StringWidth(markerChar)
		reserved := 1 + markerWidth + 1 + 1 + ansi.StringWidth(delayStrPlain)
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
