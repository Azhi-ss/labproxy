# Rules Feature Design — labproxy

**Date:** 2026-06-17
**Status:** Approved (pending user review of written spec)
**Author:** Brainstorming session

## 1. 概述

为 labproxy 添加与 clash-verge-rev 等价的规则（Rules）管理能力。覆盖**查看、增删改、启用/禁用、重新排序、导入/导出、规则提供者、重置**共 7 项子能力，通过 **TUI 弹窗** 与 **CLI 子命令** 双接口暴露给用户。

### 1.1 关键决策摘要

| 维度 | 决策 |
|---|---|
| 功能范围 | 全套 7 项能力（view / CRUD / toggle / reorder / io / providers / reset） |
| 数据存储 | mixin.yaml 持久化 + mihomo 重启生效 |
| TUI 集成 | 模态弹窗（按 R 键进入，类似现有 Settings 弹窗） |
| 规则编辑器 | 分字段表单（Type / Payload / Proxy / No-Resolve） |
| 支持类型 | 13 种 clash/mihomo 标准类型 |
| 接口 | TUI 弹窗 + Go CLI 子命令（同一二进制） |
| 架构 | 统一 Go 二进制，`internal/rules` 包承担所有业务逻辑 |

### 1.2 目标

- 与 clash-verge-rev 体验对标，但以 TUI 形态呈现
- 不要求用户理解 YAML 或规则语法
- 操作可审计（每次写回都有备份）
- 失败可回滚（.bak 自动轮转）
- 中英双语（复用现有 `internal/tui/i18n.go`）

### 1.3 非目标

- 不实现 undo/redo（操作可读回 .bak 手动恢复）
- 不做 mihomo API 直连运行时规则（与现有架构保持一致）
- 不实现规则可视化匹配链路追踪
- 不做规则冲突检测（顺序由用户负责）

## 2. 架构与模块布局

### 2.1 新增 / 修改文件

```
NEW   internal/rules/                    # 业务逻辑包
      ├── types.go                       # Rule、Provider 数据模型
      ├── validate.go                    # 13 种类型校验逻辑
      ├── store.go                       # mixin.yaml 读写（原子写 + 备份）
      ├── store_test.go
      ├── import.go                      # URL / 文件 / 预设导入
      ├── import_test.go
      ├── providers.go                   # rule-providers 增删改查
      ├── providers_test.go
      ├── preset.go                      # 内置预设加载
      └── presets/                       # 内置规则集（YAML 片段）
          ├── direct.yaml
          ├── private.yaml
          ├── gfw.yaml
          ├── tld-not-cn.yaml
          └── README.md

NEW   internal/tui/rules/                # TUI 弹窗
      ├── modal.go                       # 主弹窗（状态机、子视图路由）
      ├── list.go                        # 规则列表
      ├── form.go                        # 分字段编辑表单
      ├── providers.go                   # 提供者子页面
      ├── import_view.go                 # 导入子页面
      └── rules_test.go

MOD   internal/tui/app.go                # 注册 'R' 键 → 弹窗
MOD   internal/tui/i18n.go               # 新增翻译键
MOD   internal/tui/viewmodel.go          # 不变
MOD   cmd/labproxy-tui/main.go           # 新增 os.Args 解析：
                                         #   labproxy-tui           → TUI
                                         #   labproxy-tui rules ... → CLI 模式

MOD   scripts/proxyctl.sh                # 新增 rules 子命令，转调 Go 二进制
MOD   scripts/build-tui.sh               # 重新构建（无新增构建步骤）
MOD   .gitignore                         # 添加 mixin.yaml.bak.*
NEW   tests/rules_cli_test.sh            # CLI 集成测试
NEW   tests/rules_persistence_test.sh    # 写回 + 重启生效验证
```

### 2.2 依赖

- Go 标准库
- `gopkg.in/yaml.v3`（已在 go.sum 中）— 解析 mixin.yaml 片段
- `github.com/charmbracelet/bubbles/textarea` — 多行 payload 输入
- 现有 `internal/tui` 所有依赖

## 3. 数据模型与存储格式

### 3.1 Rule 数据模型

