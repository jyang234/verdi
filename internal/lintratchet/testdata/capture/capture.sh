#!/usr/bin/env bash
# capture.sh re-captures internal/lintratchet/testdata/reports/*.json: the JSON
# reports the Makefile's pinned golangci-lint writes for the small variant
# modules under variants/, linted with the repository's .golangci.strict.yml
# exactly as make lint-strict lints the module: GOOS=linux GOARCH=amd64,
# --config, --issues-exit-code=0, JSON to a file. TestRatchet_Verdicts reads
# these captured reports.
#
# Opt-in and non-hermetic: it runs the real golangci-lint, so it is never part
# of make verify, make test, or CI. Each variant is copied, with the strict
# configuration beside it, into a scratch directory and linted there, so the
# report's paths are relative to the variant's root (alpha/a.go) and every
# variant's packages share one name. Nothing in a report is edited.
#
# The variants, each the base module with one change:
#   base        alpha holds a global (gochecknoglobals) and an errorf %v
#               (errorlint); beta holds nothing
#   fixed       the global removed
#   lineshift   three comment lines inserted above both findings
#   movefile    the global moved to another file of alpha
#   movepkg     the global moved to beta
#   duplicate   a second errorf %v line identical to the first, in alpha
#   sameline    fresh starts from a new context, so contextcheck also flags
#               the line errorlint already flags
#   newfinding  a second global
#   broken      alpha does not compile (golangci-lint reports typecheck)
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
root="$(cd "$here/../../../.." && pwd -P)"
out="$root/internal/lintratchet/testdata/reports"

pin="$(sed -n 's/^GOLANGCI_LINT_VERSION[[:space:]]*?\{0,1\}=[[:space:]]*v\{0,1\}\([0-9.]*\).*/\1/p' "$root/Makefile")"
have="$(golangci-lint version 2>/dev/null | grep -oE 'version v?[0-9]+\.[0-9]+\.[0-9]+' | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || true)"
if [ -z "$pin" ] || [ "$have" != "$pin" ]; then
	echo "capture.sh: golangci-lint '${have:-none}' is not the Makefile's pin '${pin:-unreadable}'; install the pinned version first" >&2
	exit 2
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

variants=("$@")
if [ ${#variants[@]} -eq 0 ]; then
	variants=(base fixed lineshift movefile movepkg duplicate sameline newfinding broken)
fi

mkdir -p "$out"
for v in "${variants[@]}"; do
	cp -R "$here/variants/$v" "$tmp/$v"
	cp "$root/.golangci.strict.yml" "$tmp/$v/"
	status=0
	(cd "$tmp/$v" && GOOS=linux GOARCH=amd64 GOFLAGS= GOWORK=off GOPROXY=off GOTOOLCHAIN=local \
		golangci-lint run --config .golangci.strict.yml --issues-exit-code=0 --allow-parallel-runners \
		--output.json.path="$tmp/$v.json" ./...) || status=$?
	if [ "$status" -ne 0 ]; then
		echo "capture.sh: golangci-lint exited $status for variant $v" >&2
		exit 2
	fi
	cp "$tmp/$v.json" "$out/$v.json"
	echo "capture.sh: captured $out/$v.json"
done
