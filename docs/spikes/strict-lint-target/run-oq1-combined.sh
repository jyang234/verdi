#!/usr/bin/env bash
# Fix-round-1 evidence for R-2: ONE combined invocation of all seven
# candidate linters over ./... (the shape `make lint-strict` will actually
# run, unlike oq-1's original per-linter --enable-only loop), timed in the
# cold-cache shape a fresh CI runner would see: isolated, empty GOCACHE and
# GOLANGCI_LINT_CACHE directories, created fresh by this script and left in
# place afterward for a follow-up warm re-run (not cleaned up automatically
# — rerun this script with a fresh $WORKDIR, or point GOCACHE/
# GOLANGCI_LINT_CACHE at the same dirs again for a warm measurement).
#
# Re-runnable: bash docs/spikes/strict-lint-target/run-oq1-combined.sh
#
# Writes:
#   docs/spikes/strict-lint-target/oq1-combined-cold.txt   (this run's output + timing)
# and prints uptime before/after to the same file.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$ROOT" || exit 2

CONFIG="$ROOT/docs/spikes/strict-lint-target/golangci.strict.yml"
OUT="$ROOT/docs/spikes/strict-lint-target/oq1-combined-cold.txt"
WORKDIR="$(mktemp -d /tmp/oq1-combined-cache.XXXXXX)"

{
  echo "# oq-1 combined run, cold cache — GOCACHE=$WORKDIR/gocache GOLANGCI_LINT_CACHE=$WORKDIR/golangci-cache"
  echo "# both created fresh and empty immediately before this run"
  echo "load before: $(uptime)"
  echo "core count: $(sysctl -n hw.ncpu 2>/dev/null || nproc)"

  mkdir -p "$WORKDIR/gocache" "$WORKDIR/golangci-cache"
  DETAIL="$WORKDIR/full-output.txt"
  start=$(date +%s.%N)
  GOCACHE="$WORKDIR/gocache" GOLANGCI_LINT_CACHE="$WORKDIR/golangci-cache" \
    golangci-lint run --config "$CONFIG" --max-issues-per-linter=0 --max-same-issues=0 ./... > "$DETAIL" 2>&1
  rc=$?
  end=$(date +%s.%N)

  # Per-finding lines are identical in substance to oq1-raw/<linter>.txt;
  # only the trailing --show-stats summary is kept here to respect the
  # ~2000-line evidence cap. Full per-invocation output stays at $DETAIL
  # (not under the fence — it is exactly the union of oq1-raw/*.txt).
  tail -n 9 "$DETAIL"
  echo "full per-finding output (not committed, reproducible): $DETAIL"
  echo "exit=$rc"
  echo "elapsed seconds: $(echo "$end - $start" | bc)"
  echo "load after: $(uptime)"
  echo "cache dir used (not cleaned up — rerun against the same \$WORKDIR for a warm measurement): $WORKDIR"
} > "$OUT" 2>&1
cat "$OUT"
