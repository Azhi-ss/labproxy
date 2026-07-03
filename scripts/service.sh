#!/usr/bin/env bash
# labproxy 特权服务管理模块（方案 A）
#
# macOS 上 mihomo 创建 TUN 接口需要 root。本模块提供 launchd LaunchDaemon
# 服务，让 mihomo 以 root 运行，同时保留 labproxy 用户态管理的兼容性。
#
# 设计要点：
#   - 服务模式（已安装且运行）：mihomo 以 root 跑，KeepAlive 自动拉起
#   - 日常配置变更：通过 mihomo RESTful API 热重载（PUT /configs），零 sudo
#   - 装/卸服务：需 sudo（一次性）
#   - 未装服务时：回退原用户态 nohup 模式，行为不变

# launchd 服务标签与路径
_LABPROXY_SERVICE_LABEL="com.labproxy.mihomo"
_LABPROXY_SERVICE_PLIST="/Library/LaunchDaemons/${_LABPROXY_SERVICE_LABEL}.plist"

# 服务是否已安装（plist 文件存在）
# LABPROXY_FORCE_USER_MODE=1 时强制返回 false，便于在已装特权服务的机器上
# 仍走用户空间模式（也用于测试隔离，避免全局 plist 污染用户空间安装流程）。
_labproxy_service_installed() {
    [ "${LABPROXY_FORCE_USER_MODE:-0}" = "1" ] && return 1
    [ -f "$_LABPROXY_SERVICE_PLIST" ]
}

# 服务是否已加载并运行
# 注：系统级 LaunchDaemon 在 launchctl list 中对普通用户不可见，故用 pgrep 检测进程
_labproxy_service_active() {
    _labproxy_service_installed || return 1
    if command -v _find_existing_kernel_pid >/dev/null 2>&1; then
        _find_existing_kernel_pid >/dev/null 2>&1
        return $?
    fi
    pgrep -f "${LABPROXY_HOME_DIR}/bin/${BIN_KERNEL_NAME}" >/dev/null 2>&1
}

# 当前是否处于服务模式（已安装即视为服务模式优先）
_labproxy_service_mode() {
    _labproxy_service_installed
}

# 生成 plist 内容到指定路径
_labproxy_service_generate_plist() {
    local out="${1:-$_LABPROXY_SERVICE_PLIST}"
    local mihomo_bin="${LABPROXY_HOME_DIR}/bin/${BIN_KERNEL_NAME}"
    local log_dir="${LABPROXY_HOME_DIR}/logs"
    mkdir -p "$(dirname "$out")"
    cat > "$out" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${_LABPROXY_SERVICE_LABEL}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${mihomo_bin}</string>
        <string>-d</string>
        <string>${LABPROXY_HOME_DIR}</string>
        <string>-f</string>
        <string>${LABPROXY_CONFIG_RUNTIME}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${log_dir}/mihomo-service.log</string>
    <key>StandardErrorPath</key>
    <string>${log_dir}/mihomo-service.log</string>
</dict>
</plist>
EOF
}

# 通过 mihomo API 热重载配置（零 sudo）
# 返回 0 成功，非 0 失败
_labproxy_service_reload() {
    _get_ui_port 2>/dev/null || true
    local port="${UI_PORT:-9090}"
    local secret=""
    [ -f "$LABPROXY_CONFIG_RUNTIME" ] && secret=$("$BIN_YQ" '.secret // ""' "$LABPROXY_CONFIG_RUNTIME" 2>/dev/null)

    local url="http://127.0.0.1:${port}/configs?force=true"
    local auth_args=()
    [ -n "$secret" ] && auth_args=(-H "Authorization: Bearer ${secret}")

    # force=true 会重新加载 path 指定的配置文件
    curl -s -m 10 -X PUT "$url" \
        "${auth_args[@]}" \
        -H "Content-Type: application/json" \
        -d "{\"path\":\"${LABPROXY_CONFIG_RUNTIME}\"}" \
        -o /dev/null -w "%{http_code}" 2>/dev/null | grep -qE "^(200|204|304)$"
}

