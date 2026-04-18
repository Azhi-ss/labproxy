#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"

: "${ZSH_VERSION:=}"
: "${fish_version:=}"
: "${TMPDIR:=}"
if [ -z "$TMPDIR" ] || [ ! -d "$TMPDIR" ] || [ ! -w "$TMPDIR" ]; then
    TMPDIR=/tmp
fi
export TMPDIR

TEST_TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TEST_TMPDIR"' EXIT

TEST_ROOT="$TEST_TMPDIR/work"
TEST_HOME="$TEST_ROOT/home"
mkdir -p "$TEST_ROOT/bin" "$TEST_ROOT/config" "$TEST_ROOT/logs" "$TEST_HOME"
export HOME="$TEST_HOME"

cat > "$TEST_ROOT/bin/fake-yq" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

set_key() {
    local file=$1
    local key=$2
    local value=$3

    mkdir -p "$(dirname "$file")"
    touch "$file"

    if grep -q "^${key}:" "$file"; then
        sed -i "s#^${key}:.*#${key}: ${value}#" "$file"
    else
        printf '%s: %s\n' "$key" "$value" >> "$file"
    fi
}

get_key() {
    local file=$1
    local key=$2
    local default=${3-}

    if [ -f "$file" ]; then
        local line
        line=$(grep "^${key}:" "$file" | tail -n 1 || true)
        if [ -n "$line" ]; then
            printf '%s\n' "${line#*: }"
            return 0
        fi
    fi

    printf '%s\n' "$default"
}

case "${1-}" in
eval-all)
    cat "${3}"
    ;;
-i)
    expr=$2
    file=$3
    case "$expr" in
    '.secret = "'*'"')
        value=${expr#'.secret = "'}
        value=${value%'"'}
        set_key "$file" "secret" "$value"
        ;;
    '.tun.enable = true')
        set_key "$file" "tun.enable" "true"
        ;;
    '.tun.enable = false')
        set_key "$file" "tun.enable" "false"
        ;;
    '.allow-lan = true')
        set_key "$file" "allow-lan" "true"
        ;;
    '.allow-lan = false')
        set_key "$file" "allow-lan" "false"
        ;;
    '.system-proxy.enable = true')
        set_key "$file" "system-proxy.enable" "true"
        ;;
    '.system-proxy.enable = false')
        set_key "$file" "system-proxy.enable" "false"
        ;;
    *)
        printf 'unsupported fake-yq expression: %s\n' "$expr" >&2
        exit 1
        ;;
    esac
    ;;
'.secret // ""')
    get_key "$2" "secret" ""
    ;;
'.tun.enable')
    get_key "$2" "tun.enable" "false"
    ;;
'.allow-lan // false')
    get_key "$2" "allow-lan" "false"
    ;;
'.system-proxy.enable')
    get_key "$2" "system-proxy.enable" "false"
    ;;
'.system-proxy.enable // true')
    get_key "$2" "system-proxy.enable" "true"
    ;;
'.authentication[0] // ""')
    printf '\n'
    ;;
*)
    printf 'unsupported fake-yq invocation: %s\n' "$*" >&2
    exit 1
    ;;
esac
EOF
chmod +x "$TEST_ROOT/bin/fake-yq"

set +u
. "$ROOT_DIR/scripts/common.sh"
. "$ROOT_DIR/scripts/proxyctl.sh"
set -u

LABPROXY_HOME_DIR="$TEST_ROOT"
LABPROXY_SCRIPT_DIR="$TEST_ROOT/scripts"
LABPROXY_CONFIG_MIXIN="$TEST_ROOT/config/mixin.yaml"
LABPROXY_CONFIG_RUNTIME="$TEST_ROOT/config/runtime.yaml"
LABPROXY_CONFIG_RAW="$TEST_ROOT/config/raw.yaml"
LABPROXY_CONFIG_RAW_BAK="$TEST_ROOT/config/raw.yaml.bak"
LABPROXY_UPDATE_LOG="$TEST_ROOT/logs/labproxyctl.log"
LABPROXY_CONFIG_URL="$TEST_ROOT/config/url"
LABPROXY_PORT_STATE="$TEST_ROOT/config/ports.conf"
LABPROXY_PORT_PREF="$TEST_ROOT/config/port.pref"
LABPROXY_TUI_SRC_DIR="$TEST_ROOT/tui-src"
LABPROXY_TUI_BIN="$TEST_ROOT/bin/labproxy-tui"
BIN_YQ="$TEST_ROOT/bin/fake-yq"
mkdir -p "$LABPROXY_SCRIPT_DIR"

