#!/usr/bin/env bash
set -euo pipefail

LABPROXY_HOME="$(mktemp -d)"
export LABPROXY_HOME
mkdir -p "$LABPROXY_HOME/config" "$LABPROXY_HOME/bin"
FIXTURES="$LABPROXY_HOME/fixtures"
mkdir -p "$FIXTURES"

cat > "$FIXTURES/github.yaml" <<'YAML'
payload:
  - DOMAIN-SUFFIX,github.com
YAML

python3 - "$FIXTURES" "$LABPROXY_HOME/port" <<'PY' &
import http.server
import os
import socketserver
import sys

directory, port_file = sys.argv[1], sys.argv[2]
os.chdir(directory)
with socketserver.TCPServer(("127.0.0.1", 0), http.server.SimpleHTTPRequestHandler) as httpd:
    with open(port_file, "w", encoding="utf-8") as f:
        f.write(str(httpd.server_address[1]))
    httpd.serve_forever()
PY
server_pid=$!
trap 'kill "$server_pid" 2>/dev/null || true' EXIT
for _ in $(seq 1 50); do
  port="$(cat "$LABPROXY_HOME/port" 2>/dev/null || true)"
  if [ -n "$port" ]; then
    break
  fi
  sleep 0.1
done
if [ -z "${port:-}" ]; then
  echo "fixture server did not start"
  exit 1
fi
github_url="http://127.0.0.1:$port/github.yaml"

go build -o "$LABPROXY_HOME/bin/labproxy-tui" ./cmd/labproxy-tui

MIXIN="$LABPROXY_HOME/config/mixin.yaml"
cat > "$MIXIN" <<'YAML'
mode: rule
rules:
  - DOMAIN-SUFFIX,hf-mirror.com,DIRECT
YAML

BIN="$LABPROXY_HOME/bin/labproxy-tui"

"$BIN" rules --mixin-config "$MIXIN" workflow candidates | grep -q "github"
"$BIN" rules --mixin-config "$MIXIN" workflow inspect | grep -q "rules=1 providers=0"
"$BIN" rules --mixin-config "$MIXIN" workflow fetch --candidates=github --url-override=github="$github_url" | grep -q "github rules=1"
"$BIN" rules --mixin-config "$MIXIN" workflow validate --groups=Proxies --candidates=github --url-override=github="$github_url" | grep -q "github rules=1"
"$BIN" rules --mixin-config "$MIXIN" workflow plan --candidates=github,openai | grep -q "RULE-SET,github,Proxies"

apply_out=$("$BIN" rules --mixin-config "$MIXIN" workflow apply --groups=Proxies --candidates=github --url-override=github="$github_url")
echo "$apply_out" | grep -q "backup="
grep -q "rule-providers:" "$MIXIN"
grep -q "RULE-SET,github,Proxies" "$MIXIN"
grep -q "DOMAIN-SUFFIX,hf-mirror.com,DIRECT" "$MIXIN"

backup="${apply_out#*backup=}"
"$BIN" rules --mixin-config "$MIXIN" workflow rollback --backup="$backup"
grep -q "DOMAIN-SUFFIX,hf-mirror.com,DIRECT" "$MIXIN"
if grep -q "RULE-SET,github,Proxies" "$MIXIN"; then
  echo "rollback did not remove github ruleset"
  exit 1
fi

echo "OK: rules workflow CLI"
