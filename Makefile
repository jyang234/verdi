.PHONY: build test test-cmd test-cross test-slow test-rest vet fmt fmt-check lint verify tidy fixture lint-store fixture-regen spec-align e2e-check-node e2e-setup e2e-1 e2e-2 e2e-3 e2e lint-showcase showcase-coverage hooks

# Pin for the lint target. Both CI workflows install golangci-lint at this
# exact version before the lint step runs (verify.yml and merge-gate.yml,
# each in its static job before `make lint`), so in CI the
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
# documented cache bypass) for EXACTLY these — test-cross runs all of them
# but internal/specalign, which spec-align runs — keeping honest caching for
# the provably-not-blind majority. TestGateCacheHonesty_CrossBinaryPkgsRunFresh
# fails if any of them runs without -count=1 in `make test` or `make verify`.
# In-package cmd/verdi exec tests are NOT blind (their buildID covers
# cmd/verdi's own sources) and are deliberately absent.
# TestGateCacheHonesty_CrossBinaryPkgsListInSync (internal/specalign) fails if a
# package that builds+execs cmd/verdi from outside cmd/verdi is missing here.
# cmd/e2eharness joined with the unproven-board fixture (MVP release amendment
# R2): its tests call buildBinary — `go build ./cmd/verdi` in a subprocess —
# and exec the result as `verdi serve`, the same cache blindness.
CROSS_BINARY_PKGS := ./internal/showcasealign/... ./internal/specalign/... ./internal/experimentapp/... ./internal/designapp/... ./internal/sealedexec/claude/... ./internal/publicrelease/... ./cmd/e2eharness/...

# The Go tests run as disjoint shards (SI-266, owner directive 2026-09-24;
# SI-268 split test-slow out of test-rest, owner directive 2026-09-25), so
# the pull-request gate can run each as its own parallel CI job and no
# package runs twice. `make test` runs all of them; their package sets
# partition `go list ./...`:
#   test-cmd    ./cmd/verdi, the largest single package.
#   test-cross  CROSS_BINARY_PKGS except internal/specalign, always fresh
#               (-count=1, ADJ-68).
#   test-slow   TEST_SLOW_PKGS, the slowest of the rest (below), cached
#               honestly.
#   test-rest   every other package, cached honestly.
#   spec-align  internal/specalign alone, fresh and under -race (below).
# internal/specalign's TestGateShards_* tests read these recipes through
# `make -n` and fail if two shards share a package, a package is in none, or
# a shard drops -race. That is why test-rest's list is computed by make
# ($(shell ...)) and not by the recipe's own shell: a dry-run prints a
# shell-computed list unexpanded, and the guard refuses it.
#
# -race mirrors CI's `go test -race` exactly: a data race that would fail CI
# must fail `make test`/`make verify` locally first (CLAUDE.md: "go test
# -race ./... — must always be clean").
# -parallel 4 caps concurrent t.Parallel() tests and subtests within each
# test binary, at the vCPU count of this public repo's GitHub-hosted
# ubuntu-latest runners (lane T1 test-speed contract step 4). It does not
# limit how many package binaries run at once: that is -p, left at its
# default (GOMAXPROCS), so a machine with more cores than CI still runs more
# packages concurrently than CI does.
TEST_CMD_PKGS := ./cmd/verdi
SPEC_ALIGN_PKGS := ./internal/specalign/...

# TEST_SLOW_PKGS (SI-268) are the slowest packages test-rest used to run,
# split into their own shard so that the two CI jobs finish at about the same
# time. The list is balanced on the CI test-rest job at 704cac30 (4-vCPU
# ubuntu-latest, 103 packages, 6m03s for `make test-rest`), using the time
# each package's `ok` line reports: 821s in all. These are its fifteen
# slowest, 591s of it. internal/sealedexec alone took 158s, which puts a
# floor under this job; it is listed first so that `go test` builds and
# starts it first. Estimate: a job's wall time is about 30s of first builds
# plus a quarter of (5s of build per package + the packages' test times),
# and never less than the time until its slowest package ends. That puts
# both jobs near 200s. Rebalance when the CI test-slow and test-rest jobs
# finish more than a minute apart, or when a package left in test-rest
# approaches sealedexec's time. fixture's three packages stay in test-rest
# (see fixture).
TEST_SLOW_PKGS := ./internal/sealedexec ./internal/lint ./internal/artifact ./internal/dex ./internal/workbench ./internal/execworkspace ./internal/contextowner ./internal/constitutionapp ./internal/policyconflict ./internal/sealedreview ./cmd/public-execution-contract-release ./internal/contextcompile ./internal/specimport ./internal/readinessload ./internal/align

