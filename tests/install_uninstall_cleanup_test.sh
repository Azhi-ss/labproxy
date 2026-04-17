#!/usr/bin/env bash
set -euo pipefail

export TMPDIR=/tmp
export TMP=/tmp
export TEMP=/tmp

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
TEST_TMPDIR="$(mktemp -d)"
TEST_HOME="$TEST_TMPDIR/home"
LOG_FILE="$TEST_TMPDIR/install-uninstall.log"
SENTINEL_DIR="$ROOT_DIR/resources/bin"
SENTINEL_FILE="$SENTINEL_DIR/.install-uninstall-test-sentinel"
SENTINEL_DIR_CREATED=0

cleanup() {
    rm -rf "$TEST_TMPDIR"
    rm -f "$SENTINEL_FILE"
    if [ "$SENTINEL_DIR_CREATED" -eq 1 ]; then
        rmdir "$SENTINEL_DIR" 2>/dev/null || true
    fi
}
trap cleanup EXIT

mkdir -p "$TEST_HOME"

if [ ! -d "$SENTINEL_DIR" ]; then
    mkdir -p "$SENTINEL_DIR"
    SENTINEL_DIR_CREATED=1
fi
printf 'keep\n' > "$SENTINEL_FILE"

cat > "$TEST_HOME/.bashrc" <<'EOF'
export KEEP_BASH=1
EOF

assert_exists() {
    local path=$1
    if [ ! -e "$path" ]; then
        printf 'assertion failed: expected %s to exist\n' "$path" >&2
        return 1
    fi
}

assert_not_exists() {
    local path=$1
    if [ -e "$path" ]; then
        printf 'assertion failed: expected %s to not exist\n' "$path" >&2
        return 1
    fi
}

assert_file_contains() {
    local file=$1
    local expected=$2
    if ! grep -Fq -- "$expected" "$file"; then
        printf 'assertion failed: expected %s to contain %s\n' "$file" "$expected" >&2
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

if ! HOME="$TEST_HOME" bash "$ROOT_DIR/install.sh" > "$LOG_FILE" 2>&1; then
    cat "$LOG_FILE" >&2
    exit 1
fi

assert_exists "$TEST_HOME/.labproxy"
assert_exists "$TEST_HOME/.labproxy/bin/labproxy-tui"
assert_file_contains "$TEST_HOME/.bashrc" "export KEEP_BASH=1"
assert_file_contains "$TEST_HOME/.bashrc" "# >>> labproxy >>>"
assert_file_contains "$TEST_HOME/.bashrc" "source $TEST_HOME/.labproxy/scripts/common.sh && source $TEST_HOME/.labproxy/scripts/proxyctl.sh && watch_proxy"
assert_exists "$SENTINEL_FILE"

if ! HOME="$TEST_HOME" bash "$ROOT_DIR/uninstall.sh" >> "$LOG_FILE" 2>&1; then
    cat "$LOG_FILE" >&2
    exit 1
fi

assert_not_exists "$TEST_HOME/.labproxy"
assert_file_contains "$TEST_HOME/.bashrc" "export KEEP_BASH=1"
assert_file_not_contains "$TEST_HOME/.bashrc" "# >>> labproxy >>>"
assert_file_not_contains "$TEST_HOME/.bashrc" "source $TEST_HOME/.labproxy/scripts/common.sh && source $TEST_HOME/.labproxy/scripts/proxyctl.sh && watch_proxy"
assert_exists "$SENTINEL_FILE"

printf 'PASS install/uninstall cleanup regression\n'
