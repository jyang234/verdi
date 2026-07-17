.PHONY: build test vet fmt fmt-check lint verify tidy fixture lint-store fixture-regen spec-align e2e-check-node e2e lint-showcase showcase-coverage

# Pin for the lint target. Both CI workflows install golangci-lint at this
# exact version before `make verify` (see .github/workflows/), so in CI the
# lint gate is mandatory — the `lint` target's CI=true branch refuses to pass
# by skipping. Kept in lockstep with verdi-go's own pin so results agree
# across the workspace if both are ever run side by side.
GOLANGCI_LINT_VERSION ?= v2.5.0

# Where lint-store builds the real verdi binary (gitignored — see root
# .gitignore). Distinct from `build`, which compiles every package but
# writes no binary anywhere useful to exec.
LINT_STORE_BIN := .build/verdi

build:
	go build ./...

# CROSS_BINARY_PKGS — the cache-blind cluster list (ADJ-68). These packages'
# tests build the cmd/verdi binary in a subprocess (TestMain: `go build
# ./cmd/verdi`) and exec it; that subprocess build is invisible to the test
# binary's own buildID, so `go test` serves a STALE cached PASS after a
# cmd/verdi behavior change (empirically reproduced: `ok (cached)` over a
# genuine red; -race does NOT defeat result caching). We force -count=1 (the
# documented cache bypass) for EXACTLY these, keeping honest caching for the
# provably-not-blind majority. In-package cmd/verdi exec tests are NOT blind
# (their buildID covers cmd/verdi's own sources) and are deliberately absent.
# TestGateCacheHonesty_CrossBinaryPkgsListInSync (internal/specalign) fails if a
# package that builds+execs cmd/verdi from outside cmd/verdi is missing here.
CROSS_BINARY_PKGS := ./internal/showcasealign/... ./internal/specalign/...

# -race mirrors CI's `go test -race` exactly: a data race that would fail CI
# must fail `make test`/`make verify` locally first (CLAUDE.md: "go test
# -race ./... — must always be clean"). The second line force-reruns the
# cache-blind cross-binary clusters (ADJ-68): `go test -race ./...` above may
# serve them a stale cached PASS, so -count=1 re-executes exactly those
# packages against the freshly built binary. `./...` semantics are otherwise
# unchanged — honest caching stands for every provably-not-blind package.
test:
	go test -race ./...
	go test -race -count=1 $(CROSS_BINARY_PKGS)

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

# lint gates on golangci-lint, with a deliberate local/CI split (matching
# verdi-go's trust-parity posture):
#   - CI (CI=true, which GitHub Actions sets): golangci-lint is MANDATORY.
#     Both workflows install golangci-lint@$(GOLANGCI_LINT_VERSION) before
#     `make verify`, so a missing binary here means the install step regressed
#     — we exit 1 rather than pass by skipping (a silent skip would be exactly
#     the undisclosed gap the constitution's three-valued honesty rules out).
#   - Locally: warn-if-missing, so a fresh clone without the tool can still run
#     the rest of `make verify`; install golangci-lint to gate lint locally.
# When the tool IS present, a version drift from the CI pin is a loud warning
# (never a silent pass) so local results can't quietly diverge from CI.
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		have=$$(golangci-lint version 2>/dev/null | grep -oE 'version v?[0-9]+\.[0-9]+\.[0-9]+' | grep -oE 'v?[0-9]+\.[0-9]+\.[0-9]+' | head -1); \
		if [ -n "$$have" ] && [ "v$${have#v}" != "$(GOLANGCI_LINT_VERSION)" ]; then \
			echo "warning: golangci-lint $$have differs from CI pin $(GOLANGCI_LINT_VERSION); results may diverge from CI" >&2; \
		fi; \
		golangci-lint run; \
	elif [ "$$CI" = "true" ]; then \
		echo "ERROR: golangci-lint not installed but CI=true — the lint gate is mandatory in CI. Both workflows install golangci-lint@$(GOLANGCI_LINT_VERSION) before 'make verify'; a missing binary means that step regressed. Refusing to pass by skipping." >&2; \
		exit 1; \
	else \
		echo "WARNING: golangci-lint not installed locally; skipping lint (install it to gate this locally)" >&2; \
	fi

# fixture runs the fixturegit determinism test package (PLAN.md Phase 1 test
# strategy: "fixturegit determinism test (build twice, assert identical
# SHAs)") plus, as of phase 2, the corpus package: it builds the full
# examples/showcase fixture via fixturegit, asserts the resulting SHAs equal
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

