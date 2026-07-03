#!/usr/bin/env bash
# 特权服务模块（service.sh）的单元测试。
# 覆盖 plist 生成、服务检测逻辑，不依赖真实 launchd/sudo。
set -uo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
TEST_TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TEST_TMPDIR"' EXIT

# common.sh 在 set -u 下引用未定义变量，source 时临时放开
set +u
# shellcheck disable=SC1091
. "$ROOT_DIR/scripts/common.sh"
set -u

FAIL=0
fail() { printf 'FAIL: %s\n' "$1" >&2; FAIL=1; }

TestGeneratePlist_ValidXML() {
    local out="$TEST_TMPDIR/com.labproxy.mihomo.plist"
    _labproxy_service_generate_plist "$out" || { fail "生成 plist 失败"; return; }
    [ -f "$out" ] || { fail "plist 文件未创建"; return; }
    grep -q "com.labproxy.mihomo" "$out" || fail "plist 缺少 Label"
    grep -q "<key>KeepAlive</key>" "$out" || fail "plist 缺少 KeepAlive"
    grep -q "<true/>" "$out" || fail "plist 缺少 true 值"
    grep -q "mihomo" "$out" || fail "plist 缺少 mihomo 二进制路径"
    grep -q "runtime.yaml" "$out" || fail "plist 缺少 runtime 配置路径"
    if command -v xmllint >/dev/null 2>&1; then
        xmllint --noout "$out" 2>/dev/null || fail "plist XML 格式无效"
    fi
}

TestServiceMode_NotInstalled() {
    if [ -f "/Library/LaunchDaemons/com.labproxy.mihomo.plist" ]; then
        printf 'SKIP: 本机已装服务\n' >&2
        return 0
    fi
    _labproxy_service_mode && fail "未安装时不应判定为服务模式" || true
    _labproxy_service_installed && fail "未安装时不应报告已安装" || true
}

TestServiceActive_NotInstalled() {
    if [ -f "/Library/LaunchDaemons/com.labproxy.mihomo.plist" ]; then
        printf 'SKIP: 本机已装服务\n' >&2
        return 0
    fi
    _labproxy_service_active && fail "未安装时不应判定为活跃" || true
}

TestLabproxyservice_Help() {
    # 未知参数应打印 usage
    local out
    out=$(labproxyservice --help 2>&1)
    echo "$out" | grep -q "install" || fail "usage 缺少 install"
    echo "$out" | grep -q "uninstall" || fail "usage 缺少 uninstall"
    echo "$out" | grep -q "reload" || fail "usage 缺少 reload"
}

TestServiceActive_MatchesCurrentRuntimeOnly() {
    local fakebin="$TEST_TMPDIR/bin"
    mkdir -p "$fakebin"

    cat > "$fakebin/pgrep" <<'EOF'
#!/usr/bin/env bash
printf '11111\n'
EOF
    chmod +x "$fakebin/pgrep"

    cat > "$fakebin/ps" <<'EOF'
#!/usr/bin/env bash
case "${PS_ARGS_CASE:-wrong}" in
right)
    printf '%s\n' "$LABPROXY_HOME_DIR/bin/mihomo -d $LABPROXY_HOME_DIR -f $LABPROXY_CONFIG_RUNTIME"
    ;;
*)
    printf '%s\n' "$LABPROXY_HOME_DIR/bin/mihomo -d /tmp/other-labproxy -f /tmp/other-labproxy/runtime.yaml"
    ;;
esac
EOF
    chmod +x "$fakebin/ps"

    local old_path=$PATH
    local old_plist=${_LABPROXY_SERVICE_PLIST}
    PATH="$fakebin:$PATH"
    _LABPROXY_SERVICE_PLIST="$TEST_TMPDIR/com.labproxy.mihomo.plist"
    LABPROXY_HOME_DIR="$TEST_TMPDIR/home/.labproxy"
    LABPROXY_CONFIG_RUNTIME="$LABPROXY_HOME_DIR/runtime.yaml"
    BIN_KERNEL_NAME="mihomo"
    export LABPROXY_HOME_DIR LABPROXY_CONFIG_RUNTIME BIN_KERNEL_NAME
    mkdir -p "$LABPROXY_HOME_DIR/bin"
    : > "$_LABPROXY_SERVICE_PLIST"

    export PS_ARGS_CASE=wrong
    _labproxy_service_active \
        && fail "服务 active 检测不应接受其他 runtime 的 mihomo 进程"

    export PS_ARGS_CASE=right
    _labproxy_service_active \
        || fail "服务 active 检测应接受当前 runtime 的 mihomo 进程"
    unset PS_ARGS_CASE

    PATH="$old_path"
    _LABPROXY_SERVICE_PLIST="$old_plist"
}

TestServiceMode_ReconcilesPidFile() {
    local fakebin="$TEST_TMPDIR/reconcile-bin"
    local labhome="$TEST_TMPDIR/reconcile-home/.labproxy"
    mkdir -p "$fakebin" "$labhome/config" "$labhome/bin"

    cat > "$fakebin/pgrep" <<'EOF'
#!/usr/bin/env bash
printf '22222\n'
EOF
    chmod +x "$fakebin/pgrep"

    cat > "$fakebin/ps" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$LABPROXY_HOME_DIR/bin/mihomo -d $LABPROXY_HOME_DIR -f $LABPROXY_CONFIG_RUNTIME"
EOF
    chmod +x "$fakebin/ps"

    local old_path=$PATH
    local old_plist=${_LABPROXY_SERVICE_PLIST}
    PATH="$fakebin:$PATH"
    _LABPROXY_SERVICE_PLIST="$TEST_TMPDIR/reconcile.plist"
    LABPROXY_HOME_DIR="$labhome"
    LABPROXY_CONFIG_RUNTIME="$labhome/runtime.yaml"
    LABPROXY_PORT_STATE="$labhome/config/ports.conf"
    BIN_KERNEL_NAME="mihomo"
    export LABPROXY_HOME_DIR LABPROXY_CONFIG_RUNTIME BIN_KERNEL_NAME
    : > "$_LABPROXY_SERVICE_PLIST"
    echo 99999 > "$labhome/config/labproxy.pid"

    is_labproxy_running || fail "服务模式下存在当前 runtime 进程时应判定 running"

    local got
    got=$(cat "$labhome/config/labproxy.pid" 2>/dev/null || true)
    [ "$got" = "22222" ] || fail "服务模式 running 检测应同步 PID 文件，got=$got"

    PATH="$old_path"
    _LABPROXY_SERVICE_PLIST="$old_plist"
}

TestGeneratePlist_ValidXML
TestServiceMode_NotInstalled
TestServiceActive_NotInstalled
TestServiceActive_MatchesCurrentRuntimeOnly
TestServiceMode_ReconcilesPidFile
TestLabproxyservice_Help

if [ "$FAIL" -eq 0 ]; then
    printf 'PASS: 特权服务模块\n'
    exit 0
else
    exit 1
fi
