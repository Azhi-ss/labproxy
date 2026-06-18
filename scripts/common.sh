# shellcheck disable=SC2148
# shellcheck disable=SC2034
# shellcheck disable=SC2155
[ -n "$BASH_VERSION" ] && set +o noglob
[ -n "$ZSH_VERSION" ] && setopt glob no_nomatch

URL_GH_PROXY='https://ghfast.top'
URL_LABPROXY_UI="http://board.zash.run.place"

# ---- Download robustness (mirror pool + cache + cooldown) ----

# GitHub mirror pool - ordered by preference
_github_mirror_pool() {
    cat <<'EOF'
ghfast|https://ghfast.top|hostpath
gh-proxy|https://gh-proxy.org|full
ghproxy-net|https://ghproxy.net|hostpath
kkgithub|https://kkgithub.com|hostpath
EOF
}

# Cache directory for downloaded assets
_download_cache_dir() {
    echo "${LABPROXY_HOME_DIR}/cache/assets"
}

# Cooldown in seconds before retrying a failed mirror
_download_fail_cooldown() {
    echo "1800"
}

# State file for mirror success/failure tracking
_download_mirror_state_file() {
    echo "${LABPROXY_HOME_DIR}/cache/download-mirrors.env"
}

# Check if a URL is a GitHub URL that can be mirrored
_github_url_is_mirrorable() {
    case "$1" in
        https://github.com/*|https://raw.githubusercontent.com/*)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

# Extract host from URL
_github_url_host() {
    local url="$1"
    local no_scheme="${url#https://}"
    no_scheme="${no_scheme#http://}"
    echo "${no_scheme%%/*}"
}

# Extract path from URL
_github_url_path() {
    local url="$1"
    local no_scheme="${url#https://}"
    no_scheme="${no_scheme#http://}"
    echo "${no_scheme#*/}"
}

# Build a mirror URL from a mirror entry and original URL
_build_github_mirror_url() {
    local entry="$1"
    local url="$2"
    local label prefix mode host path

    IFS='|' read -r label prefix mode <<EOF
$entry
EOF

    case "${mode:-full}" in
        full)
            echo "${prefix%/}/${url}"
            ;;
        hostpath)
            host="$(_github_url_host "$url")"
            path="$(_github_url_path "$url")"
            echo "${prefix%/}/${host}/${path}"
            ;;
        origin)
            echo "$url"
            ;;
        *)
            echo "${prefix%/}/${url}"
            ;;
    esac
}

# Get all mirror candidates for a URL, ordered by score
_github_mirror_candidates_ordered() {
    local url="$1"
    local entry label score

    # Always include origin first
    echo "origin||origin"

    if ! _github_url_is_mirrorable "$url"; then
        return 0
    fi

    while IFS= read -r entry; do
        [ -n "${entry:-}" ] || continue
        label="${entry%%|*}"
        # Skip mirrors with recent active failure
        _download_mirror_recent_failure_active "$label" && continue
        printf '%s\n' "$entry"
    done <<EOF
$(_github_mirror_pool)
EOF
}

# Mirror state key normalization
_download_mirror_state_key() {
    local label="$1"
    printf '%s' "$label" | tr '[:lower:]-./:' '[:upper:]_____'
}

# Read a field from mirror state
_read_download_mirror_state() {
    local label="$1"
    local field="$2"
    local file key

    file="$(_download_mirror_state_file)"
    [ -f "$file" ] || return 1

    key="DOWNLOAD_MIRROR_$(_download_mirror_state_key "$label")_${field}"
    sed -nE "s/^[[:space:]]*${key}=['\"]?([^'\"]*)['\"]?$/\1/p" "$file" | head -n 1
}

# Write a field to mirror state
_write_download_mirror_state() {
    local label="$1"
    local field="$2"
    local value="$3"
    local file key

    file="$(_download_mirror_state_file)"
    mkdir -p "$(dirname "$file")"
    touch "$file"

    key="DOWNLOAD_MIRROR_$(_download_mirror_state_key "$label")_${field}"

    if grep -qE "^[[:space:]]*${key}=" "$file"; then
        awk -v k="$key" -v v="$value" '
            $0 ~ "^[[:space:]]*" k "=" {
                print k "=\"" v "\""
                next
            }
            { print }
        ' "$file" > "${file}.tmp" && mv "${file}.tmp" "$file"
    else
        printf '%s="%s"\n' "$key" "$value" >> "$file"
    fi
}

