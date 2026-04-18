# LabProxy

<img src="resources/hero-banner.png" alt="LabProxy Banner" width="100%"/>

<p align="center">
  <a href="https://github.com/Azhi-ss/labproxy/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/Azhi-ss/labproxy" alt="License">
  </a>
  <img src="https://img.shields.io/github/languages/top/Azhi-ss/labproxy" alt="Language">
</p>

<p align="center"><b>专为实验室/共享服务器设计的用户空间代理管理工具</b></p>

---

## 为什么需要 LabProxy？

| 传统方案 | LabProxy |
|---------|---------|
| 需要 sudo 权限 | ✅ 纯用户空间，无需 root |
| 依赖 GUI 或 systemd | ✅ 纯命令行，PID 文件管理 |
| 端口冲突导致启动失败 | ✅ 自动检测并分配可用端口 |
| 多用户环境配置冲突 | ✅ 完全隔离的用户目录 |

<img src="resources/concept.png" alt="概念示意" width="500" align="right"/>

**LabProxy** 基于 [clash-for-linux-install](https://github.com/nelvko/clash-for-linux-install) 二次开发，针对实验室场景优化：

- **无特权安装** — 安装到 `~/.labproxy/`，普通用户即可使用
- **智能端口** — 7890/9090 被占用？自动寻找可用端口
- **TUI 界面** — 终端下的图形化管理，实时流量/节点/连接
- **Web 控制台** — 浏览器管理，支持密钥保护
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

## TUI 交互界面

```bash
labproxy tui
```

<img src="resources/tui-art.png" alt="TUI" width="100%"/>

**快捷键**
| 键位 | 功能 |
|-----|------|
| `↑/↓` 或 `j/k` | 导航 |
| `Tab` / `←/→` | 切换面板 (Groups / Options / Settings) |
| `Enter` | 执行 |
| `s` | 聚焦 Settings |
| `m` | 切换代理模式 |
| `p` | 切换 system proxy |
| `r` | 刷新延迟 |
| `/` | 搜索 |
| `q` | 退出 |

<details>
<summary><b>📸 真实界面截图</b></summary>

| CLI 命令行 | TUI 界面 |
|:---:|:---:|
| <img src="resources/image.png" width="400"/> | <img src="resources/tui.png" width="400"/> |

</details>

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
