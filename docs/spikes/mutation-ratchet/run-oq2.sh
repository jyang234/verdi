#!/usr/bin/env bash
# run-oq2.sh — spike/mutation-ratchet oq-2: touched-files mutation yield
# over the wave-1 diff of spec/readiness-recovery (b810c302..e963f4d0).
#
# Usage: run-oq2.sh <scratch-worktree-path> [<gremlins-bin>]
#   <scratch-worktree-path>  the scratch worktree, detached at e963f4d0,
#                            containing b810c302..e963f4d0 (never committed
#                            to; must be `git status --porcelain` clean and
#                            detached at e963f4d0 both before and after —
#                            this script restores it with
#                            `git checkout -- . && git clean -fd` before
#                            exiting either way).
#   <gremlins-bin>           path to a gremlins binary pinned per oq-1
#                            (default: `gremlins` on PATH).
#
# gremlins mutates a whole package at a time (it has no single-file
# target), so this script runs gremlins once per touched package and then
# filters that package's JSON report down to just the files the wave-1
# diff touched, before tallying — dc-2's "never mutates files the change
# did not touch" is enforced by this filter, not by gremlins' own --diff
# flag (this spike could not get --diff to select anything but SKIPPED for
# every mutant, including touched ones; see the README's oq-2 section).
#
# Prints a per-package table (package, mutants, killed, survived,
# not-covered, timed-out, seconds) and a total, then the wall clock for the
# whole sweep. Reduces gremlins' own JSON --output with python3's json
# module — nothing here parses the human-readable progress stream.
#
# cmd/verdi is skipped by default (SKIP_CMD_VERDI=0 to attempt it anyway):
# this spike measured cmd/verdi's own coverage-gathering baseline pass at
# over 600s (Go's default per-package test timeout) under shared load and
# recorded it NOT ANSWERED rather than spend the same cost again here.
set -euo pipefail

WORKTREE="${1:?usage: run-oq2.sh <scratch-worktree-path> [<gremlins-bin>]}"
GREMLINS="${2:-gremlins}"
BASE_SHA="b810c302"
HEAD_SHA="e963f4d0"
SKIP_CMD_VERDI="${SKIP_CMD_VERDI:-1}"

cd "$WORKTREE"

restore() {
	git checkout -- . >/dev/null 2>&1 || true
	git clean -fd >/dev/null 2>&1 || true
}
trap restore EXIT

if [ -n "$(git status --porcelain)" ]; then
	echo "run-oq2.sh: worktree is not clean; refusing to start" >&2
	exit 2
fi

TMP_DIR="$(mktemp -d)"
ROWS_FILE="$TMP_DIR/rows.tsv"
: >"$ROWS_FILE"

# Touched non-test Go files of the wave-1 diff, one per line.
TOUCHED_FILES_FILE="$TMP_DIR/touched.txt"
git diff --name-only "$BASE_SHA" "$HEAD_SHA" -- '*.go' | grep -v _test.go | sort >"$TOUCHED_FILES_FILE"

# Group touched files by package directory (dc-2's own scoping rule,
# applied by us since gremlins takes a package path, not a file list).
PACKAGES_FILE="$TMP_DIR/packages.txt"
while IFS= read -r f; do dirname "$f"; done <"$TOUCHED_FILES_FILE" | sort -u >"$PACKAGES_FILE"

SWEEP_START=$(date +%s)

while IFS= read -r pkg; do
	if [ "$SKIP_CMD_VERDI" = "1" ] && [ "$pkg" = "cmd/verdi" ]; then
		echo "$pkg	SKIPPED (see README oq-2: >600s coverage-gather timeout witness)	-	-	-	-	-	-" >>"$ROWS_FILE"
		continue
	fi

	# Exact package match, not prefix: a naive "starts with $pkg/" match
	# would also catch a touched file in a SUBPACKAGE (e.g. pkg
	# "internal/readinesspilot" would wrongly swallow
	# "internal/readinesspilot/readinesstest/readinesstest.go", which
	# belongs to the distinct package internal/readinesspilot/readinesstest).
	files_for_pkg=""
	while IFS= read -r f; do
		if [ "$(dirname "$f")" = "$pkg" ]; then
			files_for_pkg="$files_for_pkg $f"
		fi
	done <"$TOUCHED_FILES_FILE"
	files_for_pkg="${files_for_pkg# }"
	json="$TMP_DIR/$(echo "$pkg" | tr '/' '_').json"
	pkg_start=$(date +%s)
	# --timeout-coefficient/--workers: this machine runs other lanes
	# concurrently (constraint 7); gremlins' default timeout is calibrated
	# against an uncontended baseline test run and produced spurious
	# TIMED OUT statuses under shared load in this spike unless both are
	# widened — see the README's oq-2 load-discipline note.
	"$GREMLINS" unleash --timeout-coefficient "${GREMLINS_TIMEOUT_COEFFICIENT:-10}" --workers "${GREMLINS_WORKERS:-2}" \
		--output "$json" "./$pkg" >/dev/null 2>&1 || true
	pkg_end=$(date +%s)
	pkg_seconds=$((pkg_end - pkg_start))

	python3 - "$json" "$pkg" "$pkg_seconds" "$files_for_pkg" >>"$ROWS_FILE" <<'PYEOF'
import json, sys

path, pkg, seconds, files_str = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
touched_names = {f.split("/")[-1] for f in files_str.split()}

try:
    with open(path) as fh:
        data = json.load(fh)
except FileNotFoundError:
    print(f"{pkg}\t{files_str.strip()}\tERROR\tERROR\tERROR\tERROR\tERROR\t{seconds}")
    sys.exit(0)

killed = survived = notcov = timedout = mutants = 0
for entry in data.get("files", []):
    if entry["file_name"] not in touched_names:
        continue
    for m in entry.get("mutations", []):
        status = m["status"]
        if status == "NOT COVERED":
            notcov += 1
        elif status == "TIMED OUT":
            timedout += 1
        elif status == "KILLED":
            killed += 1
            mutants += 1
        elif status == "LIVED":
            survived += 1
            mutants += 1

print(f"{pkg}\t{files_str.strip()}\t{mutants}\t{killed}\t{survived}\t{notcov}\t{timedout}\t{seconds}")
PYEOF
done <"$PACKAGES_FILE"

SWEEP_END=$(date +%s)

echo "package	touched_files	mutants	killed	survived	not_covered	timed_out	seconds"
cat "$ROWS_FILE"

awk -F'\t' '
	$3 ~ /^[0-9]+$/ { m+=$3; k+=$4; s+=$5; nc+=$6; to+=$7 }
	{ secsum += $8 }
	END { printf "TOTAL\t-\t%d\t%d\t%d\t%d\t%d\t%d\n", m, k, s, nc, to, secsum }
' "$ROWS_FILE"

echo "wall clock for whole sweep (seconds): $((SWEEP_END - SWEEP_START))"
