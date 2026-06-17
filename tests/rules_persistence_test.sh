#!/usr/bin/env bash
# tests/rules_persistence_test.sh
set -euo pipefail

LABPROXY_HOME="$(mktemp -d)"
export LABPROXY_HOME
mkdir -p "$LABPROXY_HOME/config" "$LABPROXY_HOME/bin"

go build -o "$LABPROXY_HOME/bin/labproxy-tui" ./cmd/labproxy-tui

MIXIN="$LABPROXY_HOME/config/mixin.yaml"
cat > "$MIXIN" <<'EOF'
mode: rule
rules:
  - DOMAIN,a.com,DIRECT
EOF

BIN="$LABPROXY_HOME/bin/labproxy-tui"

for i in 1 2 3 4 5 6; do
    "$BIN" rules --mixin-config "$MIXIN" add --type=DOMAIN-SUFFIX --payload="r${i}.com" --proxy=PROXY >/dev/null
done

count=$(ls "$MIXIN".bak.* 2>/dev/null | wc -l | tr -d ' ')
if [ "$count" -gt 5 ]; then
    echo "backup rotation failed: $count backups (expected <=5)"
    exit 1
fi

rules_count=$("$BIN" rules --mixin-config "$MIXIN" list | tail -n +2 | wc -l | tr -d ' ')
if [ "$rules_count" -ne 7 ]; then
    echo "expected 7 rules, got $rules_count"
    exit 1
fi

# Validate the mixin is well-formed YAML. PyYAML is not available in this
# environment (PEP 668 blocks pip installs), so parse via the repo's own
# gopkg.in/yaml.v3 dependency instead.
validate_dir="$(mktemp -d)"
cat > "$validate_dir/main.go" <<'GOEOF'
package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func main() {
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var v any
	if err := yaml.Unmarshal(data, &v); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
GOEOF
go run -mod=mod "$validate_dir/main.go" "$MIXIN" >/dev/null 2>&1 || { echo "YAML invalid"; exit 1; }
rm -rf "$validate_dir"

echo "OK: persistence test passed"