# lint-store builds the real verdi binary and runs `verdi lint` against
# this repo's own self-hosted store (PLAN.md Phase 4: "eat the dog food" —
# .verdi/specs/active/ holds the six component specs). Build-then-exec, not
# `go run`, so the gate exercises the exact binary CI would ship.
#
# `verdi model check` (extensibility phase 1, spec/model-schema ac-3,
# Task 7) runs in the same step: this repo carries no .verdi/model.yaml
# of its own, so this exercises the embedded-canonical-default path on
# every gate run (the gate grows, never shrinks — PLAN.md §2/§5).
lint-store:
	go build -o $(LINT_STORE_BIN) ./cmd/verdi
	$(LINT_STORE_BIN) lint
	$(LINT_STORE_BIN) model check

# spec-align (wave 7, PLAN.md §2/§5: "make verify grows ... to include
# ... spec-align by the end of the build") is internal/specalign's Go
# test package: self-hosted spec fidelity against ../docs/design/specs/
# (skips loudly, never fakes a pass, when the workspace layout isn't
# present — e.g. a CI checkout of verdi alone), the 00-index v0 checklist
# audit, the MCP tool inventory, and the CLI verb inventory. -race isn't
# used here (unlike `test`/`fixture`): this package execs the built verdi
# binary as a subprocess per PLAN.md's build-then-exec discipline, which
# the race detector has nothing to instrument.
#
# -count=1 (ADJ-68): this package builds+execs the cmd/verdi binary, whose
# sources never enter this test binary's cache key, so a bare `go test` here can
# serve a stale PASS after a cmd/verdi behavior change. Forcing a fresh run
# keeps `make spec-align` honest in isolation (the `test` target already
# re-runs it fresh via CROSS_BINARY_PKGS for `make test`/`make verify`).
spec-align:
	go test -count=1 ./internal/specalign/...

# SHOWCASE_REQUIRED_TESTS is the set of tests showcase-coverage's guard demands
# actually ran+passed (scripts/require-pass.sh enforces it). Kept in ONE named
# variable, not inline, so a single source of truth exists that the sync check
# (TestShowcaseCoverage_RequiredListInSync) reads and cross-checks against the
# package's own TestShowcaseCoverage* functions — so a newly-added coverage test
# missing from this list fails LOUDLY (its deletion would otherwise be silent,
# the under-inclusion gap the earlier inline list allowed). Any edit here must
# keep that test green.
SHOWCASE_REQUIRED_TESTS := TestShowcaseCoverage TestShowcaseCoverage_DetectsGaps TestShowcaseCoverage_DetectsGapsCoversAllClasses TestShowcaseCoverage_RealEnumerationDetectsGaps TestShowcaseCoverage_EnumerationIsComplete TestShowcaseCoverage_RequiredListInSync TestShowcaseCoverage_GuardScriptBites TestReadmeExamplesFresh

# lint-showcase and showcase-coverage are named gates over
# internal/showcasealign (same rationale as spec-align: `test` already runs
# this package, but a named target makes CI failure output name the gate
# instead of burying it in the full `go test -race ./...` output).
#
# lint-showcase runs TestShowcaseLintClean: the showcase corpus's own
# internal consistency check (`verdi lint` exits 0 against a freshly
# provisioned showcase store).
#
# GUARD (story CO-2/DC-2 — the gate must BITE): same mechanism as
# showcase-coverage below, for the same reason. `go test -run <pat>` exits 0
# even when <pat> matches NOTHING ("no tests to run"), so if lintclean_test.go
# were deleted or TestShowcaseLintClean renamed, a bare `go test -run` would
# pass VACUOUSLY and this lint-clean gate would silently vanish with `make
# verify` still green — the exact drift this story exists to prevent. We
# capture `-v` output and require TestShowcaseLintClean to have emitted a
# `--- PASS:` line; its absence (deletion, rename, or skip) is a hard failure
# here regardless of whether the package still compiles. The PASS-line predicate
# lives in scripts/require-pass.sh (whose own red direction is committed-tested
# by TestShowcaseCoverage_GuardScriptBites), so the guard is a tested unit, not
# merely a hand-run inline snippet.
# -count=1 (ADJ-68): showcasealign builds+execs cmd/verdi, invisible to its
# test cache key, so a bare `go test` here can serve a stale PASS after a
# cmd/verdi change — forcing a fresh run keeps this named gate honest in
# isolation (see CROSS_BINARY_PKGS).
lint-showcase:
	@out="$$(go test -count=1 ./internal/showcasealign/ -run TestShowcaseLintClean -v 2>&1)"; \
	status=$$?; \
	printf '%s\n' "$$out"; \
	if [ "$$status" -ne 0 ]; then exit "$$status"; fi; \
	printf '%s\n' "$$out" | scripts/require-pass.sh 'TestShowcaseLintClean'

