#!/usr/bin/env bash
set -euo pipefail

export TMPDIR=/tmp
export TMP=/tmp
export TEMP=/tmp

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
TEST_TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TEST_TMPDIR"' EXIT

TEST_HOME="$TEST_TMPDIR/home"
mkdir -p "$TEST_HOME"
export HOME="$TEST_HOME"

: "${ZSH_VERSION:=}"
: "${fish_version:=}"
: "${TMPDIR:=/tmp}"

set +u
. "$ROOT_DIR/scripts/common.sh"
set -u

LABPROXY_SCRIPT_DIR="$TEST_HOME/.labproxy/scripts"
SHELL_RC_BASH="$TEST_HOME/.bashrc"
SHELL_RC_ZSH="$TEST_HOME/.zshrc"
SHELL_RC_FISH="$TEST_HOME/.config/fish/config.fish"
mkdir -p "$LABPROXY_SCRIPT_DIR"

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

count_in_file() {
    local file=$1
    local needle=$2
    grep -F -c -- "$needle" "$file" 2>/dev/null || true
}

reset_rc_files() {
    cat > "$SHELL_RC_BASH" <<'EOF'
export KEEP_BASH=1
EOF

    cat > "$SHELL_RC_ZSH" <<'EOF'
export KEEP_ZSH=1
EOF

    mkdir -p "$(dirname "$SHELL_RC_FISH")"
    cat > "$SHELL_RC_FISH" <<'EOF'
set -gx KEEP_FISH 1
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

    assert_equals 1 "$(count_in_file "$SHELL_RC_BASH" "# >>> labproxy >>>")" "bash begin marker count"
    assert_equals 1 "$(count_in_file "$SHELL_RC_BASH" "# <<< labproxy <<<")" "bash end marker count"
    assert_equals 1 "$(count_in_file "$SHELL_RC_BASH" "source $LABPROXY_SCRIPT_DIR/common.sh && source $LABPROXY_SCRIPT_DIR/proxyctl.sh && watch_proxy")" "bash managed line count"

    assert_equals 1 "$(count_in_file "$SHELL_RC_ZSH" "# >>> labproxy >>>")" "zsh begin marker count"
    assert_equals 1 "$(count_in_file "$SHELL_RC_ZSH" "# <<< labproxy <<<")" "zsh end marker count"
    assert_equals 1 "$(count_in_file "$SHELL_RC_ZSH" "source $LABPROXY_SCRIPT_DIR/common.sh && source $LABPROXY_SCRIPT_DIR/proxyctl.sh && watch_proxy")" "zsh managed line count"

    assert_file_contains "$SHELL_RC_BASH" "export KEEP_BASH=1"
    assert_file_contains "$SHELL_RC_ZSH" "export KEEP_ZSH=1"
}

test_set_rc_unset_removes_managed_block_and_keeps_other_content() {
    _set_rc
    printf 'export AFTER=1\n' >> "$SHELL_RC_BASH"
    printf 'export AFTER=1\n' >> "$SHELL_RC_ZSH"

    _set_rc unset

    assert_file_not_contains "$SHELL_RC_BASH" "# >>> labproxy >>>"
    assert_file_not_contains "$SHELL_RC_BASH" "source $LABPROXY_SCRIPT_DIR/common.sh && source $LABPROXY_SCRIPT_DIR/proxyctl.sh && watch_proxy"
    assert_file_contains "$SHELL_RC_BASH" "export KEEP_BASH=1"
    assert_file_contains "$SHELL_RC_BASH" "export AFTER=1"

    assert_file_not_contains "$SHELL_RC_ZSH" "# >>> labproxy >>>"
    assert_file_not_contains "$SHELL_RC_ZSH" "source $LABPROXY_SCRIPT_DIR/common.sh && source $LABPROXY_SCRIPT_DIR/proxyctl.sh && watch_proxy"
    assert_file_contains "$SHELL_RC_ZSH" "export KEEP_ZSH=1"
    assert_file_contains "$SHELL_RC_ZSH" "export AFTER=1"
}

test_set_rc_rewrites_existing_managed_block_without_duplication() {
    cat > "$SHELL_RC_BASH" <<EOF
export KEEP_BASH=1
# >>> labproxy >>>
source $LABPROXY_SCRIPT_DIR/common.sh && source $LABPROXY_SCRIPT_DIR/proxyctl.sh && watch_proxy
# <<< labproxy <<<
export KEEP_PATH=$LABPROXY_SCRIPT_DIR
EOF

    _set_rc

    assert_equals 1 "$(count_in_file "$SHELL_RC_BASH" "# >>> labproxy >>>")" "managed block begin marker count"
    assert_equals 1 "$(count_in_file "$SHELL_RC_BASH" "source $LABPROXY_SCRIPT_DIR/common.sh && source $LABPROXY_SCRIPT_DIR/proxyctl.sh && watch_proxy")" "managed line count"
    assert_file_contains "$SHELL_RC_BASH" "export KEEP_PATH=$LABPROXY_SCRIPT_DIR"
}

run_test "_set_rc adds managed block once" test_set_rc_adds_managed_block_once
run_test "_set_rc unset removes managed block and keeps other content" test_set_rc_unset_removes_managed_block_and_keeps_other_content
run_test "_set_rc rewrites existing managed block without duplication" test_set_rc_rewrites_existing_managed_block_without_duplication

test_set_rc_adds_fish_managed_block() {
    _set_rc

    assert_equals 1 "$(count_in_file "$SHELL_RC_FISH" "# >>> labproxy >>>")" "fish begin marker count"
    assert_equals 1 "$(count_in_file "$SHELL_RC_FISH" "# <<< labproxy <<<")" "fish end marker count"
    assert_equals 1 "$(count_in_file "$SHELL_RC_FISH" "set -gx LABPROXY_HOME $HOME/.labproxy")" "fish LABPROXY_HOME line count"
    assert_equals 1 "$(count_in_file "$SHELL_RC_FISH" "set -gx PATH $HOME/.labproxy/bin \$PATH")" "fish PATH line count"
    assert_file_contains "$SHELL_RC_FISH" "set -gx KEEP_FISH 1"
}

test_set_rc_unset_removes_fish_managed_block() {
    _set_rc
    _set_rc unset

    assert_file_not_contains "$SHELL_RC_FISH" "# >>> labproxy >>>"
    assert_file_not_contains "$SHELL_RC_FISH" "set -gx LABPROXY_HOME"
    assert_file_not_contains "$SHELL_RC_FISH" "set -gx PATH $HOME/.labproxy/bin \$PATH"
    assert_file_contains "$SHELL_RC_FISH" "set -gx KEEP_FISH 1"
}

test_set_rc_fish_no_bash_source_line() {
    _set_rc

    # Fish rc should NOT contain bash-style source commands
    assert_file_not_contains "$SHELL_RC_FISH" "source $LABPROXY_SCRIPT_DIR/common.sh"
    assert_file_not_contains "$SHELL_RC_FISH" "watch_proxy"
}

test_set_rc_bash_no_fish_set_line() {
    _set_rc

    # Bash rc should NOT contain fish-style set commands
    assert_file_not_contains "$SHELL_RC_BASH" "set -gx PATH"
}

run_test "_set_rc adds fish managed block" test_set_rc_adds_fish_managed_block
run_test "_set_rc unset removes fish managed block" test_set_rc_unset_removes_fish_managed_block
run_test "_set_rc fish has no bash source line" test_set_rc_fish_no_bash_source_line
run_test "_set_rc bash has no fish set line" test_set_rc_bash_no_fish_set_line
