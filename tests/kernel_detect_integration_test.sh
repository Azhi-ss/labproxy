#!/usr/bin/env bash
# End-to-end integration checks for kernel detection fallback helpers.
set -uo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
TEST_TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TEST_TMPDIR"' EXIT

# common.sh reads variables that may be unset while sourced.
set +u
# shellcheck disable=SC1091
. "$ROOT_DIR/scripts/common.sh"
set -u

FAIL=0

fail() {
    printf 'FAIL: %s\n' "$1" >&2
    FAIL=1
}

assert_eq() {
    local name="$1" got="$2" want="$3"
    if [ "$got" != "$want" ]; then
        fail "$name: got '$got', want '$want'"
    fi
}

make_zip_case() {
    local case_dir="$1" files_csv="$2"
    local old_ifs="$IFS" file

    mkdir -p "$case_dir"
    IFS=,
    for file in $files_csv; do
        [ -n "$file" ] || continue
        mkdir -p "$(dirname "$case_dir/$file")"
        : > "$case_dir/$file"
    done
    IFS="$old_ifs"
}

TestDetectKernelZipTable() {
    local table case_name os arch files expect_base case_dir out out_base

    table='prefers-mihomo|darwin|arm64|clash-darwin-arm64.gz,mihomo-darwin-arm64-v1.19.2.gz|mihomo-darwin-arm64-v1.19.2.gz
matches-exact-os-arch|linux|amd64|mihomo-darwin-arm64-v1.19.2.gz,mihomo-linux-amd64-v1.19.2.gz|mihomo-linux-amd64-v1.19.2.gz
falls-back-to-clash|linux|arm64|clash-linux-arm64.gz,mihomo-linux-amd64-v1.19.2.gz|clash-linux-arm64.gz
does-not-cross-arch|linux|arm64|mihomo-linux-amd64-v1.19.2.gz|'

    while IFS='|' read -r case_name os arch files expect_base; do
        [ -n "$case_name" ] || continue
        case_dir="$TEST_TMPDIR/zip-$case_name"
        make_zip_case "$case_dir" "$files"

        out=$(_detect_kernel_zip "$case_dir" "$os" "$arch")
        out_base=""
        [ -n "$out" ] && out_base=$(basename "$out")
        assert_eq "_detect_kernel_zip $case_name" "$out_base" "$expect_base"
    done <<EOF
$table
EOF
}

TestFindSystemKernelFromPath() {
    local fakebin="$TEST_TMPDIR/fakebin"
    local fake_mihomo="$fakebin/mihomo"
    local out

    mkdir -p "$fakebin"
    printf '#!/usr/bin/env bash\nprintf "fake mihomo\\n"\n' > "$fake_mihomo"
    chmod +x "$fake_mihomo"

    out=$(PATH="$fakebin" _find_system_kernel)
    assert_eq "_find_system_kernel PATH mihomo" "$out" "$fake_mihomo"
}

TestFindSystemKernelNone() {
    local out
    out=$(PATH="$TEST_TMPDIR/empty-path" _find_system_kernel)
    assert_eq "_find_system_kernel empty PATH" "$out" ""
}

TestDetectKernelZipTable
TestFindSystemKernelFromPath
TestFindSystemKernelNone

if [ "$FAIL" -ne 0 ]; then
    printf 'FAIL kernel detect integration checks\n' >&2
    exit 1
fi

printf 'PASS kernel detect integration checks\n'
