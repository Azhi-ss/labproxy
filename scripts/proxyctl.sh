# shellcheck disable=SC2148
# shellcheck disable=SC2155

_set_system_proxy() {
    # Ensure config files exist before reading
    [ ! -f "$LABPROXY_CONFIG_RUNTIME" ] && {
        _failcat "运行时配置文件不存在：${LABPROXY_CONFIG_RUNTIME}"
        return 1
    }
    
    local auth=$("$BIN_YQ" '.authentication[0] // ""' "$LABPROXY_CONFIG_RUNTIME" 2>/dev/null)
    [ -n "$auth" ] && auth=$auth@

    local http_proxy_addr="http://${auth}127.0.0.1:${MIXED_PORT}"
    local socks_proxy_addr="socks5h://${auth}127.0.0.1:${MIXED_PORT}"
    local no_proxy_addr="localhost,127.0.0.1,::1"

    # Save pre-existing proxy values so we can restore them when labproxy proxy is turned off
    # (e.g. Windows autoProxy in WSL mirrored mode)
    _save_external_proxy

    export http_proxy=$http_proxy_addr
    export https_proxy=$http_proxy
    export HTTP_PROXY=$http_proxy
    export HTTPS_PROXY=$http_proxy

    export all_proxy=$socks_proxy_addr
    export ALL_PROXY=$all_proxy

    export no_proxy=$no_proxy_addr
    export NO_PROXY=$no_proxy

    # Ensure mixin config directory exists and update using user permissions
    mkdir -p "$(dirname "$LABPROXY_CONFIG_MIXIN")"
    "$BIN_YQ" -i '.system-proxy.enable = true' "$LABPROXY_CONFIG_MIXIN" 2>/dev/null || {
        _failcat "无法更新系统代理配置"
        return 1
    }
}

_unset_system_proxy() {
    # Restore external proxy (e.g. Windows autoProxy in WSL mirrored mode)
    # if one was saved before labproxy overwrote it.
    if ! _restore_external_proxy; then
        unset http_proxy
        unset https_proxy
        unset HTTP_PROXY
        unset HTTPS_PROXY
        unset all_proxy
        unset ALL_PROXY
        unset no_proxy
        unset NO_PROXY
    fi

    # Ensure mixin config exists and update using user permissions
    mkdir -p "$(dirname "$LABPROXY_CONFIG_MIXIN")"
    "$BIN_YQ" -i '.system-proxy.enable = false' "$LABPROXY_CONFIG_MIXIN" 2>/dev/null || {
        _failcat "无法更新系统代理配置"
    }
}

function labproxyon() {
    _prepare_runtime_start || return 1

    if ! _start_runtime; then
        _failcat '代理启动失败'
        return 1
    fi

    _finalize_runtime_start
}