# showcase-coverage runs TestShowcaseCoverage (the capability-coverage gate).
#
# GUARD (story CO-2/DC-2 — the gate must BITE): `go test -run <pat>` exits 0
# even when <pat> matches NOTHING ("no tests to run"). So if coverage_test.go
# were deleted or TestShowcaseCoverage renamed, a bare `go test -run` would
# pass VACUOUSLY and this whole capability-coverage gate would silently vanish
# with `make verify` still green — the exact drift this story exists to
# prevent. We therefore capture `-v` output and require each NAMED test in the
# `required` list below to have emitted a `--- PASS:` line:
#   - TestShowcaseCoverage is a hard FLOOR: it MUST run+pass. Its absence is
#     the deletion/rename attack, and is a hard failure here regardless of
#     whether the package still compiles (siblings only mention its helpers in
#     comments, so removing it does NOT break the build — the vacuous pass is
#     real, not hypothetical).
#   - TestShowcaseCoverage_DetectsGaps is the gate's own failure-path proof
#     (it feeds computeCoverageGaps deliberately-broken inventories and asserts
#     the RIGHT gap class names the RIGHT capability). It is equally a hard
#     FLOOR: without it the gate's RED direction is unexercised — itself a
#     silent pass. It is subject to the exact same vacuous-`-run` deletion/
#     rename/skip attack (nothing else references it, so removing it does NOT
#     break the build), so it too MUST emit its own `--- PASS:` line. The `-run`
#     pattern above already selects it (an unanchored TestShowcaseCoverage
#     match); this makes its presence a demanded invariant, not incidental.
#   - TestShowcaseCoverage_DetectsGapsCoversAllClasses guards the DetectsGaps
#     table at ROW granularity, the layer this name-only guard cannot reach: it
#     re-drives computeCoverageGaps over the same committed cases and fails if
#     the table stops exercising any gap class (deleting the load-bearing row
#     would otherwise keep DetectsGaps green). A hard FLOOR for the same
#     vacuous-`-run` reason.
#   - TestShowcaseCoverage_RealEnumerationDetectsGaps is the RED-direction proof
#     on the REAL enumeration (dispatch.go's verbPhase walk + live tools/list),
#     not a synthetic caps map: a real capability whose mapping is removed, and
#     a newly-added capability, both surface as named gaps. A hard FLOOR too.
#   - TestShowcaseCoverage_EnumerationIsComplete proves the CLI axis enumeration
#     is COMPLETE: run()'s pre-phase special-cases are exactly {lint}, so no verb
#     can ship dispatched-but-unenumerated behind a second pre-phase arm.
#   - TestShowcaseCoverage_RequiredListInSync fails if a TestShowcaseCoverage*
#     function exists in the package but is absent from SHOWCASE_REQUIRED_TESTS —
#     closing the silent under-inclusion an inline list allowed.
#   - TestShowcaseCoverage_GuardScriptBites is the committed red-direction proof
#     of scripts/require-pass.sh itself (feeds it a transcript missing a required
#     PASS line, asserts exit 1) — the guard's own outermost layer, tested.
#
# README freshness (DC-3) — WIRED, both steps landed: TestReadmeExamplesFresh
# exists in internal/showcasealign/readme_test.go (sibling public-readme story,
# 059915a) and is named in SHOWCASE_REQUIRED_TESTS above, so `make
# showcase-coverage` both selects it (the `-run` pattern names it) and
# hard-demands its `--- PASS:` line. DC-3's disclosed "passes vacuously until
# that sibling lands it" is therefore resolved. The wiring was DELIBERATELY
# two-step, not a silent auto-detect, and that discipline still governs the next
# such gate: (a) the `-run` pattern selecting a test only enforces its VERDICT (a
# failing run exits non-zero); (b) naming it in SHOWCASE_REQUIRED_TESTS is what
# guards against the test being DELETED, RENAMED, or SKIPPED (the vacuous-`-run`
# class, a `-run` matching nothing still exits 0) — so a new gate earns BOTH, by
# hand, where the next author is looking, never a fragile output pattern match.
#
# The PASS-line predicate now lives in scripts/require-pass.sh so it is a tested
# unit (TestShowcaseCoverage_GuardScriptBites), not an un-exercised inline
# snippet; the required set is the SHOWCASE_REQUIRED_TESTS variable above so the
# sync check (TestShowcaseCoverage_RequiredListInSync) can bind it to the
# package's actual TestShowcaseCoverage* functions. The Makefile is still the
# right home for the wiring: the vacuous-`-run` risk it defends is a build-gate
# fact, and a Go-only guard would itself be deletable the same way.
# -count=1 (ADJ-68): as with lint-showcase — force a fresh run of the
# cache-blind showcasealign cluster so this named gate is honest in isolation.
showcase-coverage:
	@out="$$(go test -count=1 ./internal/showcasealign/ -run 'TestShowcaseCoverage|TestReadmeExamplesFresh' -v 2>&1)"; \
	status=$$?; \
	printf '%s\n' "$$out"; \
	if [ "$$status" -ne 0 ]; then exit "$$status"; fi; \
	printf '%s\n' "$$out" | scripts/require-pass.sh '$(SHOWCASE_REQUIRED_TESTS)'

