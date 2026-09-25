#!/bin/sh
# merge-gate-verdict.sh — the decision of the required `merge-gate` check
# (SI-266). merge-gate.yml runs the gate as parallel jobs; its `merge-gate`
# job needs every one of them, runs with `if: always()`, and calls this
# script with one `<job>=<result>` argument per gate job, taken from
# `${{ needs.<job>.result }}`.
#
# Exits 0 only when at least one argument was given and every result is
# exactly `success`. A `failure`, `cancelled`, or `skipped` result, an empty
# result (a job missing from `needs:` expands to ""), or any other value
# exits 1. No arguments, or an argument that is not `<job>=<result>` with a
# non-empty job, exits 2. Silence is never a pass: every result is printed.
#
# The call's exact text is pinned by internal/specalign's
# TestMergeGateAggregatorDecidesOverEveryGateJob, and this script's behavior
# by TestMergeGateVerdictScript.
set -u

if [ "$#" -eq 0 ]; then
	echo "merge-gate: no gate job results given; refusing to pass over nothing" >&2
	exit 2
fi

usage=0
failed=0
for arg in "$@"; do
	job=${arg%%=*}
	result=${arg#*=}
	if [ "$job" = "$arg" ] || [ -z "$job" ]; then
		echo "merge-gate: malformed argument '$arg' (want <job>=<result>)" >&2
		usage=1
		continue
	fi
	if [ "$result" = success ]; then
		echo "  $job: success"
	else
		echo "  $job: ${result:-<empty>} (not success)"
		failed=1
	fi
done

if [ "$usage" -ne 0 ]; then
	exit 2
fi
if [ "$failed" -ne 0 ]; then
	echo "merge-gate: FAILED — every gate job must succeed" >&2
	exit 1
fi
echo "merge-gate: every gate job succeeded"
