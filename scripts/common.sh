# shellcheck disable=SC2148
# shellcheck disable=SC2034
# shellcheck disable=SC2155
[ -n "$BASH_VERSION" ] && set +o noglob
[ -n "$ZSH_VERSION" ] && setopt glob no_nomatch

URL_GH_PROXY='https://ghfast.top'
URL_LABPROXY_UI="http://board.zash.run.place"

SCRIPT_BASE_DIR='./scripts'

RESOURCES_BASE_DIR='./resources'
RESOURCES_BIN_DIR="${RESOURCES_BASE_DIR}/bin"
RESOURCES_CONFIG="${RESOURCES_BASE_DIR}/config.yaml"
RESOURCES_CONFIG_MIXIN="${RESOURCES_BASE_DIR}/mixin.yaml"

ZIP_BASE_DIR="${RESOURCES_BASE_DIR}/zip"
ZIP_CLASH=$(echo "${ZIP_BASE_DIR}"/clash*)
ZIP_MIHOMO=$(echo "${ZIP_BASE_DIR}"/mihomo*)
ZIP_YQ=$(echo "${ZIP_BASE_DIR}"/yq*)
ZIP_SUBCONVERTER=$(echo "${ZIP_BASE_DIR}"/subconverter*)
ZIP_LABPROXY_TUI=""

ZIP_UI="${ZIP_BASE_DIR}/zashboard.zip"

LABPROXY_HOME_DIR="$HOME/.labproxy"
LABPROXY_SCRIPT_DIR="${LABPROXY_HOME_DIR}/$(basename $SCRIPT_BASE_DIR)"
LABPROXY_CONFIG_URL="${LABPROXY_HOME_DIR}/url"
LABPROXY_CONFIG_RAW="${LABPROXY_HOME_DIR}/$(basename $RESOURCES_CONFIG)"
LABPROXY_CONFIG_RAW_BAK="${LABPROXY_CONFIG_RAW}.bak"
LABPROXY_CONFIG_MIXIN="${LABPROXY_HOME_DIR}/$(basename $RESOURCES_CONFIG_MIXIN)"
LABPROXY_CONFIG_RUNTIME="${LABPROXY_HOME_DIR}/runtime.yaml"
LABPROXY_UPDATE_LOG="${LABPROXY_HOME_DIR}/labproxyctl.log"
LABPROXY_TUI_SRC_DIR="${LABPROXY_HOME_DIR}/tui-src"
LABPROXY_TUI_BIN="${LABPROXY_HOME_DIR}/bin/labproxy-tui"
LABPROXY_LANG_FILE="${LABPROXY_HOME_DIR}/.lang"

_is_dir_writable() {
    local dir=$1
    [ -n "$dir" ] && [ -d "$dir" ] && [ -w "$dir" ] && [ -x "$dir" ]
}

_labproxy_tmpdir_candidates() {
    local uid user_tag

    uid=$(id -u 2>/dev/null || true)
    user_tag=${USER:-${uid:-unknown}}

    case "$uid" in
    '' | *[!0-9]*)
        ;;
    *)
        printf '%s\n' "/run/user/$uid/labproxy-tmp"
        ;;
    esac

    printf '%s\n' "/dev/shm/labproxy-tmp-${user_tag}"
    [ -n "$HOME" ] && printf '%s\n' "$HOME/.cache/labproxy/tmp"
    [ -n "$LABPROXY_HOME_DIR" ] && printf '%s\n' "$LABPROXY_HOME_DIR/tmp"
}

_is_labproxy_tmpdir_path() {
    local dir=$1
    local candidate

    [ -n "$dir" ] || return 1

    while IFS= read -r candidate; do
        [ -n "$candidate" ] || continue
        [ "$dir" = "$candidate" ] && return 0
    done <<EOF
$(_labproxy_tmpdir_candidates)
EOF

    return 1
}

_cleanup_labproxy_tmpdirs() {
    local dir

    while IFS= read -r dir; do
        [ -n "$dir" ] || continue
        [ -e "$dir" ] || continue
        _is_labproxy_tmpdir_path "$dir" || continue
        rm -rf -- "$dir" 2>/dev/null || true
    done <<EOF
$(_labproxy_tmpdir_candidates)
EOF

    [ -n "$HOME" ] && rmdir "$HOME/.cache/labproxy" 2>/dev/null || true
}

_set_tmpdir_default() {
    # Respect user override if it is usable.
    if _is_dir_writable "$TMPDIR"; then
        export TMPDIR
        export TMP="$TMPDIR"
        export TEMP="$TMPDIR"
        return 0
    fi

    local candidate
    while IFS= read -r candidate; do
        [ -n "$candidate" ] || continue
        mkdir -p "$candidate" 2>/dev/null || true
        if _is_dir_writable "$candidate"; then
            export TMPDIR="$candidate"
            export TMP="$TMPDIR"
            export TEMP="$TMPDIR"
            return 0
        fi
    done <<EOF
$(_labproxy_tmpdir_candidates)
EOF

    return 1
}