# TEST_REST_PKGS is `go list ./...` minus the packages of test-cmd,
# CROSS_BINARY_PKGS (spec-align's is among them), and TEST_SLOW_PKGS, so a new
# package always lands here. If any `go list` fails (a TEST_SLOW_PKGS entry
# that names no package, say), or nothing is left, the list is the single
# word test-rest-package-list-failed, which `go test` rejects: the shard fails
# loudly instead of testing nothing.
TEST_REST_PKGS = $(shell all="$$(go list ./...)" && skip="$$(go list $(TEST_CMD_PKGS) $(CROSS_BINARY_PKGS) $(TEST_SLOW_PKGS))" && printf '%s\n' "$$all" | grep -vxF -e "$$skip" || echo test-rest-package-list-failed)

test: test-cmd test-cross test-slow test-rest spec-align

test-cmd:
	go test -race -parallel 4 $(TEST_CMD_PKGS)

test-cross:
	go test -race -count=1 -parallel 4 $(filter-out $(SPEC_ALIGN_PKGS),$(CROSS_BINARY_PKGS))

test-slow:
	go test -race -parallel 4 $(TEST_SLOW_PKGS)

test-rest:
	go test -race -parallel 4 $(TEST_REST_PKGS)

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
#     this target runs (verify.yml and merge-gate.yml, each in its static job
#     before `make lint`), so a missing binary here means the
#     install step regressed — we exit 1 rather than pass by skipping (a
#     silent skip would be exactly the undisclosed gap the constitution's
#     three-valued honesty rules out).
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
		echo "ERROR: golangci-lint not installed but CI=true — the lint gate is mandatory in CI. Both CI workflows install golangci-lint@$(GOLANGCI_LINT_VERSION) before the lint step (verify.yml and merge-gate.yml, each in its static job before 'make lint'); a missing binary means that install step regressed. Refusing to pass by skipping." >&2; \
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
#
# These three packages are also in test-rest. fixture passes test-rest's
# exact flags (-race -parallel 4), so after test-rest on the same machine
# the Go test cache replays their result here instead of executing them a
# second time (SI-266: every package executes once under -race;
# TestGateShards_VerifyExecutesEachPackageOnceUnderRace). That holds inside
# `make verify`, which runs test-rest first, and in the pull-request gate,
# which runs fixture in the test-rest job right after test-rest
# (TestMergeGateParity_CacheReplaysRunAfterTheirExecutorInOneJob). Run
# alone, fixture executes them.
fixture:
	go test -race -parallel 4 ./internal/fixturegit/... ./internal/corpus/... ./internal/svcfixcanned/...

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
#
# `verdi harness check` (spec/spec-documents ac-7, wave 3) verifies that
# this repository's own rendered skills (.claude/skills/verdi-*/SKILL.md,
# .agents/skills/verdi-*/SKILL.md) match the templates embedded in the
# binary just built — the drift gate; regenerate with `verdi harness render`.
lint-store:
	go build -o $(LINT_STORE_BIN) ./cmd/verdi
	$(LINT_STORE_BIN) lint
	$(LINT_STORE_BIN) model check
	$(LINT_STORE_BIN) harness check

