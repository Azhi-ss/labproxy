#!/usr/bin/env bash
# Regression test for avoiding duplicate mihomo starts when an existing
# LabProxy kernel process is already running but the PID file is missing.
set -uo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
TEST_TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TEST_TMPDIR"' EXIT

TEST_HOME="$TEST_TMPDIR/home"
TEST_BIN="$TEST_TMPDIR/bin"
TEST_ROOT="$TEST_HOME/.labproxy"
mkdir -p "$TEST_BIN" "$TEST_ROOT/bin" "$TEST_ROOT/config" "$TEST_ROOT/logs"

EXISTING_PID=42424
export EXISTING_PID

cat > "$TEST_BIN/kill" <<'EOF'
#!/usr/bin/env bash
if [ "${1-}" = "-0" ] && [ "${2-}" = "$EXISTING_PID" ]; then
    exit 1
fi
exit 1
EOF
chmod +x "$TEST_BIN/kill"

cat > "$TEST_BIN/ps" <<'EOF'
#!/usr/bin/env bash
if [ "${1-}" = "-p" ] && [ "${2-}" = "$EXISTING_PID" ]; then
    case "$*" in
    *args=*) printf '%s\n' "/tmp/test-home/.labproxy/bin/mihomo -d $LABPROXY_HOME_DIR -f $LABPROXY_CONFIG_RUNTIME" ;;
    *) printf '  PID\n%s\n' "$EXISTING_PID" ;;
    esac
    exit 0
fi
exit 1
EOF
chmod +x "$TEST_BIN/ps"

cat > "$TEST_BIN/pgrep" <<'EOF'
#!/usr/bin/env bash
case "$*" in
*mihomo*) printf '%s\n' "$EXISTING_PID"; exit 0 ;;
*) exit 1 ;;
esac
EOF
chmod +x "$TEST_BIN/pgrep"

cat > "$TEST_BIN/nohup" <<'EOF'
#!/usr/bin/env bash
printf 'duplicate start attempted\n' >> "$NOHUP_LOG"
exit 0
EOF
chmod +x "$TEST_BIN/nohup"

cat > "$TEST_ROOT/bin/mihomo" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "$TEST_ROOT/bin/mihomo"

PATH="$TEST_BIN:/usr/bin:/bin"
export PATH HOME="$TEST_HOME"

set +u
# shellcheck disable=SC1091
. "$ROOT_DIR/scripts/common.sh"
set -u

LABPROXY_HOME_DIR="$TEST_ROOT"
LABPROXY_CONFIG_RUNTIME="$TEST_ROOT/runtime.yaml"
LABPROXY_PORT_STATE="$TEST_ROOT/config/ports.conf"
BIN_KERNEL="$TEST_ROOT/bin/mihomo"
BIN_KERNEL_NAME="mihomo"
NOHUP_LOG="$TEST_TMPDIR/nohup.log"
export LABPROXY_HOME_DIR LABPROXY_CONFIG_RUNTIME NOHUP_LOG

cat > "$LABPROXY_CONFIG_RUNTIME" <<'EOF'
mode: rule
mixed-port: 7893
EOF

_okcat() { :; }
_failcat() { :; }
_valid_config() { return 0; }
_labproxy_service_active() { return 1; }

FAIL=0
fail() {
    printf 'FAIL: %s\n' "$1" >&2
    FAIL=1
}

start_labproxy || fail "start_labproxy should reuse existing process"

if [ -s "$NOHUP_LOG" ]; then
    fail "start_labproxy attempted to start a duplicate kernel"
fi

pid_file="$TEST_ROOT/config/labproxy.pid"
if [ "$(cat "$pid_file" 2>/dev/null)" != "$EXISTING_PID" ]; then
    fail "PID file should be reconciled to existing process"
fi

if [ "$FAIL" -ne 0 ]; then
    exit 1
fi
printf 'PASS start reconciles existing kernel\n'