_set_var() {
    local user=$USER
    local home=$HOME

    [ -n "$BASH_VERSION" ] && {
        _SHELL=bash
    }
    [ -n "$ZSH_VERSION" ] && {
        _SHELL=zsh
    }
    [ -n "$fish_version" ] && {
        _SHELL=fish
    }

    # rc文件路径
    if command -v bash >&/dev/null; then
        SHELL_RC_BASH="${home}/.bashrc"
    else
        SHELL_RC_BASH=""
    fi
    if command -v zsh >&/dev/null; then
        SHELL_RC_ZSH="${home}/.zshrc"
    else
        SHELL_RC_ZSH=""
    fi
    if command -v fish >&/dev/null; then
        SHELL_RC_FISH="${home}/.config/fish/config.fish"
    else
        SHELL_RC_FISH=""
    fi


    LABPROXY_CRON_TAB="user"  # 标记使用用户级crontab

    # Avoid using /tmp when / is full (bash heredoc, yq -i, mktemp, etc.).
    _set_tmpdir_default || true
}
_set_var

# shellcheck disable=SC2120
_set_bin() {
    local bin_base_dir="${LABPROXY_HOME_DIR}/bin"
    [ -n "$1" ] && bin_base_dir=$1
    BIN_CLASH="${bin_base_dir}/clash"
    BIN_MIHOMO="${bin_base_dir}/mihomo"
    BIN_YQ="${bin_base_dir}/yq"
    BIN_SUBCONVERTER_DIR="${bin_base_dir}/subconverter"
    BIN_SUBCONVERTER_CONFIG="$BIN_SUBCONVERTER_DIR/pref.yml"
    BIN_SUBCONVERTER_PORT="25500"
    BIN_SUBCONVERTER="${BIN_SUBCONVERTER_DIR}/subconverter"
    BIN_SUBCONVERTER_LOG="${BIN_SUBCONVERTER_DIR}/latest.log"

    [ -f "$BIN_CLASH" ] && {
        BIN_KERNEL=$BIN_CLASH
    }
    [ -f "$BIN_MIHOMO" ] && {
        BIN_KERNEL=$BIN_MIHOMO
    }
    BIN_KERNEL_NAME=$(basename "$BIN_KERNEL")
}
_set_bin

_rc_managed_line() {
    printf 'source %s/common.sh && source %s/proxyctl.sh && watch_proxy' "$LABPROXY_SCRIPT_DIR" "$LABPROXY_SCRIPT_DIR"
}

_rc_managed_line_fish() {
    printf 'set -gx LABPROXY_HOME %s/.labproxy' "$HOME"
}

_rc_managed_line_fish_path() {
    printf 'set -gx PATH %s/.labproxy/bin $PATH' "$HOME"
}

_rc_block_begin() {
    printf '%s\n' '# >>> labproxy >>>'
}

_rc_block_end() {
    printf '%s\n' '# <<< labproxy <<<'
}

_write_rc_block() {
    local rc_file=$1
    local tmp_file="${rc_file}.tmp.$$"
    local begin_marker end_marker managed_line

    begin_marker=$(_rc_block_begin)
    end_marker=$(_rc_block_end)
    managed_line=$(_rc_managed_line)

    mkdir -p "$(dirname "$rc_file")"
    touch "$rc_file"

    awk \
        -v begin_marker="$begin_marker" \
        -v end_marker="$end_marker" \
        -v managed_line="$managed_line" '
        $0 == begin_marker {
            in_block = 1
            next
        }
        $0 == end_marker {
            in_block = 0
            next
        }
        in_block { next }
        $0 == managed_line { next }
        { print }
        ' "$rc_file" > "$tmp_file" && mv "$tmp_file" "$rc_file"

    [ -s "$rc_file" ] && printf '\n' >> "$rc_file"
    printf '%s\n%s\n%s\n' "$begin_marker" "$managed_line" "$end_marker" >> "$rc_file"
}

_write_rc_block_fish() {
    local rc_file=$1
    local tmp_file="${rc_file}.tmp.$$"
    local begin_marker end_marker managed_line_home managed_line_path

    begin_marker=$(_rc_block_begin)
    end_marker=$(_rc_block_end)
    managed_line_home=$(_rc_managed_line_fish)
    managed_line_path=$(_rc_managed_line_fish_path)

    mkdir -p "$(dirname "$rc_file")"
    touch "$rc_file"

    awk \
        -v begin_marker="$begin_marker" \
        -v end_marker="$end_marker" \
        -v managed_line_home="$managed_line_home" \
        -v managed_line_path="$managed_line_path" '
        $0 == begin_marker {
            in_block = 1
            next
        }
        $0 == end_marker {
            in_block = 0
            next
        }
        in_block { next }
        $0 == managed_line_home { next }
        $0 == managed_line_path { next }
        { print }
        ' "$rc_file" > "$tmp_file" && mv "$tmp_file" "$rc_file"

    [ -s "$rc_file" ] && printf '\n' >> "$rc_file"
    printf '%s\n%s\n%s\n%s\n' "$begin_marker" "$managed_line_home" "$managed_line_path" "$end_marker" >> "$rc_file"
}

