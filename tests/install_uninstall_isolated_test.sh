#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
REAL_BASH="$(command -v bash)"
ORIGINAL_PATH=$PATH
HAS_ZSH=false
if command -v zsh >/dev/null 2>&1; then
    HAS_ZSH=true
fi

if [ -z "${TMPDIR:-}" ] || [ ! -d "$TMPDIR" ] || [ ! -w "$TMPDIR" ]; then
    TMPDIR=/tmp
fi

TEST_TMPDIR="$(mktemp -d)"
TEST_HOME="$TEST_TMPDIR/home"
TEST_BIN="$TEST_TMPDIR/bin"
TEST_TMP="$TEST_TMPDIR/tmp"
CRONTAB_FILE="$TEST_TMPDIR/crontab"
INSTALL_LOG="$TEST_TMPDIR/install.log"
UNINSTALL_LOG="$TEST_TMPDIR/uninstall.log"

cleanup() {
    if [ -f "$TEST_HOME/.labproxy/config/labproxy.pid" ]; then
        local pid
        pid=$(cat "$TEST_HOME/.labproxy/config/labproxy.pid" 2>/dev/null || true)
        if [[ ${pid:-} =~ ^[0-9]+$ ]]; then
            kill "$pid" 2>/dev/null || true
            sleep 0.2 2>/dev/null || true
            kill -9 "$pid" 2>/dev/null || true
        fi
    fi

    if [ "${KEEP_TEST_TMPDIR:-false}" = "true" ]; then
        printf 'kept test tmpdir: %s\n' "$TEST_TMPDIR" >&2
        return 0
    fi

    rm -rf "$TEST_TMPDIR"
}
trap cleanup EXIT

mkdir -p "$TEST_HOME" "$TEST_BIN" "$TEST_TMP"

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

assert_equals() {
    local expected=$1
    local actual=$2
    local label=$3
    if [ "$expected" != "$actual" ]; then
        printf 'assertion failed: %s (expected=%s actual=%s)\n' "$label" "$expected" "$actual" >&2
        return 1
    fi
}

count_in_file() {
    local file=$1
    local needle=$2
    grep -F -c -- "$needle" "$file" 2>/dev/null || true
}

create_wrappers() {
    cat > "$TEST_BIN/gzip" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
cat <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail

validate=false
home_dir=""
config_file=""

while [ "$#" -gt 0 ]; do
    case "$1" in
    -t)
        validate=true
        ;;
    -d)
        shift
        home_dir=${1-}
        ;;
    -f)
        shift
        config_file=${1-}
        ;;
    esac
    shift || true
done

if [ "$validate" = true ]; then
    exit 0
fi

if [ -n "$home_dir" ]; then
    mkdir -p "$home_dir/logs"
    cat > "$home_dir/logs/labproxy.log" <<'LOG'
HTTP proxy listening at: 127.0.0.1:7893
RESTful API listening at: 127.0.0.1:9090
DNS server(UDP) listening at: [::]:15353
LOG
fi

trap 'exit 0' TERM INT
while :; do
    sleep 1
done
SCRIPT
EOF
    chmod +x "$TEST_BIN/gzip"

    cat > "$TEST_BIN/tar" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

archive=""
dest="$(pwd)"

while [ "$#" -gt 0 ]; do
    case "$1" in
    -C)
        shift
        dest=$1
        ;;
    -f|-xf|-xzf|-xzf|-xvf|-x)
        ;;
    -*f*)
        ;;
    *.tar|*.tar.gz|*.tgz)
        archive=$1
        ;;
    esac
    shift || true
done

[ -n "$archive" ] || {
    printf 'fake tar: archive not found in args\n' >&2
    exit 1
}

mkdir -p "$dest"
base=$(basename "$archive")
name=${base%.tar.gz}
name=${name%.tgz}
name=${name%.tar}

case "$base" in
subconverter_linux64.tar.gz)
    mkdir -p "$dest/subconverter"
    cat > "$dest/subconverter/subconverter" <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail
trap 'exit 0' TERM INT
while :; do
    sleep 1
done
SCRIPT
    chmod +x "$dest/subconverter/subconverter"
    cat > "$dest/subconverter/pref.example.yml" <<'SCRIPT'
server.port: 25500
SCRIPT
    ;;
yq_linux_amd64.tar.gz)
    cat > "$dest/yq_linux_amd64" <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail

