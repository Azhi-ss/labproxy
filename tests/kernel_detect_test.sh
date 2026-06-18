#!/usr/bin/env bash
# 内核安装检测机制的表驱动单元测试。
# 覆盖 _detect_os_arch / _detect_kernel_zip / _find_system_kernel 纯函数。
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

TestDetectOSArch() {
    local os arch
    read -r os arch <<< "$(_detect_os_arch)"
    [ -n "$os" ] || fail "_detect_osarch: os empty"
    [ -n "$arch" ] || fail "_detect_osarch: arch empty"
    case "$os" in
    linux|darwin) ;;
    *) fail "_detect_osarch: unexpected os=$os" ;;
    esac
    case "$arch" in
    amd64|arm64|386) ;;
    *) fail "_detect_osarch: unexpected arch=$arch" ;;
    esac
}

TestDetectKernelZip_MatchDarwinArm64() {
    local zipdir="$TEST_TMPDIR/zip1"
    mkdir -p "$zipdir"
    : > "$zipdir/mihomo-darwin-arm64-v1.19.2.gz"
    : > "$zipdir/mihomo-linux-amd64-v1.19.2.gz"

    local out
    out=$(_detect_kernel_zip "$zipdir" darwin arm64)
    [ -n "$out" ] || { fail "darwin/arm64: no zip matched"; return; }
    case "$out" in
    *mihomo-darwin-arm64*) ;;
    *) fail "darwin/arm64: matched wrong zip: $out" ;;
    esac
}

TestDetectKernelZip_MatchLinuxAmd64() {
    local zipdir="$TEST_TMPDIR/zip2"
    mkdir -p "$zipdir"
    : > "$zipdir/mihomo-linux-amd64-v1.19.2.gz"

    local out
    out=$(_detect_kernel_zip "$zipdir" linux amd64)
    [ -n "$out" ] || { fail "linux/amd64: no zip matched"; return; }
    case "$out" in
    *mihomo-linux-amd64*) ;;
    *) fail "linux/amd64: matched wrong zip: $out" ;;
    esac
}

TestDetectKernelZip_NoMatch() {
    local zipdir="$TEST_TMPDIR/zip3"
    mkdir -p "$zipdir"
    : > "$zipdir/mihomo-linux-amd64-v1.19.2.gz"

    local out
    out=$(_detect_kernel_zip "$zipdir" darwin arm64)
    [ -z "$out" ] || fail "darwin/arm64: should not match linux zip: $out"
}

TestDetectKernelZip_PrefersMihomoOverClash() {
    local zipdir="$TEST_TMPDIR/zip4"
    mkdir -p "$zipdir"
    : > "$zipdir/clash-darwin-arm64.gz"
    : > "$zipdir/mihomo-darwin-arm64-v1.19.2.gz"

    local out
    out=$(_detect_kernel_zip "$zipdir" darwin arm64)
    case "$out" in
    *mihomo*) ;;
    *) fail "should prefer mihomo over clash: $out" ;;
    esac
}

TestFindSystemKernel_None() {
    local out
    out=$(PATH=/nonexistent _find_system_kernel)
    [ -z "$out" ] || fail "empty PATH should yield no system kernel: $out"
}

TestFindSystemKernel_UsesWhich() {
    local fakedir="$TEST_TMPDIR/fakebin"
    mkdir -p "$fakedir"
    cat > "$fakedir/mihomo" <<'EOF'
#!/usr/bin/env bash
echo fake
EOF
    chmod +x "$fakedir/mihomo"
    local out
    out=$(PATH="$fakedir:/usr/bin:/bin" _find_system_kernel)
    [ -n "$out" ] || { fail "should detect mihomo in PATH"; return; }
    case "$out" in
    *mihomo) ;;
    *) fail "unexpected system kernel: $out" ;;
    esac
}

TestDetectOSArch
TestDetectKernelZip_MatchDarwinArm64
TestDetectKernelZip_MatchLinuxAmd64
TestDetectKernelZip_NoMatch
TestDetectKernelZip_PrefersMihomoOverClash
TestFindSystemKernel_None
TestFindSystemKernel_UsesWhich

if [ "$FAIL" -ne 0 ]; then
    printf 'FAIL kernel detect checks\n' >&2
    exit 1
fi
printf 'PASS kernel detect checks\n'
