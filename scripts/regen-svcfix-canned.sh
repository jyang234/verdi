#!/usr/bin/env bash
# regen-svcfix-canned.sh — `make fixture-regen`'s implementation (PLAN.md
# §4: "a non-hermetic opt-in target ... regenerates them", "never in
# verify"). Re-captures testdata/svcfix-canned/*.json from the real,
# pinned flowmap/groundwork toolchain run against testdata/svcfix, then
# recomputes digests.json's sha256 ratchet.
#
# Toolchain source, in priority order:
#   1. $VERDI_S1_BIN/{flowmap,groundwork} if that directory exists (spike
#      S1's prebuilt binaries — fastest, no network).
#   2. `go run <module>/cmd/<bin>@<commit>`, reading module/commit from
#      this repo's own .verdi/verdi.yaml toolchain: block (I-4). Needs
#      network (module proxy resolution) — never run in `make verify` or
#      any test.
#
# This script is deliberately a thin, readable shell wrapper, not Go:
# fixture regeneration is a maintainer-run, opt-in tool, not production
# code under this module's strict-decode/no-network-in-tests discipline.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SVCFIX_DIR="$ROOT_DIR/testdata/svcfix"
CANNED_DIR="$ROOT_DIR/testdata/svcfix-canned"

resolve_bin() {
	local name="$1"
	if [ -n "${VERDI_S1_BIN:-}" ] && [ -x "$VERDI_S1_BIN/$name" ]; then
		echo "$VERDI_S1_BIN/$name"
		return
	fi
	# Falls back to `go run` below; callers check for this sentinel.
	echo ""
}

FLOWMAP_BIN="$(resolve_bin flowmap)"
GROUNDWORK_BIN="$(resolve_bin groundwork)"

if [ -z "$FLOWMAP_BIN" ] || [ -z "$GROUNDWORK_BIN" ]; then
	MODULE="$(grep -A2 '^toolchain:' "$ROOT_DIR/.verdi/verdi.yaml" | grep 'module:' | sed 's/.*module: *//')"
	COMMIT="$(grep -A2 '^toolchain:' "$ROOT_DIR/.verdi/verdi.yaml" | grep 'commit:' | sed 's/ *#.*//' | sed 's/.*commit: *//')"
	if [ -z "$MODULE" ] || [ -z "$COMMIT" ]; then
		echo "regen-svcfix-canned: could not read toolchain module/commit from .verdi/verdi.yaml" >&2
		exit 1
	fi
	echo "regen-svcfix-canned: VERDI_S1_BIN not set (or incomplete); using go run $MODULE/cmd/{flowmap,groundwork}@$COMMIT (needs network)" >&2
	FLOWMAP_BIN="go run $MODULE/cmd/flowmap@$COMMIT"
	GROUNDWORK_BIN="go run $MODULE/cmd/groundwork@$COMMIT"
fi

echo "regen-svcfix-canned: flowmap  = $FLOWMAP_BIN"
echo "regen-svcfix-canned: groundwork = $GROUNDWORK_BIN"

cd "$SVCFIX_DIR"

echo "regen-svcfix-canned: capturing graph.json"
$FLOWMAP_BIN graph -stamp deadbeef "$SVCFIX_DIR" >"$CANNED_DIR/graph.json"

echo "regen-svcfix-canned: capturing boundary-contract-base.json"
$FLOWMAP_BIN boundary "$SVCFIX_DIR"
cp "$SVCFIX_DIR/.flowmap/boundary-contract.json" "$CANNED_DIR/boundary-contract-base.json"

echo "regen-svcfix-canned: this script reproduces the base captures only;"
echo "the branch-state captures (boundary-contract-branch.json, the three"
echo "review-*.json verdicts) were captured against deliberate, temporary"
echo "source edits documented in testdata/svcfix-canned/README.md and are"
echo "not mechanically reproducible by this script — regenerate them by"
echo "hand-following that README if the toolchain's output shape changes."

echo "regen-svcfix-canned: recomputing digests.json"
python3 - "$CANNED_DIR" <<'PYEOF'
import hashlib, json, os, sys, collections

canned_dir = sys.argv[1]
files = sorted(
    f for f in os.listdir(canned_dir)
    if f not in ("README.md", "digests.json") and os.path.isfile(os.path.join(canned_dir, f))
)
digests = collections.OrderedDict()
for f in files:
    with open(os.path.join(canned_dir, f), "rb") as fh:
        digests[f] = "sha256:" + hashlib.sha256(fh.read()).hexdigest()

out = collections.OrderedDict()
out["schema"] = "verdi.fixture-digests/v1"
out["files"] = digests
with open(os.path.join(canned_dir, "digests.json"), "w") as fh:
    json.dump(out, fh, indent=2)
    fh.write("\n")
print("wrote", os.path.join(canned_dir, "digests.json"))
PYEOF

echo "regen-svcfix-canned: done. Review the diff before committing."
