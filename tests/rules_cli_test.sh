#!/usr/bin/env bash
# tests/rules_cli_test.sh
set -euo pipefail

LABPROXY_HOME="$(mktemp -d)"
export LABPROXY_HOME
mkdir -p "$LABPROXY_HOME/config" "$LABPROXY_HOME/bin"

go build -o "$LABPROXY_HOME/bin/labproxy-tui" ./cmd/labproxy-tui

MIXIN="$LABPROXY_HOME/config/mixin.yaml"
cat > "$MIXIN" <<'EOF'
mode: rule
rules:
  - DOMAIN,api64.ipify.org,DIRECT
EOF

BIN="$LABPROXY_HOME/bin/labproxy-tui"

# list
out=$("$BIN" rules --mixin-config "$MIXIN" list)
echo "$out" | grep -q "api64.ipify.org" || { echo "list failed: $out"; exit 1; }

# add
"$BIN" rules --mixin-config "$MIXIN" add --type=DOMAIN-SUFFIX --payload=example.com --proxy=PROXY
grep -q "DOMAIN-SUFFIX,example.com,PROXY" "$MIXIN" || { echo "add failed"; exit 1; }

# disable
"$BIN" rules --mixin-config "$MIXIN" disable 1
grep -q "^  # - DOMAIN-SUFFIX,example.com" "$MIXIN" || { echo "disable failed"; exit 1; }

# enable
"$BIN" rules --mixin-config "$MIXIN" enable 1
grep -q "^  - DOMAIN-SUFFIX,example.com" "$MIXIN" || { echo "enable failed"; exit 1; }

# import preset
"$BIN" rules --mixin-config "$MIXIN" import --source=preset:private
grep -q "IP-CIDR,10.0.0.0/8" "$MIXIN" || { echo "import preset failed"; exit 1; }

# reset
"$BIN" rules --mixin-config "$MIXIN" reset -y
grep -q "^rules: \[\]" "$MIXIN" || { echo "reset failed"; cat "$MIXIN"; exit 1; }

# backup exists
ls "$MIXIN".bak.* >/dev/null 2>&1 || { echo "no backup file"; exit 1; }

echo "OK: all CLI tests passed"