# 验证实际监听端口与配置是否一致
_verify_actual_ports() {
    local log_file="$LABPROXY_HOME_DIR/logs/labproxy.log"
    [ ! -f "$log_file" ] && return 0
    
    # Extract actual listening ports from log
    # Try both old format (Mixed) and new format (HTTP proxy)
    local actual_proxy_port=$(grep "Mixed(http+socks) proxy listening at:" "$log_file" | tail -1 | sed -n 's/.*127\.0\.0\.1:\([0-9]*\).*/\1/p')
    [ -z "$actual_proxy_port" ] && actual_proxy_port=$(grep "HTTP proxy listening at:" "$log_file" | tail -1 | sed -n 's/.*127\.0\.0\.1:\([0-9]*\).*/\1/p')
    
    local actual_ui_port=$(grep "RESTful API listening at:" "$log_file" | tail -1 | sed -n 's/.*:\([0-9]\+\)[^0-9]*$/\1/p')
    local actual_dns_port=$(grep "DNS server(UDP) listening at:" "$log_file" | tail -1 | sed -n 's/.*:\([0-9]\+\)[^0-9]*$/\1/p')
    
    # 从配置文件获取期望端口进行比较
    local config_proxy_port=$("$BIN_YQ" '.mixed-port // 7890' "$LABPROXY_CONFIG_RUNTIME" 2>/dev/null)
    local config_ui_addr=$("$BIN_YQ" '.external-controller // "127.0.0.1:9090"' "$LABPROXY_CONFIG_RUNTIME" 2>/dev/null)
    local config_ui_port=${config_ui_addr##*:}
    local config_dns_addr=$("$BIN_YQ" '.dns.listen // "0.0.0.0:15353"' "$LABPROXY_CONFIG_RUNTIME" 2>/dev/null)
    local config_dns_port=${config_dns_addr##*:}
    
    local port_changed=false
    
    # 设置实际监听端口到变量
    if [ -n "$actual_proxy_port" ]; then
        MIXED_PORT=$actual_proxy_port
        [ "$actual_proxy_port" != "$config_proxy_port" ] && {
            _failcat "🔄" "LabProxy 自动调整代理端口：${config_proxy_port} → ${actual_proxy_port}"
            port_changed=true
        }
    else
        MIXED_PORT=$config_proxy_port
    fi
    
    if [ -n "$actual_ui_port" ]; then
        UI_PORT=$actual_ui_port
        [ "$actual_ui_port" != "$config_ui_port" ] && {
            _failcat "🔄" "LabProxy 自动调整 UI 端口：${config_ui_port} → ${actual_ui_port}"
            port_changed=true
        }
    else
        UI_PORT=$config_ui_port
    fi
    
    if [ -n "$actual_dns_port" ]; then
        DNS_PORT=$actual_dns_port
        [ "$actual_dns_port" != "$config_dns_port" ] && {
            _failcat "🔄" "LabProxy 自动调整 DNS 端口：${config_dns_port} → ${actual_dns_port}"
            port_changed=true
        }
    else
        DNS_PORT=$config_dns_port
    fi
    
    # 只有当端口有变化时才显示最终端口分配并重新设置系统代理
    if [ "$port_changed" = true ]; then
        _okcat "最终端口分配 — 代理:${MIXED_PORT} UI:${UI_PORT} DNS:${DNS_PORT}"
        # 保存实际监听端口到状态文件
        _save_port_state "$MIXED_PORT" "$UI_PORT" "$DNS_PORT"
        # 端口变化时重新设置系统代理环境变量
        _set_system_proxy
    fi
}

# Detect if current proxy is injected by WSL mirrored mode autoProxy
_is_wsl_auto_proxy() {
    # Only relevant in WSL with mirrored networking and autoProxy enabled
    [ -n "$http_proxy" ] || return 1
    [ -f /proc/version ] || return 1
    grep -qi microsoft /proc/version 2>/dev/null || return 1

    # Check if .wslconfig has autoProxy=true (via /mnt/c/Users/*/.wslconfig)
    local found_auto_proxy=false
    local wslconfig
    for wslconfig in /mnt/c/Users/*/.wslconfig; do
        [ -f "$wslconfig" ] || continue
        grep -qi 'autoProxy.*true' "$wslconfig" 2>/dev/null && {
            found_auto_proxy=true
            break
        }
    done

    [ "$found_auto_proxy" = "true" ]
}

# External proxy state file (for saving/restoring Windows autoProxy etc.)
_EXTERNAL_PROXY_STATE="${LABPROXY_HOME_DIR}/config/external-proxy.state"

# Save current proxy environment variables as "external" before labproxy overwrites them
_save_external_proxy() {
    [ -z "$http_proxy" ] && return 0

    # Don't save if current proxy is labproxy's own proxy
    _get_proxy_port
    case "$http_proxy" in
        *"127.0.0.1:${MIXED_PORT}"*|*"localhost:${MIXED_PORT}"*)
            # This is already labproxy's proxy, skip
            return 0
            ;;
    esac

    mkdir -p "$(dirname "$_EXTERNAL_PROXY_STATE")"
    cat > "$_EXTERNAL_PROXY_STATE" <<PROXYEOF
http_proxy=${http_proxy}
https_proxy=${https_proxy}
HTTP_PROXY=${HTTP_PROXY}
HTTPS_PROXY=${HTTPS_PROXY}
all_proxy=${all_proxy}
ALL_PROXY=${ALL_PROXY}
no_proxy=${no_proxy}
NO_PROXY=${NO_PROXY}
PROXYEOF
}

# Restore previously saved external proxy, return 1 if nothing to restore
_restore_external_proxy() {
    [ -f "$_EXTERNAL_PROXY_STATE" ] || return 1

    # Read and export saved values
    while IFS='=' read -r key value; do
        [ -n "$key" ] && [ -n "$value" ] && export "$key=$value"
    done < "$_EXTERNAL_PROXY_STATE"

    rm -f "$_EXTERNAL_PROXY_STATE"
    [ -n "$http_proxy" ] && _okcat '🔄' "已恢复外部代理：${http_proxy}"
    return 0
}

watch_proxy() {
    # 新开交互式shell时
    [[ $- == *i* ]] || return 0

    # 检查用户是否启用系统代理
    local system_proxy_status=$("$BIN_YQ" '.system-proxy.enable // true' "$LABPROXY_CONFIG_MIXIN" 2>/dev/null)
    [ "$system_proxy_status" = "true" ] || return 0

    # 仅当 labproxy 进程运行时才设置代理
    is_labproxy_running || return 0

    _get_proxy_port
    _get_ui_port
    _get_dns_port

    if [ -z "$http_proxy" ]; then
        # 无现有代理，直接设置 labproxy 代理
        _set_system_proxy
    elif _is_wsl_auto_proxy; then
        # WSL mirrored autoProxy 已注入 Windows 代理
        # 保存 Windows 代理，然后覆盖为 labproxy 代理
        _okcat '🔄' "检测到 Windows autoProxy（${http_proxy}），已切换为 LabProxy 代理（端口 ${MIXED_PORT}）"
        _save_external_proxy
        local auth=$("$BIN_YQ" '.authentication[0] // ""' "$LABPROXY_CONFIG_RUNTIME" 2>/dev/null)
        [ -n "$auth" ] && auth=$auth@
        export http_proxy="http://${auth}127.0.0.1:${MIXED_PORT}"
        export https_proxy=$http_proxy
        export HTTP_PROXY=$http_proxy
        export HTTPS_PROXY=$http_proxy
        export all_proxy="socks5h://${auth}127.0.0.1:${MIXED_PORT}"
        export ALL_PROXY=$all_proxy
    fi
    # 如果 http_proxy 已有值且非 autoProxy，说明用户手动设置了其他代理，不覆盖
}

function labproxyoff() {
    # Stop mihomo process
    stop_labproxy
    _unset_system_proxy
    _okcat '代理环境已关闭'
}

function labproxyrestart() {
    _okcat "正在重启代理服务..."
    { labproxyoff && labproxyon; } >&/dev/null && _okcat "代理服务重启成功"
}

function labproxyproxy() {
    case "$1" in
    on)
        if is_labproxy_running; then
            _get_proxy_port
            _get_ui_port
            _get_dns_port
            _set_system_proxy
            _okcat '系统代理已开启'
        else
            _failcat '无法开启系统代理：LabProxy 进程未运行'
            return 1
        fi
        ;;
    off)
        _unset_system_proxy
        _okcat '系统代理已关闭'
        ;;
    status)
        local system_proxy_status=$("$BIN_YQ" '.system-proxy.enable' "$LABPROXY_CONFIG_MIXIN" 2>/dev/null)
        if [ "$system_proxy_status" = "false" ]; then
            _failcat "系统代理：关闭"
            return 1
        fi
        
        if is_labproxy_running; then
            _okcat "系统代理：开启
http_proxy： $http_proxy
socks_proxy：$all_proxy"
        else
            _failcat "系统代理：配置为开启，但 LabProxy 进程未运行"
            return 1
        fi
        ;;
    *)
        cat <<EOF
用法: labproxyproxy [on|off|status]
    on      开启系统代理
    off     关闭系统代理
    status  查看系统代理状态
EOF
        ;;
    esac
}

function labproxyport() {
    local action=$1
    shift || true

    case "$action" in
    ""|status)
        _load_port_preferences
        _get_proxy_port
        local mode_msg
        if [ "$PORT_PREF_MODE" = "manual" ] && [ -n "$PORT_PREF_VALUE" ]; then
            mode_msg="固定(${PORT_PREF_VALUE})"
        else
            mode_msg="自动"
        fi
        _okcat "端口模式：${mode_msg}"
        _okcat "当前代理端口：${MIXED_PORT}"
        ;;
    auto)
        _save_port_preferences auto ""
        _okcat "已切换为自动分配代理端口"
        if is_labproxy_running; then
            _okcat "正在重新应用配置..."
            labproxyrestart
        fi
        ;;
    set|manual)
        local manual_port=$1
        local prefer_auto=false

        while true; do
            if [ -z "$manual_port" ]; then
                printf "请输入想要固定的代理端口 [1024-65535]: "
                read -r manual_port
            fi

            if [ -z "$manual_port" ]; then
                _failcat "未输入端口"
                continue
            fi

            if ! [[ $manual_port =~ ^[0-9]+$ ]] || [ "$manual_port" -lt 1024 ] || [ "$manual_port" -gt 65535 ]; then
                _failcat "端口号无效，请输入 1024-65535 之间的数字"
                manual_port=""
                continue
            fi

            if _is_already_in_use "$manual_port" "$BIN_KERNEL_NAME"; then
                _failcat "端口 ${manual_port} 已被占用"
                printf "选择操作 [r]重新输入/[a]自动分配: "
                read -r choice
                case "$choice" in
                [aA])
                    prefer_auto=true
                    break
                    ;;
                [rR])
                    manual_port=""
                    continue
                    ;;
                *)
                    manual_port=""
                    continue
                    ;;
                esac
            fi

            break
        done

        if [ "$prefer_auto" = true ]; then
            _save_port_preferences auto ""
            _okcat "已切换为自动分配代理端口"
        else
            _save_port_preferences manual "$manual_port"
            _okcat "已固定代理端口：${manual_port}"
        fi

        if is_labproxy_running; then
            _okcat "正在重新应用配置..."
            labproxyrestart
        fi
        ;;
    *)
        cat <<EOF
用法: labproxyport [status|auto|set <port>]
    status          查看当前代理端口模式与端口
    auto            切换为自动分配代理端口
    set <port>      固定代理端口，端口冲突时可选择重新输入或自动分配
EOF
        ;;
    esac
}

function labproxystatus() {
    local pid_file="$LABPROXY_HOME_DIR/config/labproxy.pid"
    local log_file="$LABPROXY_HOME_DIR/logs/labproxy.log"
    
    # Show subscription URL
    local subscription_url=$(cat "$LABPROXY_CONFIG_URL" 2>/dev/null)
    if [ -n "$subscription_url" ]; then
        _okcat "订阅地址：${subscription_url}"
    else
        _failcat "订阅地址：未设置"
    fi
    
    if is_labproxy_running; then
        local pid=$(cat "$pid_file" 2>/dev/null)
        local uptime=$(ps -o etime= -p "$pid" 2>/dev/null | tr -d ' ')
        _okcat "LabProxy 进程状态：运行中"
        _okcat "进程 PID：${pid}"
        _okcat "运行时间：${uptime:-未知}"
        _okcat "配置文件：${LABPROXY_CONFIG_RUNTIME}"
        _okcat "日志文件：${log_file}"
        
        # Show proxy port status
        if [ -f "$LABPROXY_CONFIG_RUNTIME" ]; then
            _get_proxy_port
            _get_ui_port
            _get_dns_port
            _okcat "代理端口：${MIXED_PORT}"
            _okcat "管理端口：${UI_PORT}"
            _okcat "DNS 端口：${DNS_PORT}"
        else
            _failcat "配置文件不存在，无法获取端口信息"
        fi
        
        # Show system proxy status
        labproxyproxy status
    else
        _failcat "LabProxy 进程状态：未运行"
        [ -f "$pid_file" ] && {
            _failcat "发现残留 PID 文件，已清理"
            rm -f "$pid_file"
        }
        return 1
    fi
}

function labproxyui() {
    _get_ui_port
    # 公网ip
    # ifconfig.me
    local query_url='api64.ipify.org'
    local public_ip=$(curl -s --noproxy "*" --connect-timeout 2 $query_url)
    local public_address="http://${public_ip:-公网}:${UI_PORT}/ui"
    # 内网ip
    # ip route get 1.1.1.1 | grep -oP 'src \K\S+'
    local local_ip=$(hostname -I | awk '{print $1}')
    local local_address="http://${local_ip}:${UI_PORT}/ui"
    printf "\n"
    printf "╔═══════════════════════════════════════════════╗\n"
    printf "║                %s                  ║\n" "$(_okcat '🐙 Web 控制台')"
    printf "║═══════════════════════════════════════════════║\n"
    printf "║                                               ║\n"
    printf "║     🔓 注意放行端口：%-5s                    ║\n" "$UI_PORT"
    printf "║     🖥️  内网：%-31s  ║\n" "$local_address"
    printf "║     🌐 公网：%-31s  ║\n" "$public_address"
    printf "║     ☁️  公共：%-31s  ║\n" "$URL_LABPROXY_UI"
    printf "║                                               ║\n"
    printf "╚═══════════════════════════════════════════════╝\n"
    printf "\n"
}

function labproxytui() {
    local tui_bin="${LABPROXY_TUI_BIN}"

    # 懒加载 / 能力升级: 首次使用时构建，或检测到旧版二进制时自动重建
    if ! _ensure_tui_binary "$tui_bin"; then
        if [ ! -x "$tui_bin" ]; then
            return 1
        fi
        _failcat '⚠️' '当前 TUI 二进制较旧，将以兼容模式启动（Apply / Restart 不可用）'
    fi

    # 确保 mihomo 运行
    if ! is_labproxy_running; then
        _okcat "正在启动 Mihomo..."
        labproxyon || return 1
    fi

    # 获取实际端口
    _verify_actual_ports
    [ -z "${UI_PORT:-}" ] && _get_ui_port

    # 检查端口可用性
    if ! _is_bind "$UI_PORT" 2>/dev/null; then
        _failcat "API 端口 ${UI_PORT} 未监听，请执行 labproxy status 检查"
        return 1
    fi

    # 生成配置并启动 TUI
    local endpoint="http://127.0.0.1:${UI_PORT}"
    local api_secret=$("$BIN_YQ" '.secret // ""' "$LABPROXY_CONFIG_RUNTIME" 2>/dev/null)
    local restart_command="source \"$LABPROXY_SCRIPT_DIR/common.sh\" && source \"$LABPROXY_SCRIPT_DIR/proxyctl.sh\" && labproxyrestart"

    # 读取语言偏好
    local lang_flag=""
    if [ -f "$LABPROXY_LANG_FILE" ]; then
        local lang_val
        lang_val=$(head -n 1 "$LABPROXY_LANG_FILE" | tr -d '[:space:]')
        if [ "$lang_val" = "zh" ] || [ "$lang_val" = "en" ]; then
            lang_flag="--lang $lang_val"
        fi
    fi

    _okcat "正在连接 $endpoint ..."
    if _tui_supports_restart_command "$tui_bin"; then
        "$tui_bin" \
            --endpoint "$endpoint" \
            --secret "$api_secret" \
            --mixin-config "$LABPROXY_CONFIG_MIXIN" \
            --restart-command "$restart_command" \
            $lang_flag
    else
        "$tui_bin" \
            --endpoint "$endpoint" \
            --secret "$api_secret" \
            --mixin-config "$LABPROXY_CONFIG_MIXIN" \
            $lang_flag
    fi
}

_merge_config_restart() {
    _apply_runtime_change
}

_build_runtime_config() {
    # Use user-accessible temp directory instead of /tmp
    local backup="${LABPROXY_HOME_DIR}/config/runtime.backup"

    # Ensure config directory exists
    mkdir -p "$(dirname "$backup")"

    # Backup current runtime config
    cat "$LABPROXY_CONFIG_RUNTIME" 2>/dev/null > "$backup"

    # Merge configurations using user permissions
    "$BIN_YQ" eval-all '. as $item ireduce ({}; . *+ $item) | (.. | select(tag == "!!seq")) |= unique' \
        "$LABPROXY_CONFIG_MIXIN" "$LABPROXY_CONFIG_RAW" "$LABPROXY_CONFIG_MIXIN" > "$LABPROXY_CONFIG_RUNTIME" || {
        cat "$backup" > "$LABPROXY_CONFIG_RUNTIME" 2>/dev/null
        _error_quit "生成运行时配置失败"
        return 1
    }

    # Validate merged configuration
    _valid_config "$LABPROXY_CONFIG_RUNTIME" || {
        # Restore backup on validation failure
        cat "$backup" > "$LABPROXY_CONFIG_RUNTIME" 2>/dev/null
        _error_quit "校验失败，请检查 Mixin 配置"
        return 1
    }

    # Clean up backup file
    rm -f "$backup"
}

_prepare_runtime_start() {
    _build_runtime_config || return 1
    _resolve_port_conflicts "$LABPROXY_CONFIG_RUNTIME" true
}

_start_runtime() {
    start_labproxy
}

_finalize_runtime_start() {
    # Wait for mihomo to fully start
    sleep 2

    # 验证实际端口并设置端口变量
    _verify_actual_ports

    # 保存端口状态并设置系统代理
    _save_port_state "$MIXED_PORT" "$UI_PORT" "$DNS_PORT"
    _set_system_proxy
    _okcat '代理环境已开启'
    _show_proxy_info
}

_show_proxy_info() {
    _okcat "代理端口：${MIXED_PORT}  管理端口：${UI_PORT}  DNS 端口：${DNS_PORT}"
    _okcat "http://127.0.0.1:${MIXED_PORT}  socks5h://127.0.0.1:${MIXED_PORT}"
}

_restart_runtime() {
    labproxyrestart
}

_apply_runtime_change() {
    _build_runtime_config
    _restart_runtime
}

_update_mixin_config() {
    local expression=$1
    local error_message=$2

    mkdir -p "$(dirname "$LABPROXY_CONFIG_MIXIN")"
    "$BIN_YQ" -i "$expression" "$LABPROXY_CONFIG_MIXIN" 2>/dev/null || {
        _failcat "$error_message"
        return 1
    }
}

_save_subscription_url() {
    mkdir -p "$(dirname "$LABPROXY_CONFIG_URL")"
    echo "$1" > "$LABPROXY_CONFIG_URL"
}

_append_update_log() {
    mkdir -p "$(dirname "$LABPROXY_UPDATE_LOG")"
    echo "[$(date +"%Y-%m-%d %H:%M:%S")] $1：$2" >> "${LABPROXY_UPDATE_LOG}"
}

_resolve_update_url() {
    local url=$1

    # 如果没有提供有效的订阅链接（url为空或者不是http开头），则使用默认配置文件
    if [ "${url:0:4}" != "http" ]; then
        _failcat "没有提供有效的订阅链接：使用 ${LABPROXY_CONFIG_RAW} 进行更新..."
        url="file://$LABPROXY_CONFIG_RAW"
    fi

    printf '%s\n' "$url"
}

_enable_auto_subscription_update() {
    local url=$1

    # Persist URL for cron runs (cron executes `labproxyctl update`, which reads LABPROXY_CONFIG_URL).
    [ "${url:0:4}" = "http" ] && _save_subscription_url "$url"

    # Check if crontab entry already exists
    crontab -l 2>/dev/null | grep -qs 'labproxyctl_auto_update' || {
        # Add user-level crontab entry (every 2 days at midnight)
        (crontab -l 2>/dev/null; echo "0 0 */2 * * $_SHELL -i -c 'labproxyctl update' # labproxyctl_auto_update") | crontab -
    }
    _okcat "已设置用户级定时更新订阅（每 2 天执行一次）"
}

_download_and_apply_subscription() {
    local url=$1

    _okcat '⏳' "正在下载，原配置已备份..."

    # Ensure directories exist and backup using user permissions
    mkdir -p "$(dirname "$LABPROXY_CONFIG_RAW_BAK")" "$(dirname "$LABPROXY_UPDATE_LOG")"
    cp "$LABPROXY_CONFIG_RAW" "$LABPROXY_CONFIG_RAW_BAK" 2>/dev/null

    _rollback() {
        _failcat '❌' "$1"
        # Restore backup using user permissions
        cp "$LABPROXY_CONFIG_RAW_BAK" "$LABPROXY_CONFIG_RAW" 2>/dev/null
        _append_update_log "订阅更新失败" "$url"
        return 1
    }

    _download_config "$LABPROXY_CONFIG_RAW" "$url" || {         _rollback "下载失败，已回滚配置" || true; return 1; }
    _valid_config "$LABPROXY_CONFIG_RAW" || { _rollback "转换失败，已回滚配置，转换日志：${BIN_SUBCONVERTER_LOG}" || true; return 1; }

    _merge_config_restart || return 1
    _okcat '✅' '订阅更新成功'

    # Save URL and log success using user permissions
    _save_subscription_url "$url"
    _append_update_log "订阅更新成功" "$url"
}

function labproxysecret() {
    case "$#" in
    0)
        if [ -f "$LABPROXY_CONFIG_RUNTIME" ]; then
            _okcat '🔑' "当前密钥：$("$BIN_YQ" '.secret // ""' "$LABPROXY_CONFIG_RUNTIME" 2>/dev/null)"
        else
            _failcat "运行时配置文件不存在"
        fi
        ;;
    1)
        _update_mixin_config ".secret = \"$1\"" "密钥更新失败，请重新输入" || return 1
        _apply_runtime_change
        _okcat '🔑' "密钥更新成功，已重启生效"
        ;;
    *)
        _failcat "密钥不要包含空格或使用引号包围"
        ;;
    esac
}

