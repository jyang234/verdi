.PHONY: build test vet fmt fmt-check lint verify tidy fixture fixture-regen

# Pin for the lint target's version-drift warning. Not yet installed by the
# CI skeleton (phase 1 has no network-dependent tool bootstrap); once CI
# installs golangci-lint at this pin, lint stops degrading to a warning
# there too. Kept in lockstep with verdi-go's own pin so results agree
# across the workspace if both are ever run side by side.
GOLANGCI_LINT_VERSION ?= v2.5.0

build:
	go build ./...

# -race mirrors CI's `go test -race` exactly: a data race that would fail CI
# must fail `make test`/`make verify` locally first (CLAUDE.md: "go test
# -race ./... — must always be clean").
test:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

# fmt-check is the fast pre-push gate mirroring CI's gofmt step exactly, so a
# formatting slip fails locally instead of costing a CI round-trip.
fmt-check:
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi
	@echo "gofmt OK"

# lint runs golangci-lint when it is installed and warns (without failing)
# when it is not, mirroring verdi-go/Makefile's posture: a version drift
# from the CI pin is a loud warning, never a silent pass. Phase 1's CI
# skeleton does not yet install golangci-lint, so this is also what keeps
# `make verify` green in CI today; a later phase adds the install step and
# this same target starts gating for real, with no Makefile change needed.
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		have=$$(golangci-lint version 2>/dev/null | grep -oE 'version v?[0-9]+\.[0-9]+\.[0-9]+' | grep -oE 'v?[0-9]+\.[0-9]+\.[0-9]+' | head -1); \
		if [ -n "$$have" ] && [ "v$${have#v}" != "$(GOLANGCI_LINT_VERSION)" ]; then \
			echo "warning: golangci-lint $$have differs from CI pin $(GOLANGCI_LINT_VERSION); results may diverge from CI" >&2; \
		fi; \
		golangci-lint run; \
	else \
		echo "WARNING: golangci-lint not installed locally; skipping lint (install it to gate this locally)" >&2; \
	fi

# fixture runs the fixturegit determinism test package (PLAN.md Phase 1 test
# strategy: "fixturegit determinism test (build twice, assert identical
# SHAs)") plus, as of phase 2, the corpus package: it builds the full
# testdata/corpus fixture via fixturegit, asserts the resulting SHAs equal
# the committed golden constants, and decodes every corpus file (committed
# and mutable/derived) through internal/artifact. As of phase 5, also
# internal/svcfixcanned: verifies testdata/svcfix-canned/digests.json's
# sha256 ratchet against the committed canned upstream captures — hermetic
# (no exec, no network); regenerating the captures for real is
# `make fixture-regen`'s job, never this one's.
fixture:
	go test -race ./internal/fixturegit/... ./internal/corpus/... ./internal/svcfixcanned/...

# fixture-regen re-captures testdata/svcfix-canned/*.json from the real,
# pinned toolchain (spike S1's bin/, or `go run …@pin` over the network —
# see scripts/regen-svcfix-canned.sh) and recomputes the digest ratchet.
# Opt-in and non-hermetic (PLAN.md §4): never part of `make verify`, `make
# test`, or `make fixture`, and never run by CI.
fixture-regen:
	./scripts/regen-svcfix-canned.sh

# verify is the per-phase gate: it must stay green at the end of every
# phase (CLAUDE.md). It grows — never shrinks — to add integration, e2e,
# and spec-align gates by the end of the build (PLAN.md §2).
verify: build fmt-check vet lint test fixture
	@echo "verify OK"

tidy:
	go mod tidy