```go
type Rule struct {
    Type      RuleType // 13 种
    Payload   string
    Proxy     string
    NoResolve bool
    Enabled   bool     // 派生自 YAML 注释状态
}

type RuleType string
const (
    TypeDomain        RuleType = "DOMAIN"
    TypeDomainSuffix  RuleType = "DOMAIN-SUFFIX"
    TypeDomainKeyword RuleType = "DOMAIN-KEYWORD"
    TypeDomainRegex   RuleType = "DOMAIN-REGEX"
    TypeIPCIDR        RuleType = "IP-CIDR"
    TypeIPCIDR6       RuleType = "IP-CIDR6"
    TypeSrcIPCidr     RuleType = "SRC-IP-CIDR"
    TypeSrcPort       RuleType = "SRC-PORT"
    TypeGEOIP         RuleType = "GEOIP"
    TypeGEOSITE       RuleType = "GEOSITE"
    TypeRuleSet       RuleType = "RULE-SET"
    TypeMatch         RuleType = "MATCH"
    TypeMatchSrc      RuleType = "MATCH-SRC"
)
```

### 3.2 Provider 数据模型

```go
type Provider struct {
    Name     string
    Type     string // http | file
    Behavior string // domain | ipcidr | classical
    URL      string
    Path     string
    Interval int
}
```

### 3.3 YAML 存储格式

```yaml
# 自定义规则
rules:
  - DOMAIN,api64.ipify.org,DIRECT                  # 启用
  # - DOMAIN-SUFFIX,example.com,PROXY              # 禁用（注释）
  - IP-CIDR,8.8.8.0/24,DIRECT,no-resolve
  - RULE-SET,google,PROXY
  - MATCH,DIRECT

# 规则提供者
rule-providers:
  google:
    type: http
    behavior: domain
    url: "https://cdn.jsdelivr.net/.../google.yaml"
    path: ./rule-providers/google.yaml
    interval: 86400
```

**关键点**：
- 注释承载启用/禁用状态（YAGNI，单独字段）
- 格式兼容 clash 标准（4 段逗号语法）
- 与现有 mixin.yaml 缩进一致（2 空格）
- 写入策略：先解析整个 `rules` 块为 `[]string` 再回写
- 原子替换：`tmp` 文件 + `os.Rename`

## 4. `internal/rules` API

### 4.1 数据模型

```go
type Rule struct { ... }     // 见 §3.1
type Provider struct { ... } // 见 §3.2
type Diff struct { ... }     // 变更详情（供 UI 反馈）
```

### 4.2 解析 & 校验

```go
func ParseRule(line string) (Rule, error)
func (r Rule) String() string                  // 反向序列化
func (r Rule) Validate() error
func ValidateProvider(p Provider) error
```

### 4.3 持久化

```go
type Store struct{ Path string }
func NewStore(path string) (*Store, error)
func (s *Store) LoadRules() ([]Rule, error)
func (s *Store) SaveRules(rules []Rule) error
func (s *Store) LoadProviders() ([]Provider, error)
func (s *Store) SaveProviders(providers []Provider) error
func (s *Store) Backup() (string, error)
func (s *Store) RotateBackups(keep int) error  // 默认 5
```

### 4.4 增删改

```go
func (s *Store) AddRule(r Rule) (Diff, error)
func (s *Store) UpdateRule(index int, r Rule) (Diff, error)
func (s *Store) DeleteRule(index int) (Diff, error)
func (s *Store) ToggleRule(index int) (Diff, error)
func (s *Store) MoveRule(index, target int) (Diff, error)
```

### 4.5 导入

```go
type ImportSource struct {
    Kind string // "url" | "file" | "preset"
    Ref  string
}
func (s *Store) Import(src ImportSource, mode string) (Diff, error)
// mode: "append" | "replace"
```

### 4.6 提供者

```go
func (s *Store) AddProvider(p Provider) (Diff, error)
func (s *Store) UpdateProvider(name string, p Provider) (Diff, error)
func (s *Store) DeleteProvider(name string) (Diff, error)
func (s *Store) RefreshProvider(name string) error
```

### 4.7 重置 & 工具

```go
func (s *Store) ResetRules() (Diff, error)
func KnownProxyNames(rules []string) []string
func (r Rule) FormatHint() string
```

**设计原则**：
- `Store` 实例持 `sync.Mutex`，TUI 命令串行化
- `Diff` 同时返回成功/失败/跳过/回滚状态
- `error` 仅在不可恢复时返回
- `Validate()` 在 `AddRule` 内部调用