# Record a successful download through a mirror
_record_download_mirror_success() {
    local label="$1"
    local candidate_url="${2:-}"
    local now
    now=$(date +%s)

    _write_download_mirror_state "$label" "LAST_SUCCESS_AT" "$now"
    _write_download_mirror_state "$label" "LAST_SUCCESS_URL" "$candidate_url"
    _write_download_mirror_state "$label" "FAIL_STREAK" "0"
}

# Record a failed download through a mirror
_record_download_mirror_failure() {
    local label="$1"
    local candidate_url="${2:-}"
    local now fail_streak

    now=$(date +%s)
    fail_streak="$(_read_download_mirror_state "$label" "FAIL_STREAK" 2>/dev/null || echo "0")"

    case "$fail_streak" in
        ''|*[!0-9]*) fail_streak="0" ;;
    esac

    fail_streak=$((fail_streak + 1))

    _write_download_mirror_state "$label" "LAST_FAILURE_AT" "$now"
    _write_download_mirror_state "$label" "LAST_FAILURE_URL" "$candidate_url"
    _write_download_mirror_state "$label" "FAIL_STREAK" "$fail_streak"
}

# Check if a mirror has a recent failure still in cooldown
_download_mirror_recent_failure_active() {
    local label="$1"
    local fail_at success_at now cooldown delta

    [ "$label" = "origin" ] && return 1  # origin never has cooldown

    fail_at="$(_read_download_mirror_state "$label" "LAST_FAILURE_AT" 2>/dev/null || true)"
    success_at="$(_read_download_mirror_state "$label" "LAST_SUCCESS_AT" 2>/dev/null || true)"
    cooldown="$(_download_fail_cooldown)"
    now=$(date +%s)

    case "$fail_at" in ''|*[!0-9]*) return 1 ;; esac
    case "$success_at" in ''|*[!0-9]*) success_at=0 ;; esac
    case "$cooldown" in ''|*[!0-9]*) cooldown=1800 ;; esac

    [ "$fail_at" -gt "$success_at" ] || return 1
    delta=$((now - fail_at))
    [ "$delta" -lt "$cooldown" ]
}

# Download cache key (SHA256 hash of URL)
_download_cache_key() {
    local url="$1"
    if command -v sha256sum >/dev/null 2>&1; then
        printf '%s' "$url" | sha256sum | awk '{print $1}'
    elif command -v shasum >/dev/null 2>&1; then
        printf '%s' "$url" | shasum -a 256 | awk '{print $1}'
    else
        printf '%s' "$url" | cksum | awk '{print $1 "-" $2}'
    fi
}

# Cache file path for a URL
_download_cache_file() {
    local url="$1"
    echo "$(_download_cache_dir)/$(_download_cache_key "$url").bin"
}

# Restore a file from download cache
_download_cache_restore() {
    local url="$1"
    local out="$2"
    local cache_file

    cache_file="$(_download_cache_file "$url")"
    [ -s "$cache_file" ] || return 1

    mkdir -p "$(dirname "$out")"
    cp -f "$cache_file" "$out"
    _okcat "📦" "使用缓存：${url}"
    return 0
}

# Store a file in download cache
_download_cache_store() {
    local url="$1"
    local src="$2"
    local source_url="${3:-}"

    [ -s "$src" ] || return 0

    local cache_file="$(_download_cache_file "$url")"
    local meta_file="${cache_file}.meta"

    mkdir -p "$(_download_cache_dir)"
    cp -f "$src" "$cache_file"

    cat > "$meta_file" <<EOF
CACHE_URL="$url"
CACHE_SOURCE_URL="$source_url"
CACHE_TIME="$(date '+%Y-%m-%d %H:%M:%S')"
EOF
}

# Clear all download cache
_clear_download_cache() {
    rm -rf "$(_download_cache_dir)" 2>/dev/null || true
}

