#!/usr/bin/env bash
# coverage-report.sh — Count test coverage across xrpl-hive simulators.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# Count .io tests
io_tests=$(find "$ROOT/simulators/rpccompat/tests" -name '*.io' 2>/dev/null | wc -l | tr -d ' ')

# Count unique RPC methods in .io tests
io_methods=$(grep -rh '"method"' "$ROOT/simulators/rpccompat/tests/" 2>/dev/null \
  | sed 's/.*"method":"\([^"]*\)".*/\1/' | sort -u | wc -l | tr -d ' ')

# Count stateful tests (TestSpec registrations in rpccompat-stateful)
stateful_tests=$(grep -c 'suite.Add(' "$ROOT/simulators/rpccompat-stateful/main.go" 2>/dev/null || echo 0)

# Count txcompat tests
txcompat_tests=0
if [ -f "$ROOT/simulators/txcompat/main.go" ]; then
  txcompat_tests=$(grep -c 'suite.Add(' "$ROOT/simulators/txcompat/main.go" 2>/dev/null || echo 0)
fi

# Count wscompat tests
wscompat_tests=0
if [ -f "$ROOT/simulators/wscompat/main.go" ]; then
  wscompat_tests=$(grep -c 'suite.Add(' "$ROOT/simulators/wscompat/main.go" 2>/dev/null || echo 0)
fi

# Count other simulator tests
smoke_tests=$(grep -c 'suite.Add(' "$ROOT/simulators/smoke/main.go" 2>/dev/null || echo 0)
consensus_tests=$(grep -c 'suite.Add(' "$ROOT/simulators/consensus/main.go" 2>/dev/null || echo 0)
propagation_tests=$(grep -c 'suite.Add(' "$ROOT/simulators/propagation/main.go" 2>/dev/null || echo 0)
sync_tests=$(grep -c 'suite.Add(' "$ROOT/simulators/sync/main.go" 2>/dev/null || echo 0)
soak_tests=$(grep -c 'suite.Add(' "$ROOT/simulators/soak/main.go" 2>/dev/null || echo 0)

# Count error .io tests
error_io=$(find "$ROOT/simulators/rpccompat/tests/errors" -name '*.io' 2>/dev/null | wc -l | tr -d ' ')

total=$((io_tests + stateful_tests + txcompat_tests + wscompat_tests + smoke_tests + consensus_tests + propagation_tests + sync_tests + soak_tests))

echo "=========================================="
echo "  xrpl-hive Test Coverage Report"
echo "=========================================="
echo ""
echo "Simulator         | Tests"
echo "------------------|------"
echo "rpccompat (.io)   | $io_tests"
echo "  methods covered | $io_methods"
echo "  error tests     | $error_io"
echo "rpccompat-stateful| $stateful_tests"
echo "txcompat          | $txcompat_tests"
echo "wscompat          | $wscompat_tests"
echo "smoke             | $smoke_tests"
echo "consensus         | $consensus_tests"
echo "propagation       | $propagation_tests"
echo "sync              | $sync_tests"
echo "soak              | $soak_tests"
echo "------------------|------"
echo "TOTAL             | $total"
echo ""
echo "Categories:"
echo "  .io test dirs: $(ls -d "$ROOT/simulators/rpccompat/tests"/*/ 2>/dev/null | wc -l | tr -d ' ')"
echo "  Simulators:    $(ls -d "$ROOT/simulators"/*/ 2>/dev/null | wc -l | tr -d ' ')"
