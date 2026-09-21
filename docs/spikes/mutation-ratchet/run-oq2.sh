#!/usr/bin/env bash
# run-oq2.sh — spike/mutation-ratchet oq-2: touched-files mutation yield
# over the wave-1 diff of spec/readiness-recovery (b810c302..e963f4d0).
#
# Usage: run-oq2.sh <clone-path> [<gremlins-bin>]
#   <clone-path>   a THROWAWAY LOCAL CLONE of the scratch worktree,
#                  detached at e963f4d0:
#                    git clone -q --no-hardlinks <scratch-worktree> /tmp/fix-mr
#                    git -C /tmp/fix-mr checkout --detach e963f4d0
#                  Never point this at a worktree. Mutation tools mutate,
#                  and refs are shared across every worktree of the repo.
#                  The clone must be `git status --porcelain` clean to
#                  start; the script restores it on exit either way.
#   <gremlins-bin> path to a gremlins binary pinned per oq-1
#                  (default: `gremlins` on PATH).
#
# WHAT THIS SCRIPT PINS, AND WHY (fix round 1)
#
# 1. The operator set, explicitly, all eleven. gremlins runs only 5 of its
#    11 operators by default; a version string alone therefore does not
#    reproduce a baseline, which is what the feature's ac-1 (byte-identical
#    report) and ac-2 (ratchet) need. Override with OPERATORS=default to
#    reproduce the spike's first-version figures (the 5 default-on ones,
#    pinned explicitly so the run is still self-describing).
#
# 2. File scoping natively, with gremlins' own -E/--exclude-files. dc-2
#    says the gate must never MUTATE a file the change did not touch;
#    -E drops files before mutant generation (engine.go's WalkDir consults
#    exclusion.Rules before runOnFile), so that is a real constraint on
#    what runs, not a post-filter over a whole-package run. The regexps are
#    matched against the path relative to the target package directory, so
#    they are anchored bare file names. This script ASSERTS afterwards that
#    the report contains only touched files.
#
# 3. `uptime` before and after every package (constraint 7). Other lanes
#    run on this machine; a seconds figure without its load reading is not
#    a measurement.
#
# PACKAGES THIS SCRIPT REFUSES TO MEASURE, AND WHY
#
#   cmd/verdi, cmd/e2eharness — `package main`. gremlins v0.6.0 derives the
#   package under test from the Go package-clause identifier
#   (internal/engine/engine.go:160), so for any `package main` directory
#   not literally named "main" it runs `go test <module-path>`, which has
#   no package, fails `[setup failed]` in ~80ms, and is scored KILLED
#   (executor.go:257 maps exit 1 to KILLED). Every mutant in those packages
#   is falsely killed and the report reads 100.00% efficacy. Root cause,
#   trace and counterfactual: oq1-gremlins-mainpkg-defect.txt. Set
#   MEASURE_MAIN_PKGS=1 to run them anyway and see the false result for
#   yourself; the rows are labelled DEFECT either way.
#
# Prints: the pinned configuration; one row per package (package, touched
# files, mutants, killed, survived, not-covered, timed-out, seconds, load
# before/after); a total; and the wall clock for the whole sweep.
set -euo pipefail

CLONE="${1:?usage: run-oq2.sh <clone-path> [<gremlins-bin>]}"
GREMLINS="${2:-gremlins}"
BASE_SHA="b810c302"
HEAD_SHA="e963f4d0"
OPERATORS="${OPERATORS:-all}"
MEASURE_MAIN_PKGS="${MEASURE_MAIN_PKGS:-0}"
ONLY_PKGS="${ONLY_PKGS:-}"
COEFFICIENT="${GREMLINS_TIMEOUT_COEFFICIENT:-10}"
WORKERS="${GREMLINS_WORKERS:-2}"

# gremlins' eleven operators. The five marked (d) are its defaults.
OPS_ALL=(--arithmetic-base=true --conditionals-boundary=true
	--conditionals-negation=true --increment-decrement=true
	--invert-negatives=true --invert-assignments=true --invert-bitwise=true
	--invert-bwassign=true --invert-logical=true --invert-loopctrl=true
	--remove-self-assignments=true)
