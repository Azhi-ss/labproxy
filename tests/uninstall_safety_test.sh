#!/usr/bin/env bash
# 卸载路径安全校验的表驱动单元测试。
# 覆盖 _labproxy_home_is_safe 纯函数：防止 $HOME 异常时 rm -rf 误删。
set -eo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
TEST_TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TEST_TMPDIR"' EXIT

# common.sh 在 set -u 下会引用未定义的 ZSH_VERSION；source 时临时放开。
set +u
# shellcheck disable=SC1091
. "$ROOT_DIR/scripts/common.sh"
set -u

FAIL=0

# 用法: run_case <home> <target> <expected_safe:0|1> <label>
run_case() {
    local home="$1"
    local target="$2"
    local expected="$3"
    local label="$4"

    HOME="$home"
    LABPROXY_HOME_DIR="$target"

    local actual
    if _labproxy_home_is_safe; then
        actual=0   # safe
    else
        actual=1   # unsafe
    fi

    if [ "$actual" != "$expected" ]; then
        printf 'FAIL: %s (HOME=%q target=%q expected=%s actual=%s)\n' \
            "$label" "$home" "$target" "$expected" "$actual" >&2
        FAIL=1
    fi
}

# ---- 安全用例（应通过校验）----
run_case "$TEST_TMPDIR/home" "$TEST_TMPDIR/home/.labproxy" 0 "normal home subdir"
run_case "/home/user"        "/home/user/.labproxy"        0 "absolute home subdir"

# ---- 危险用例（应拒绝）----
run_case ""                  "$TEST_TMPDIR/home/.labproxy"  1 "empty HOME"
run_case "$TEST_TMPDIR/home" ""                            1 "empty target"
run_case "$TEST_TMPDIR/home" "relative/.labproxy"          1 "relative target"
run_case "$TEST_TMPDIR/home" "/etc/labproxy"               1 "outside HOME"
run_case "$TEST_TMPDIR/home" "$TEST_TMPDIR/home"           1 "equals HOME itself"
run_case "$TEST_TMPDIR/home" "/"                           1 "root path"
run_case "$TEST_TMPDIR/home" "/tmp"                        1 "sibling of HOME scope"

# ---- 路径遍历绕过用例（应拒绝：含 .. 逃逸 HOME）----
run_case "$TEST_TMPDIR/home" "$TEST_TMPDIR/home/../etc/labproxy" 1 "traversal escapes HOME"
run_case "$TEST_TMPDIR/home" "$TEST_TMPDIR/home/.labproxy/../../etc" 1 "nested traversal"
# 尾随斜杠应被规范化处理，仍判定安全
run_case "$TEST_TMPDIR/home" "$TEST_TMPDIR/home/.labproxy/" 0 "trailing slash still safe"

if [ "$FAIL" -ne 0 ]; then
    printf 'FAIL uninstall safety checks\n' >&2
    exit 1
fi

printf 'PASS uninstall safety checks\n'