_remove_rc_block() {
    local rc_file=$1
    local tmp_file="${rc_file}.tmp.$$"
    local begin_marker end_marker managed_line

    [ -n "$rc_file" ] || return 0
    [ -f "$rc_file" ] || return 0

    begin_marker=$(_rc_block_begin)
    end_marker=$(_rc_block_end)
    managed_line=$(_rc_managed_line)

    awk \
        -v begin_marker="$begin_marker" \
        -v end_marker="$end_marker" \
        -v managed_line="$managed_line" '
        $0 == begin_marker {
            in_block = 1
            next
        }
        $0 == end_marker {
            in_block = 0
            next
        }
        in_block { next }
        $0 == managed_line { next }
        { print }
        ' "$rc_file" > "$tmp_file" && mv "$tmp_file" "$rc_file"
}

_remove_rc_block_fish() {
    local rc_file=$1
    local tmp_file="${rc_file}.tmp.$$"
    local begin_marker end_marker managed_line_home managed_line_path

    [ -n "$rc_file" ] || return 0
    [ -f "$rc_file" ] || return 0

    begin_marker=$(_rc_block_begin)
    end_marker=$(_rc_block_end)
    managed_line_home=$(_rc_managed_line_fish)
    managed_line_path=$(_rc_managed_line_fish_path)

    awk \
        -v begin_marker="$begin_marker" \
        -v end_marker="$end_marker" \
        -v managed_line_home="$managed_line_home" \
        -v managed_line_path="$managed_line_path" '
        $0 == begin_marker {
            in_block = 1
            next
        }
        $0 == end_marker {
            in_block = 0
            next
        }
        in_block { next }
        $0 == managed_line_home { next }
        $0 == managed_line_path { next }
        { print }
        ' "$rc_file" > "$tmp_file" && mv "$tmp_file" "$rc_file"
}

_set_rc() {
    local rc_file

    [ "${1-}" = "unset" ] && {
        for rc_file in "$SHELL_RC_BASH" "$SHELL_RC_ZSH"; do
            _remove_rc_block "$rc_file"
        done
        [ -n "${SHELL_RC_FISH:-}" ] && _remove_rc_block_fish "$SHELL_RC_FISH"
        return 0
    }

    for rc_file in "$SHELL_RC_BASH" "$SHELL_RC_ZSH"; do
        [ -n "$rc_file" ] || continue
        _write_rc_block "$rc_file"
    done
    [ -n "${SHELL_RC_FISH:-}" ] && _write_rc_block_fish "$SHELL_RC_FISH"
    :
}

# 默认集成、安装mihomo内核
# 移除/删除mihomo：下载安装clash内核
function _get_kernel() {
    [ -f "$ZIP_CLASH" ] && {
        ZIP_KERNEL=$ZIP_CLASH
        BIN_KERNEL=$BIN_CLASH
    }

    [ -f "$ZIP_MIHOMO" ] && {
        ZIP_KERNEL=$ZIP_MIHOMO
        BIN_KERNEL=$BIN_MIHOMO
    }

    [ ! -f "$ZIP_MIHOMO" ] && [ ! -f "$ZIP_CLASH" ] && {
        local arch=$(uname -m)
        _failcat "${ZIP_BASE_DIR}：未检测到可用的内核压缩包"
        _download_clash "$arch"
        ZIP_KERNEL=$ZIP_CLASH
        BIN_KERNEL=$BIN_CLASH
    }

    BIN_KERNEL_NAME=$(basename "$BIN_KERNEL")
    _okcat "安装内核：${BIN_KERNEL_NAME}"
}

# 检测并选择预编译的 labproxy-tui 压缩包
function _get_tui_archive() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)

    # 标准化架构名称
    case "$arch" in
    x86_64)
        arch="amd64"
        ;;
    aarch64)
        arch="arm64"
        ;;
    armv*)
        arch="arm64"
        ;;
    i686|i386)
        arch="386"
        ;;
    esac

    # 查找匹配的预编译 TUI
    local candidate="${ZIP_BASE_DIR}/labproxy-tui-${os}-${arch}.tar.gz"
    if [ -f "$candidate" ]; then
        ZIP_LABPROXY_TUI="$candidate"
        _okcat "使用预编译 TUI：$(basename "$ZIP_LABPROXY_TUI")"
        return 0
    fi

    # 没有找到预编译版本
    ZIP_LABPROXY_TUI=""
    _failcat "未找到预编译 TUI（${os}/${arch}），将尝试从源码构建"
    return 1
}

_get_random_port() {
    local randomPort
    # Try shuf first (Linux), then use alternative methods
    if command -v shuf >/dev/null 2>&1; then
        randomPort=$(shuf -i 1024-65535 -n 1)
    elif command -v jot >/dev/null 2>&1; then
        # macOS/BSD
        randomPort=$(jot -r 1 1024 65535)
    else
        # Fallback using RANDOM (bash/zsh)
        randomPort=$((RANDOM % 64512 + 1024))
    fi

    ! _is_bind "$randomPort" && { echo "$randomPort" && return; }
    _get_random_port
}