OPS_DEFAULT=(--arithmetic-base=true --conditionals-boundary=true
	--conditionals-negation=true --increment-decrement=true
	--invert-negatives=true --invert-assignments=false --invert-bitwise=false
	--invert-bwassign=false --invert-logical=false --invert-loopctrl=false
	--remove-self-assignments=false)
case "$OPERATORS" in
all) OPS=("${OPS_ALL[@]}") ;;
default) OPS=("${OPS_DEFAULT[@]}") ;;
*)
	echo "run-oq2.sh: OPERATORS must be 'all' or 'default'" >&2
	exit 2
	;;
esac

cd "$CLONE"

if [ -e "$CLONE/.git" ] && [ ! -d "$CLONE/.git" ]; then
	echo "run-oq2.sh: $CLONE looks like a worktree (.git is a file), not a clone; refusing" >&2
	exit 2
fi

restore() {
	git checkout -- . >/dev/null 2>&1 || true
	git clean -fd >/dev/null 2>&1 || true
}
trap restore EXIT

if [ -n "$(git status --porcelain)" ]; then
	echo "run-oq2.sh: clone is not clean; refusing to start" >&2
	exit 2
fi

TMP_DIR="$(mktemp -d)"
ROWS_FILE="$TMP_DIR/rows.tsv"
: >"$ROWS_FILE"

# Touched non-test Go files of the wave-1 diff, one per line.
TOUCHED_FILES_FILE="$TMP_DIR/touched.txt"
git diff --name-only "$BASE_SHA" "$HEAD_SHA" -- '*.go' | grep -v _test.go | sort >"$TOUCHED_FILES_FILE"

# Group touched files by package directory (dc-2's own scoping rule).
PACKAGES_FILE="$TMP_DIR/packages.txt"
while IFS= read -r f; do dirname "$f"; done <"$TOUCHED_FILES_FILE" | sort -u >"$PACKAGES_FILE"

echo "run-oq2.sh configuration (every figure below is produced under exactly this):"
echo "  clone:              $CLONE @ $(git rev-parse --short=8 HEAD)"
echo "  gremlins:           $GREMLINS -> $("$GREMLINS" --version 2>&1 | head -1)"
echo "  range:              $BASE_SHA..$HEAD_SHA"
echo "  touched non-test files: $(wc -l <"$TOUCHED_FILES_FILE" | tr -d ' ') across $(wc -l <"$PACKAGES_FILE" | tr -d ' ') package directories"
echo "  operator set:       $OPERATORS -> ${OPS[*]}"
echo "  timeout-coefficient: $COEFFICIENT   workers: $WORKERS"
echo "  cores:              $(sysctl -n hw.ncpu 2>/dev/null || nproc)"
echo "  main-pkg defect:    MEASURE_MAIN_PKGS=$MEASURE_MAIN_PKGS"
echo

SWEEP_START=$(date +%s)