# spec-align (wave 7, PLAN.md §2/§5: "make verify grows ... to include
# ... spec-align by the end of the build") is internal/specalign's Go
# test package: self-hosted spec fidelity against ../docs/design/specs/
# (skips loudly, never fakes a pass, when the workspace layout isn't
# present — e.g. a CI checkout of verdi alone), the 00-index v0 checklist
# audit, the MCP tool inventory, and the CLI verb inventory.
#
# spec-align is the only target that runs internal/specalign (SI-266): the
# test-cross shard excludes it and `make test` runs this target instead, so
# specalign runs once per `make test` and once per `make verify`, under
# -race and -parallel 4 like every other package (it used to run three
# times per `make verify`: twice inside `test`, once more here without
# -race).
#
# -count=1 (ADJ-68): this package builds+execs the cmd/verdi binary, whose
# sources never enter this test binary's cache key, so a bare `go test` here can
# serve a stale PASS after a cmd/verdi behavior change. Forcing a fresh run
# keeps `make spec-align` honest every time it runs.
#
# -v + skip surfacing (judged-ac3-resolution-check-skips-in-authoring-layout):
# this package's workspace-side checks (guide-claims cite RESOLUTION and
# transcription fidelity, self-hosted spec fidelity) SKIP loudly, disclosed,
# when the out-of-repo workspace tree is absent (a bare verdi checkout). A
# plain `go test` prints nothing for a t.Skipf in a passing package, making a
# skip indistinguishable from a pass at the make verify surface. We run -v,
# capture the transcript, and reprint every `--- SKIP:` notice (with its
# disclosed reason) so a skip is never a silent pass here — CLAUDE.md's
# three-valued honesty. On any failure the full transcript is printed for
# debugging; on success only the disclosed skips and the package result line.
spec-align:
	@out="$$(go test -race -v -count=1 -parallel 4 $(SPEC_ALIGN_PKGS) 2>&1)"; \
	status=$$?; \
	if [ "$$status" -ne 0 ]; then printf '%s\n' "$$out"; exit "$$status"; fi; \
	skips="$$(printf '%s\n' "$$out" | grep -B1 -- '--- SKIP:' || true)"; \
	if [ -n "$$skips" ]; then \
		echo '=== spec-align DISCLOSED SKIPS (a skip is NOT a pass — CLAUDE.md three-valued honesty) ==='; \
		printf '%s\n' "$$skips"; \
		echo '==============================================================================='; \
	fi; \
	printf '%s\n' "$$out" | grep -E '^(ok|\?)[[:space:]]' || true

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
# internal/showcasealign: the test-cross shard already runs this whole
# package, but a named target makes CI failure output name the gate instead
# of burying it in test-cross's output for every cross-binary package.
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

# e2e-check-node is the e2e shards' Node/Playwright preflight: CLAUDE.md
# made e2e a merge blocker ("every browser-facing behavioral path ... a
# Playwright e2e test"), so a missing Node toolchain must FAIL verify
# loudly, never silently skip e2e (a silent skip would be exactly the
# kind of undisclosed gap the constitution's three-valued honesty rules
# out). Checked separately from e2e-setup so the failure message is about
# the missing toolchain, not a confusing `npm: command not found` buried in
# `cd e2e && npm install`'s output.
e2e-check-node:
	@if ! command -v node >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then \
		echo "ERROR: node/npm not found — e2e (verdi/e2e/, Playwright) is a merge blocker per CLAUDE.md's testing regime, not optional." >&2; \
		echo "        Install Node.js (e.g. https://nodejs.org, nvm, or your OS package manager; e2e/package.json has no engines pin, any current LTS works) and re-run 'make verify'." >&2; \
		exit 1; \
	fi

# The Playwright suite under e2e/ (PLAN.md Phase 10 deliverable 4) runs as
# three shards, e2e-1, e2e-2, and e2e-3 (SI-268, owner directive 2026-09-25,
# BL-73). Each is its own VERIFY_STEPS entry and its own CI gate job; each
# gets its own `go run ./cmd/e2eharness` (e2e/playwright.config.ts's
# webServer), and so its own verdi binary, scratch store, and servers. A
# shard's spec files run serially in file-name order against its one store,
# exactly as the whole suite once did against its own.
#
# e2e-setup is the shards' shared prerequisite: the Node check, then `npm
# install` and `npx playwright install --with-deps chromium` in e2e/. Make
# builds a prerequisite once per invocation, so `make e2e` installs once
# before its three concurrent shards, and no two installs run in e2e/ at the
# same time. Each e2e-N step of `make verify` is its own invocation and
# installs again (a no-op once installed).
e2e-setup: e2e-check-node
	cd e2e && npm install && npx playwright install --with-deps chromium

