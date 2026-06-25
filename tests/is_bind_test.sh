#!/usr/bin/env bash
# _is_bind 跨平台端口检测的单元测试。
# 复现 macOS 上 ss 缺失 + netstat -lnptu 误用导致的误判 bug，并验证修复后能正确检测到监听端口。
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

fail() {
    printf 'FAIL: %s\n' "$1" >&2
    FAIL=1
}

# 找一个空闲端口并起一个临时 TCP 监听，返回端口号。
# 用 python(广泛可用)或 nc 兜底；都不行则跳过。
_start_listener() {
    local port
    if command -v python3 >/dev/null 2>&1; then
        port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); s.listen(1); print(s.getsockname()[1])' 2>/dev/null) || return 1
        # 后台持有监听
        python3 -c "import socket,sys,time; s=socket.socket(); s.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1); s.bind(('127.0.0.1',${port})); s.listen(1); time.sleep(30)" >/dev/null 2>&1 &
        echo "$port"
        return 0
    fi
    return 1
}

TestIsBind_DetectsListeningPort() {
    local port
    port=$(_start_listener) || { printf 'SKIP: 无 python3 起监听\n' >&2; return 0; }
    # 等监听就绪
    local i
    for i in 1 2 3 4 5; do
        _is_bind "$port" | grep -q . && break
        sleep 0.3
    done
    _is_bind "$port" | grep -q . || fail "_is_bind 未检测到正在监听的端口 ${port}（macOS 误判 bug 未修复）"
}

TestIsBind_NotDetectClosedPort() {
    # 59999 几乎不可能在测试机监听
    _is_bind 59999 | grep -q . && fail "_is_bind 对未监听端口 59999 误报为监听" || true
}

TestIsBind_DetectsActualLabProxyPort() {
    # 若 9090 当前在监听(本机 labproxy 场景)，应能检测到；否则跳过
    if command -v lsof >/dev/null 2>&1; then
        if ! lsof -iTCP:9090 -sTCP:LISTEN -P -n >/dev/null 2>&1; then
            printf 'SKIP: 9090 未监听\n' >&2
            return 0
        fi
        _is_bind 9090 | grep -q . || fail "_is_bind 未检测到 9090（实际监听却报未监听）"
    fi
}

TestIsBind_DetectsListeningPort
TestIsBind_NotDetectClosedPort
TestIsBind_DetectsActualLabProxyPort

if [ "$FAIL" -eq 0 ]; then
    printf 'PASS: _is_bind 跨平台端口检测\n'
    exit 0
else
    exit 1
fi