while IFS= read -r pkg; do
	if [ -n "$ONLY_PKGS" ] && ! printf '%s\n' $ONLY_PKGS | grep -qxF "$pkg"; then
		continue
	fi

	# Exact package match, not prefix: a naive "starts with $pkg/" match
	# would also catch a touched file in a SUBPACKAGE (e.g. pkg
	# "internal/readinesspilot" would wrongly swallow
	# "internal/readinesspilot/readinesstest/readinesstest.go", which
	# belongs to the distinct package internal/readinesspilot/readinesstest).
	touched_for_pkg=()
	while IFS= read -r f; do
		if [ "$(dirname "$f")" = "$pkg" ]; then
			touched_for_pkg+=("$(basename "$f")")
		fi
	done <"$TOUCHED_FILES_FILE"
	files_for_pkg="${touched_for_pkg[*]}"

	# A `package main` directory whose basename is not "main" is scored
	# 100% killed by gremlins v0.6.0 without running one test. Refuse by
	# default rather than print a number that is a tool defect.
	clause=""
	for g in "$pkg"/*.go; do
		case "$g" in *_test.go) continue ;; esac
		clause="$(awk '/^package /{print $2; exit}' "$g")"
		break
	done
	if [ "$clause" = "main" ] && [ "$(basename "$pkg")" != "main" ] && [ "$MEASURE_MAIN_PKGS" != "1" ]; then
		printf '%s\t%s\tDEFECT\tDEFECT\tDEFECT\tDEFECT\tDEFECT\t-\t-\n' \
			"$pkg" "$files_for_pkg" >>"$ROWS_FILE"
		echo "SKIP $pkg: package main + basename $(basename "$pkg") -> gremlins v0.6.0 tests github.com/jyang234/verdi, scores every mutant KILLED (see oq1-gremlins-mainpkg-defect.txt)"
		continue
	fi

	# Native file scoping: exclude every non-test .go file in the package
	# that the diff did not touch. Regexps are matched against the path
	# relative to the package dir, so anchor bare basenames.
	exclude_args=()
	excluded=()
	for g in "$pkg"/*.go; do
		case "$g" in *_test.go) continue ;; esac
		b="$(basename "$g")"
		keep=0
		for t in "${touched_for_pkg[@]}"; do [ "$t" = "$b" ] && keep=1; done
		if [ "$keep" = "0" ]; then
			exclude_args+=(-E "^${b%.go}\.go\$")
			excluded+=("$b")
		fi
	done

	json="$TMP_DIR/$(echo "$pkg" | tr '/' '_').json"
	load_before="$(uptime | sed 's/.*load averages*: //')"
	pkg_start=$(date +%s)
	"$GREMLINS" unleash --timeout-coefficient "$COEFFICIENT" --workers "$WORKERS" \
		"${OPS[@]}" "${exclude_args[@]}" --output "$json" "./$pkg" >"$TMP_DIR/$(echo "$pkg" | tr '/' '_').log" 2>&1 || true
	pkg_end=$(date +%s)
	load_after="$(uptime | sed 's/.*load averages*: //')"
	pkg_seconds=$((pkg_end - pkg_start))
	echo "  $pkg: excluded ${#excluded[@]} untouched file(s): ${excluded[*]:-none}"

	python3 - "$json" "$pkg" "$pkg_seconds" "$files_for_pkg" "$load_before" "$load_after" >>"$ROWS_FILE" <<'PYEOF'
import json, sys

path, pkg, seconds, files_str, lb, la = sys.argv[1:7]
touched_names = set(files_str.split())

try:
    with open(path) as fh:
        data = json.load(fh)
except FileNotFoundError:
    print(f"{pkg}\t{files_str}\tERROR\tERROR\tERROR\tERROR\tERROR\t{seconds}\t{lb} -> {la}")
    sys.exit(0)

# dc-2 assertion: -E must have kept mutation inside the touched set. If a
# report names any other file, the native scoping did NOT hold and the row
# is not reportable as a touched-files figure.
reported = {e["file_name"] for e in data.get("files", [])}
stray = sorted(reported - touched_names)
if stray:
    print(f"{pkg}\t{files_str}\tSCOPE-LEAK:{','.join(stray)}\t-\t-\t-\t-\t{seconds}\t{lb} -> {la}")
    sys.exit(0)

killed = survived = notcov = timedout = mutants = 0
for entry in data.get("files", []):
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

print(f"{pkg}\t{files_str}\t{mutants}\t{killed}\t{survived}\t{notcov}\t{timedout}\t{seconds}\t{lb} -> {la}")
PYEOF
done <"$PACKAGES_FILE"

SWEEP_END=$(date +%s)

echo
printf 'package\ttouched_files\tmutants\tkilled\tsurvived\tnot_covered\ttimed_out\tseconds\tload_1min_before_after\n'
cat "$ROWS_FILE"

awk -F'\t' '
	$3 ~ /^[0-9]+$/ { m+=$3; k+=$4; s+=$5; nc+=$6; to+=$7; secsum += $8 }
	END { printf "TOTAL(measured rows only)\t-\t%d\t%d\t%d\t%d\t%d\t%d\t-\n", m, k, s, nc, to, secsum }
' "$ROWS_FILE"

echo "wall clock for whole sweep (seconds): $((SWEEP_END - SWEEP_START))"
echo
echo "NOTE: 'seconds' is now the touched-files-only cost, because -E kept"
echo "mutation inside the touched set. It still includes gremlins' own"
echo "whole-package coverage pass, which -E does not scope: gremlins runs"
echo "\`go test -cover -coverprofile <f> ./<pkg>/...\` once per package"
echo "before generating a mutant, with no -timeout flag, so Go's"
echo "unconfigurable 10-minute default applies to that pass."
