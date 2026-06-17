#!/usr/bin/env bash
# 卸载时残留进程清理的集成测试。
# 覆盖 _cleanup_residual_processes：停止 subconverter 等残留进程。
set -eo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
TEST_TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TEST_TMPDIR"' EXIT

TEST_HOME="$TEST_TMPDIR/home"
TEST_BIN="$TEST_TMPDIR/bin"
mkdir -p "$TEST_HOME/.labproxy/bin/subconverter" "$TEST_BIN"

# common.sh 在 set -u 下会引用未定义的 ZSH_VERSION；source 时临时放开。
set +u
# shellcheck disable=SC1091
. "$ROOT_DIR/scripts/common.sh"
set -u

# 指向测试隔离的 home
HOME="$TEST_HOME"
LABPROXY_HOME_DIR="$TEST_HOME/.labproxy"

# 手动设定 _cleanup_residual_processes 依赖的变量，避免 _set_bin 在无内核时漏设。
BIN_SUBCONVERTER_DIR="${LABPROXY_HOME_DIR}/bin/subconverter"
BIN_SUBCONVERTER="${BIN_SUBCONVERTER_DIR}/subconverter"
BIN_KERNEL="${LABPROXY_HOME_DIR}/bin/mihomo"
BIN_KERNEL_NAME="mihomo"

# fake subconverter 进程：长期休眠，便于被清理
cat > "$BIN_SUBCONVERTER" <<'EOF'
#!/usr/bin/env bash
trap 'exit 0' TERM
sleep 600
EOF
chmod +x "$BIN_SUBCONVERTER"

# 启动残留 subconverter 进程
"$BIN_SUBCONVERTER" &
SUB_PID=$!

# fake mihomo 残留进程（用同样目录的 stub 模拟）
cat > "$BIN_KERNEL" <<'EOF'
#!/usr/bin/env bash
trap 'exit 0' TERM
sleep 600
EOF
chmod +x "$BIN_KERNEL"
"$BIN_KERNEL" -d "$LABPROXY_HOME_DIR" -f "$LABPROXY_HOME_DIR/runtime.yaml" &
KERNEL_PID=$!

# 确认进程确实存活
kill -0 "$SUB_PID" 2>/dev/null || { printf 'FAIL: subconverter not started\n' >&2; exit 1; }
kill -0 "$KERNEL_PID" 2>/dev/null || { printf 'FAIL: kernel not started\n' >&2; exit 1; }

# 写 PID 文件模拟 labproxy 运行态（让 _cleanup_residual_processes 顺带清理）
mkdir -p "$LABPROXY_HOME_DIR/config"
echo "$KERNEL_PID" > "$LABPROXY_HOME_DIR/config/labproxy.pid"

# 执行清理
_cleanup_residual_processes || true

# 断言：两个进程都应已终止
if kill -0 "$SUB_PID" 2>/dev/null; then
    printf 'FAIL: subconverter still running after cleanup\n' >&2
    exit 1
fi
if kill -0 "$KERNEL_PID" 2>/dev/null; then
    printf 'FAIL: kernel still running after cleanup\n' >&2
    exit 1
fi

printf 'PASS uninstall residual process cleanup\n'