_tunstatus() {
    if [ -f "$LABPROXY_CONFIG_RUNTIME" ]; then
        local tun_status=$("$BIN_YQ" '.tun.enable' "${LABPROXY_CONFIG_RUNTIME}" 2>/dev/null)
        # shellcheck disable=SC2015
        [ "$tun_status" = 'true' ] && _okcat 'Tun 状态：启用' || _failcat 'Tun 状态：关闭'
    else
        _failcat 'Tun 状态：配置文件不存在'
        return 1
    fi
}

_tunoff() {
    _tunstatus >/dev/null || return 0
    _update_mixin_config '.tun.enable = false' "无法更新 Tun 配置" || return 1
    _apply_runtime_change && _okcat "Tun 模式已关闭"
}

_tunon() {
    _tunstatus 2>/dev/null && return 0
    _update_mixin_config '.tun.enable = true' "无法更新 Tun 配置" || return 1
    _apply_runtime_change
    sleep 0.5s
    
    # Check if mihomo is running and tun mode is working
    if is_labproxy_running; then
        local log_file="$LABPROXY_HOME_DIR/logs/labproxy.log"
        # Check recent log entries for tun mode status
        if [ -f "$log_file" ]; then
            # Look for tun-related messages in the last few lines
            tail -20 "$log_file" 2>/dev/null | grep -i "tun" >/dev/null 2>&1 && {
                _okcat "Tun 模式已开启"
            } || {
                _okcat "Tun 模式已开启（请检查日志确认状态：${log_file}）"
            }
        else
            _okcat "Tun 模式已开启"
        fi
    else
        _failcat "Tun 模式配置已更新，但 LabProxy 进程未运行"
    fi
}