set_key() {
    local file=$1
    local key=$2
    local value=$3

    mkdir -p "$(dirname "$file")"
    touch "$file"

    if grep -Fq -- "$key:" "$file"; then
        python3 - "$file" "$key" "$value" <<'PY'
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
key = sys.argv[2]
value = sys.argv[3]
lines = path.read_text().splitlines()
updated = False
for idx, line in enumerate(lines):
    if line.startswith(f"{key}:"):
        lines[idx] = f"{key}: {value}"
        updated = True
        break
if not updated:
    lines.append(f"{key}: {value}")
path.write_text("\n".join(lines) + "\n")
PY
    else
        printf '%s: %s\n' "$key" "$value" >> "$file"
    fi
}

get_key() {
    local file=$1
    local key=$2
    local default=${3-}
    if [ -f "$file" ]; then
        local line
        line=$(grep -F -- "$key:" "$file" | tail -n 1 || true)
        if [ -n "$line" ]; then
            printf '%s\n' "${line#*: }"
            return 0
        fi
    fi
    printf '%s\n' "$default"
}

case "${1-}" in
eval-all)
    cat <<'YAML'
mixed-port: 7893
external-controller: 0.0.0.0:9090
dns.listen: 0.0.0.0:15353
system-proxy.enable: true
YAML
    ;;
-i)
    expr=$2
    file=$3
    case "$expr" in
    '.system-proxy.enable = true')
        set_key "$file" 'system-proxy.enable' 'true'
        ;;
    '.system-proxy.enable = false')
        set_key "$file" 'system-proxy.enable' 'false'
        ;;
    .mixed-port\ =\ *)
        set_key "$file" 'mixed-port' "${expr#'.mixed-port = '}"
        ;;
    '.external-controller = "'*'"')
        value=${expr#'.external-controller = "'}
        value=${value%'"'}
        set_key "$file" 'external-controller' "$value"
        ;;
    '.dns.listen = "'*'"')
        value=${expr#'.dns.listen = "'}
        value=${value%'"'}
        set_key "$file" 'dns.listen' "$value"
        ;;
    '.server.port = '*)
        set_key "$file" 'server.port' "${expr#'.server.port = '}"
        ;;
    *)
        printf 'unsupported fake-yq expression: %s\n' "$expr" >&2
        exit 1
        ;;
    esac
    ;;
'.authentication[0] // ""')
    printf '\n'
    ;;
'.mixed-port // ""')
    get_key "$2" 'mixed-port' ''
    ;;
'.mixed-port // 7890')
    get_key "$2" 'mixed-port' '7890'
    ;;
'.external-controller // ""')
    get_key "$2" 'external-controller' ''
    ;;
'.external-controller // "127.0.0.1:9090"')
    get_key "$2" 'external-controller' '127.0.0.1:9090'
    ;;
'.dns.listen // ""')
    get_key "$2" 'dns.listen' ''
    ;;
'.dns.listen // "0.0.0.0:15353"')
    get_key "$2" 'dns.listen' '0.0.0.0:15353'
    ;;
'.system-proxy.enable // true')
    get_key "$2" 'system-proxy.enable' 'true'
    ;;
*)
    printf 'unsupported fake-yq invocation: %s\n' "$*" >&2
    exit 1
    ;;
esac
SCRIPT
    chmod +x "$dest/yq_linux_amd64"
    ;;
clash-tui-*.tar.gz)
    cat > "$dest/$name" <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail
if [ "${1-}" = '-h' ]; then
    printf 'Usage: fake-tui --endpoint URL --restart-command CMD\n'
    printf '  -restart-command string\n'
    exit 0
fi
printf 'fake labproxy tui\n'
SCRIPT
    chmod +x "$dest/$name"
    ;;
*)
    printf 'fake tar: unsupported archive %s\n' "$base" >&2
    exit 1
    ;;
esac
EOF
    chmod +x "$TEST_BIN/tar"

    cat > "$TEST_BIN/unzip" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

dest="$(pwd)"
while [ "$#" -gt 0 ]; do
    case "$1" in
    -d)
        shift
        dest=$1
        ;;
    esac
    shift || true
done

mkdir -p "$dest/dist"
printf '<html>fake ui</html>\n' > "$dest/dist/index.html"
EOF
    chmod +x "$TEST_BIN/unzip"

    cat > "$TEST_BIN/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '203.0.113.10\n'
EOF
    chmod +x "$TEST_BIN/curl"

    cat > "$TEST_BIN/crontab" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
store=${FAKE_CRONTAB_FILE:?}
case "${1-}" in
-l)
    if [ -f "$store" ]; then
        cat "$store"
        exit 0
    fi
    exit 1
    ;;
-)
    cat > "$store"
    ;;
*)
    printf 'fake crontab: unsupported args %s\n' "$*" >&2
    exit 1
    ;;
