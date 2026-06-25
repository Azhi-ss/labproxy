# LabProxy TUI 视觉主题与布局对齐重新设计

## 背景

LabProxy 是一个基于 Bubble Tea + Lipgloss 的终端 TUI 应用，用于管理 mihomo 代理。当前代码在 `internal/tui/` 下，包含：
- `theme.go` — 全局 `var` 颜色 token 和样式定义
- `app.go` — 主模型、Update/View 循环、布局拼接
- `header.go` — 顶部状态条渲染
- `footer.go` — 底部状态栏和帮助浮层
- `nav.go` — 左侧导航栏
- `view_proxies.go` — 代理组/节点列表
- `view_conns.go` — 连接列表
- `view_logs.go` — 日志流
- `view_config.go` — 设置页
- `helpers.go` — `fitLine`、`fitStyledLine`、`renderPanel` 等工具函数

## 目标

1. **浅色背景清晰可读**：白/近白背景下，所有文字 WCAG AA 对比度达标（≥4.5:1 正文，≥3:1 大文本）
2. **CJK 中文完美对齐**：所有视图中混合中英文文本列对齐，无错位
3. **无边框无装饰**：纯靠颜色和间距区分层次，不用 box-drawing 字符
4. **自动检测终端背景**：`tea.RequestBackgroundColor` 检测深色/浅色终端，自动适配
5. **双模式颜色 token**：支持深色/浅色切换，不硬编码单一主题
6. **信息密度克制**：减少视觉噪音，留白充足

## 已知问题

1. `theme.go` 全局 `var` 无法动态切换主题
2. `fitStyledLine` 用 `ansi.StringWidth` 补齐，但 `JoinHorizontal` 的列宽可能不统一导致 CJK 错位
3. 弱化文本 `colorTextMuted=246` 在浅色背景上对比度偏低（~3:1）
4. 无 `tea.RequestBackgroundColor` 背景检测
5. 颜色 token 扁平化，无语义分层

## 约束

- Go 语言，Bubble Tea + Lipgloss v1.1.0
- 不能引入新的外部依赖（除非是 charmbracelet 生态）
- 保持现有功能完整，只改视觉层
- 所有现有测试必须通过
- 支持 `--lang zh` 中文界面
