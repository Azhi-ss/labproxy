#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
TEST_TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TEST_TMPDIR"' EXIT

: "${ZSH_VERSION:=}"
: "${fish_version:=}"
: "${TMPDIR:=/tmp}"

set +u
. "$ROOT_DIR/script/common.sh"
set -u

TEST_HOME="$TEST_TMPDIR/home"
mkdir -p "$TEST_HOME"

MIHOMO_SCRIPT_DIR="$TEST_HOME/tools/mihomo/script"
SHELL_RC_BASH="$TEST_HOME/.bashrc"
SHELL_RC_ZSH="$TEST_HOME/.zshrc"

assert_equals() {
    local expected=$1
    local actual=$2
    local label=$3
    if [ "$expected" != "$actual" ]; then
        printf 'assertion failed: %s (expected=%s actual=%s)\n' "$label" "$expected" "$actual" >&2
        return 1
    fi
}

assert_file_contains() {
    local file=$1
    local expected=$2
    if ! grep -Fq "$expected" "$file"; then
        printf 'assertion failed: expected %s to contain %s\n' "$file" "$expected" >&2
        return 1
    fi
}

assert_file_not_contains() {
    local file=$1
    local unexpected=$2
    if [ -f "$file" ] && grep -Fq "$unexpected" "$file"; then
        printf 'assertion failed: expected %s to not contain %s\n' "$file" "$unexpected" >&2
        return 1
    fi
}

count_in_file() {
    local file=$1
    local needle=$2
    grep -F -c "$needle" "$file" 2>/dev/null || true
}

reset_rc_files() {
    cat > "$SHELL_RC_BASH" <<'EOF'
export KEEP_BASH=1
EOF

    cat > "$SHELL_RC_ZSH" <<'EOF'
export KEEP_ZSH=1
EOF
}

run_test() {
    local name=$1
    shift
    reset_rc_files
    "$@"
    printf 'PASS %s\n' "$name"
}

test_set_rc_adds_managed_block_once() {
    _set_rc
    _set_rc

    assert_equals 1 "$(count_in_file "$SHELL_RC_BASH" "# >>> clash-for-lab >>>")" "bash begin marker count"
    assert_equals 1 "$(count_in_file "$SHELL_RC_BASH" "# <<< clash-for-lab <<<")" "bash end marker count"
    assert_equals 1 "$(count_in_file "$SHELL_RC_BASH" "source $MIHOMO_SCRIPT_DIR/common.sh && source $MIHOMO_SCRIPT_DIR/clashctl.sh && watch_proxy")" "bash managed line count"

    assert_equals 1 "$(count_in_file "$SHELL_RC_ZSH" "# >>> clash-for-lab >>>")" "zsh begin marker count"
    assert_equals 1 "$(count_in_file "$SHELL_RC_ZSH" "# <<< clash-for-lab <<<")" "zsh end marker count"
    assert_equals 1 "$(count_in_file "$SHELL_RC_ZSH" "source $MIHOMO_SCRIPT_DIR/common.sh && source $MIHOMO_SCRIPT_DIR/clashctl.sh && watch_proxy")" "zsh managed line count"

    assert_file_contains "$SHELL_RC_BASH" "export KEEP_BASH=1"
    assert_file_contains "$SHELL_RC_ZSH" "export KEEP_ZSH=1"
}

test_set_rc_unset_removes_managed_block_and_keeps_other_content() {
    _set_rc
    printf 'export AFTER=1\n' >> "$SHELL_RC_BASH"
    printf 'export AFTER=1\n' >> "$SHELL_RC_ZSH"

    _set_rc unset

    assert_file_not_contains "$SHELL_RC_BASH" "# >>> clash-for-lab >>>"
    assert_file_not_contains "$SHELL_RC_BASH" "source $MIHOMO_SCRIPT_DIR/common.sh && source $MIHOMO_SCRIPT_DIR/clashctl.sh && watch_proxy"
    assert_file_contains "$SHELL_RC_BASH" "export KEEP_BASH=1"
    assert_file_contains "$SHELL_RC_BASH" "export AFTER=1"

    assert_file_not_contains "$SHELL_RC_ZSH" "# >>> clash-for-lab >>>"
    assert_file_not_contains "$SHELL_RC_ZSH" "source $MIHOMO_SCRIPT_DIR/common.sh && source $MIHOMO_SCRIPT_DIR/clashctl.sh && watch_proxy"
    assert_file_contains "$SHELL_RC_ZSH" "export KEEP_ZSH=1"
    assert_file_contains "$SHELL_RC_ZSH" "export AFTER=1"
}

test_set_rc_replaces_legacy_line_with_managed_block() {
    printf 'source %s/common.sh && source %s/clashctl.sh && watch_proxy\n' "$MIHOMO_SCRIPT_DIR" "$MIHOMO_SCRIPT_DIR" >> "$SHELL_RC_BASH"
    printf 'export KEEP_PATH=%s\n' "$MIHOMO_SCRIPT_DIR" >> "$SHELL_RC_BASH"

    _set_rc

    assert_equals 1 "$(count_in_file "$SHELL_RC_BASH" "# >>> clash-for-lab >>>")" "legacy migration begin marker count"
    assert_equals 1 "$(count_in_file "$SHELL_RC_BASH" "source $MIHOMO_SCRIPT_DIR/common.sh && source $MIHOMO_SCRIPT_DIR/clashctl.sh && watch_proxy")" "legacy migration managed line count"
    assert_file_contains "$SHELL_RC_BASH" "export KEEP_PATH=$MIHOMO_SCRIPT_DIR"
}

run_test "_set_rc adds managed block once" test_set_rc_adds_managed_block_once
run_test "_set_rc unset removes managed block and keeps other content" test_set_rc_unset_removes_managed_block_and_keeps_other_content
run_test "_set_rc replaces legacy line with managed block" test_set_rc_replaces_legacy_line_with_managed_block
