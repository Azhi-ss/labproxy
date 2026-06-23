# TUI 重构设计：Clash 式导航 + 视图标签

- 日期：2026-06-22
- 状态：已设计，待评审
- 范围：`internal/tui/` 全量重构（不改后端 `internal/proxy`、`internal/config`）

## 1. 背景与问题

当前 TUI 基于 Bubble Tea + Lip Gloss，结构为「Groups | Options | Connections」三栏同屏 +
Settings / Log / Rules 三个 overlay。`app.go` 已达 1916 行，单文件承载全部状态与渲染。

用户反馈四类痛点全中：

1. 布局/信息层级混乱：三栏硬塞一行，窄终端拥挤，缺视觉重心。
2. 交互/键位反直觉：Tab/方向键/字母散落，焦点切换绕，常用操作步数多。
3. 视觉风格粗糙：边框廉价、色彩散落硬编码、无层次引导，像默认模板。
4. 功能操作不顺手：搜索/筛选/排序/批量缺失，只能逐条翻。

## 2. 目标

- 对齐 Clash 生态（yacd / metacubexd）的心智模型：侧边导航 + 标签主区。
- 消除所有 overlay，统一为平等的常驻视图。
- 文件按视图拆分，单文件 < 800 行。
- 键位统一可发现（`1-5` 切视图、`j/k` 移动、`/` 过滤、`?` 帮助）。
- 视觉 token 化，暗色专业调性，延迟色阶引导。

## 3. 非目标

- 不改后端 proxy client 与 config 协议。
- 不引入新依赖（继续用 bubbletea/lipgloss）。
- 不做节点 grid 展示（用户选定 list 形态）。
- 不做 web UI。

## 4. 架构

从「单 model + overlay」重构为「导航 + 视图」状态机。

```
model
├── 全局状态：endpoint / mode / sysproxy / lan / tun / 流量（顶部状态条）
├── activeView: proxies | connections | logs | rules | config
├── 各视图独立的子状态（焦点 / 筛选词 / 游标 / 滚动偏移）
└── 共享：refresh tick、log 流、proxy client
```

- `Update` 顶层按 `activeView` 分派到对应视图 handler。
- 视图切换是纯本地状态变更，不发命令，零延迟。
- overlay 全部消除：Settings / Log / Rules 改为常驻视图。

## 5. 视图划分

| 视图 | 键 | 内容 | 来源 |
|------|----|------|------|
| Proxies | `1` | 左 group 列表 + 右节点 list（搜索/测速/延迟着色） | 现三栏合并 |
| Connections | `2` | 连接表（host/network/rule/上下行/时长），可选中关闭 | 现 Connections 面板升级 |
| Logs | `3` | 实时日志流，级别着色，`/` 过滤 | 现 LogOverlay 常驻化 |
| Rules | `4` | 规则增删改查 | 现 RulesModal 常驻化 |
| Config | `5` | mode/sysproxy/lan/tun/重启 | 现 SettingsOverlay 常驻化 |

顶部状态条保留全局信息，所有视图共享。底部键位栏随当前视图动态变化。

## 6. 键位

| 键 | 全局 | Proxies | Connections | Logs | Rules | Config |
|----|------|---------|-------------|------|-------|--------|
| `1-5` | 切视图 | | | | | |
| `Tab` | 下一视图 | | | | | |
| `/` | — | 搜索节点 | 过滤 host | 过滤内容 | 过滤规则 | — |
| `j/k` `↑↓` | — | 移动游标 | 移动游标 | 滚动 | 移动游标 | 切换项 |
| `Enter` | — | 切换节点 | 关闭连接 | — | 编辑规则 | 切换开关 |
| `t` | — | 批量测速 | — | — | — | — |
| `r` | 刷新 | | | | | |
| `?` | 帮助浮层（全键位） | | | | | |
| `q` | 退出 | | | | | |

## 7. 视觉

暗色专业调性。

- **色彩 token 化**（`theme.go` 集中管理，消除散落 `lipgloss.Color("237")`）：
  - 延迟色阶：<50ms 绿 / <150ms 黄 / <300ms 橙 / ≥300ms 红 / 未测灰 / timeout 暗红。
  - 状态色：运行绿点 / 异常红点 / 开启亮色 / 关闭暗色。
- **边框**：圆角 `RoundedBorder` 替代 `NormalBorder`；导航栏选中项左侧 `▌` 高亮条。
- **间距**：面板内 padding 统一，group 间留空行，避免信息墙。
- **焦点**：当前视图/选中项用主题色边框 + 左侧 `▌`，非焦点降饱和度。

## 8. 文件拆分

```
internal/tui/
├── app.go            (~150行) App/Run/顶层 model + Update 路由
├── keys.go           keyMap + 各视图 binding
├── header.go         顶部状态条
├── footer.go         底部键位栏（随视图动态）
├── nav.go            左侧导航栏 + 视图切换
├── view_proxies.go   Proxies 视图
├── view_conns.go     Connections 视图
├── view_logs.go      Logs 视图
├── view_rules.go     Rules 视图（复用 rules/ 包）
├── view_config.go    Config 视图（复用现有 settings 逻辑）
├── theme.go          色彩/样式 token
├── viewmodel.go      保留（数据模型）
└── i18n.go           保留（新增视图标签 key）
```

每视图文件自包含：子状态、Update 分支、View。`app.go` 只做消息路由与全局状态。

## 9. 数据流

复用现有消息，不改后端：

- `refreshMsg` / `logEntryMsg` / `testGroupResultMsg` / `switchResultMsg` / `settingsResultMsg` / `configFlagsMsg` / `errMsg` 全部保留。
- `Update` 顶层按 `activeView` 分派；全局消息（refresh/tick/err）广播到当前视图。
- 视图切换纯本地状态，不发命令。

## 10. 测试

- 每视图文件配 `*_test.go`：游标移动/边界、搜索过滤、消息应用、键位分派。
- 现有 `app_test.go`(1780行) / `adaptive_layout_test.go`(513行) 拆分到对应视图。
- 保留并增强 CJK 宽度对齐测试（`ansi.StringWidth`）。
- 目标覆盖：视图层 ≥ 80%。

## 11. 迁移与风险

| 风险 | 缓解 |
|------|------|
| 一次性大改破坏现有功能 | 按视图分阶段迁移，每视图独立可测；保留 viewmodel/i18n 减少 diff |
| 键位变更影响肌肉记忆 | `?` 帮助浮层 + README 更新键位表 |
| CJK 对齐回归 | 保留 width 测试，theme token 不引入新宽度逻辑 |
| 现有 overlay 测试失效 | 随对应视图迁移测试，不丢弃 |

## 12. 验收

- 五个视图均可通过 `1-5` 切换，无 overlay 残留。
- `app.go` < 200 行，所有文件 < 800 行。
- 键位表与 README 一致，`?` 帮助可用。
- 延迟色阶、状态色、焦点指示按设计渲染。
- 视图层测试 ≥ 80%，`go test ./internal/tui/...` 通过。
- `VERSION=dev bash scripts/build-tui.sh` 构建成功。