function labproxytun() {
    case "$1" in
    on)
        _tunon
        ;;
    off)
        _tunoff
        ;;
    *)
        _tunstatus
        ;;
    esac
}

_lanstatus() {
    if [ -f "$LABPROXY_CONFIG_RUNTIME" ]; then
        local lan_status=$("$BIN_YQ" '.allow-lan // false' "${LABPROXY_CONFIG_RUNTIME}" 2>/dev/null)
        if [ "$lan_status" = 'true' ]; then
            _okcat '局域网访问：已开启'
        else
            _failcat '局域网访问：已关闭'
        fi
    else
        _failcat '局域网访问：配置文件不存在'
        return 1
    fi
}

_lanoff() {
    _lanstatus >/dev/null 2>&1 && {
        local current_status=$("$BIN_YQ" '.allow-lan // false' "${LABPROXY_CONFIG_RUNTIME}" 2>/dev/null)
        [ "$current_status" = 'false' ] && return 0
    }

    _update_mixin_config '.allow-lan = false' "无法更新局域网访问配置" || return 1
    _apply_runtime_change && _okcat "局域网访问已关闭"
}

_lanon() {
    local current_status=$("$BIN_YQ" '.allow-lan // false' "${LABPROXY_CONFIG_RUNTIME}" 2>/dev/null)
    [ "$current_status" = 'true' ] && return 0

    _update_mixin_config '.allow-lan = true' "无法更新局域网访问配置" || return 1
    _apply_runtime_change && _okcat "局域网访问已开启"
}

