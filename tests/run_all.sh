#!/usr/bin/env bash
# Cross-language test runner for wgrok
# Reports pass/fail per feature area across all four languages.
set -uo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

pass_or_fail() {
  if [ "$1" -eq 0 ]; then echo "PASS"; else echo "FAIL"; fi
}

echo "Running all tests..."
echo ""

# Python
cd "$ROOT"
py_protocol=$(python -m pytest python/tests/test_protocol.py -q --no-header 2>&1; echo $?)
py_allowlist=$(python -m pytest python/tests/test_allowlist.py -q --no-header 2>&1; echo $?)
py_config=$(python -m pytest python/tests/test_config.py -q --no-header 2>&1; echo $?)
py_logging=$(python -m pytest python/tests/test_logging.py -q --no-header 2>&1; echo $?)
py_webex=$(python -m pytest python/tests/test_webex.py -q --no-header 2>&1; echo $?)
py_sender=$(python -m pytest python/tests/test_sender.py -q --no-header 2>&1; echo $?)
py_routerbot=$(python -m pytest python/tests/test_router_bot.py -q --no-header 2>&1; echo $?)
py_receiver=$(python -m pytest python/tests/test_receiver.py -q --no-header 2>&1; echo $?)

# Go
cd "$ROOT/go"
go_protocol=$(go test -run 'TestEcho|TestFormat|TestParse|TestIs|TestResponse' -count=1 2>/dev/null; echo $?)
go_allowlist=$(go test -run TestAllowlist -count=1 2>/dev/null; echo $?)
go_config=$(go test -run 'Config|Debug' -count=1 2>/dev/null; echo $?)
go_logging=$(go test -run 'Logger|GetLogger' -count=1 2>/dev/null; echo $?)
go_webex=$(go test -run 'TestExtract|TestSendMessage|TestSendCard|TestGetMessage|TestGetAttachment|TestRetryAfter' -count=1 2>/dev/null; echo $?)
go_sender=$(go test -run TestWgrokSender -count=1 2>/dev/null; echo $?)
go_routerbot=$(go test -run TestWgrokRouterBot -count=1 2>/dev/null; echo $?)
go_receiver=$(go test -run TestWgrokReceiver -count=1 2>/dev/null; echo $?)

# TypeScript
cd "$ROOT/ts"
ts_cmd="node --experimental-vm-modules ./node_modules/jest/bin/jest.js --config jest.config.cjs --silent"
ts_protocol=$($ts_cmd --testPathPattern 'protocol\.test' 2>/dev/null; echo $?)
ts_allowlist=$($ts_cmd --testPathPattern 'allowlist\.test' 2>/dev/null; echo $?)
ts_config=$($ts_cmd --testPathPattern 'config\.test' 2>/dev/null; echo $?)
ts_logging=$($ts_cmd --testPathPattern 'logging\.test' 2>/dev/null; echo $?)
ts_webex=$($ts_cmd --testPathPattern 'webex' 2>/dev/null; echo $?)
ts_sender=$($ts_cmd --testPathPattern 'sender\.test' 2>/dev/null; echo $?)
ts_routerbot=$($ts_cmd --testPathPattern 'router-bot\.test' 2>/dev/null; echo $?)
ts_receiver=$($ts_cmd --testPathPattern 'receiver\.test' 2>/dev/null; echo $?)

# Rust
cd "$ROOT/rust"
rs_protocol=$(cargo test --test protocol_test 2>/dev/null; echo $?)
rs_allowlist=$(cargo test --test allowlist_test 2>/dev/null; echo $?)
rs_config=$(cargo test --test config_test 2>/dev/null; echo $?)
rs_logging=$(cargo test --test logging_test 2>/dev/null; echo $?)
rs_webex=$(cargo test --test webex_test --test webex_http_test 2>/dev/null; echo $?)
rs_sender=$(cargo test --test sender_test 2>/dev/null; echo $?)
rs_routerbot=$(cargo test --test router_bot_test 2>/dev/null; echo $?)
rs_receiver=$(cargo test --test receiver_test 2>/dev/null; echo $?)

# Extract last line (exit code) from each
get_rc() { echo "$1" | tail -1; }

echo "========================================="
echo "  wgrok Feature Parity Test Report"
echo "========================================="
echo ""
printf "%-12s | %-6s | %-6s | %-6s | %-6s\n" "Feature" "Python" "Go" "TS" "Rust"
printf "%-12s-|--------|--------|--------|--------\n" "------------"

features=(protocol allowlist config logging webex sender routerbot receiver)
for f in "${features[@]}"; do
  eval py_rc="\$(get_rc \"\$py_$f\")"
  eval go_rc="\$(get_rc \"\$go_$f\")"
  eval ts_rc="\$(get_rc \"\$ts_$f\")"
  eval rs_rc="\$(get_rc \"\$rs_$f\")"
  printf "%-12s | %-6s | %-6s | %-6s | %-6s\n" \
    "$f" "$(pass_or_fail "$py_rc")" "$(pass_or_fail "$go_rc")" "$(pass_or_fail "$ts_rc")" "$(pass_or_fail "$rs_rc")"
done

echo ""

# Count totals
total=0; passing=0
for f in "${features[@]}"; do
  for lang in py go ts rs; do
    eval rc="\$(get_rc \"\$${lang}_$f\")"
    ((total++))
    [ "$rc" -eq 0 ] && ((passing++))
  done
done

echo "$passing/$total feature checks passing."
if [ "$passing" -eq "$total" ]; then
  echo "Full parity achieved across all languages."
fi
