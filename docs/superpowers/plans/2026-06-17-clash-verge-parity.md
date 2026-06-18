# LabProxy → Clash Verge 终端替代品：实施计划

> 目标：把 labproxy 从"能代理"推进到"终端里真正替代 clash verge 的产品"，并以 **agent-native** 为差异化壁垒。
> 本文件是 `/loop` 持续执行的剧本。每次唤醒读取本文件，按"进度看板"找到第一个 `[ ]` 任务，TDD 推进，更新看板，提交。

## 1. 现状基线（2026-06-17）

| 功能域 | 状态 | 实现位置 |
|---|---|---|
| 内核（mihomo） | ✅ | `scripts/common.sh` install |
| 开关/重启 | ✅ | `labproxyon/off/restart` proxyctl.sh |
| 订阅管理（多订阅） | ✅ | `add/use/ls/sub/update` |
| 节点分组/选择 | ✅ | `internal/proxy/client.go` SwitchProxy |
| 单节点延迟测速 | ✅ | `Delay()` |
| 规则管理（13 类） | ✅ | `internal/rules/` + `rules` CLI + TUI 弹窗 |
| 系统代理/TUN/LAN/端口 | ✅ | `proxy/tun/lan/port/mixin/secret` |
| 实时流量 | ✅ | `Traffic()` 轮询 |
| 连接管理 | ⚠️ 只读 | `Connections()` 仅展示，**缺断连** |
| Web UI | ✅ | zashboard |
| TUI | ✅ | Bubble Tea，分组/设置/规则弹窗/i18n |

## 2. 差距与优先级

| 优先级 | 缺口 | 目标 |
|---|---|---|
| HIGH | `--json` 结构化输出 | agent 可解析的全局 `--json`，稳定 schema |
| HIGH | 连接管理 | 关闭单条连接 + 关闭全部连接 |
| HIGH | 批量测速 | 分组一键测速、按延迟排序、自动选优 |
| HIGH | 日志实时流 | TUI 内实时日志视图 + `labproxy logs` |
| MEDIUM | profile 抽象 | 整套配置覆写与切换 |
| MEDIUM | DNS 可视化 | DNS 查询日志、fake-ip 状态 |
| LOW | 主题/外观一致性 | TUI 配色 token 化 |
| LOW | doctor 增强 | 一键诊断覆盖新功能 |

## 3. 执行原则

- **TDD 优先**：每个任务先写测试（RED）→ 实现（GREEN）→ 重构。
- **复用 `internal/proxy/client.go`**：所有 mihomo API 交互集中在此 Go client，CLI 与 TUI 共享。
- **agent-native**：CLI 新增能力必须同时提供 `--json` 输出；人类输出走 `_okcat`，机器输出走 JSON envelope。
- **小步提交**：一个任务一个 commit，`<type>: <desc>` 格式，不加 Co-Authored-By。
- **不破坏既有**：每次推进后跑 `go test -race ./...` + 关键 bash 测试。
- **不 push**除非用户要求；不执行破坏性操作除非明确授权。

## 4. 进度看板

> 每完成一个任务，把对应行的 `[ ]` 改为 `[x]`，并在该任务下追加 commit SHA。loop 唤醒时从第一个 `[ ]` 继续。

### Phase 1：agent-native 结构化输出（差异化最高）

- [x] **T1.1** 定义统一 JSON envelope 与 `--json` 全局解析
  - 新建 `internal/cli/output.go`：`Envelope{OK bool, Data any, Error string}`，`PrintJSON(env)`、`IsJSONFlag(args)`。
  - 入口识别 `--json`。
  - 验收：`go test` 覆盖 envelope 序列化、`--json` 解析（含 `--json` 出现在任意位置）。
  - 文件：`internal/cli/output.go`、`internal/cli/output_test.go`
  - commit: see below (本任务)

- [x] **T1.2** `status --json` 输出结构化运行状态
  - 字段：`running bool, pid int, proxy_port, ui_port, dns_port, mode, system_proxy bool, uptime`。
  - 验收：人读输出不变；`--json` 输出合法 JSON 且字段齐全。
  - commit: 本任务（status 子命令 + bash 转发）

- [x] **T1.3** `proxies --json` / `connections --json` / `delay --json` 输出
  - 新增 `labproxy proxies`、`labproxy connections`、`labproxy delay <name>` CLI（薄封装 client）。
  - 验收：每个命令 `--json` 与无 flag 双形态；表驱动测试。
  - commit: 本任务（proxy_cli.go + main/proxyctl 接线）

### Phase 2：连接管理（断连）