# 端口状态与偏好文件路径
LABPROXY_PORT_STATE="${LABPROXY_HOME_DIR}/config/ports.conf"
LABPROXY_PORT_PREF="${LABPROXY_HOME_DIR}/config/port.pref"

# 读取代理端口偏好设置
_load_port_preferences() {
    PORT_PREF_MODE=auto
    PORT_PREF_VALUE=""

    [ -f "$LABPROXY_PORT_PREF" ] || return 0

    while IFS='=' read -r key value; do
        case "$key" in
        PROXY_MODE)
            [ -n "$value" ] && PORT_PREF_MODE=$value
            ;;
        PROXY_PORT)
            PORT_PREF_VALUE=$value
            ;;
        esac
    done < "$LABPROXY_PORT_PREF"

    [ "$PORT_PREF_MODE" = "manual" ] || PORT_PREF_MODE=auto
}

# 保存代理端口偏好
_save_port_preferences() {
    local mode=$1
    local value=$2

    mkdir -p "$(dirname "$LABPROXY_PORT_PREF")"
    cat > "$LABPROXY_PORT_PREF" <<EOF
PROXY_MODE=$mode
PROXY_PORT=$value
EOF
}

# 保存实际监听端口到状态文件
_save_port_state() {
    local proxy_port=$1
    local ui_port=$2
    local dns_port=$3

    mkdir -p "$(dirname "$LABPROXY_PORT_STATE")"
    cat > "$LABPROXY_PORT_STATE" << EOF
PROXY_PORT=$proxy_port
UI_PORT=$ui_port
DNS_PORT=$dns_port
TIMESTAMP=$(date +%s)
EOF
}

# 从状态文件读取实际监听端口
function _get_proxy_port() {
    if [ -f "$LABPROXY_PORT_STATE" ]; then
        MIXED_PORT=$(grep "^PROXY_PORT=" "$LABPROXY_PORT_STATE" 2>/dev/null | cut -d'=' -f2)
    fi
    # 如果状态文件不存在或读取失败，使用默认值
    MIXED_PORT=${MIXED_PORT:-7890}
}

function _get_ui_port() {
    if [ -f "$LABPROXY_PORT_STATE" ]; then
        UI_PORT=$(grep "^UI_PORT=" "$LABPROXY_PORT_STATE" 2>/dev/null | cut -d'=' -f2)
    fi
    # 如果状态文件不存在或读取失败，使用默认值
    UI_PORT=${UI_PORT:-9090}
}

function _get_dns_port() {
    if [ -f "$LABPROXY_PORT_STATE" ]; then
        DNS_PORT=$(grep "^DNS_PORT=" "$LABPROXY_PORT_STATE" 2>/dev/null | cut -d'=' -f2)
    fi
    # 如果状态文件不存在或读取失败，使用默认值
    DNS_PORT=${DNS_PORT:-15353}
}