## 5. TUI 弹窗设计

### 5.1 入口与状态机

```
[主 TUI] ──按 R 键──→ [Rules Modal]
                          │
              ┌───────────┼───────────┐
              ▼           ▼           ▼
          [列表]      [编辑表单]  [提供者]
              │           │
              │  ──Esc──  │
              ▼           ▼
          [导入视图]  [弹窗顶层]
```

### 5.2 弹窗顶层布局

```
┌─ Rules Manager ─────────────────────── [Esc 关闭 / Tab 切页]┐
│ > [1] 规则列表  (12 条 / 启用 9 / 禁用 3)                   │
│   [2] 规则提供者  (3 个)                                    │
│   [3] 导入规则    (URL / 文件 / 预设)                       │
│   [4] 重置为默认                                             │
└────────────────────────────────────────────────────────────┘
  ↑↓ 选择  Enter 进入  R 退出
```

### 5.3 规则列表页

```
┌─ Rules (12) ─────────────────────── / 过滤  n 新增 ─┐
│ ▸ ● DOMAIN-SUFFIX  *.google.com          → PROXY   │ ← 启用
│   ● IP-CIDR        8.8.8.0/24            → DIRECT │
│   ● RULE-SET       google                → PROXY   │
│   ○ DOMAIN         example.com           → REJECT  │ ← 禁用
│   ...                                              │
└─────────────────────────────────────────────────────┘
  ↑↓ 移动  Space 启用/禁用  n 新增  e 编辑  d 删除
  J/K 上下移  r 刷新  / 过滤  ? 帮助
```

### 5.4 规则编辑表单

```
┌─ 新增规则 ──────────────────────────────────────┐
│  类型:      [DOMAIN-SUFFIX      ▼]             │
│  Payload:   [*.example.com                    ] │
│  目标:      [PROXY              ▼]  (DIRECT/REJECT/代理组)│
│  选项:      [ ] no-resolve                     │
│                                                │
│  [Enter 提交]  [Esc 取消]  [Tab 切换字段]      │
└────────────────────────────────────────────────┘
  错误提示行（粘贴时实时校验）：✗ Payload 不能为空
```

### 5.5 按键绑定

| 键 | 列表页 | 编辑表单 | 弹窗顶层 |
|---|---|---|---|
| `↑/↓` / `j/k` | 移动光标 | 切换字段 | 移动 |
| `Enter` | 进入编辑 | 提交 | 进入子页 |
| `Esc` | 退出弹窗 | 取消 | 退出弹窗 |
| `n` | 新增规则 | — | — |
| `e` | 编辑当前 | — | — |
| `d` | 删除当前（确认） | — | — |
| `Space` | 启用/禁用 | — | — |
| `J/K`（大写） | 下移/上移 | — | — |
| `/` | 过滤 | — | — |
| `Tab` | — | 字段间移动 | — |
| `?` | 切换帮助 | 切换帮助 | 切换帮助 |

**设计要点**：
- 弹窗内部维护独立的 `rulesModalState`，关闭时丢弃
- 加载态：进入列表页时调 `LoadRules()`，200ms 内无 loading；超过显示骨架屏
- 错误展示：表单字段下方一行红字
- 删除确认：按 `d` 后迷你确认（"按 y 确认删除"）
- i18n：所有字符串走 `T().RulesXXX` 翻译键

## 6. CLI 子命令设计

### 6.1 入口分发

```go
// cmd/labproxy-tui/main.go
func main() {
    if len(os.Args) >= 2 && os.Args[1] == "rules" {
        os.Exit(runRulesCLI(os.Args[2:]))
    }
    runTUI()
}
```

### 6.2 子命令树

```
labproxy-tui rules
  list
    --type=DOMAIN-SUFFIX
    --enabled
    --json
  add
    --type=DOMAIN-SUFFIX
    --payload=*.example.com
    --proxy=PROXY
    --no-resolve
    --at=10
  edit <index>
    --type=... --payload=... --proxy=... --no-resolve=bool
  delete <index>
    -y
  enable <index>
  disable <index>
  move <from> <to>
  import
    --source=url|file:/path|preset:direct
    --mode=append|replace
    --no-backup
  providers
    list
    add --name=... --type=... --behavior=... --url=... --interval=...
    edit <name>
    delete <name>
    refresh <name>
  reset
    -y
```