# Core download function with mirror pool, cache, and cooldown
# Usage: _download_file <url> <out> [asset_name]
_download_file() {
    local url="$1"
    local out="$2"
    local asset_name="${3:-$(basename "$url")}"

    mkdir -p "$(dirname "$out")"
    rm -f "$out" 2>/dev/null || true

    # Try cache first
    if _download_cache_restore "$url" "$out"; then
        return 0
    fi

    local fetch_tmp
    fetch_tmp=$(mktemp 2>/dev/null) || fetch_tmp="${out}.tmp.$$"
    rm -f "$fetch_tmp" 2>/dev/null || true

    local entry label candidate_url tried_urls=""
    local probe_ok=false

    # Phase 1: probe mirrors to find working ones
    _okcat '⏳' "正在查找可用镜像：${asset_name}"
    while IFS= read -r entry; do
        [ -n "${entry:-}" ] || continue

        candidate_url="$(_build_github_mirror_url "$entry" "$url")"
        [ -n "${candidate_url:-}" ] || continue

        printf '%s\n' "$tried_urls" | grep -Fxq "$candidate_url" && continue
        label="${entry%%|*}"

        if _download_mirror_recent_failure_active "$label"; then
            continue
        fi

        # Probe
        if curl -fsSIL --location --connect-timeout 4 --max-time 4 "$candidate_url" >/dev/null 2>&1; then
            probe_ok=true
            break
        fi
    done <<EOF
$(_github_mirror_candidates_ordered "$url")
EOF

    # Phase 2: fetch from the first working mirror
    if [ "$probe_ok" = true ]; then
        _okcat '⏳' "正在下载：${asset_name} [${label}]"
        if curl \
            --progress-bar \
            --show-error \
            --fail \
            --location \
            --connect-timeout 8 \
            --max-time 1200 \
            --retry 1 \
            --output "$fetch_tmp" \
            "$candidate_url"; then
            mv -f "$fetch_tmp" "$out"
            _download_cache_store "$url" "$out" "$candidate_url"
            _record_download_mirror_success "$label" "$candidate_url"
            return 0
        fi
        _record_download_mirror_failure "$label" "$candidate_url"
        rm -f "$fetch_tmp" 2>/dev/null || true
    fi

    # Phase 3: blind fallback - try all mirrors without probing
    _okcat '⏳' "镜像探测未通过，尝试盲连..."
    while IFS= read -r entry; do
        [ -n "${entry:-}" ] || continue

        candidate_url="$(_build_github_mirror_url "$entry" "$url")"
        [ -n "${candidate_url:-}" ] || continue

        printf '%s\n' "$tried_urls" | grep -Fxq "$candidate_url" && continue
        label="${entry%%|*}"

        if _download_mirror_recent_failure_active "$label"; then
            continue
        fi

        fetch_tmp=$(mktemp 2>/dev/null) || fetch_tmp="${out}.tmp.$$"
        rm -f "$fetch_tmp" 2>/dev/null || true

        _okcat '⏳' "正在下载：${asset_name} [${label}]"
        if curl \
            --progress-bar \
            --show-error \
            --fail \
            --location \
            --connect-timeout 8 \
            --max-time 1200 \
            --retry 1 \
            --output "$fetch_tmp" \
            "$candidate_url"; then
            mv -f "$fetch_tmp" "$out"
            _download_cache_store "$url" "$out" "$candidate_url"
            _record_download_mirror_success "$label" "$candidate_url"
            return 0
        fi

        _record_download_mirror_failure "$label" "$candidate_url"
        rm -f "$fetch_tmp" 2>/dev/null || true
        tried_urls="${tried_urls}${candidate_url}"$'\n'
    done <<EOF
$(_github_mirror_candidates_ordered "$url")
EOF

    rm -f "$fetch_tmp" 2>/dev/null || true
    return 1
}

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
LABPROXY_SUBS_FILE="${LABPROXY_HOME_DIR}/config/subscriptions.yaml"
LABPROXY_SUBS_CONFIG_DIR="${LABPROXY_HOME_DIR}/config/subscriptions"

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

