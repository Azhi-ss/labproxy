#!/usr/bin/env bash
# _is_dead_local_proxy_env 单元测试（proxyctl.sh）。
# 验证：当 http_proxy 指向本地端口且该端口无监听时判定为「失效本地代理」，
# 从而触发 watch_proxy 切换到 LabProxy 代理；其他情形（非本地、端口存活、无代理）应返回 false。
set -uo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"

# 提供最小变量供 proxyctl.sh / common.sh 引用
LABPROXY_HOME_DIR="${LABPROXY_HOME_DIR:-/tmp/labproxy-dlp-test}"
LABPROXY_CONFIG_MIXIN="${LABPROXY_CONFIG_MIXIN:-/tmp/labproxy-dlp-test/mixin.yaml}"
BIN_YQ="${BIN_YQ:-true}"
export LABPROXY_HOME_DIR LABPROXY_CONFIG_MIXIN BIN_YQ

set +u
# shellcheck disable=SC1091
. "$ROOT_DIR/scripts/common.sh"
# shellcheck disable=SC1091
. "$ROOT_DIR/scripts/proxyctl.sh"
set -u

FAIL=0
fail() { printf 'FAIL: %s\n' "$1" >&2; FAIL=1; }

# mock _is_bind：仅当端口 51569 视为「无监听」（返回 1），其余视为监听
_is_bind() {
    [ "$1" = "51569" ] && return 1
    return 0
}

# Case 1: 失效本地代理（127.0.0.1:51569 无监听）→ 应判定 dead（返回 0）
run_case_dead() {
    http_proxy="http://127.0.0.1:51569" _is_dead_local_proxy_env \
        || fail "失效本地代理 127.0.0.1:51569 应判定为 dead"
}

# Case 2: 端口存活的本地代理（51570 视为监听）→ 非 dead
run_case_alive() {
    http_proxy="http://127.0.0.1:51570" _is_dead_local_proxy_env \
        && fail "存活的本地代理 127.0.0.1:51570 不应判定为 dead"
}

# Case 3: 远程代理 → 非 dead（不应误切）
run_case_remote() {
    http_proxy="http://10.0.0.5:7890" _is_dead_local_proxy_env \
        && fail "远程代理 10.0.0.5:7890 不应判定为 dead"
}

# Case 4: 无 http_proxy → 非 dead（函数应返回 1）
run_case_none() {
    unset http_proxy
    _is_dead_local_proxy_env \
        && fail "无 http_proxy 时不应判定为 dead"
}

# Case 5: localhost 主机名也应识别为本地
run_case_localhost() {
    http_proxy="http://localhost:51569" _is_dead_local_proxy_env \
        || fail "localhost:51569 失效代理应判定为 dead"
}

run_case_dead
run_case_alive
run_case_remote
run_case_none
run_case_localhost

if [ "$FAIL" -eq 0 ]; then
    printf 'PASS: _is_dead_local_proxy_env 失效本地代理检测\n'
    exit 0
else
    exit 1
fi