### 6.3 Shell 包装

```bash
# scripts/proxyctl.sh
rules)  shift; exec "$LABPROXY_HOME/bin/labproxy-tui" rules "$@" ;;
```

### 6.4 输出格式

- 默认：人类可读（带颜色）
- `--json`：JSON 行（适合 `jq` 管道）
- 错误：stderr 打印，退出码 0=成功 / 1=校验 / 2=文件 IO / 3=mihomo API 失败

### 6.5 示例

```
$ labproxy-tui rules list
INDEX  ENABLED  TYPE            PAYLOAD              PROXY
0      ●        DOMAIN-SUFFIX   *.google.com         PROXY
1      ●        IP-CIDR         8.8.8.0/24           DIRECT
2      ○        DOMAIN          example.com          REJECT     # disabled
3      ●        RULE-SET        google               PROXY
4      ●        MATCH           -                    DIRECT
```

**设计要点**：
- 单一二进制：Shell 只转发，不做业务逻辑
- `index` 指当前规则列表位置（含禁用）
- 配置路径走 `LABPROXY_HOME` 环境变量
- 所有写操作支持 `--dry-run`
- 可选 `rules __complete` 后续补全

## 7. 导入 / 导出 / 规则提供者

### 7.1 导入源类型

| Kind | 格式 | 示例 |
|---|---|---|
| `url` | HTTP(S) URL | `https://cdn.jsdelivr.net/.../google.yaml` |
| `file` | 本地 YAML 路径 | `/path/to/rules.yaml` |
| `preset` | 内置预设名 | `direct` / `private` / `gfw` / `tld-not-cn` |

### 7.2 内置预设

| 预设 | 行为 | 适用 |
|---|---|---|
| `direct` | 国内直连 | 中国大陆用户 |
| `private` | 私有地址直连 | 任何用户（防回环） |
| `gfw` | GFW 列表走代理 | 科学上网 |
| `tld-not-cn` | 非中国大陆域名走代理 | 反向需求 |

每个预设文件结构：
```yaml
# preset: direct
# description: 中国大陆常见域名直连
- DOMAIN-SUFFIX,cn,DIRECT
- DOMAIN-SUFFIX,baidu.com,DIRECT
- GEOIP,CN,DIRECT
```

### 7.3 URL 导入流程

```
1. GET URL（10s timeout）
2. 校验 HTTP 200 + Content-Type
3. 解析 YAML 为 []string
4. 解析每行为 Rule
5. 按 --mode 追加或替换
6. 原子写回 mixin.yaml
7. 输出 Diff
```

### 7.4 安全护栏

- URL scheme 限制 `http`/`https`
- 文件路径拒绝 `..`、绝对路径需 `--allow-external-path`
- 响应体大小限制 5MB
- 拉取后先落盘到 `$LABPROXY_HOME/cache/import-<sha256>.yaml`

### 7.5 导出

- 写当前规则列表到指定路径
- 输出与 mihomo 标准完全兼容
- 不导出禁用的（除非 `--include-disabled`）

### 7.6 规则提供者 schema

复用 mihomo 原生：
```yaml
rule-providers:
  <name>:
    type: http | file
    behavior: domain | ipcidr | classical
    url: "https://..."
    path: "./providers/x.yaml"
    interval: 86400
```

**管理操作**：
- add / edit / delete 走 `internal/rules/providers.go`
- `refresh <name>` 走 HTTP 拉取，30s 超时
- `list` 显示 name / type / interval / 上次更新时间

**安全要点**：
- 导入去重：若新规则与已存在重复（Type+Payload），默认跳过
- `RULE-SET,xxx,PROXY` 自动检查 `xxx` 是否在 `rule-providers` 中存在
- `provider.path` 相对 `LABPROXY_HOME` 解析（不允许写 `/etc/...`）
- HTTP 拉取走 `proxy.Client`（与代理同源）

## 8. 错误处理与测试

### 8.1 错误分类

