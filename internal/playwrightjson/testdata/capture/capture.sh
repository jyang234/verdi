#!/usr/bin/env bash
# capture.sh re-captures internal/playwrightjson/testdata/reports/*.json: the
# JSON reports the pinned Playwright (e2e/package-lock.json) writes for the
# small fixture specs under specs/, one report per scenario config here.
#
# Opt-in and non-hermetic: it runs Node and the real Playwright, so it is
# never part of `make verify`, `make test`, or CI. Run it through `make
# playwright-report-capture`, which installs e2e/'s packages first; the specs
# need no browser. The fixture specs live here, never under e2e/tests/, so
# they never join the e2e shards.
#
# Each scenario runs exactly as the producer runs its named files: one worker,
# --retries=0, the JSON reporter writing to a file, output outside the tree,
# and recording off. The only change made to a report is path normalization,
# so the committed bytes do not depend on who captured them: the repository
# root becomes /verdi, the scratch directory /capture-tmp, and the node
# binary node. Nothing else is edited; timings are the capture's own.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
root="$(cd "$here/../../../.." && pwd -P)"
out="$root/internal/playwrightjson/testdata/reports"
e2e="$root/e2e"

pinned="$(node -e 'const l=require(process.argv[1]); process.stdout.write(l.packages["node_modules/@playwright/test"].version)' "$e2e/package-lock.json")"
installed="$(node -e 'process.stdout.write(require(process.argv[1]).version)' "$e2e/node_modules/@playwright/test/package.json" 2>/dev/null || true)"
if [ "$installed" != "$pinned" ]; then
	echo "capture.sh: e2e/node_modules has @playwright/test '${installed:-none}', the lockfile pins '$pinned'; run make e2e-setup first" >&2
	exit 2
fi

tmp="$(mktemp -d)"
tmp="$(cd "$tmp" && pwd -P)"
trap 'rm -rf "$tmp"' EXIT
nodebin="$(command -v node)"

# run_scenario runs one scenario's config in e2e/ (where the pinned
# Playwright is installed), writing its report to $tmp/<scenario>.json.
run_scenario() {
	cd "$e2e" && NODE_PATH="$e2e/node_modules" PLAYWRIGHT_JSON_OUTPUT_FILE="$tmp/$1.json" \
		CAPTURE_MARKER="$tmp/$1.marker" \
		exec npx playwright test -c "$here/$1.config.ts" --workers=1 --retries=0 \
		--reporter=json --output="$tmp/output-$1"
}

# Every scenario by default; name scenarios to re-capture only those.
scenarios=("$@")
if [ ${#scenarios[@]} -eq 0 ]; then
	scenarios=(outcomes duplicate setup-fails global-timeout sigint two-projects helper-declared)
fi

for scenario in "${scenarios[@]}"; do
	report="$tmp/$scenario.json"
	set +e
	if [ "$scenario" = sigint ]; then
		# Interrupt the run the way a cancelled job's runner is: SIGINT to the
		# run's whole process group, once the second test is running. Job
		# control gives the background run its own process group.
		set -m
		(run_scenario "$scenario") &
		pid=$!
		set +m
		for _ in $(seq 1 600); do
			[ -e "$tmp/$scenario.marker" ] && break
			sleep 0.1
		done
		if [ ! -e "$tmp/$scenario.marker" ]; then
			kill -KILL -- "-$pid" 2>/dev/null
			echo "capture.sh: scenario $scenario never reached its running test" >&2
			exit 1
		fi
		kill -INT -- "-$pid"
		wait "$pid"
	else
		(run_scenario "$scenario")
	fi
	status=$?
	set -e
	if [ ! -s "$report" ]; then
		echo "capture.sh: scenario $scenario (exit $status) wrote no report" >&2
		exit 1
	fi
	node -e '
		const fs = require("fs");
		const [src, dst, root, tmp, nodebin] = process.argv.slice(1);
		const text = fs.readFileSync(src, "utf8")
			.split(root).join("/verdi")
			.split(tmp).join("/capture-tmp")
			.split(nodebin).join("node");
		if (text.includes(process.env.HOME)) {
			console.error("capture.sh: " + dst + " still names " + process.env.HOME);
			process.exit(1);
		}
		fs.writeFileSync(dst, text);
	' "$report" "$out/$scenario.json" "$root" "$tmp" "$nodebin"
	echo "capture.sh: $scenario: playwright exit $status -> ${out#"$root"/}/$scenario.json"
done
