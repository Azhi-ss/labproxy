package tui

// renderRulesView 渲染 Rules 视图：调用 rulesModal.View() 放入主区。
func (m model) renderRulesView(width, height int) string {
	if m.rulesModal == nil {
		return mutedStyle(m.theme).Render("rules unavailable")
	}
	return m.rulesModal.View()
}
