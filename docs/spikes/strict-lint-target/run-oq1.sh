#!/usr/bin/env bash
# oq-1 measurement: per-linter finding count and wall-clock runtime, one
# candidate linter at a time, over ./... at whatever commit this worktree's
# HEAD is on (main 5f60c76c at the time this spike ran).
#
# Re-runnable: `bash docs/spikes/strict-lint-target/run-oq1.sh` from the
# worktree root. Writes raw JSON+text per linter under
# docs/spikes/strict-lint-target/oq1-raw/<linter>.{json,txt} and a summary
# table to docs/spikes/strict-lint-target/oq1-summary.txt.
#
# Methodology note (disclosed, affects the numbers): golangci-lint's `run`
# defaults to --max-issues-per-linter=50 and --max-same-issues=3, which
# would silently truncate exactly the "hundreds of findings" case the
# parent spec flags for gochecknoglobals. Both caps are disabled below
# (`=0`) so the counts are true totals, not capped ones. --uniq-by-line
# stays at its default (true).
#
# Second methodology note: --output.json.path silently writes nothing (no
# error, exit code unaffected) when given a path relative to cwd — verified
# by hand against golangci-lint 2.5.0 (`--output.json.path=dupl.json` from
# the worktree root produces no file; the identical run with an absolute
# path does). Absolute paths are used throughout below for that reason.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$ROOT" || exit 2

CONFIG="docs/spikes/strict-lint-target/golangci.strict.yml"
OUTDIR="$ROOT/docs/spikes/strict-lint-target/oq1-raw"
SUMMARY="docs/spikes/strict-lint-target/oq1-summary.txt"
LINTERS=(containedctx noctx contextcheck errorlint exhaustive dupl gochecknoglobals)

mkdir -p "$OUTDIR"

{
  echo "# oq-1 summary — golangci-lint v2.5.0, --config $CONFIG, --enable-only <linter>, ./..."
  echo "# generated $(date -u +%Y-%m-%dT%H:%M:%SZ) at HEAD=$(git rev-parse HEAD)"
  printf '%-20s %10s %10s\n' linter findings seconds
} > "$SUMMARY"

for l in "${LINTERS[@]}"; do
  echo "== $l ==" >&2
  echo "load before ($l): $(uptime)" >> "$SUMMARY"
  start=$(date +%s.%N)
  golangci-lint run --config "$CONFIG" --enable-only "$l" \
    --max-issues-per-linter=0 --max-same-issues=0 \
    --output.json.path="$OUTDIR/$l.json" \
    ./... > "$OUTDIR/$l.txt" 2>&1
  rc=$?
  end=$(date +%s.%N)
  echo "exit=$rc" >> "$OUTDIR/$l.txt"
  echo "load after ($l): $(uptime)" >> "$SUMMARY"
  elapsed=$(echo "$end - $start" | bc)
  count=$(jq '.Issues | length' "$OUTDIR/$l.json" 2>/dev/null)
  printf '%-20s %10s %10.2f\n' "$l" "${count:-ERR}" "$elapsed" >> "$SUMMARY"
done

echo "core count (sysctl hw.ncpu / nproc): $(sysctl -n hw.ncpu 2>/dev/null || nproc)" >> "$SUMMARY"
cat "$SUMMARY"