IS_RUNNING_RET=0
VALID_CONFIG_RET=0
RESTART_LOG="$TEST_ROOT/restarts.log"
START_LABPROXY_RET=0
ACTION_LOG="$TEST_ROOT/actions.log"
DOWNLOAD_CONFIG_RET=0

_okcat() { :; }
_failcat() { return 1; }
_error_quit() { return 1; }
labproxyrestart() { echo restart >> "$RESTART_LOG"; }
is_labproxy_running() { return "$IS_RUNNING_RET"; }
_valid_config() { return "$VALID_CONFIG_RET"; }
start_labproxy() { echo start_labproxy >> "$ACTION_LOG"; return "$START_LABPROXY_RET"; }
_resolve_port_conflicts() { echo "resolve_port_conflicts:$1:$2" >> "$ACTION_LOG"; MIXED_PORT=1111; UI_PORT=2222; DNS_PORT=3333; }
_verify_actual_ports() { echo verify_actual_ports >> "$ACTION_LOG"; MIXED_PORT=4444; UI_PORT=5555; DNS_PORT=6666; }
_save_port_state() { echo "save_port_state:$1:$2:$3" >> "$ACTION_LOG"; }
_set_system_proxy() { echo set_system_proxy >> "$ACTION_LOG"; }
sleep() { echo "sleep:$1" >> "$ACTION_LOG"; }
_is_bind() { return 0; }
_tui_supports_restart_command() { return 0; }
_download_config() {
    echo "download_config:$2" >> "$ACTION_LOG"
    [ "$DOWNLOAD_CONFIG_RET" -eq 0 ] || return 1
    cat > "$1" <<'EOF'
secret: downloaded-secret
tun.enable: false
allow-lan: false
system-proxy.enable: false
EOF
}
_build_labproxy_tui() {
    echo build_labproxy_tui >> "$ACTION_LOG"
    cat > "$LABPROXY_TUI_BIN" <<EOF
#!/usr/bin/env bash
echo "labproxy_tui:\$*" >> "$ACTION_LOG"
EOF
    chmod +x "$LABPROXY_TUI_BIN"
}

restart_count() {
    if [ -f "$RESTART_LOG" ]; then
        wc -l < "$RESTART_LOG" | tr -d ' '
    else
        echo 0
    fi
}

reset_files() {
    VALID_CONFIG_RET=0
    IS_RUNNING_RET=0
    START_LABPROXY_RET=0
    DOWNLOAD_CONFIG_RET=0
    : > "$RESTART_LOG"
    : > "$ACTION_LOG"

    cat > "$LABPROXY_CONFIG_MIXIN" <<'EOF'
secret:
tun.enable: false
allow-lan: false
system-proxy.enable: false
EOF

    cat > "$LABPROXY_CONFIG_RUNTIME" <<'EOF'
secret:
tun.enable: false
allow-lan: false
system-proxy.enable: false
EOF

    cat > "$LABPROXY_CONFIG_RAW" <<'EOF'
secret:
tun.enable: false
allow-lan: false
system-proxy.enable: false
EOF
}

assert_file_contains() {
    local file=$1
    local expected=$2
    if ! grep -Fq -- "$expected" "$file"; then
        printf 'assertion failed: expected %s to contain %s\n' "$file" "$expected" >&2
        return 1
    fi
}

assert_equals() {
    local expected=$1
    local actual=$2
    local label=$3
    if [ "$expected" != "$actual" ]; then
        printf 'assertion failed: %s (expected=%s actual=%s)\n' "$label" "$expected" "$actual" >&2
        return 1
    fi
}