esac
EOF
    chmod +x "$TEST_BIN/crontab"
}

expected_rc_files() {
    printf '%s\n' "$TEST_HOME/.bashrc"
    if [ "$HAS_ZSH" = true ]; then
        printf '%s\n' "$TEST_HOME/.zshrc"
    fi
}

prepare_home() {
    cat > "$TEST_HOME/.bashrc" <<'EOF'
export KEEP_BASH=1
EOF

    if [ "$HAS_ZSH" = true ]; then
        cat > "$TEST_HOME/.zshrc" <<'EOF'
export KEEP_ZSH=1
EOF
    fi

    cat > "$CRONTAB_FILE" <<'EOF'
15 1 * * * echo keep-me
0 0 */2 * * bash -i -c 'labproxyctl update' # labproxyctl_auto_update
EOF
}

run_install() {
    (
        cd "$ROOT_DIR"
        HOME="$TEST_HOME" \
        USER='labproxy-test' \
        TMPDIR="$TEST_TMP" \
        FAKE_CRONTAB_FILE="$CRONTAB_FILE" \
        PATH="$TEST_BIN:$ORIGINAL_PATH" \
        "$REAL_BASH" ./install.sh
    ) > "$INSTALL_LOG" 2>&1
}

run_uninstall() {
    (
        cd "$ROOT_DIR"
        HOME="$TEST_HOME" \
        USER='labproxy-test' \
        TMPDIR="$TEST_TMP" \
        FAKE_CRONTAB_FILE="$CRONTAB_FILE" \
        PATH="$TEST_BIN:$ORIGINAL_PATH" \
        "$REAL_BASH" ./uninstall.sh
    ) > "$UNINSTALL_LOG" 2>&1
}

create_wrappers
prepare_home

baseline_git_status=$(git -C "$ROOT_DIR" status --porcelain=v1 --untracked-files=all)

run_install

labproxy_home="$TEST_HOME/.labproxy"
managed_line="source $labproxy_home/scripts/common.sh && source $labproxy_home/scripts/proxyctl.sh && watch_proxy"

assert_exists "$labproxy_home"
assert_exists "$labproxy_home/bin/mihomo"
assert_exists "$labproxy_home/bin/yq"
assert_exists "$labproxy_home/bin/labproxy-tui"
assert_exists "$labproxy_home/ui/index.html"
assert_exists "$labproxy_home/scripts/common.sh"
assert_exists "$labproxy_home/scripts/proxyctl.sh"
assert_exists "$labproxy_home/config/labproxy.pid"

while IFS= read -r rc_file; do
    assert_exists "$rc_file"
    assert_file_contains "$rc_file" '# >>> labproxy >>>'
    assert_file_contains "$rc_file" '# <<< labproxy <<<'
    assert_file_contains "$rc_file" "$managed_line"
    assert_equals 1 "$(count_in_file "$rc_file" '# >>> labproxy >>>')" "managed block begin count for $rc_file"
    printf 'export AFTER_INSTALL=1\n' >> "$rc_file"
done < <(expected_rc_files)

assert_file_contains "$TEST_HOME/.bashrc" 'export KEEP_BASH=1'
if [ "$HAS_ZSH" = true ]; then
    assert_file_contains "$TEST_HOME/.zshrc" 'export KEEP_ZSH=1'
fi

assert_equals "$baseline_git_status" "$(git -C "$ROOT_DIR" status --porcelain=v1 --untracked-files=all)" 'git status after install'

run_uninstall

assert_not_exists "$labproxy_home"

while IFS= read -r rc_file; do
    assert_exists "$rc_file"
    assert_file_not_contains "$rc_file" '# >>> labproxy >>>'
    assert_file_not_contains "$rc_file" '# <<< labproxy <<<'
    assert_file_not_contains "$rc_file" "$managed_line"
    assert_file_contains "$rc_file" 'export AFTER_INSTALL=1'
done < <(expected_rc_files)

assert_file_contains "$TEST_HOME/.bashrc" 'export KEEP_BASH=1'
if [ "$HAS_ZSH" = true ]; then
    assert_file_contains "$TEST_HOME/.zshrc" 'export KEEP_ZSH=1'
fi

assert_file_contains "$CRONTAB_FILE" '15 1 * * * echo keep-me'
assert_file_not_contains "$CRONTAB_FILE" 'labproxyctl_auto_update'

assert_equals "$baseline_git_status" "$(git -C "$ROOT_DIR" status --porcelain=v1 --untracked-files=all)" 'git status after uninstall'

printf 'PASS install/uninstall isolated regression\n'
