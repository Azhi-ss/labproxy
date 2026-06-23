package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// renderProxiesView 渲染 Proxies 视图主体：左 group 列表 + 右节点 list。
func (m model) renderProxiesView(width, height int) string {
	panelFrameWidth := panelBaseStyle.GetHorizontalFrameSize()
	columnContentWidth := max(0, width-columnGap-panelFrameWidth*2)
	groupsWidth := min(m.calcGroupsMinWidth(columnContentWidth), columnContentWidth)
	optionsWidth := max(0, columnContentWidth-groupsWidth)
	groups := m.renderGroupsPanel(groupsWidth, height)
	options := m.renderOptionsPanel(optionsWidth, height)
	return lipgloss.JoinHorizontal(lipgloss.Left, groups, strings.Repeat(" ", columnGap), options)
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