assert_file_not_contains() {
    local file=$1
    local unexpected=$2
    if [ -f "$file" ] && grep -Fq -- "$unexpected" "$file"; then
        printf 'assertion failed: expected %s to not contain %s\n' "$file" "$unexpected" >&2
        return 1
    fi
}

run_test() {
    local name=$1
    shift
    reset_files
    "$@"
    printf 'PASS %s\n' "$name"
}

test_merge_config_restart_rebuilds_and_restarts() {
    cat > "$LABPROXY_CONFIG_MIXIN" <<'EOF'
secret: merged-secret
tun.enable: false
allow-lan: false
system-proxy.enable: false
EOF

    _merge_config_restart

    assert_file_contains "$LABPROXY_CONFIG_RUNTIME" "secret: merged-secret"
    assert_equals 1 "$(restart_count)" "restart count"
}

test_merge_config_restart_rolls_back_and_skips_restart_on_validation_failure() {
    echo 'secret: old-runtime' > "$LABPROXY_CONFIG_RUNTIME"
    echo 'secret: new-runtime' > "$LABPROXY_CONFIG_MIXIN"
    VALID_CONFIG_RET=1

    (
        _error_quit() { exit 1; }
        _merge_config_restart
    ) || true

    assert_file_contains "$LABPROXY_CONFIG_RUNTIME" "secret: old-runtime"
    assert_equals 0 "$(restart_count)" "restart count after failed validation"
}

test_labproxysecret_updates_mixin_and_restarts() {
    labproxysecret "fresh-secret"

    assert_file_contains "$LABPROXY_CONFIG_MIXIN" "secret: fresh-secret"
    assert_equals 1 "$(restart_count)" "restart count"
}

test_tun_commands_update_mixin_and_restart() {
    IS_RUNNING_RET=0
    echo 'tun.enable: true' > "$LABPROXY_CONFIG_RUNTIME"
    echo 'tun.enable: true' > "$LABPROXY_CONFIG_MIXIN"
    _tunoff
    assert_file_contains "$LABPROXY_CONFIG_MIXIN" "tun.enable: false"
    assert_equals 1 "$(restart_count)" "restart count after tun off"

    : > "$RESTART_LOG"
    echo 'tun.enable: false' > "$LABPROXY_CONFIG_RUNTIME"
    echo 'tun.enable: false' > "$LABPROXY_CONFIG_MIXIN"
    _tunon
    assert_file_contains "$LABPROXY_CONFIG_MIXIN" "tun.enable: true"
    assert_equals 1 "$(restart_count)" "restart count after tun on"
}

test_lan_commands_update_mixin_and_restart() {
    echo 'allow-lan: false' > "$LABPROXY_CONFIG_RUNTIME"
    echo 'allow-lan: false' > "$LABPROXY_CONFIG_MIXIN"
    _lanon
    assert_file_contains "$LABPROXY_CONFIG_MIXIN" "allow-lan: true"
    assert_equals 1 "$(restart_count)" "restart count after lan on"

    : > "$RESTART_LOG"
    echo 'allow-lan: true' > "$LABPROXY_CONFIG_RUNTIME"
    echo 'allow-lan: true' > "$LABPROXY_CONFIG_MIXIN"
    _lanoff
    assert_file_contains "$LABPROXY_CONFIG_MIXIN" "allow-lan: false"
    assert_equals 1 "$(restart_count)" "restart count after lan off"
}

test_labproxyon_builds_runtime_and_finalizes_startup() {
    cat > "$LABPROXY_CONFIG_MIXIN" <<'EOF'
secret: startup-secret
tun.enable: false
allow-lan: false
system-proxy.enable: false
EOF

    IS_RUNNING_RET=1
    labproxyon

    assert_file_contains "$LABPROXY_CONFIG_RUNTIME" "secret: startup-secret"
    assert_file_contains "$ACTION_LOG" "resolve_port_conflicts:$LABPROXY_CONFIG_RUNTIME:true"
    assert_file_contains "$ACTION_LOG" "start_labproxy"
    assert_file_contains "$ACTION_LOG" "sleep:2"
    assert_file_contains "$ACTION_LOG" "verify_actual_ports"
    assert_file_contains "$ACTION_LOG" "save_port_state:4444:5555:6666"
    assert_file_contains "$ACTION_LOG" "set_system_proxy"
}