# The shards' spec files. E2E_SHARD_1 and E2E_SHARD_2 are explicit;
# E2E_SHARD_3 is every *.spec.ts file under e2e/tests/ that they do not list,
# so a new spec file always runs, in shard 3. If that remainder is empty, it
# is the single word e2e-shard-3-list-failed, which playwright.config.ts
# refuses: the shard fails loudly instead of testing nothing. Each target
# passes its list as VERDI_E2E_SPECS, which playwright.config.ts turns into
# an exact testMatch (it also refuses a name that is not a spec file under
# e2e/tests/, and a name listed twice). internal/specalign's
# e2eshards_test.go reads the lists through `make -n` and fails unless the
# three shards partition every file Playwright collects under e2e/tests/
# (which it matches with spec or test in any letter case, so a new
# 99-x.Spec.ts or 99-x.test.ts runs in no shard and fails the guard until it
# is renamed to *.spec.ts), each with its own ports and output directory.
#
# The lists are chosen by the per-file test time of the serial suite at
# 704cac30 (757s over 67 files; 50-design-workbench.spec.ts alone is 153s):
#   shard 1  10 through 32, the board suites up to the scoping yarn  263s
#   shard 2  33 through 49                                            246s
#   shard 3  00 through 06, and 50 onward                             248s
# plus each shard's own setup and harness start. Rebalance when a CI e2e job
# finishes more than a minute after the others; shard 3 grows with every new
# spec file.
#
# Dependency chains: a few spec files depend on state an earlier file left
# in the store. Each chain stays inside one shard, in its original order
# (Playwright runs a shard's files in file-name order). When these lists
# were chosen, each shard passed alone on its own fresh store (93, 127, and
# 110 tests: the suite's 330).
#   - 30 -> 31 -> 32, in shard 1. 32-board-scoping-yarn.spec.ts lines 9-12:
#     "This suite runs after 30/31 in the shared store, so
#     SHOWCASE.DESIGN_SPEC already carries the stubs suite 30 graduated". A
#     shard that began at 32 failed its :100, :128, and :168 tests (probe
#     at 704cac30).
#   - 13 -> 26 -> 28, 29, in shard 1. 28-board-pin-import.spec.ts lines
#     68-69 and 29-board-trash.spec.ts lines 21-23 need a card for
#     adr/0001-outbox-events, held up by "the fixture's exempts edge" or "an
#     earlier suite file's re-drawn one". 26-board-deletion.spec.ts retypes
#     the fixture's edge (line 99) and then removes it (lines 123-125 and
#     146); 13-board-scratch-tier.spec.ts lines 120-123 graduate a
#     decision->ADR thread into the typed exempts edge that keeps the card.
#     26, 28, 29 alone fail 28:60 and 29:143 with the card gone; with 13
#     first, all 23 tests pass.
E2E_SHARD_1 := \
  10-board-projection.spec.ts \
  11-board-git-affordance.spec.ts \
  12-board-type-picker.spec.ts \
  13-board-scratch-tier.spec.ts \
  14-board-layout-stability.spec.ts \
  15-board-review-mode.spec.ts \
  16-dex-v2.spec.ts \
  17-board-positions.spec.ts \
  18-dex-by-story.spec.ts \
  19-disclosures.spec.ts \
  20-board-drag-robustness.spec.ts \
  21-board-document-edges.spec.ts \
  22-board-collision-free.spec.ts \
  23-board-dialog-usability.spec.ts \
  24-board-sticky-types.spec.ts \
  25-board-ref-peek.spec.ts \
  26-board-deletion.spec.ts \
  27-board-legibility.spec.ts \
  28-board-pin-import.spec.ts \
  29-board-trash.spec.ts \
  30-board-scoping-canvas.spec.ts \
  31-board-stub-instantiate.spec.ts \
  32-board-scoping-yarn.spec.ts
E2E_SHARD_2 := \
  33-board-expand.spec.ts \
  34-board-superseded-status.spec.ts \
  35-board-obligation-graduate.spec.ts \
  36-board-obligation-wall.spec.ts \
  37-board-diagram-editor.spec.ts \
  37-board-wall-badges.spec.ts \
  37-directory-home.spec.ts \
  38-board-evidence-slot.spec.ts \
  38-board-size-smell.spec.ts \
  38-derivation-drawer.spec.ts \
  38-draft-boards.spec.ts \
  39-diagram-tier.spec.ts \
  40-showcase-draft.spec.ts \
  41-showcase-ladder-badge.spec.ts \
  42-matrix-preview.spec.ts \
  43-family-board-links.spec.ts \
  43-home-status-glance.spec.ts \
  43-tool-view-exit.spec.ts \
  44-branch-family-links.spec.ts \
  45-vocabulary.spec.ts \
  46-board-refresh-interaction.spec.ts \
  47-board-statusless-lifecycle.spec.ts \
  47-board-sticky-keys.spec.ts \
  48-board-creation-form.spec.ts \
  49-readiness-pilot.spec.ts