# fish-compatible watch_proxy equivalent: auto-inject proxy env vars
# when opening a new interactive fish shell.
# NOTE: bash variables ($HOME, $LABPROXY_HOME) are expanded at generation time
# by bash; fish variables ($LABPROXY_HOME etc.) are expanded at fish runtime.
_rc_managed_line_fish_watch_proxy() {
    cat <<'FISH_EOF'

function _labproxy_watch_proxy
    status is-interactive; or return 0

    set -l mixin_config "$LABPROXY_HOME/mixin.yaml"
    test -f "$mixin_config"; or return 0

    set -l system_proxy_status (yq '.system-proxy.enable // true' "$mixin_config" 2>/dev/null)
    test "$system_proxy_status" = "true"; or return 0

    # Check if labproxy process is running
    set -l pid_file "$LABPROXY_HOME/config/labproxy.pid"
    test -f "$pid_file"; or return 0
    set -l pid (cat "$pid_file" 2>/dev/null)
    test -n "$pid"; or return 0
    kill -0 "$pid" 2>/dev/null; or return 0

    # Read proxy port from state file
    set -l port_state "$LABPROXY_HOME/config/ports.conf"
    test -f "$port_state"; or return 0
    set -l proxy_port (grep "^PROXY_PORT=" "$port_state" | cut -d'=' -f2)
    test -n "$proxy_port"; or set proxy_port 7890

    # Read auth from runtime config
    set -l runtime_config "$LABPROXY_HOME/runtime.yaml"
    set -l auth (yq '.authentication[0] // ""' "$runtime_config" 2>/dev/null)
    test -n "$auth"; and set auth "$auth@"

    set -l http_proxy_addr "http://$auth""127.0.0.1:$proxy_port"
    set -l socks_proxy_addr "socks5h://$auth""127.0.0.1:$proxy_port"

    if not set -q http_proxy; or test -z "$http_proxy"
        # No existing proxy, set labproxy proxy
        set -gx http_proxy $http_proxy_addr
        set -gx https_proxy $http_proxy_addr
        set -gx HTTP_PROXY $http_proxy_addr
        set -gx HTTPS_PROXY $http_proxy_addr
        set -gx all_proxy $socks_proxy_addr
        set -gx ALL_PROXY $socks_proxy_addr
        set -gx no_proxy "localhost,127.0.0.1,::1"
        set -gx NO_PROXY "localhost,127.0.0.1,::1"
    else if _labproxy_wsl_auto_proxy
        # WSL mirrored autoProxy detected, override with labproxy
        set -gx http_proxy $http_proxy_addr
        set -gx https_proxy $http_proxy_addr
        set -gx HTTP_PROXY $http_proxy_addr
        set -gx HTTPS_PROXY $http_proxy_addr
        set -gx all_proxy $socks_proxy_addr
        set -gx ALL_PROXY $socks_proxy_addr
    end
    # else: http_proxy already set by user, do not override
end

function _labproxy_wsl_auto_proxy
    test -n "$http_proxy"; or return 1
    test -f /proc/version; or return 1
    grep -qi microsoft /proc/version 2>/dev/null; or return 1
    for wslconfig in /mnt/c/Users/*/.wslconfig
        test -f "$wslconfig"; or continue
        grep -qi 'autoProxy.*true' "$wslconfig" 2>/dev/null; and return 0
    end
    return 1
end

_labproxy_watch_proxy
FISH_EOF
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
    {
        printf '%s\n' "$begin_marker"
        printf '%s\n' "$managed_line_home"
        printf '%s\n' "$managed_line_path"
        _rc_managed_line_fish_watch_proxy
        printf '%s\n' "$end_marker"
    } >> "$rc_file"
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

    # Strip the entire labproxy block (between markers) and any
    # standalone managed lines that may have been written outside the block.
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
#
# 内核检测优先级（_get_kernel）：
#   1. 内置 zip 按 OS/架构匹配（mihomo-<os>-<arch>，优先 mihomo 后 clash）
#   2. brew 安装的 mihomo（/opt/homebrew/bin/mihomo 等）
#   3. 系统 PATH 的 mihomo/clash
#   4. 按 OS/架构从 mihomo releases 下载

# MIHOMO_DOWNLOAD_VERSION 是无内置 zip 时下载的 mihomo 版本。
MIHOMO_DOWNLOAD_VERSION="v1.19.2"

# _detect_os_arch 输出规范化后的 "os arch"（如 "darwin arm64" / "linux amd64"）。
_detect_os_arch() {
    local os arch
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)
    case "$arch" in
    x86_64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    armv7l|armv*) arch="arm64" ;;
    i686|i386) arch="386" ;;
    esac
    printf '%s %s\n' "$os" "$arch"
}

# _detect_kernel_zip 在 zipdir 内按 os/arch 匹配内核压缩包，输出匹配的 zip 路径（无匹配输出空）。
# 优先 mihomo，其次 clash。
_detect_kernel_zip() {
    local zipdir="$1" os="$2" arch="$3"
    [ -n "$zipdir" ] && [ -n "$os" ] && [ -n "$arch" ] || return 0

    local f
    for f in "$zipdir"/mihomo-"${os}"-"${arch}"*.gz; do
        [ -f "$f" ] || continue
        printf '%s\n' "$f"
        return 0
    done
    for f in "$zipdir"/clash-"${os}"-"${arch}"*.gz; do
        [ -f "$f" ] || continue
        printf '%s\n' "$f"
        return 0
    done
    return 0
}

# _find_system_kernel 检测 brew 或 PATH 中的 mihomo/clash，输出路径（无则空）。
_find_system_kernel() {
    local bin
    for bin in /opt/homebrew/bin/mihomo /usr/local/bin/mihomo /opt/homebrew/bin/clash /usr/local/bin/clash; do
        [ -x "$bin" ] || continue
        printf '%s\n' "$bin"
        return 0
    done
    bin=$(command -v mihomo 2>/dev/null || true)
    [ -n "$bin" ] && { printf '%s\n' "$bin"; return 0; }
    bin=$(command -v clash 2>/dev/null || true)
    [ -n "$bin" ] && { printf '%s\n' "$bin"; return 0; }
    return 0
}

# _download_mihomo 按 os/arch 从 mihomo releases 下载内核到 zipdir，输出下载的 zip 路径。
_download_mihomo() {
    local zipdir="$1" os="$2" arch="$3"
    local version="${MIHOMO_DOWNLOAD_VERSION}"
    local url="https://github.com/MetaCubeX/mihomo/releases/download/${version}/mihomo-${os}-${arch}-${version}.gz"
    local dest="${zipdir}/mihomo-${os}-${arch}-${version}.gz"

    _okcat '⏳' "正在下载 mihomo 内核（${os}/${arch}）..."
    if _download_file "$url" "$dest" "mihomo-${os}-${arch}"; then
        printf '%s\n' "$dest"
        return 0
    fi
    _failcat "❌" "下载 mihomo 失败：${url}"
    return 1
}

function _get_kernel() {
    local os arch
    read -r os arch <<< "$(_detect_os_arch)"

    # 1. 内置 zip 按 OS/架构匹配
    local matched_zip
    matched_zip=$(_detect_kernel_zip "$ZIP_BASE_DIR" "$os" "$arch")
    if [ -n "$matched_zip" ] && [ -f "$matched_zip" ]; then
        ZIP_KERNEL=$matched_zip
        case "$(basename "$matched_zip")" in
        mihomo*) BIN_KERNEL=$BIN_MIHOMO ;;
        clash*) BIN_KERNEL=$BIN_CLASH ;;
        esac
        BIN_KERNEL_NAME=$(basename "$BIN_KERNEL")
        _okcat "安装内核（内置 zip，${os}/${arch}）：${BIN_KERNEL_NAME}"
        return 0
    fi

    # 2 & 3. brew / PATH 系统内核
    local sys_kernel
    sys_kernel=$(_find_system_kernel)
    if [ -n "$sys_kernel" ] && [ -x "$sys_kernel" ]; then
        case "$(basename "$sys_kernel")" in
        mihomo*) BIN_KERNEL=$BIN_MIHOMO ;;
        clash*) BIN_KERNEL=$BIN_CLASH ;;
        esac
        ZIP_KERNEL=""  # 系统内核无 zip
        LABPROXY_SYSTEM_KERNEL="$sys_kernel"  # 供 install.sh 复制
        BIN_KERNEL_NAME=$(basename "$BIN_KERNEL")
        _okcat "安装内核（系统 ${sys_kernel}）：${BIN_KERNEL_NAME}"
        return 0
    fi

    # 4. 下载 mihomo
    _failcat "⚠️" "未检测到 ${os}/${arch} 的内置内核 zip 或系统内核，尝试下载 mihomo..."
    local dl_zip
    if dl_zip=$(_download_mihomo "$ZIP_BASE_DIR" "$os" "$arch"); then
        ZIP_KERNEL=$dl_zip
        BIN_KERNEL=$BIN_MIHOMO
        BIN_KERNEL_NAME=$(basename "$BIN_KERNEL")
        _okcat "安装内核（下载，${os}/${arch}）：${BIN_KERNEL_NAME}"
        return 0
    fi

    _error_quit "无法获取 ${os}/${arch} 内核：请手动下载 mihomo-${os}-${arch} 至 ${ZIP_BASE_DIR}，或用 brew install mihomo"
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

    # Use mirror pool for GitHub-hosted URLs
    if _github_url_is_mirrorable "$url"; then
        _download_file "$url" "$clash_zip" "clash-${arch}" || \
            _error_quit "下载失败，请自行下载对应版本至 ${ZIP_BASE_DIR} 目录下：https://downloads.clash.wiki/ClashPremium/"
    else
        curl \
            --progress-bar \
            --show-error \
            --fail \
            --insecure \
            --connect-timeout 15 \
            --retry 1 \
            --output "$clash_zip" \
            "$url" || \
            _error_quit "下载失败，请自行下载对应版本至 ${ZIP_BASE_DIR} 目录下：https://downloads.clash.wiki/ClashPremium/"
    fi

    echo $sha256sum "$clash_zip" | sha256sum -c ||
        _error_quit "下载文件校验失败，请自行下载对应版本至 ${ZIP_BASE_DIR} 目录下：https://downloads.clash.wiki/ClashPremium/"
}

_download_raw_config() {
    local dest=$1
    local url=$2
    local agent='clash-verge/v2.0.4'
    local tmp
    tmp=$(mktemp 2>/dev/null) || tmp="${dest}.tmp.$$"

    _cleanup_tmp() { rm -f "$tmp"; }

    # Try cache first
    if _download_cache_restore "$url" "$dest"; then
        return 0
    fi

    # If it's a GitHub URL, use mirror pool
    if _github_url_is_mirrorable "$url"; then
        if _download_file "$url" "$tmp" "subscription"; then
            mv -f "$tmp" "$dest"
            return 0
        fi
        _cleanup_tmp
        return 1
    fi

    # Non-GitHub URL: direct + proxy fallback
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
        _download_cache_store "$url" "$dest" "$url"
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
        _download_cache_store "$url" "$dest" "$url"
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

# 校验 LABPROXY_HOME_DIR 删除目标是否安全。
# 防止 $HOME 为空或路径异常时 rm -rf 误删非预期目录。
# 安全条件：非空、绝对路径、严格位于 $HOME 之下、不等于 $HOME 本身、非根路径。
# 返回 0=安全，1=危险（拒绝删除）。
# 规范化路径：解析 .. 与 . 段、去尾随斜杠。
# 使用子 shell + cd 以解析符号链接与 .. 到真实物理路径；失败则原样返回。
_normalize_path() {
    local p="${1:-}"
    [ -n "$p" ] || return 1
    case "$p" in
        /*) ;;
        *) return 1 ;;
    esac

    # 优先用物理解析（解析符号链接），失败则做纯词法规范化
    local resolved
    resolved=$(
        cd -P -- "$p" 2>/dev/null && pwd -P
    ) && [ -n "$resolved" ] && { printf '%s\n' "$resolved"; return 0; }

    # 词法兜底：逐段消除 .. 与 .
    local out=""
    while [ -n "$p" ]; do
        local seg="${p%%/*}"
        local rest="${p#"$seg"}"
        rest="${rest#/}"
        if [ "$seg" = ".." ]; then
            out="${out%/*}"
            [ -z "$out" ] && out=""
        elif [ "$seg" = "." ] || [ -z "$seg" ]; then
            :
        else
            out="${out:+$out/}$seg"
        fi
        p="$rest"
    done
    [ -n "$out" ] || return 1
    printf '/%s\n' "$out"
}

# 校验 LABPROXY_HOME_DIR 删除目标是否安全。
# 防止 $HOME 为空或路径异常时 rm -rf 误删非预期目录。
# 安全条件：非空、绝对路径、规范化后严格位于 $HOME 之下、不等于 $HOME 本身、非根路径。
# 返回 0=安全，1=危险（拒绝删除）。
_labproxy_home_is_safe() {
    local target="${LABPROXY_HOME_DIR:-}"
    local home="${HOME:-}"

    [ -n "$target" ] || return 1
    [ -n "$home" ] || return 1

    # 必须是绝对路径
    case "$target" in
        /*) ;;
        *) return 1 ;;
    esac

    # 规范化后再校验，防止 .. / 符号链接绕过前缀检查
    local norm_target norm_home
    norm_target=$(_normalize_path "$target") || return 1
    norm_home=$(_normalize_path "$home") || return 1

    # 拒绝根路径与 HOME 本身
    [ "$norm_target" != "/" ] || return 1
    [ "$norm_target" != "$norm_home" ] || return 1

    # 必须严格位于规范化的 $HOME 之下
    case "$norm_target" in
        "${norm_home}/"*) return 0 ;;
        *) return 1 ;;
    esac
}

# 卸载时兜底清理残留进程：subconverter 与内核。
# install 中断或异常退出可能残留 subconverter；此处确保卸载干净。
_cleanup_residual_processes() {
    # 停止 subconverter 残留（与 _stop_convert 一致，但容忍其不存在）
    if [ -n "${BIN_SUBCONVERTER:-}" ]; then
        pkill -9 -f "$BIN_SUBCONVERTER" 2>/dev/null || true
    fi

    # 通过 PID 文件优雅停止内核；失败则兜底按内核二进制名清理
    local pid_file="${LABPROXY_HOME_DIR}/config/labproxy.pid"
    if [ -f "$pid_file" ]; then
        local pid
        pid=$(cat "$pid_file" 2>/dev/null || true)
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null || true
            local count=0
            while kill -0 "$pid" 2>/dev/null && [ $count -lt 5 ]; do
                sleep 1
                count=$((count + 1))
            done
            kill -9 "$pid" 2>/dev/null || true
        fi
        rm -f "$pid_file"
    fi

    # 兜底：按内核运行特征清理（仅当内核二进制名已知）
    if [ -n "${BIN_KERNEL_NAME:-}" ]; then
        pkill -9 -f "$BIN_KERNEL_NAME -d ${LABPROXY_HOME_DIR}" 2>/dev/null || true
    fi

    return 0
}

# ---- Multi-subscription management ----

# Ensure subscriptions YAML file exists
_ensure_subs_file() {
    if [ ! -f "$LABPROXY_SUBS_FILE" ]; then
        mkdir -p "$(dirname "$LABPROXY_SUBS_FILE")"
        cat > "$LABPROXY_SUBS_FILE" <<'EOF'
# LabProxy subscriptions
# active: name of the currently active subscription
active: ""
subscriptions: {}
EOF
    fi
}

# Get the active subscription name
_active_subscription_name() {
    _ensure_subs_file
    "$BIN_YQ" '.active // ""' "$LABPROXY_SUBS_FILE" 2>/dev/null
}

# Get the active subscription URL
_active_subscription_url() {
    local name
    name="$(_active_subscription_name)"
    [ -z "$name" ] && return 1
    "$BIN_YQ" ".subscriptions[\"$name\"].url // \"\"" "$LABPROXY_SUBS_FILE" 2>/dev/null
}

# Get config file path for a subscription
_sub_config_file() {
    local name="$1"
    echo "${LABPROXY_SUBS_CONFIG_DIR}/${name}.yaml"
}

# Set the active subscription
_set_active_subscription() {
    local name="$1"
    _ensure_subs_file
    "$BIN_YQ" -i ".active = \"$name\"" "$LABPROXY_SUBS_FILE" 2>/dev/null
}

# Apply the active subscription's config to raw config
_apply_active_subscription() {
    local name
    name="$(_active_subscription_name)"
    [ -z "$name" ] && return 1

    local sub_config="$(_sub_config_file "$name")"
    [ -f "$sub_config" ] || return 1

    cp "$sub_config" "$LABPROXY_CONFIG_RAW"
    return 0
}

# Save subscription config to its file
_save_sub_config() {
    local name="$1"
    local config_file="$2"
    mkdir -p "$LABPROXY_SUBS_CONFIG_DIR"
    cp "$config_file" "$(_sub_config_file "$name")"
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