- [x] **T2.1** client 新增 `CloseConnection(id)` 与 `CloseAllConnections()`
  - 对应 mihomo `DELETE /connections/:id`、`DELETE /connections`。
  - 验收：`client_test.go` 用 httptest mock 验证方法、URL、状态码。
  - commit: 本任务（client.go CloseConnection/CloseAllConnections）

- [x] **T2.2** CLI `labproxy connections close <id|all>`
  - 验收：`close all`、`close <id>` 双路径；`--json` 返回操作结果；错误明确。
  - commit: 本任务（proxy_cli.go connections close 子动作）

- [x] **T2.3** TUI 连接视图支持断连快捷键
  - 连接列表项可选中，`d` 关闭当前、`D` 关闭全部，带确认。
  - 验收：TUI 测试覆盖按键 → 触发 client 调用（mock）。
  - commit: 本任务（app.go 焦点循环+断连快捷键+i18n）

### Phase 3：批量测速

- [x] **T3.1** client 新增 `DelayGroup(groupName, timeout)` 并发测全组
  - 复用 `Delay()`，`errgroup` 并发，返回 `map[name]int`。
  - 验收：并发安全测试（`-race`）、超时处理、部分失败不阻断。
  - commit: 本任务（client.go DelayGroup 并发测全组）

- [x] **T3.2** CLI `labproxy test [group]` 批量测速
  - 默认测当前组，输出按延迟排序表格；`--json` 输出 map。
  - 验收：表格 + JSON 双形态；空组/失败组优雅处理。
  - commit: 本任务（proxy_cli.go runTestCLI + main/proxyctl 接线）

- [x] **T3.3** TUI 分组视图批量测速
  - `T` 测当前组全部节点，延迟列实时刷新，超时显示 `timeout`。
  - 验收：TUI 测试覆盖 `T` 键 → 批量刷新。
  - commit: 本任务（app.go T键+testGroupCmd+applyTestGroupResult+i18n）

### Phase 4：日志实时流

- [x] **T4.1** client 新增 `Logs(ctx)` SSE 流式日志订阅
  - mihomo `GET /logs?level=info`（text/event-stream 或 chunked）。
  - 验收：context 取消能停止；逐行产出 `(level, msg)`。
  - commit: 本任务（client.go Logs + types.go LogEntry）

- [ ] **T4.2** CLI `labproxy logs [-f] [--level]`
  - `-f` 跟随流；无 `-f` 输出最近 N 行（读 log 文件）。
  - 验收：`-f` 可 Ctrl-C 干净退出；`--json` 每行一个 envelope。
  - commit: -

- [ ] **T4.3** TUI 日志视图 Tab
  - 新增 Log tab，滚屏显示，`l` 切换级别过滤，自动截断保持最近 500 行。
  - 验收：TUI 测试覆盖 tab 切换、缓冲截断。
  - commit: -

### Phase 5：profile 抽象（MEDIUM）

- [ ] **T5.1** profile 数据模型与存储
  - `internal/profile/`：profile = mixin + rules + overrides 的命名集合，存 `~/.labproxy/profiles/<name>/`。
  - 验收：CRUD 测试、原子写入复用 rules store 模式。
  - commit: -

- [ ] **T5.2** CLI `labproxy profile [list|use|create|delete]`
  - `use` 切换 = 应用整套覆写到 runtime 并重启。
  - 验收：切换后 runtime 与 profile 一致；`--json`。
  - commit: -

### Phase 6：DNS 可视化（MEDIUM）

- [ ] **T6.1** client 新增 `DNSQuery(name)`（如 mihomo 支持）与 fake-ip 状态读取
  - 验收：能力探测；不支持时优雅降级。
  - commit: -

- [ ] **T6.2** TUI DNS 视图 / `labproxy dns` CLI
  - 验收：查询演示 + JSON。
  - commit: -

### Phase 7：收尾

- [ ] **T7.1** doctor 增强：覆盖连接/测速/日志/profile 健康检查
- [ ] **T7.2** README 更新：agent-native 章节 + `--json` schema 文档
- [ ] **T7.3** 全量回归：`go test -race ./...` + bash 测试全绿

## 5. loop 运行约定

- 每次唤醒：读本文件 → 找进度看板第一个 `[ ]` 任务 → TDD 执行 → 更新看板（`[x]` + SHA）→ commit → 调度下次唤醒（15m）。
- 单次唤醒只做 **1 个任务**（或一个任务的 RED→GREEN），避免上下文爆炸。
- 遇到 BLOCKED（API 不支持、设计歧义）→ 记录在看板任务下，跳到下一个可做任务，不空转。
- 每次唤醒开头跑一次 `git log --oneline -3` 确认上次进度，结尾跑 `go build ./...` 确认可编译。
