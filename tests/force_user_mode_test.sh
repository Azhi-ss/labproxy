#!/usr/bin/env bash
# LABPROXY_FORCE_USER_MODE 逃生口的单元测试。
# 验证：在已装特权服务 plist 的机器上，设置该变量后服务模式判定被强制关闭，
# 从而保证用户空间安装/启动流程不被全局 plist 污染。
set -uo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"

# 提供最小变量供 service.sh 引用
LABPROXY_HOME_DIR="${LABPROXY_HOME_DIR:-/tmp/labproxy-fum-test}"
BIN_KERNEL_NAME="${BIN_KERNEL_NAME:-mihomo}"
export LABPROXY_HOME_DIR BIN_KERNEL_NAME

set +u
# shellcheck disable=SC1091
. "$ROOT_DIR/scripts/service.sh"
set -u

FAIL=0
fail() { printf 'FAIL: %s\n' "$1" >&2; FAIL=1; }

# Case 1: 未设逃生口时，行为依赖真实 plist 存在性（不在此断言，避免环境耦合）
# Case 2: 设逃生口后，无论 plist 是否真实存在，都应判定为「未安装/非服务模式」
run_case_force_disables_service() {
    LABPROXY_FORCE_USER_MODE=1 _labproxy_service_installed \
        && fail "FORCE_USER_MODE=1 时 _labproxy_service_installed 应返回 false"

    LABPROXY_FORCE_USER_MODE=1 _labproxy_service_mode \
        && fail "FORCE_USER_MODE=1 时 _labproxy_service_mode 应返回 false（非服务模式）"

    # _labproxy_service_active 依赖 _labproxy_service_installed，也应 false
    LABPROXY_FORCE_USER_MODE=1 _labproxy_service_active \
        && fail "FORCE_USER_MODE=1 时 _labproxy_service_active 应返回 false"
}

# Case 3: 默认（不设）时不应改变既有行为——不抛错即可（环境无关）
run_case_default_no_crash() {
    unset LABPROXY_FORCE_USER_MODE
    _labproxy_service_installed >/dev/null 2>&1 || true
    _labproxy_service_mode >/dev/null 2>&1 || true
}

run_case_force_disables_service
run_case_default_no_crash

if [ "$FAIL" -eq 0 ]; then
    printf 'PASS: LABPROXY_FORCE_USER_MODE 逃生口\n'
    exit 0
else
    exit 1
fi