| 错误 | 触发场景 | 处理 |
|---|---|---|
| `ErrValidation` | 字段缺失/类型不匹配 | TUI 红字 / CLI 退出 1 |
| `ErrFileIO` | mixin.yaml 读写失败 | 回滚 .bak / CLI 退出 2 |
| `ErrParseYAML` | mixin.yaml 格式损坏 | 强制用户修复 |
| `ErrImport` | URL 拉取失败/超时 | 保留缓存供排查 |
| `ErrProvider` | provider 配置错误 | 同 ErrValidation |
| `ErrRestart` | mihomo 重启失败 | 不阻断持久化 |
| `ErrConflict` | index 越界/重复引用 | 立即报错 |

### 8.2 处理原则

- **Fail fast**：写操作前先校验
- **原子性**：每个 Store 写操作都是 tmp + rename
- **可恢复性**：所有错误带修复提示
- **不静默吞错**：`Errorf("...: %w", err)` 包装
- **mihomo 重启失败不阻断持久化**

### 8.3 测试策略（覆盖率 ≥ 80%）

**单元测试**（`internal/rules/*_test.go`，目标 90%）：
- `validate_test.go` — 13 种类型 × 边界
- `parse_test.go` — 注释、no-resolve、空 payload、特殊字符
- `store_test.go` — 加载/保存/原子写/备份轮转
- `import_test.go` — URL（httptest）/ file / preset
- `providers_test.go` — CRUD + refresh（mock mihomo）

**集成测试**（`tests/rules_*.sh`）：
- `rules_cli_test.sh` — 所有子命令、校验失败、--json 输出
- `rules_persistence_test.sh` — mixin.yaml 格式合法、备份存在、轮转正确、回滚

**TUI 测试**（`internal/tui/rules/rules_test.go`）：
- 使用 `tea.NewProgram` + 注入测试消息
- 测弹窗状态变化，不测视觉

**额外**：
- i18n 测试：中英文下所有新增键都不为空
- `go test -race ./...` 验证并发
- 不写 e2e（不实际启动 mihomo）

## 9. 实施计划（高层）

按以下顺序分阶段实施：

1. **数据模型 + 校验 + 解析**（`internal/rules/types.go`、`validate.go` + 单元测试）
2. **Store 持久化**（`store.go` + 原子写 + 备份 + 单元测试）
3. **规则 CRUD**（`AddRule`/`UpdateRule`/`DeleteRule`/`ToggleRule`/`MoveRule` + 测试）
4. **CLI 子命令**（`cmd/labproxy-tui/main.go` 分发 + `rules` 子命令 + `proxyctl.sh` 包装 + shell 测试）
5. **导入 / 导出**（`import.go` + `presets/*.yaml` + 测试）
6. **规则提供者**（`providers.go` + 集成）
7. **TUI 弹窗**（`internal/tui/rules/*` + 主 TUI 'R' 键注册 + i18n）
8. **集成测试**（`tests/rules_*.sh` + 整体联调）
9. **文档**（`README.md` 新增 Rules 章节 + `docs/superpowers/specs/2026-06-17-rules-feature-design.md` 留档）

每个阶段交付后跑 `go test ./...` + `tests/rules_*.sh`，确保无回归。

## 10. 风险与缓解

| 风险 | 影响 | 缓解 |
|---|---|---|
| mixin.yaml 注释丢失 | 用户数据丢失 | 写入前先 `Backup()`；写入后回读验证 |
| YAML 库选型差异 | 格式差异 | 使用项目已有的 `gopkg.in/yaml.v3` |
| mihomo 重启不生效 | 规则不生效 | 持久化后立即重启，状态条提示 |
| TUI 弹窗与现有 keybinding 冲突 | 操作误触发 | 'R' 键不与现有任何功能冲突 |
| 大量规则下 TUI 渲染慢 | 体验差 | 列表分页/虚拟滚动（YAGNI，先支持 ≤500 条） |
| 导入 URL 拖慢 TUI | 体验差 | 后台 goroutine + 进度提示 |

## 11. 未来扩展（不在本次范围）

- 规则匹配链路追踪（mihomo API 返回）
- undo/redo 历史栈
- 规则冲突检测
- Web UI（zashboard 集成）
- 规则可视化编辑器（拖拽排序）
- 规则模板市场

---

**Spec 状态**：等待用户最终审查。审查通过后转入 `writing-plans` 阶段，输出可执行实施计划。