test_labproxyon_stops_after_start_failure() {
    START_LABPROXY_RET=1
    IS_RUNNING_RET=1

    if labproxyon; then
        printf 'expected labproxyon to fail when start_labproxy fails\n' >&2
        return 1
    fi

    assert_file_contains "$ACTION_LOG" "resolve_port_conflicts:$LABPROXY_CONFIG_RUNTIME:true"
    assert_file_contains "$ACTION_LOG" "start_labproxy"
    assert_file_not_contains "$ACTION_LOG" "sleep:2"
    assert_file_not_contains "$ACTION_LOG" "verify_actual_ports"
    assert_file_not_contains "$ACTION_LOG" "save_port_state:"
    assert_file_not_contains "$ACTION_LOG" "set_system_proxy"
}

test_labproxysubscribe_saves_url_without_immediate_update() {
    if ! printf 'n\n' | (
        labproxyupdate() { echo "labproxyupdate:$*" >> "$ACTION_LOG"; }
        labproxysubscribe "http://example.com/subscription"
    ); then
        printf 'expected labproxysubscribe to succeed on negative reply\n' >&2
        return 1
    fi

    assert_file_contains "$LABPROXY_CONFIG_URL" "http://example.com/subscription"
    assert_file_not_contains "$ACTION_LOG" "labproxyupdate:http://example.com/subscription"
}

test_labproxyupdate_persists_url_logs_success_and_restarts() {
    labproxyupdate "http://example.com/updated"

    assert_file_contains "$ACTION_LOG" "download_config:http://example.com/updated"
    assert_file_contains "$LABPROXY_CONFIG_URL" "http://example.com/updated"
    assert_file_contains "$LABPROXY_UPDATE_LOG" "订阅更新成功：http://example.com/updated"
    assert_equals 1 "$(restart_count)" "restart count after labproxyupdate"
}

test_labproxytui_builds_and_launches_first_party_binary() {
    echo 'secret: super-secret' > "$LABPROXY_CONFIG_RUNTIME"
    rm -f "$LABPROXY_TUI_BIN"
    IS_RUNNING_RET=0

    labproxytui

    assert_file_contains "$ACTION_LOG" "build_labproxy_tui"
    assert_file_contains "$ACTION_LOG" "verify_actual_ports"
    assert_file_contains "$ACTION_LOG" "labproxy_tui:--endpoint http://127.0.0.1:5555"
    assert_file_contains "$ACTION_LOG" "--mixin-config $LABPROXY_CONFIG_MIXIN"
    assert_file_contains "$ACTION_LOG" "--restart-command source \"$LABPROXY_SCRIPT_DIR/common.sh\" && source \"$LABPROXY_SCRIPT_DIR/proxyctl.sh\" && labproxyrestart"
}

run_test "_merge_config_restart rebuilds runtime and restarts" test_merge_config_restart_rebuilds_and_restarts
run_test "_merge_config_restart rolls back and skips restart on validation failure" test_merge_config_restart_rolls_back_and_skips_restart_on_validation_failure
run_test "labproxysecret updates mixin and restarts" test_labproxysecret_updates_mixin_and_restarts
run_test "tun commands update mixin and restart" test_tun_commands_update_mixin_and_restart
run_test "lan commands update mixin and restart" test_lan_commands_update_mixin_and_restart
run_test "labproxyon builds runtime and finalizes startup" test_labproxyon_builds_runtime_and_finalizes_startup
run_test "labproxyon stops after start failure" test_labproxyon_stops_after_start_failure
run_test "labproxysubscribe saves url without immediate update" test_labproxysubscribe_saves_url_without_immediate_update
run_test "labproxyupdate persists url logs success and restarts" test_labproxyupdate_persists_url_logs_success_and_restarts
run_test "labproxytui builds and launches first-party binary" test_labproxytui_builds_and_launches_first_party_binary