function labproxylan() {
    case "$1" in
    on)
        _lanon
        ;;
    off)
        _lanoff
        ;;
    status)
        _lanstatus
        ;;
    *)
        _lanstatus
        ;;
    esac
}

function labproxylang() {
    case "$1" in
    zh|en)
        echo "$1" > "$LABPROXY_LANG_FILE"
        _okcat "语言已设置为: $1"
        ;;
    "")
        if [ -f "$LABPROXY_LANG_FILE" ]; then
            local current
            current=$(head -n 1 "$LABPROXY_LANG_FILE" | tr -d '[:space:]')
            _okcat "当前语言: ${current:-en}"
        else
            _okcat "当前语言: en (默认)"
        fi
        ;;
    *)
        _failcat "用法: labproxy lang [zh|en]"
        ;;
    esac
}

function labproxysubscribe() {
    case "$#" in
    0)
        # Show current subscription URL
        local url=$(cat "$LABPROXY_CONFIG_URL" 2>/dev/null)
        if [ -n "$url" ]; then
            _okcat "当前订阅地址：${url}"
        else
            _failcat "未设置订阅地址"
            return 1
        fi
        ;;
    1)
        # Set new subscription URL
        local new_url="$1"
        if [ "${new_url:0:4}" != "http" ]; then
            _failcat "无效的订阅地址，必须以 http 或 https 开头"
            return 1
        fi

        # Save URL
        _save_subscription_url "$new_url"
        _okcat "订阅地址已设置：${new_url}"

        # Ask if user wants to update immediately
        printf "是否立即更新订阅配置? [y/N]: "
        read -r response
        case "$response" in
        [yY]|[yY][eE][sS])
            labproxysubupdate "" "$new_url"
            ;;
        *)
            _okcat "订阅地址已保存，使用 'labproxy update' 命令更新配置"
            ;;
        esac
        ;;
    *)
        cat <<EOF
用法: labproxy subscribe [URL]
    无参数      显示当前订阅地址
    URL         设置新的订阅地址
EOF
        ;;
    esac
}

# ---- Multi-subscription management commands ----

# Add a new subscription: labproxy add <name> <url>
function labproxyadd() {
    local name="$1"
    local url="$2"

    [ -z "$name" ] && { _failcat "用法: labproxy add <名称> <订阅URL>"; return 1; }
    [ -z "$url" ] && { _failcat "用法: labproxy add <名称> <订阅URL>"; return 1; }
    [ "${url:0:4}" != "http" ] && { _failcat "无效的订阅地址，必须以 http 或 https 开头"; return 1; }

    _ensure_subs_file

    # Check if name already exists
    local existing_url
    existing_url=$("$BIN_YQ" ".subscriptions[\"$name\"].url // \"\"" "$LABPROXY_SUBS_FILE" 2>/dev/null)
    if [ -n "$existing_url" ]; then
        _failcat "订阅 '${name}' 已存在 (URL: ${existing_url})"
        printf "是否覆盖? [y/N]: "
        read -r response
        case "$response" in
            [yY]|[yY][eE][sS]) ;;
            *) return 1 ;;
        esac
    fi

    # Add to subscriptions YAML
    "$BIN_YQ" -i ".subscriptions[\"$name\"].url = \"$url\"" "$LABPROXY_SUBS_FILE" 2>/dev/null
    "$BIN_YQ" -i ".subscriptions[\"$name\"].added_at = \"$(date '+%Y-%m-%d %H:%M:%S')\"" "$LABPROXY_SUBS_FILE" 2>/dev/null

    _okcat "已添加订阅：${name}"

    # If this is the first subscription, set it as active
    local active
    active="$(_active_subscription_name)"
    if [ -z "$active" ]; then
        _set_active_subscription "$name"
        _okcat "已设为当前订阅：${name}"
    fi

    # Ask to download now
    printf "是否立即下载? [y/N]: "
    read -r response
    case "$response" in
        [yY]|[yY][eE][sS])
            labproxysubupdate "$name" "$url"
            ;;
        *)
            _okcat "使用 'labproxy update' 更新订阅配置"
            ;;
    esac
}

# Switch active subscription: labproxy use <name>
function labproxyuse() {
    local name="$1"
    [ -z "$name" ] && {
        _okcat "当前订阅：$(_active_subscription_name)"
        return 0
    }

    _ensure_subs_file

    local exists
    exists=$("$BIN_YQ" ".subscriptions[\"$name\"] // \"\"" "$LABPROXY_SUBS_FILE" 2>/dev/null)
    [ "$exists" = "null" ] || [ -z "$exists" ] && {
        _failcat "订阅 '${name}' 不存在，使用 'labproxy ls' 查看可用订阅"
        return 1
    }

    _set_active_subscription "$name"

    # Apply the subscription config
    if ! _apply_active_subscription; then
        _failcat "订阅 '${name}' 配置文件不存在，请先更新: labproxy update"
        return 1
    fi

    # Update legacy URL file for backward compatibility
    local url
    url="$(_active_subscription_url)"
    [ -n "$url" ] && _save_subscription_url "$url"

    _merge_config_restart
    _okcat "已切换到订阅：${name}"
}

