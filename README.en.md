# LabProxy

<img src="resources/hero-banner.png" alt="LabProxy Banner" width="100%"/>

<p align="center">
  <a href="./README.md">简体中文</a> | <strong>English</strong>
</p>

<p align="center">
  <a href="https://github.com/Azhi-ss/labproxy/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/Azhi-ss/labproxy" alt="License">
  </a>
  <img src="https://img.shields.io/github/languages/top/Azhi-ss/labproxy" alt="Language">
</p>

<p align="center"><b>A user-space proxy manager built for labs and shared servers</b></p>

---

## Why LabProxy?

| Traditional approach | LabProxy |
|---------|---------|
| Requires sudo privileges | ✅ Pure user space, no root required |
| Depends on GUI or systemd | ✅ CLI-only, PID-file based management |
| Fails to start when ports conflict | ✅ Automatically detects and assigns available ports |
| Conflicts in multi-user environments | ✅ Fully isolated per-user directory |

<img src="resources/concept.png" alt="Concept Diagram" width="500" align="right"/>

**LabProxy** is based on [clash-for-linux-install](https://github.com/nelvko/clash-for-linux-install) and optimized for lab/shared-server scenarios:

- **Unprivileged install** — installs into `~/.labproxy/`, works for regular users
- **Smart port handling** — 7890/9090 already taken? It automatically finds available ports
- **TUI interface** — terminal-native management with live traffic, proxy, and connection views
- **Web dashboard** — browser-based management with secret protection
- **Automatic subscription conversion** — built-in subconverter for multiple subscription formats

---

## Quick Start

```bash
# 1. Clone and install
git clone https://github.com/Azhi-ss/labproxy.git && cd labproxy
bash install.sh

# 2. Configure subscription (required)
labproxy subscribe https://your-subscription-url

# 3. Start
labproxy on

# 4. Verify
curl -I https://www.google.com
```

<details>
<summary><b>📋 Full Installation Guide</b></summary>

**Requirements**
- Shell: `bash` / `zsh` / `fish`
- Privileges: regular user only, no sudo required
- Dependency: a valid Clash subscription URL

**Installation flow**
```bash
git clone https://github.com/Azhi-ss/labproxy.git
cd labproxy
bash install.sh        # installs to ~/.labproxy/ by default
```

The installer automatically:
- downloads the correct mihomo binary for your architecture
- configures shell environment variables
- sets command aliases
- detects and assigns available ports

</details>

---

## Core Commands

```
labproxy on              # start proxy
labproxy off             # stop proxy
labproxy status          # show status
labproxy tui             # open the TUI
```

| Command | Description |
|-----|------|
| `labproxy port [set <port>\|auto\|status]` | pin a port / auto-assign |
| `labproxy lan [on\|off\|status]` | LAN access control |
| `labproxy proxy [on\|off\|status]` | system proxy toggle |
| `labproxy subscribe [URL]` | set/show subscription |
| `labproxy update [auto]` | refresh subscription config |
| `labproxy ui` | show web dashboard address |
| `labproxy mixin [-e\|-r]` | edit/show config |

---

## TUI Interface

```bash
labproxy tui
```

<img src="resources/tui-art.png" alt="TUI" width="100%"/>

**Hotkeys**
| Key | Action |
|-----|------|
| `↑/↓` or `j/k` | navigate |
| `Tab` / `←/→` | switch panels (Groups / Options / Settings) |
| `Enter` | apply |
| `s` | focus Settings |
| `m` | switch proxy mode |
| `p` | toggle system proxy |
| `r` | refresh delay |
| `/` | search |
| `q` | quit |

<details>
<summary><b>📸 Real UI Screenshots</b></summary>

| CLI | TUI |
|:---:|:---:|
| <img src="resources/image.png" width="400"/> | <img src="resources/tui.png" width="400"/> |

</details>

> **Maintainer note**: after changing TUI source code, run `VERSION=dev bash scripts/build-tui.sh` to regenerate prebuilt archives.

---

## Project Layout

```
labproxy/                          ~/.labproxy/
├── cmd/labproxy-tui/              ├── bin/
├── internal/                      │   ├── mihomo              # proxy core
│   ├── config/                    │   ├── labproxy-tui        # TUI
│   ├── proxy/                     │   ├── subconverter        # subscription conversion
│   └── tui/                       │   └── yq                  # YAML utility
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

## FAQ

**Q: Will the proxy stop after my SSH session disconnects?**  
A: No. It runs in the background via `nohup`, independent of your SSH session.

**Q: How do I pin the proxy port?**  
A: Use `labproxy port set 7890`. If that port is occupied, LabProxy will prompt/handle reassignment.

**Q: The web dashboard does not open. What should I check?**  
A: Make sure the management port is allowed by your firewall. The default is 9090, but it may change if there is a conflict.

**Q: How can other devices on my LAN use it?**  
A: Run `labproxy lan on`, then configure those devices to use `http://<your-host-ip>:<port>` as the proxy.

---

## Related Projects

- [mihomo](https://github.com/MetaCubeX/mihomo) — proxy core
- [subconverter](https://github.com/tindy2013/subconverter) — subscription conversion
- [zashboard](https://github.com/Zephyruso/zashboard) — Web UI
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework

Originally derived from [clash-for-linux-install](https://github.com/nelvko/clash-for-linux-install).

## License

[MIT License](LICENSE)

---

<p align="center">
  If this tool helps you, please give it a ⭐ <a href="https://github.com/Azhi-ss/labproxy">Star</a>
</p>