_get_color() {
    local hex="${1#\#}"
    local r=$((16#${hex:0:2}))
    local g=$((16#${hex:2:2}))
    local b=$((16#${hex:4:2}))
    printf "\e[38;2;%d;%d;%dm" "$r" "$g" "$b"
}
_get_color_msg() {
    local color=$(_get_color "$1")
    local msg=$2
    local reset="\033[0m"
    printf "%b%s%b\n" "$color" "$msg" "$reset"
}

function _okcat() {
    local color=#c8d6e5
    local emoji=🐙
    [ $# -gt 1 ] && emoji=$1 && shift
    local msg="${emoji} $1"
    _get_color_msg "$color" "$msg" && return 0
}

function _failcat() {
    local color=#fd79a8
    local emoji=🦑
    [ $# -gt 1 ] && emoji=$1 && shift
    local msg="${emoji} $1"
    _get_color_msg "$color" "$msg" >&2 && return 1
}

_has_tty() {
    [ -t 0 ] && [ -t 1 ]
}

function _quit() {
    if [ -n "$_SHELL" ] && _has_tty; then
        if [ -n "$TERM_PROGRAM_VERSION" ] || [ -n "$VSCODE_IPC_HOOK_CLI" ] || [ -n "$ELECTRON_RUN_AS_NODE" ]; then
            _okcat '💡' '检测到 VS Code 终端，请手动执行 source ~/.bashrc 或重新打开终端以生效'
        else
            exec "$_SHELL" -i
        fi
    fi
    return 0
}

function _error_quit() {
    [ $# -gt 0 ] && {
        local color=#f92f60
        local emoji=🚨
        [ $# -gt 1 ] && emoji=$1 && shift
        local msg="${emoji} $1"
        _get_color_msg "$color" "$msg"
    }
    [ -z "$_SHELL" ] && _SHELL=bash

    if _has_tty; then
        exec "$_SHELL" -i
    fi

    exit 1
}

_is_bind() {
    local port=$1
    { ss -lnptu || netstat -lnptu; } 2>/dev/null | grep ":${port}\b"
}

_is_already_in_use() {
    local port=$1
    local progress=$2
    _is_bind "$port" | grep -qs -v "$progress"
}


# Removed _is_root function - not needed in userspace

function _valid_env() {
    # 用户空间运行，不需要root权限检查
    if [ -z "$ZSH_VERSION" ] && [ -z "$BASH_VERSION" ]; then
        _failcat "仅支持 bash、zsh（例如：bash install.sh）"
        return 1
    fi
    return 0
}

function _valid_config() {
    [ -e "$1" ] && [ "$(wc -l <"$1")" -gt 1 ] || return 1

    local msg
    msg=$("$BIN_KERNEL" -d "$(dirname "$1")" -f "$1" -t 2>&1) || {
        echo "$msg" | grep -qs "unsupport proxy type" && _error_quit "不支持的代理协议，请安装 mihomo 内核"
        return 1
    }

    return 0
}

_download_clash() {
    local arch=$1
    local url sha256sum
    case "$arch" in
    x86_64)
        url=https://downloads.clash.wiki/ClashPremium/clash-linux-amd64-2023.08.17.gz
        sha256sum='92380f053f083e3794c1681583be013a57b160292d1d9e1056e7fa1c2d948747'
        ;;
    *86*)
        url=https://downloads.clash.wiki/ClashPremium/clash-linux-386-2023.08.17.gz
        sha256sum='254125efa731ade3c1bf7cfd83ae09a824e1361592ccd7c0cccd2a266dcb92b5'
        ;;
    armv*)
        url=https://downloads.clash.wiki/ClashPremium/clash-linux-armv5-2023.08.17.gz
        sha256sum='622f5e774847782b6d54066f0716114a088f143f9bdd37edf3394ae8253062e8'
        ;;
    aarch64)
        url=https://downloads.clash.wiki/ClashPremium/clash-linux-arm64-2023.08.17.gz
        sha256sum='c45b39bb241e270ae5f4498e2af75cecc0f03c9db3c0db5e55c8c4919f01afdd'
        ;;
    *)
        _error_quit "未知的架构：${arch}，请自行下载对应版本至 ${ZIP_BASE_DIR} 目录下：https://downloads.clash.wiki/ClashPremium/"
        ;;
    esac

    _okcat '⏳' "正在下载 Clash 内核（${arch} 架构）..."
    local clash_zip="${ZIP_BASE_DIR}/$(basename "$url")"
    curl \
        --progress-bar \
        --show-error \
        --fail \
        --insecure \
        --connect-timeout 15 \
        --retry 1 \
        --output "$clash_zip" \
        "$url"
    echo $sha256sum "$clash_zip" | sha256sum -c ||
        _error_quit "下载失败，请自行下载对应版本至 ${ZIP_BASE_DIR} 目录下：https://downloads.clash.wiki/ClashPremium/"
}

_download_raw_config() {
    local dest=$1
    local url=$2
    local agent='clash-verge/v2.0.4'
    local tmp
    tmp=$(mktemp 2>/dev/null) || tmp="${dest}.tmp.$$"

    _cleanup_tmp() { rm -f "$tmp"; }

    # 订阅地址常见 302 跳转；同时需要对 4xx/5xx 做失败处理，避免写入 HTML/错误页导致后续解析失败。
    # 优先直连（历史行为），失败后再尝试走当前环境代理（mihomo 开启后可用）。
    if curl \
        --silent \
        --show-error \
        --fail \
        --location \
        --max-redirs 5 \
        --compressed \
        --insecure \
        --connect-timeout 10 \
        --max-time 30 \
        --retry 2 \
        --noproxy "*" \
        --user-agent "$agent" \
        --output "$tmp" \
        "$url"; then
        mv -f "$tmp" "$dest"
        return 0
    fi

    if curl \
        --silent \
        --show-error \
        --fail \
        --location \
        --max-redirs 5 \
        --compressed \
        --insecure \
        --connect-timeout 10 \
        --max-time 30 \
        --retry 2 \
        --user-agent "$agent" \
        --output "$tmp" \
        "$url"; then
        mv -f "$tmp" "$dest"
        return 0
    fi

    if wget \
        --no-verbose \
        --no-check-certificate \
        --timeout 10 \
        --tries 2 \
        --user-agent "$agent" \
        --output-document "$tmp" \
        "$url" 2>/dev/null; then
        mv -f "$tmp" "$dest"
        return 0
    fi

    if wget \
        --no-verbose \
        --no-check-certificate \
        --timeout 10 \
        --tries 1 \
        --no-proxy \
        --user-agent "$agent" \
        --output-document "$tmp" \
        "$url" 2>/dev/null; then
        mv -f "$tmp" "$dest"
        return 0
    fi

    _cleanup_tmp
    return 1
}

_build_labproxy_tui() {
    local source_dir="${1:-$LABPROXY_TUI_SRC_DIR}"
    local dest="${2:-$LABPROXY_TUI_BIN}"

    command -v go >/dev/null 2>&1 || {
        _failcat "未检测到 Go 环境，无法构建内置 TUI"
        return 1
    }

    [ -f "$source_dir/go.mod" ] || {
        _failcat "未找到内置 TUI 源码：${source_dir}"
        return 1
    }

    mkdir -p "$(dirname "$dest")"
    _okcat "正在构建内置 TUI..."
    (
        cd "$source_dir" &&
            GO111MODULE=on CGO_ENABLED=0 go build -o "$dest" ./cmd/labproxy-tui
    ) || {
        rm -f "$dest"
        _failcat "构建内置 TUI 失败"
        return 1
    }

    chmod +x "$dest"
    _okcat "内置 TUI 构建完成 ✨"
}

_tui_supports_restart_command() {
    local bin="${1:-$LABPROXY_TUI_BIN}"
    [ -x "$bin" ] || return 1
    "$bin" -h 2>&1 | grep -Fqs -- '-restart-command'
}

_ensure_tui_binary() {
    local bin="${1:-$LABPROXY_TUI_BIN}"

    if [ ! -x "$bin" ]; then
        _build_labproxy_tui "$LABPROXY_TUI_SRC_DIR" "$bin" || return 1
    fi

    if _tui_supports_restart_command "$bin"; then
        return 0
    fi

    _failcat '⚠️' '检测到旧版 TUI 二进制，正在尝试重新构建...'
    if command -v go >/dev/null 2>&1 && [ -f "$LABPROXY_TUI_SRC_DIR/go.mod" ]; then
        _build_labproxy_tui "$LABPROXY_TUI_SRC_DIR" "$bin" || return 1
        _tui_supports_restart_command "$bin" && return 0
    fi

    return 1
}

_install_tui_from_source() {
    _okcat "从源码构建 TUI..."

    # 复制 TUI 源码
    mkdir -p "$LABPROXY_TUI_SRC_DIR"
    cp -rf "$SCRIPT_DIR"/cmd "$LABPROXY_TUI_SRC_DIR"/ 2>/dev/null || true
    cp -rf "$SCRIPT_DIR"/internal "$LABPROXY_TUI_SRC_DIR"/ 2>/dev/null || true
    cp "$SCRIPT_DIR"/go.mod "$LABPROXY_TUI_SRC_DIR"/ 2>/dev/null || true
    [ -f "$SCRIPT_DIR"/go.sum ] && cp "$SCRIPT_DIR"/go.sum "$LABPROXY_TUI_SRC_DIR"/ 2>/dev/null || true

    if command -v go >/dev/null 2>&1; then
        _build_labproxy_tui "$LABPROXY_TUI_SRC_DIR" "$LABPROXY_TUI_BIN" || _failcat "安装阶段未能构建内置 TUI，可在首次执行 'labproxy tui' 时重试"
    else
        _failcat "未检测到 Go 环境，首次执行 'labproxy tui' 前需要先安装 Go 以构建内置 TUI"
    fi
}

_download_convert_config() {
    local dest=$1
    local url=$2
    _start_convert || return 1
    local convert_url=$(
        target='clash'
        base_url="http://127.0.0.1:${BIN_SUBCONVERTER_PORT}/sub"
        curl \
            --get \
            --silent \
            --output /dev/null \
            --data-urlencode "target=$target" \
            --data-urlencode "url=$url" \
            --write-out '%{url_effective}' \
            "$base_url"
    )
    _download_raw_config "$dest" "$convert_url"
    local status=$?
    _stop_convert
    return $status
}
function _download_config() {
    local dest=$1
    local url=$2
    [ "${url:0:4}" = 'file' ] && return 0
    _download_raw_config "$dest" "$url" || return 1
    _okcat '✅' '下载成功，正在校验配置...'
    _valid_config "$dest" || {
        _failcat '⚠️' "校验失败，尝试订阅转换..."
        _download_convert_config "$dest" "$url" || _failcat '❌' "转换失败，请检查日志：${BIN_SUBCONVERTER_LOG}"
    }
}

_start_convert() {
    # Ensure config exists (YAML) so we can manage port reliably.
    [ ! -e "$BIN_SUBCONVERTER_CONFIG" ] && {
        cp -f "$BIN_SUBCONVERTER_DIR/pref.example.yml" "$BIN_SUBCONVERTER_CONFIG" 2>/dev/null || true
    }

    local config_port
    config_port=$("$BIN_YQ" '.server.port // ""' "$BIN_SUBCONVERTER_CONFIG" 2>/dev/null)
    [[ $config_port =~ ^[0-9]+$ ]] && BIN_SUBCONVERTER_PORT=$config_port

    _is_already_in_use $BIN_SUBCONVERTER_PORT 'subconverter' && {
        local newPort=$(_get_random_port)
        _failcat "端口 ${BIN_SUBCONVERTER_PORT} 已占用，随机分配：${newPort}"
        "$BIN_YQ" -i ".server.port = $newPort" "$BIN_SUBCONVERTER_CONFIG"
        BIN_SUBCONVERTER_PORT=$newPort
    }
    local start=$(date +%s)
    # 子shell运行，屏蔽kill时的输出
    (cd "$BIN_SUBCONVERTER_DIR" && "$BIN_SUBCONVERTER" 2>&1 | tee "$BIN_SUBCONVERTER_LOG" >/dev/null &)
    while ! _is_bind "$BIN_SUBCONVERTER_PORT" >&/dev/null; do
        sleep 1s
        local now=$(date +%s)
        [ $((now - start)) -gt 10 ] && _error_quit "订阅转换服务启动超时，请检查日志：${BIN_SUBCONVERTER_LOG}"
    done
}
_stop_convert() {
    pkill -9 -f "$BIN_SUBCONVERTER" >&/dev/null || true
}

# User-space process management functions
_is_labproxy_pid() {
    local pid=$1
    [[ $pid =~ ^[0-9]+$ ]] || return 1

    if [ -r "/proc/$pid/exe" ] && command -v readlink >/dev/null 2>&1; then
        local exe expected
        exe=$(readlink -f "/proc/$pid/exe" 2>/dev/null || true)
        expected=$(readlink -f "$BIN_KERNEL" 2>/dev/null || true)
        [ -n "$exe" ] && [ -n "$expected" ] && [ "$exe" = "$expected" ] && return 0
    fi

    local args
    args=$(ps -p "$pid" -o args= 2>/dev/null || true)
    [ -n "$args" ] || return 1
    echo "$args" | grep -Fqs " -d $LABPROXY_HOME_DIR" || return 1
    echo "$args" | grep -Fqs " -f $LABPROXY_CONFIG_RUNTIME" || return 1
    return 0
}

start_labproxy() {
    local pid_file="$LABPROXY_HOME_DIR/config/labproxy.pid"
    local log_file="$LABPROXY_HOME_DIR/logs/labproxy.log"

    # Create necessary directories
    mkdir -p "$(dirname "$pid_file")" "$(dirname "$log_file")"

    # Check if labproxy is already running
    if is_labproxy_running; then
        _okcat "LabProxy 进程已在运行"
        return 0
    fi

    # Validate configuration before starting
    _valid_config "$LABPROXY_CONFIG_RUNTIME" || {
        _failcat "配置校验失败，无法启动 LabProxy"
        return 1
    }

    # Start labproxy process in background using nohup
    nohup "$BIN_KERNEL" -d "$LABPROXY_HOME_DIR" -f "$LABPROXY_CONFIG_RUNTIME" \
        > "$log_file" 2>&1 &

    local pid=$!
    echo "$pid" > "$pid_file"

    # Wait a moment and verify the process started successfully
    sleep 1
    if is_labproxy_running; then
        _okcat "LabProxy 进程启动成功（PID: ${pid}）"
        return 0
    else
        rm -f "$pid_file"
        _failcat "LabProxy 进程启动失败，请检查日志：${log_file}"
        return 1
    fi
}

stop_labproxy() {
    local pid_file="$LABPROXY_HOME_DIR/config/labproxy.pid"

    if [ ! -f "$pid_file" ]; then
        _okcat "LabProxy 进程未运行"
        return 0
    fi

    local pid=$(cat "$pid_file" 2>/dev/null)
    if [ -z "$pid" ]; then
        rm -f "$pid_file"
        _okcat "PID 文件为空，已清理"
        return 0
    fi

    if ! _is_labproxy_pid "$pid"; then
        _failcat "PID 文件指向非 LabProxy 进程，已清理以避免误杀（PID: ${pid}）"
        rm -f "$pid_file"
        return 1
    fi

    # Try graceful shutdown first
    if kill "$pid" 2>/dev/null; then
        # Wait for graceful shutdown
        local count=0
        while kill -0 "$pid" 2>/dev/null && [ $count -lt 10 ]; do
            sleep 1
            count=$((count + 1))
        done

        # Force kill if still running
        if kill -0 "$pid" 2>/dev/null; then
            kill -9 "$pid" 2>/dev/null
            _okcat "LabProxy 进程已强制终止（PID: ${pid}）"
        else
            _okcat "LabProxy 进程已优雅停止（PID: ${pid}）"
        fi
    else
        _okcat "LabProxy 进程已停止"
    fi

    rm -f "$pid_file"
    # 清理端口状态文件
    rm -f "$LABPROXY_PORT_STATE"
    return 0
}

is_labproxy_running() {
    local pid_file="$LABPROXY_HOME_DIR/config/labproxy.pid"

    [ ! -f "$pid_file" ] && return 1

    local pid=$(cat "$pid_file" 2>/dev/null)
    [ -z "$pid" ] && return 1

    # Check if process is actually running
    kill -0 "$pid" 2>/dev/null && _is_labproxy_pid "$pid"
}

_resolve_port_conflicts() {
    local config_file=$1
    local show_message=${2:-true}
    local port_changed=false

    _load_port_preferences

    # Check mixed-port (proxy port)
    local mixed_port=$("$BIN_YQ" '.mixed-port // ""' "$config_file" 2>/dev/null)
    if [ "$PORT_PREF_MODE" = "manual" ]; then
        if ! [[ $PORT_PREF_VALUE =~ ^[0-9]+$ ]]; then
            PORT_PREF_VALUE=7890
        fi
        MIXED_PORT=$PORT_PREF_VALUE
        "$BIN_YQ" -i ".mixed-port = $MIXED_PORT" "$config_file"
    else
        MIXED_PORT=${mixed_port:-7890}
    fi

    if _is_already_in_use "$MIXED_PORT" "$BIN_KERNEL_NAME"; then
        local require_auto=false

        if [ "$PORT_PREF_MODE" = "manual" ]; then
            local interactive=false
            [ -t 0 ] && interactive=true

            if [ "$interactive" = true ]; then
                while true; do
                    [ "$show_message" = true ] && _failcat "代理端口 ${MIXED_PORT} 已占用"
                    printf "端口 %s 已被占用，选择操作 [r]重新输入/[a]自动分配: " "$MIXED_PORT"
                    read -r choice
                    case "$choice" in
                    [rR])
                        printf "请输入新的代理端口 [1024-65535]: "
                        read -r manual_port
                        if ! [[ $manual_port =~ ^[0-9]+$ ]] || [ "$manual_port" -lt 1024 ] || [ "$manual_port" -gt 65535 ]; then
                            _failcat '❌' "请输入有效的端口号"
                            continue
                        fi
                        if _is_already_in_use "$manual_port" "$BIN_KERNEL_NAME"; then
                            MIXED_PORT=$manual_port
                            continue
                        fi
                        "$BIN_YQ" -i ".mixed-port = $manual_port" "$config_file"
                        MIXED_PORT=$manual_port
                        PORT_PREF_VALUE=$manual_port
                        _save_port_preferences manual "$manual_port"
                        port_changed=true
                        break
                        ;;
                    [aA])
                        _save_port_preferences auto ""
                        PORT_PREF_VALUE=""
                        PORT_PREF_MODE=auto
                        require_auto=true
                        break
                        ;;
                    *)
                        _failcat '❌' "无效的选项，请重新选择"
                        ;;
                    esac
                done
            else
                [ "$show_message" = true ] && _failcat "代理端口 ${MIXED_PORT} 已占用"
                _okcat "检测到非交互环境，已切换为自动分配端口"
                _save_port_preferences auto ""
                PORT_PREF_VALUE=""
                PORT_PREF_MODE=auto
                require_auto=true
            fi
        else
            require_auto=true
            [ "$show_message" = true ] && _failcat "代理端口 ${MIXED_PORT} 已占用"
        fi

        if [ "$require_auto" = true ]; then
            local newPort=$(_get_random_port)
            [ "$show_message" = true ] && _failcat "代理端口 ${MIXED_PORT} 已占用，已分配 ${newPort}"
            "$BIN_YQ" -i ".mixed-port = $newPort" "$config_file"
            MIXED_PORT=$newPort
            port_changed=true
        fi
    fi

    # Check external-controller (UI port)
    local ext_addr=$("$BIN_YQ" '.external-controller // ""' "$config_file" 2>/dev/null)
    if [ -n "$ext_addr" ]; then
        local ext_port=${ext_addr##*:}
        UI_PORT=${ext_port:-9090}
        # Preserve the original bind address format
        local bind_addr=${ext_addr%:*}
        [ "$bind_addr" = "$ext_addr" ] && bind_addr="127.0.0.1"  # fallback if no colon found
    else
        UI_PORT=9090
        bind_addr="127.0.0.1"
    fi

    if _is_already_in_use "$UI_PORT" "$BIN_KERNEL_NAME"; then
        local newPort=$(_get_random_port)
        [ "$show_message" = true ] && _failcat "UI 端口 ${UI_PORT} 已占用，已分配 ${newPort}"
        "$BIN_YQ" -i ".external-controller = \"${bind_addr}:$newPort\"" "$config_file"
        UI_PORT=$newPort
        port_changed=true
    fi

    # Check DNS listen port
    local dns_listen=$("$BIN_YQ" '.dns.listen // ""' "$config_file" 2>/dev/null)
    if [ -n "$dns_listen" ]; then
        local dns_port=${dns_listen##*:}
        DNS_PORT=${dns_port:-15353}
        # Preserve the original bind address format
        local dns_bind_addr=${dns_listen%:*}
        [ "$dns_bind_addr" = "$dns_listen" ] && dns_bind_addr="0.0.0.0"  # fallback if no colon found
    else
        DNS_PORT=15353
        dns_bind_addr="0.0.0.0"
    fi

    if _is_already_in_use "$DNS_PORT" "$BIN_KERNEL_NAME"; then
        local newPort=$(_get_random_port)
        [ "$show_message" = true ] && _failcat "DNS 端口 ${DNS_PORT} 已占用，已分配 ${newPort}"
        "$BIN_YQ" -i ".dns.listen = \"${dns_bind_addr}:$newPort\"" "$config_file"
        DNS_PORT=$newPort
        port_changed=true
    fi

    if [ "$port_changed" = true ] && [ "$show_message" = true ]; then
        _okcat "端口分配完成 — 代理:${MIXED_PORT} UI:${UI_PORT} DNS:${DNS_PORT}"
    fi

    return 0
}