# List all subscriptions: labproxy ls
function labproxyls() {
    _ensure_subs_file

    local active
    active="$(_active_subscription_name)"

    local count
    count=$("$BIN_YQ" '.subscriptions | length' "$LABPROXY_SUBS_FILE" 2>/dev/null)
    if [ "$count" = "0" ] || [ -z "$count" ]; then
        _failcat "暂无保存的订阅，使用 'labproxy add <名称> <URL>' 添加"
        return 1
    fi

    _okcat "已保存的订阅 (${count} 个)："
    echo ""

    "$BIN_YQ" '.subscriptions | to_entries | .[] | "\(.key)|\(.value.url)|\(.value.added_at // "")"' "$LABPROXY_SUBS_FILE" 2>/dev/null | while IFS='|' read -r name url added; do
        local marker="  "
        if [ "$name" = "$active" ]; then
            marker="🔵"
        fi
        local config_file="$(_sub_config_file "$name")"
        local node_count=""
        if [ -f "$config_file" ]; then
            node_count=$("$BIN_YQ" '(.. | select(tag == "!!map" and .name and .type)) | .name' "$config_file" 2>/dev/null | wc -l)
            node_count="(${node_count// /} 节点)"
        fi
        printf "%s %-20s %s %s\n" "$marker" "$name" "$node_count" "$url"
    done
}

# Update a specific subscription: labproxy update [name]
function labproxysubupdate() {
    local name="$1"
    local url="$2"

    # Handle backward-compatible subcommands
    case "$name" in
    auto)
        # Auto-update mode: set up cron for active subscription
        local active_name
        active_name="$(_active_subscription_name)"
        [ -z "$active_name" ] && { _failcat "无活跃订阅"; return 1; }
        local active_url
        active_url="$(_active_subscription_url)"
        [ -n "$url" ] && active_url="$url"
        _enable_auto_subscription_update "$active_url"
        return 0
        ;;
    log)
        tail "${LABPROXY_UPDATE_LOG}" 2>/dev/null || _failcat "暂无更新日志"
        return 0
        ;;
    esac

    # If no name given, use active subscription
    if [ -z "$name" ]; then
        name="$(_active_subscription_name)"
        [ -z "$name" ] && { _failcat "无活跃订阅，使用 'labproxy add <名称> <URL>' 添加"; return 1; }
        url="$(_active_subscription_url)"
    fi

    [ -z "$url" ] && { _failcat "订阅 '${name}' 的 URL 为空"; return 1; }

    _okcat "正在更新订阅：${name}"

    url=$(_resolve_update_url "$url")

    _okcat '⏳' "正在下载，原配置已备份..."
    mkdir -p "$(dirname "$LABPROXY_CONFIG_RAW_BAK")" "$(dirname "$LABPROXY_UPDATE_LOG")"
    cp "$LABPROXY_CONFIG_RAW" "$LABPROXY_CONFIG_RAW_BAK" 2>/dev/null

    _rollback() {
        _failcat '❌' "$1"
        cp "$LABPROXY_CONFIG_RAW_BAK" "$LABPROXY_CONFIG_RAW" 2>/dev/null
        _append_update_log "订阅更新失败" "$name"
        return 1
    }

    _download_config "$LABPROXY_CONFIG_RAW" "$url" || { _rollback "下载失败，已回滚配置" || true; return 1; }
    _valid_config "$LABPROXY_CONFIG_RAW" || { _rollback "转换失败，已回滚配置，转换日志：${BIN_SUBCONVERTER_LOG}" || true; return 1; }

    # Save subscription config for multi-subscription
    _save_sub_config "$name" "$LABPROXY_CONFIG_RAW"

    _merge_config_restart || return 1
    _okcat '✅' '订阅更新成功'

    _save_subscription_url "$url"
    _append_update_log "订阅更新成功" "$name"
}

# Remove a subscription: labproxy sub remove <name>
function labproxysubremove() {
    local name="$1"
    [ -z "$name" ] && { _failcat "用法: labproxy sub remove <名称>"; return 1; }

    _ensure_subs_file

    local exists
    exists=$("$BIN_YQ" ".subscriptions[\"$name\"] // \"\"" "$LABPROXY_SUBS_FILE" 2>/dev/null)
    [ "$exists" = "null" ] || [ -z "$exists" ] && {
        _failcat "订阅 '${name}' 不存在"
        return 1
    }

    printf "确认删除订阅 '%s'? [y/N]: " "$name"
    read -r response
    case "$response" in
        [yY]|[yY][eE][sS]) ;;
        *) return 1 ;;
    esac

    "$BIN_YQ" -i "del(.subscriptions[\"$name\"])" "$LABPROXY_SUBS_FILE" 2>/dev/null
    rm -f "$(_sub_config_file "$name")"

    # If removed the active subscription, clear active
    local active
    active="$(_active_subscription_name)"
    if [ "$active" = "$name" ]; then
        "$BIN_YQ" -i '.active = ""' "$LABPROXY_SUBS_FILE" 2>/dev/null
        _failcat "已删除当前订阅，请使用 'labproxy use <名称>' 切换"
    fi

    _okcat "已删除订阅：${name}"
}

# Rename a subscription: labproxy sub rename <old> <new>
function labproxysubrename() {
    local old_name="$1"
    local new_name="$2"

    [ -z "$old_name" ] && { _failcat "用法: labproxy sub rename <旧名称> <新名称>"; return 1; }
    [ -z "$new_name" ] && { _failcat "用法: labproxy sub rename <旧名称> <新名称>"; return 1; }

    _ensure_subs_file

    local exists
    exists=$("$BIN_YQ" ".subscriptions[\"$old_name\"] // \"\"" "$LABPROXY_SUBS_FILE" 2>/dev/null)
    [ "$exists" = "null" ] || [ -z "$exists" ] && {
        _failcat "订阅 '${old_name}' 不存在"
        return 1
    }

    local new_exists
    new_exists=$("$BIN_YQ" ".subscriptions[\"$new_name\"] // \"\"" "$LABPROXY_SUBS_FILE" 2>/dev/null)
    [ "$new_exists" != "null" ] && [ -n "$new_exists" ] && {
        _failcat "订阅 '${new_name}' 已存在"
        return 1
    }

    # Copy the subscription entry
    local url
    url=$("$BIN_YQ" ".subscriptions[\"$old_name\"].url" "$LABPROXY_SUBS_FILE" 2>/dev/null)
    "$BIN_YQ" -i ".subscriptions[\"$new_name\"] = .subscriptions[\"$old_name\"]" "$LABPROXY_SUBS_FILE" 2>/dev/null
    "$BIN_YQ" -i "del(.subscriptions[\"$old_name\"])" "$LABPROXY_SUBS_FILE" 2>/dev/null

    # Rename config file
    [ -f "$(_sub_config_file "$old_name")" ] && mv "$(_sub_config_file "$old_name")" "$(_sub_config_file "$new_name")"

    # Update active if needed
    local active
    active="$(_active_subscription_name)"
    [ "$active" = "$old_name" ] && _set_active_subscription "$new_name"

    _okcat "已重命名：${old_name} → ${new_name}"
}