E2E_SHARD_3 = $(or $(filter-out $(E2E_SHARD_1) $(E2E_SHARD_2),$(sort $(notdir $(wildcard e2e/tests/*.spec.ts)))),e2e-shard-3-list-failed)

# e2e_shard is one shard's Playwright command, run in e2e/: $(1) is the
# shard's number, $(2) its VERDI_E2E_PORT_BASE, and $(3) its spec files.
# VERDI_E2E_PORT_BASE (D6-28; cmd/e2eharness/ports.go, e2e/ports.ts) moves
# the harness's four ports to base..base+3. Each shard sets its own fixed
# base and its own --output directory (Playwright empties it when a run
# starts), so the three shards never collide when they run at once. A
# VERDI_E2E_PORT_BASE exported before make does not reach a shard: the
# shard's own value wins. The E2E_RUN_N variables are the one definition of
# each shard's command, used by e2e-N and by e2e.
e2e_shard = VERDI_E2E_PORT_BASE=$(2) VERDI_E2E_SPECS='$(strip $(3))' npx playwright test --output=test-results/e2e-$(1)
E2E_RUN_1 = $(call e2e_shard,1,21000,$(E2E_SHARD_1))
E2E_RUN_2 = $(call e2e_shard,2,22000,$(E2E_SHARD_2))
E2E_RUN_3 = $(call e2e_shard,3,23000,$(E2E_SHARD_3))

e2e-1: e2e-setup
	cd e2e && $(E2E_RUN_1)

e2e-2: e2e-setup
	cd e2e && $(E2E_RUN_2)

e2e-3: e2e-setup
	cd e2e && $(E2E_RUN_3)

# e2e runs the whole suite as the three shards at once, a local convenience
# and not a gate step (make verify runs the shards one after another, CI in
# three jobs). GNU make 3.81, which macOS ships, has no --output-sync, so
# every line a shard prints carries its name ("[e2e-2] ..."). Each shard's
# exit status and seconds are kept apart from its output, and the target
# ends with one summary line per shard; it fails if any shard failed or
# left no status. internal/specalign holds this recipe to the gate recipes'
# rules (TestGateParity_GateRecipesNeverIgnoreErrors), and runs it over fake
# shards to prove it fails when any one shard fails alone or all three fail
# (TestE2EShards_SuiteFailsWhenAnyShardFails); no test makes a shard leave no
# status.
e2e: e2e-setup
	@cd e2e && tmp=$$(mktemp -d) && start=$$(date +%s) && \
	shard() { n=$$1; shift; { s=$$(date +%s); "$$@" 2>&1; echo "$$? $$(( $$(date +%s) - s ))" > "$$tmp/$$n"; } | awk -v p="[e2e-$$n] " '{ print p $$0; fflush() }'; } && \
	{ shard 1 env $(E2E_RUN_1) & shard 2 env $(E2E_RUN_2) & shard 3 env $(E2E_RUN_3) & wait; }; \
	failed=""; \
	echo "e2e: the three shards ran at once in $$(( $$(date +%s) - start ))s:"; \
	for n in 1 2 3; do \
		rc=missing; secs=?; \
		if [ -s "$$tmp/$$n" ]; then read rc secs < "$$tmp/$$n"; fi; \
		printf '  e2e-%s  exit %-7s %ss\n' "$$n" "$$rc" "$$secs"; \
		if [ "$$rc" != 0 ]; then failed="$$failed e2e-$$n"; fi; \
	done; \
	rm -rf "$$tmp"; \
	if [ -n "$$failed" ]; then echo "e2e: FAILED:$$failed" >&2; exit 1; fi; \
	echo "e2e OK"

# verify is the full gate (CLAUDE.md: "grows — never shrinks — to
# include integration, e2e, and spec-align by the end of the build").
# The e2e shards run LAST: they are by far the slowest steps (browser
# install + a real server round-trip) and every faster gate should fail
# first when something's broken, so a run that fails early doesn't pay
# e2e's cost for nothing.
#
# The gate runs its steps SERIALLY through a recursive make and records the
# wall-clock of every step (process-audit PA-025: gate duration was measured
# nowhere, so its growth had no trend). Each run appends one row per step to
# $(GATE_TIMINGS) — `<utc-time> <head> <step> <seconds> <ok|fail>` — under
# .verdi/data/, which the store's own .gitignore already excludes, and prints
# a summary table at the end so a CI log carries the same series. Steps run in
# order and the gate fails fast on the first red step.
#
# VERIFY_STEPS lists `test` as its shards (test-cmd test-cross test-slow
# test-rest), not as `test`, because `test` also runs spec-align, which keeps
# its own named step here; listing `test` would run internal/specalign twice
# (SI-266). It lists the e2e shards (e2e-1 e2e-2 e2e-3), not `e2e`, which
# runs the same three concurrently (SI-268); local `make verify` stays serial
# and fail-fast.
#
# The pull-request gate (.github/workflows/merge-gate.yml) runs exactly these
# steps, split across parallel jobs, plus the post-verify self-lint; its
# required `merge-gate` job fails unless every one of those jobs succeeds.
# internal/specalign's TestMergeGateParity_GateJobsRunExactlyVerifySteps fails
# if a step is dropped from, duplicated in, or added to that workflow alone.
# That parity holds only while `make verify` runs VERIFY_STEPS and nothing
# else, so verify takes no prerequisites and its recipe below is pinned
# (TestGateParity_VerifyRunsOnlyItsStepLoop): add a check to VERIFY_STEPS,
# never to verify's rule. Two more guards read this file's source for what
# `make -n` cannot show. TestGateParity_GateRecipesNeverIgnoreErrors fails if:
#   - the Makefile declares .IGNORE, .ONESHELL, .POSIX, .SILENT, or
#     .RECIPEPREFIX;
#   - any line names MAKEFLAGS, MFLAGS, GNUMAKEFLAGS, or GOFLAGS, or assigns
#     SHELL, .SHELLFLAGS, or MAKE;
#   - a recipe line of a gate target (a VERIFY_STEPS entry, a test shard,
#     test, verify, the whole-suite e2e, or anything they pull in) carries a
#     `-` prefix, written, through a leading variable, or on any line of a
#     define value the line reaches; or runs a sub-make with -i, -k, -n, -t,
#     or -q, written or in a value the line reaches;
#   - a gate target's recipe cannot be read.
# TestGateParity_GateVariablesAssignedOnceAndNothingIncluded fails unless
# VERIFY_STEPS, CROSS_BINARY_PKGS, TEST_CMD_PKGS, SPEC_ALIGN_PKGS,
# TEST_SLOW_PKGS, TEST_REST_PKGS, and E2E_SHARD_1..3 are each assigned exactly
# once, with `=` or `:=`, and the Makefile includes or evals no other makefile
# text: the guards read only the first assignment, and only this file.
VERIFY_STEPS := build fmt-check vet lint test-cmd test-cross test-slow test-rest fixture lint-store spec-align lint-showcase showcase-coverage e2e-1 e2e-2 e2e-3
GATE_TIMINGS ?= .verdi/data/gate/timings.tsv

verify:
	@mkdir -p $(dir $(GATE_TIMINGS)); \
	run=$$(date -u +%Y-%m-%dT%H:%M:%SZ); head=$$(git rev-parse --short=12 HEAD 2>/dev/null || echo unknown); \
	total=0; summary=""; \
	for step in $(VERIFY_STEPS); do \
		start=$$(date +%s); \
		if $(MAKE) --no-print-directory $$step; then status=ok; else status=fail; fi; \
		secs=$$(( $$(date +%s) - start )); total=$$(( total + secs )); \
		printf '%s\t%s\t%s\t%s\t%s\n' "$$run" "$$head" "$$step" "$$secs" "$$status" >> $(GATE_TIMINGS); \
		summary="$$summary$$(printf '  %-18s %6ss  %s' "$$step" "$$secs" "$$status")\n"; \
		if [ "$$status" = fail ]; then \
			printf 'verify: step %s FAILED after %ss\n%b' "$$step" "$$secs" "$$summary" >&2; exit 1; \
		fi; \
	done; \
	printf 'verify timings (run %s @ %s, appended to %s):\n%b  %-18s %6ss\n' "$$run" "$$head" "$(GATE_TIMINGS)" "$$summary" total "$$total"; \
	echo "verify OK"

# hooks installs this repository's git hooks (scripts/githooks/) into the
# SHARED hooks folder — `git rev-parse --git-common-dir`/hooks — so every
# worktree of the repository runs them, whatever branch it has checked out.
# Today: pre-commit, which builds the STAGED tree (process-audit PA-004).
hooks:
	@dir="$$(git rev-parse --git-common-dir)/hooks"; mkdir -p "$$dir"; \
	install -m 0755 scripts/githooks/pre-commit "$$dir/pre-commit"; \
	echo "hooks: installed pre-commit -> $$dir/pre-commit"

tidy:
	go mod tidy

# Auxiliary local paired release proof. Both source/build identities and baseline
# paths are mandatory command arguments; ordinary verify composition is unchanged.
.PHONY: public-execution-contract-release
public-execution-contract-release:
	mkdir -p .build
	go build -trimpath -o .build/public-execution-contract-release ./cmd/public-execution-contract-release
	.build/public-execution-contract-release $(PUBLIC_RELEASE_ARGS)