# 安装特权服务（需 sudo）
_labproxy_service_install() {
    if _labproxy_service_installed; then
        _okcat '✅' "特权服务已安装：${_LABPROXY_SERVICE_PLIST}"
        return 0
    fi

    # 先生成到临时位置，再 sudo 安装
    local tmp_plist
    tmp_plist="$(mktemp /tmp/labproxy-service.XXXXXX.plist)"
    _labproxy_service_generate_plist "$tmp_plist" || {
        _failcat "生成 plist 失败"
        rm -f "$tmp_plist"
        return 1
    }

    _okcat '⏳' "安装特权服务（需要 sudo 密码）..."
    sudo install -m 644 -o root -g wheel "$tmp_plist" "$_LABPROXY_SERVICE_PLIST" 2>/dev/null
    local rc=$?
    rm -f "$tmp_plist"
    [ $rc -ne 0 ] && { _failcat "安装 plist 失败（sudo 未授权？）"; return 1; }

    # 先停掉用户态 mihomo，避免端口冲突
    if is_labproxy_running; then
        _okcat '⏳' "停止用户态 mihomo 以避免端口冲突..."
        stop_labproxy >/dev/null 2>&1
    fi

    # 加载服务
    sudo launchctl bootstrap system "$_LABPROXY_SERVICE_PLIST" 2>/dev/null || {
        _failcat "加载服务失败，请检查：sudo launchctl bootstrap system ${_LABPROXY_SERVICE_PLIST}"
        return 1
    }
    sleep 2

    if _labproxy_service_active; then
        _okcat '✅' "特权服务已启动（root 模式，TUN 可用）"
        _okcat '💡' "日常改配置走 API 热重载，无需 sudo"
    else
        _failcat "服务已安装但未运行，查看日志：${LABPROXY_HOME_DIR}/logs/mihomo-service.log"
        return 1
    fi
}

# 卸载特权服务（需 sudo）
_labproxy_service_uninstall() {
    if ! _labproxy_service_installed; then
        _okcat '✅' "特权服务未安装"
        return 0
    fi

    _okcat '⏳' "卸载特权服务（需要 sudo 密码）..."
    sudo launchctl bootout "system/${_LABPROXY_SERVICE_LABEL}" 2>/dev/null
    sudo rm -f "$_LABPROXY_SERVICE_PLIST" 2>/dev/null

    if _labproxy_service_installed; then
        _failcat "卸载失败"
        return 1
    fi
    _okcat '✅' "特权服务已卸载，回退用户态模式"
    _okcat '💡' "运行 labproxy on 以用户态启动"
}

# 服务状态查询
_labproxy_service_status() {
    if ! _labproxy_service_installed; then
        _failcat "特权服务：未安装"
        return 1
    fi
    _okcat "特权服务：已安装"
    if _labproxy_service_active; then
        _okcat '✅' "服务运行中"
    else
        _failcat "服务已安装但未运行"
        _okcat '💡' "启动：sudo launchctl bootstrap system ${_LABPROXY_SERVICE_PLIST}"
    fi
}

# labproxy service <install|uninstall|status|reload> 命令入口
labproxyservice() {
    local sub="${1:-status}"
    case "$sub" in
    install)
        _labproxy_service_install
        ;;
    uninstall|remove)
        _labproxy_service_uninstall
        ;;
    status)
        _labproxy_service_status
        ;;
    reload)
        _labproxy_service_reload && _okcat '✅' "配置已热重载" || _failcat "热重载失败"
        ;;
    *)
        cat <<EOF
Usage: labproxy service <install|uninstall|status|reload>

特权服务管理（macOS TUN 模式需要 root）：
    install     安装 launchd 特权服务（需 sudo，一次性）
    uninstall   卸载特权服务（需 sudo）
    status      查看服务状态
    reload      通过 API 热重载配置（零 sudo）

说明：
    • 安装后 mihomo 以 root 运行，TUN 模式可用
    • 日常改配置自动走 API 热重载，无需重启进程
    • 未安装时回退用户态 nohup 模式
EOF
        ;;
    esac
}