# Sub command dispatcher: labproxy sub <list|enable|disable|rename|remove>
function labproxysub() {
    case "$1" in
        list|ls)
            labproxyls
            ;;
        remove|rm|delete)
            shift
            labproxysubremove "$@"
            ;;
        rename|mv)
            shift
            labproxysubrename "$@"
            ;;
        *)
            cat <<EOF
用法: labproxy sub <command> [args]

多订阅管理命令:
    list                列出所有订阅
    remove <名称>       删除订阅
    rename <旧名> <新名> 重命名订阅

快捷命令:
    labproxy add <名称> <URL>   添加订阅
    labproxy use [名称]         切换/查看当前订阅
    labproxy ls                 列出所有订阅
    labproxy update [名称]      更新指定/当前订阅
EOF
            ;;
    esac
}

function labproxymixin() {
    case "$1" in
    -e)
        vim "$LABPROXY_CONFIG_MIXIN" && {
            _apply_runtime_change && _okcat "配置更新成功，已重启生效"
        }
        ;;
    -r)
        less -f "$LABPROXY_CONFIG_RUNTIME"
        ;;
    *)
        less -f "$LABPROXY_CONFIG_MIXIN"
        ;;
    esac
}

# ---- Doctor diagnostic command ----

# Check if running in container
_is_container() {
    [ -f "/.dockerenv" ] && return 0
    grep -qaE '(docker|containerd|kubepods|lxc)' /proc/1/cgroup 2>/dev/null && return 0
    [ -f /proc/1/cgroup ] && grep -q '^0::/' /proc/1/cgroup 2>/dev/null && {
        local _init_comm
        _init_comm="$(cat /proc/1/comm 2>/dev/null || true)"
        case "${_init_comm:-}" in systemd|sysvinit|init|launchd|openrc-init) ;; *) return 0 ;; esac
    }
    return 1
}

# Check if running in WSL
_is_wsl() {
    grep -qiE 'microsoft|wsl' /proc/version 2>/dev/null
}

