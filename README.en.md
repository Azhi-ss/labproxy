# LabProxy

<p align="center">
  <strong>简体中文</strong> | <a href="./README.md">English</a>
</p>

<p align="center">
  <a href="https://github.com/Azhi-ss/labproxy/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/Azhi-ss/labproxy" alt="License">
  </a>
  <img src="https://img.shields.io/github/languages/top/Azhi-ss/labproxy" alt="Language">
</p>

<p align="center"><b>终端原生的代理控制面 · agent-native · clash verge 的命令行替代</b></p>

---

## 为什么需要 LabProxy？

| 传统方案 | LabProxy |
|---------|---------|
| 需要 sudo 权限 | ✅ 纯用户空间，无需 root |
| 依赖 GUI 或 systemd | ✅ 纯命令行，PID 文件管理 |
| 端口冲突导致启动失败 | ✅ 自动检测并分配可用端口 |
| 多用户环境配置冲突 | ✅ 完全隔离的用户目录 |
| GUI 工具无法被脚本/agent 调用 | ✅ 所有命令支持 `--json`，agent 可直接控制 |

**LabProxy** 基于 [clash-for-linux-install](https://github.com/nelvko/clash-for-linux-install) 二次开发，定位为**终端里替代 clash verge 的代理控制面**，并以 **agent-native** 为差异化壁垒：

- **无特权安装** — 安装到 `~/.labproxy/`，普通用户即可使用；自动检测内核（内置 zip / brew / PATH / 在线下载，跨 macOS 与 Linux）
- **智能端口** — 7890/9090 被占用？自动寻找可用端口
- **TUI 界面** — 终端下的图形化管理：分组/节点选择、连接管理（断连）、批量测速、实时日志、规则编辑
- **Web 控制台** — 浏览器管理，支持密钥保护
- **agent-native CLI** — `status`/`proxies`/`connections`/`delay`/`test`/`logs`/`dns`/`profile`/`doctor` 全部支持 `--json`，统一 `{ok, data, error}` 信封，便于脚本与 AI agent 解析
- **自动订阅转换** — 内置 subconverter，兼容各种订阅格式

---

## 快速开始

```bash
# 1. 克隆并安装
git clone https://github.com/Azhi-ss/labproxy.git && cd labproxy
bash install.sh

# 2. 配置订阅（必须）
labproxy subscribe https://your-subscription-url

# 3. 启动
labproxy on

# 4. 验证
curl -I https://www.google.com
```

<details>
<summary><b>📋 完整安装指南</b></summary>

**环境要求**
- Shell: `bash` / `zsh` / `fish`
- 权限: 普通用户（无需 sudo）
- 依赖: 有效的 Clash 订阅链接

**安装流程**
```bash
git clone https://github.com/Azhi-ss/labproxy.git
cd labproxy
bash install.sh        # 默认安装到 ~/.labproxy/
```

安装完成后自动配置：
- 下载适配架构的 mihomo 内核
- 配置 shell 环境变量
- 设置命令别名
- 检测并分配可用端口

</details>

---

## 核心命令

```
labproxy on              # 启动代理
labproxy off             # 停止代理
labproxy status          # 查看状态
labproxy tui             # 打开 TUI 界面
```

| 命令 | 功能 |
|-----|------|
| `labproxy port [set <port>\|auto\|status]` | 固定端口 / 自动分配 |
| `labproxy lan [on\|off\|status]` | 局域网访问控制 |
| `labproxy proxy [on\|off\|status]` | 系统代理开关 |
| `labproxy subscribe [URL]` | 设置/查看订阅 |
| `labproxy update [auto]` | 更新订阅配置 |
| `labproxy ui` | Web 控制台地址 |
| `labproxy mixin [-e\|-r]` | 编辑/查看配置 |

---

## Agent-Native（机器可读输出）

LabProxy 的差异化定位是 **agent-native 代理控制面**：所有查询/操作命令都支持 `--json`，输出稳定 schema 的 JSON envelope，便于脚本与 AI agent 解析。

### JSON Envelope Schema

所有 `--json` 输出遵循统一信封：

```json
{
  "ok": true,          // 操作是否成功
  "data": <any>,       // 成功时的数据负载，失败时为 null
  "error": ""          // 成功时为空字符串，失败时为错误信息
}
```

### 支持 --json 的命令

| 命令 | data 字段 |
|------|-----------|
| `labproxy status --json` | `{running, pid, proxy_port, ui_port, dns_port, mode, system_proxy, uptime}` |
| `labproxy proxies --json` | mihomo `/proxies` 原始响应 |
| `labproxy connections --json` | mihomo `/connections` 原始响应 |
| `labproxy connections close <id\|all> --json` | `{closed: "<id\|all>"}` |
| `labproxy delay <name> --json` | `{name, delay}`（ms，失败时 ok=false） |
| `labproxy test [group] --json` | `{group, results: {name: delay}}`（失败节点 delay=-1） |
| `labproxy logs -f --json` | 每行一个 envelope `{level, payload}` |
| `labproxy dns <name> [--type A] --json` | mihomo `/dns/query` 响应 `{Status, Question, Answer}` |
| `labproxy profile <list\|create\|delete\|use> --json` | `[]` profile 名 或 `{name}` |
| `labproxy doctor --json` | `{checks: [{name, ok, detail}]}`（每项独立 ok） |

### 示例：agent 拿到运行状态

```bash
$ labproxy status --json
{"ok":true,"data":{"running":true,"pid":12345,"proxy_port":7890,"ui_port":9090,"dns_port":15353,"mode":"rule","system_proxy":true,"uptime":"01:23:45"},"error":""}
```

### 示例：批量测速并按延迟选优

```bash
$ labproxy test GLOBAL --json
{"ok":true,"data":{"group":"GLOBAL","results":{"Node-A":50,"Node-B":120,"Node-C":-1}},"error":""}
```

> `--json` 输出不含 ANSI 颜色码；无 `--json` 时输出人类可读的彩色文本。

---

## 规则管理

```bash
# TUI 中按 R 键打开规则管理弹窗
labproxy tui  # 按 R 键

# CLI 子命令
labproxy rules list
labproxy rules add --type=DOMAIN-SUFFIX --payload=example.com --proxy=PROXY
labproxy rules disable 0
labproxy rules enable 0
labproxy rules move 0 2
labproxy rules delete 0
labproxy rules import --source=preset:direct
labproxy rules import --source=https://example.com/rules.yaml
labproxy rules export --out=./my-rules.yaml
labproxy rules reset -y
labproxy rules providers list
labproxy rules providers add --name=google --type=http --behavior=domain \
    --url=https://example.com/g.yaml --path=./providers/google.yaml --interval=86400
labproxy rules providers refresh google
```

支持的规则类型：`DOMAIN`, `DOMAIN-SUFFIX`, `DOMAIN-KEYWORD`, `DOMAIN-REGEX`, `IP-CIDR`, `IP-CIDR6`, `SRC-IP-CIDR`, `SRC-PORT`, `GEOIP`, `GEOSITE`, `RULE-SET`, `MATCH`, `MATCH-SRC`。

---

## TUI 交互界面

```bash
labproxy tui
```

**快捷键**
| 键位 | 功能 |
|-----|------|
| `↑/↓` 或 `j/k` | 导航 |
| `Tab` / `←/→` | 切换面板 (Groups / Options / Connections) |
| `Enter` | 执行 / 选中 |
| `d` / `D` | 关闭当前连接 / 关闭全部连接（连接面板，二次确认） |
| `T` | 批量测速当前组 |
| `L` | 实时日志覆盖视图（`l` 切级别，`esc` 退出） |
| `R` | 规则管理弹窗 |
| `s` | 聚焦 Settings |
| `m` | 切换代理模式 |
| `p` | 切换 system proxy |
| `r` | 刷新延迟 |
| `/` | 搜索 |
| `q` | 退出 |

> **维护者提示**：修改 TUI 源码后执行 `VERSION=dev bash scripts/build-tui.sh` 重新生成预编译包。

---

## 目录结构

```
labproxy/                          ~/.labproxy/
├── cmd/labproxy-tui/              ├── bin/
├── internal/                      │   ├── mihomo              # 代理内核
│   ├── config/                    │   ├── labproxy-tui        # TUI
│   ├── proxy/                     │   ├── subconverter        # 订阅转换
│   └── tui/                       │   └── yq                  # YAML 工具
├── scripts/                       ├── config/
│   ├── proxyctl.sh                │   ├── mixin.yaml
│   ├── common.sh                  │   └── ports.conf
│   └── build-tui.sh               ├── logs/
├── resources/zip/                 │   └── labproxy.log
├── install.sh                     ├── scripts/
├── go.mod                         └── ui/
└── README.md
```

---

## 常见问题

**Q: SSH 断开后代理会停止吗？**  
A: 不会。使用 `nohup` 后台运行，与 SSH 会话无关。

**Q: 如何固定代理端口？**  
A: `labproxy port set 7890`，冲突时自动提示重新选择。

**Q: Web 控制台打不开？**  
A: 检查防火墙是否放行管理端口（默认 9090，冲突时自动调整）。

**Q: 局域网内其他设备如何使用？**  
A: `labproxy lan on` 开启后，其他设备设置代理为 `http://<本机IP>:<端口>`。

---

## 相关项目

- [mihomo](https://github.com/MetaCubeX/mihomo) — 代理内核
- [subconverter](https://github.com/tindy2013/subconverter) — 订阅转换
- [zashboard](https://github.com/Zephyruso/zashboard) — Web UI
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI 框架

基于 [clash-for-linux-install](https://github.com/nelvko/clash-for-linux-install) 二次开发。

## License

[MIT License](LICENSE)

---

<p align="center">
  如果这个工具对你有帮助，请给我们一颗 ⭐ <a href="https://github.com/Azhi-ss/labproxy">Star</a>
</p>

## 致谢

本项目受到 [LINUX DO](https://linux.do/) 社区的启发和支持。
