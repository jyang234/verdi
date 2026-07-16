#!/usr/bin/env bash
# require-pass.sh — the committed, testable core of the lint-showcase and
# showcase-coverage make guards (story spec/showcase-drift-gate CO-2/DC-2).
#
# `go test -run <pat>` exits 0 even when <pat> matches NOTHING ("no tests to
# run"), so a deleted or renamed required test would let a whole gate vanish
# with `make verify` still green — the exact silent drift this story exists to
# prevent. Given the required test names as "$1" (space-separated) and a
# `go test -v` transcript on stdin, this exits 1 (naming the first offender)
# unless every required name emitted a `--- PASS: <name> (` line.
#
# It lives in a script, not inline in the Makefile, so its OWN red direction is
# committed-testable (internal/showcasealign/guard_test.go feeds it canned
# transcripts and asserts exit 1 on a missing name) — the guard's outermost
# layer proven, not merely hand-run. The `required` list it is called with is
# itself kept in sync with the package's test functions by
# TestShowcaseCoverage_RequiredListInSync, so under-inclusion is not silent.
set -u

if [ "$#" -ne 1 ]; then
	echo "usage: require-pass.sh '<space-separated required test names>' < go-test-v-output" >&2
	exit 2
fi

required="$1"
out="$(cat)"

for tc in $required; do
	if ! printf '%s\n' "$out" | grep -qF -- "--- PASS: $tc ("; then
		echo "ERROR: require-pass guard: required test $tc did NOT run+pass (deleted, renamed, or skipped?)." >&2
		echo "       'go test -run' matching nothing exits 0 vacuously; this guard makes that silent drift a hard failure (story CO-2/DC-2)." >&2
		exit 1
	fi
done