# Check if running in WSL and on Windows mount
_is_wsl_windows_mount() {
    _is_wsl || return 1
    local resolved
    resolved="$(readlink -f "$LABPROXY_HOME_DIR" 2>/dev/null || echo "$LABPROXY_HOME_DIR")"
    case "$resolved" in /mnt/[a-zA-Z]|/mnt/[a-zA-Z]/*) return 0 ;; *) return 1 ;; esac
}

# Doctor diagnostic command
function labproxydoctor() {
    echo ""
    echo "🩺 LabProxy 诊断报告"
    echo "══════════════════════════════════════"
    echo ""

    local issues=0
    local ok_count=0

    _check() {
        local desc="$1"
        local result="$2"
        local fix="${3:-}"
        if [ "$result" = "ok" ]; then
            _okcat "✅" "$desc"
            ok_count=$((ok_count + 1))
        else
            _failcat "❌" "$desc"
            [ -n "$fix" ] && echo "   👉 $fix"
            issues=$((issues + 1))
        fi
    }

    # 1. Environment checks
    echo "── 环境检测 ──"
    local env_type="主机"
    _is_container && env_type="容器"
    _is_wsl && env_type="WSL"
    _check "运行环境: ${env_type}" "ok"

    if _is_wsl_windows_mount; then
        _check "WSL 挂载路径 (应在 Linux 原生目录)" "fail" "请将项目移出 /mnt/c/ 到 Linux 原生目录"
    else
        _check "WSL 挂载路径" "ok"
    fi

    # Shell check
    if [ -n "$BASH_VERSION" ] || [ -n "$ZSH_VERSION" ]; then
        _check "Shell: ${_SHELL}" "ok"
    else
        _check "Shell: ${_SHELL:-unknown}" "fail" "仅支持 bash/zsh"
    fi

    # 2. Binary checks
    echo ""
    echo "── 二进制文件检测 ──"
    if [ -x "$BIN_MIHOMO" ]; then
        _check "mihomo 内核: $(basename "$BIN_MIHOMO")" "ok"
    elif [ -x "$BIN_CLASH" ]; then
        _check "clash 内核: $(basename "$BIN_CLASH")" "ok"
    else
        _check "代理内核" "fail" "重新执行 install.sh 安装"
    fi

    if [ -x "$BIN_YQ" ]; then
        _check "yq: $(basename "$BIN_YQ")" "ok"
    else
        _check "yq" "fail" "重新执行 install.sh 安装"
    fi

    if [ -x "$BIN_SUBCONVERTER" ]; then
        _check "subconverter: $(basename "$BIN_SUBCONVERTER")" "ok"
    else
        _check "subconverter" "fail" "重新执行 install.sh 安装"
    fi

    # 3. Config checks
    echo ""
    echo "── 配置文件检测 ──"
    if [ -f "$LABPROXY_CONFIG_RUNTIME" ] && [ -s "$LABPROXY_CONFIG_RUNTIME" ]; then
        _check "运行时配置: runtime.yaml" "ok"
        local node_count
        node_count=$("$BIN_YQ" '(.. | select(tag == "!!map" and .name and .type)) | .name' "$LABPROXY_CONFIG_RUNTIME" 2>/dev/null | wc -l)
        echo "   📊 节点数量: ${node_count// /}"
    else
        _check "运行时配置: runtime.yaml" "fail" "执行 labproxy update 下载订阅"
    fi

    if [ -f "$LABPROXY_CONFIG_MIXIN" ]; then
        _check "Mixin 配置: mixin.yaml" "ok"
    else
        _check "Mixin 配置: mixin.yaml" "fail" "重新执行 install.sh"
    fi

    # 4. Subscription checks
    echo ""
    echo "── 订阅检测 ──"
    _ensure_subs_file
    local sub_count
    sub_count=$("$BIN_YQ" '.subscriptions | length' "$LABPROXY_SUBS_FILE" 2>/dev/null)
    if [ "$sub_count" -gt 0 ] 2>/dev/null; then
        _check "订阅数量: ${sub_count}" "ok"
        local active
        active="$(_active_subscription_name)"
        if [ -n "$active" ]; then
            _check "当前订阅: ${active}" "ok"
        else
            _check "当前订阅: 未设置" "fail" "执行 labproxy use <名称> 选择订阅"
        fi
    else
        _check "订阅数量: 0" "fail" "执行 labproxy add <名称> <URL> 添加订阅"
    fi

    # 5. Process checks
    echo ""
    echo "── 进程检测 ──"
    if is_labproxy_running; then
        local pid
        pid=$(cat "$LABPROXY_HOME_DIR/config/labproxy.pid" 2>/dev/null)
        local uptime
        uptime=$(ps -o etime= -p "$pid" 2>/dev/null | tr -d ' ')
        _check "LabProxy 进程: 运行中 (PID: ${pid:-unknown}, 运行: ${uptime:-未知})" "ok"
    else
        _check "LabProxy 进程: 未运行" "fail" "执行 labproxy on 启动"
    fi

    # 6. Port checks
    echo ""
    echo "── 端口检测 ──"
    _get_proxy_port
    _get_ui_port
    _get_dns_port

    if _is_bind "$MIXED_PORT" >/dev/null 2>&1; then
        _check "代理端口: ${MIXED_PORT}" "ok"
    else
        _check "代理端口: ${MIXED_PORT} (未监听)" "fail" "执行 labproxy restart"
    fi

    if _is_bind "$UI_PORT" >/dev/null 2>&1; then
        _check "管理端口: ${UI_PORT}" "ok"
    else
        _check "管理端口: ${UI_PORT} (未监听)" "fail" "执行 labproxy restart"
    fi

    if _is_bind "$DNS_PORT" >/dev/null 2>&1; then
        _check "DNS 端口: ${DNS_PORT}" "ok"
    else
        _check "DNS 端口: ${DNS_PORT} (未监听)" "info"
    fi

    # 7. Cache checks
    echo ""
    echo "── 缓存检测 ──"
    local cache_dir="$(_download_cache_dir)"
    if [ -d "$cache_dir" ]; then
        local cache_size
        cache_size=$(du -sh "$cache_dir" 2>/dev/null | cut -f1)
        _check "下载缓存: ${cache_size:-未知}" "ok"
    else
        _check "下载缓存: 无" "info"
    fi

    # 8. Log checks
    local log_file="$LABPROXY_HOME_DIR/logs/labproxy.log"
    if [ -f "$log_file" ]; then
        local log_size
        log_size=$(du -sh "$log_file" 2>/dev/null | cut -f1)
        local recent_errors
        recent_errors=$(grep -ciE 'error|fatal|panic' "$log_file" 2>/dev/null || echo 0)
        if [ "$recent_errors" -gt 0 ] 2>/dev/null; then
            _check "日志: ${log_size} (${recent_errors} 条错误)" "fail" "执行 tail -100 ${log_file} 查看详情"
        else
            _check "日志: ${log_size}" "ok"
        fi
    fi

    # Summary
    echo ""
    echo "══════════════════════════════════════"
    local total=$((ok_count + issues))
    if [ "$issues" -eq 0 ]; then
        _okcat "🎉" "诊断完成：${ok_count}/${total} 项通过，一切正常！"
    else
        _failcat "🩺" "诊断完成：${ok_count}/${total} 项通过，${issues} 项需要处理"
        echo ""
        echo "💡 建议操作："
        if ! is_labproxy_running; then
            echo "   1. 启动代理: labproxy on"
        fi
        if [ "$sub_count" = "0" ] 2>/dev/null || [ -z "$sub_count" ]; then
            echo "   2. 添加订阅: labproxy add <名称> <URL>"
        elif [ -z "$(_active_subscription_name)" ]; then
            echo "   2. 选择订阅: labproxy use <名称>"
        fi
        if [ ! -f "$LABPROXY_CONFIG_RUNTIME" ] || [ ! -s "$LABPROXY_CONFIG_RUNTIME" ]; then
            echo "   3. 更新订阅: labproxy update"
        fi
        echo ""
        echo "   如问题仍然存在，请查看日志: tail -100 ${log_file}"
    fi
}

function labproxyctl() {
    case "$1" in
    on)
        labproxyon
        ;;
    off)
        labproxyoff
        ;;
    restart)
        labproxyrestart
        ;;
    ui)
        labproxyui
        ;;
    status)
        shift
        labproxystatus "$@"
        ;;
    proxy)
        shift
        labproxyproxy "$@"
        ;;
    port)
        shift
        labproxyport "$@"
        ;;
    tun)
        shift
        labproxytun "$@"
        ;;
    lan)
        shift
        labproxylan "$@"
        ;;
    mixin)
        shift
        labproxymixin "$@"
        ;;
    secret)
        shift
        labproxysecret "$@"
        ;;
    subscribe)
        shift
        labproxysubscribe "$@"
        ;;
    add)
        shift
        labproxyadd "$@"
        ;;
    use)
        shift
        labproxyuse "$@"
        ;;
    ls)
        shift
        labproxyls "$@"
        ;;
    sub)
        shift
        labproxysub "$@"
        ;;
    update)
        shift
        labproxysubupdate "$@"
        ;;
    tui)
        labproxytui
        ;;
    rules)
        shift
        exec "$LABPROXY_TUI_BIN" rules --mixin-config "$LABPROXY_CONFIG_MIXIN" "$@"
        ;;
    lang)
        shift
        labproxylang "$@"
        ;;
    doctor)
        shift
        labproxydoctor "$@"
        ;;
    *)
        cat <<EOF

Usage:
    labproxy COMMAND  [OPTION]
    labproxyctl COMMAND [OPTION]

Backward compatibility:
    clash COMMAND [OPTION]
    mihomo COMMAND [OPTION]
    clashctl COMMAND [OPTION]
    mihomoctl COMMAND [OPTION]

Commands:
    on                      开启代理
    off                     关闭代理
    restart                 重启代理服务
    status                  进程运行状态
    tui                     交互式终端界面（TUI）
    ui                      Web 控制台地址
    proxy    [on|off|status]       系统代理环境变量
    port     [status|auto|set]     代理端口模式设置
    tun      [on|off|status]       Tun 模式 (需要权限)
    lan      [on|off|status]       局域网访问控制
    mixin    [-e|-r]        Mixin 配置文件
    secret   [SECRET]       Web 控制台密钥
    subscribe [URL]         设置或查看订阅地址
    add      <名称> <URL>   添加订阅
    use      [名称]         切换或查看当前订阅
    ls                      列出所有订阅
    sub      <list|remove|rename>  订阅管理
    update   [名称]         更新指定或当前订阅
    lang     [zh|en]        切换 TUI 界面语言
    doctor                  诊断环境与运行状态

说明:
    • 用户空间运行，无需 sudo 权限
    • 配置目录: ~/.labproxy/
    • 日志目录: ~/.labproxy/logs/
    • 进程管理: 基于 PID 文件和 nohup

EOF
        ;;
    esac
}

# Backward compatibility aliases
function clashctl() {
    labproxyctl "$@"
}

function clash() {
    labproxyctl "$@"
}

function mihomoctl() {
    labproxyctl "$@"
}

function mihomo() {
    labproxyctl "$@"
}

function labproxy() {
    labproxyctl "$@"
}