# e2e-check-node is verify's Node/Playwright preflight: CLAUDE.md made
# e2e a merge blocker ("every browser-facing behavioral path ... a
# Playwright e2e test"), so a missing Node toolchain must FAIL verify
# loudly, never silently skip e2e (a silent skip would be exactly the
# kind of undisclosed gap the constitution's three-valued honesty rules
# out). Checked separately from `e2e` itself so the failure message is
# about the missing toolchain, not a confusing `npm: command not found`
# buried in `cd e2e && npm install`'s output.
e2e-check-node:
	@if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then \
		echo "ERROR: node/npm not found — e2e (verdi/e2e/, Playwright) is a merge blocker per CLAUDE.md's testing regime, not optional." >&2; \
		echo "        Install Node.js (e.g. https://nodejs.org, nvm, or your OS package manager; e2e/package.json has no engines pin, any current LTS works) and re-run 'make verify'." >&2; \
		exit 1; \
	fi

# e2e runs the Playwright suite under e2e/ (PLAN.md Phase 10 deliverable
# 4): `npm install` then `npx playwright install --with-deps chromium`
# then `npx playwright test`. e2e/playwright.config.ts's webServer stanza
# does the rest — builds the real verdi binary, provisions a scratch
# store, starts `verdi serve` plus a static server over a built dex site,
# waits for readiness, and tears both down after the run.
#
# Wave 7: now wired into `verify` (see the `verify` target below) — both
# CI configs install Node + Playwright browsers before `make verify` so
# local/CI parity holds (CLAUDE.md: "CI runs exactly `make verify` —
# trust parity"). Depends on e2e-check-node so a missing toolchain fails
# with the install message above, not a raw shell error.
#
# VERDI_E2E_PORT_BASE (D6-28): the harness (cmd/e2eharness/ports.go) and
# this suite's runner (e2e/ports.ts) both hard-code 4173/4174/4177 unless
# this var is set, in which case every port derives from it as base,
# base+1, base+2 in lockstep on both sides — letting concurrent `make
# verify` runs in sibling git worktrees each claim a disjoint port range
# instead of racing for the same three. It is a plain env var, not a make
# variable, so it needs no plumbing here: export it before invoking make
# (e.g. `VERDI_E2E_PORT_BASE=4300 make e2e`) and the recipe's child
# processes (npm/npx/go run) inherit it automatically. Unset: unchanged
# behavior.
e2e: e2e-check-node
	cd e2e && npm install && npx playwright install --with-deps chromium && npx playwright test

# verify is the full gate (CLAUDE.md: "grows — never shrinks — to
# include integration, e2e, and spec-align by the end of the build").
# e2e runs LAST: it is by far the slowest step (browser install + a real
# server round-trip) and every faster gate should fail first when
# something's broken, so a run that fails early doesn't pay e2e's cost
# for nothing.
verify: build fmt-check vet lint test fixture lint-store spec-align lint-showcase showcase-coverage e2e
	@echo "verify OK"

tidy:
	go mod tidy
