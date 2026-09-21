#!/usr/bin/env bash
# sg-battery.sh <repo-dir> <out-dir>
# Runs the oq-1 investigation battery (verdi lint, verdi model check,
# verdi journey --json for two feature specs + one component-class
# refusal probe, verdi spec doc for three specs) against <repo-dir> using
# the binary at /tmp/verdi-sg, writing one file per command plus its exit
# status into <out-dir>. Read-only: makes no commit.
set -u
BIN=/tmp/verdi-sg
REPO="$1"
OUT="$2"
mkdir -p "$OUT"

run() {
  local name="$1"; shift
  ( cd "$REPO" && "$BIN" "$@" ) >"$OUT/$name.stdout" 2>"$OUT/$name.stderr"
  echo "$?" >"$OUT/$name.exit"
  echo "== $name: exit $(cat "$OUT/$name.exit") ==" >>"$OUT/SUMMARY.txt"
}

: >"$OUT/SUMMARY.txt"
run lint lint
run model-check model check
run journey-readiness-recovery journey --json spec/readiness-recovery
run journey-spec-documents journey --json spec/spec-documents
run journey-component-refusal journey --json spec/verdi-store-layout
run specdoc-readiness-recovery spec doc spec/readiness-recovery
run specdoc-spec-documents spec doc spec/spec-documents
run specdoc-component spec doc spec/verdi-store-layout
