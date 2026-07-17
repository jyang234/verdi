// TestCLIShowcaseCoverage and TestCLIShowcaseGC (Task 3.4, CLI axis) drive
// every remaining CLI-verb capability gap against a real, provisioned
// examples/showcase store via runBinary — the exact same
// build-then-exec-the-real-binary discipline TestShowcaseLintClean (Task
// 3.1, cli:lint) and cmd/verdi/{matrix_test.go,sync_test.go} (cli:matrix,
// cli:sync) already established, extended here to cover the rest of the
// v0 verb table.
//
// Every assertion below is checked against REAL examples/showcase content
// (spec/stale-decline, spec/borrower-update-api, the real committed
// borrower-update-mobile deviation report, the real STORY-1482 board) —
// never a synthetic fixture. Several results below are deterministic
// CONSEQUENCES of real, disclosed facts about the showcase store as
// committed, not weakened assertions:
//
//   - `verdi audit` exits 1 (FLAGGED) because
//     .verdi/specs/active/borrower-update-mobile/deviation-report.md
//     carries a real accepted-deviation finding whose id (ac-1) equals
//     that story's own declared AC id — evidence.SpecStale's trigger (a)
//     — exactly the "deterministic, showcased violated-with-witness
//     outcome" the public-rollout plan's Task 1.6 names.
//   - `verdi align` exits 2 (operational) because examples/showcase's own
//     committed verdi.yaml carries no `toolchain:` block (I-4) —
//     align.Compute refuses outright rather than silently skipping the
//     boundary-diff section (internal/align/computed.go: "no toolchain
//     configured"). This is a genuine, disclosed fact about the showcase
//     store, not a workaround; `verdi gate`'s condition 3 (below) fails as
//     a direct, honest downstream consequence — no deviation-report.md was
//     ever written.
//   - `verdi gate` (on a build branch cut from the real, accepted
//     spec/borrower-update-api): conditions 1/2/4 hold for real against
//     the committed corpus (accepted-pending-build on the default branch;
//     no violated AC; no unresolved cascade); condition 3 fails per the
//     align finding above. Overall: gate: FAIL, exit 1.
//   - `verdi close` (on spec/borrower-update-api): the closure gate's
//     eligibility condition fails for real — this harness's
//     provisionShowcaseStore deliberately does not copy
//     examples/showcase/derived/ (helpers_test.go's own disclosed gap), so
//     borrower-update-api's ac-1 carries no evidence and is not eligible.
//     close: FAIL, exit 1 — reached WITHOUT ever calling the jira
//     provider's PublishRollup (runClose returns before that point), so no
//     verdi.yaml patch is needed for this one, unlike rollup below.
//   - `verdi rollup --publish` (spec/stale-decline): ALWAYS calls
//     PublishRollup regardless of eligibility, and examples/showcase's own
//     verdi.yaml configures a REAL (non-fake) jira provider — so this
//     subtest patches its own provisioned store's verdi.yaml (a
//     working-tree-only edit, nothing committed to examples/showcase) to
//     providers.jira.mode: fake, the round-6 hermetic switch
//     (cmd/verdi/rollup.go's buildProviderRegistry) built exactly for this
//     — never touching the network (CLAUDE.md: "No network in any test").
//     The published fold matches get_matrix's own finding in
//     mcp_showcase_test.go exactly (ac-4 waived, the rest no-signal,
//     eligible=false) — the same real showcase content, cross-checked from
//     a second, independent code path.
//   - `verdi board commit` (STORY-1482): examples/showcase's own real
//     board.json/annotation-stream fixtures — copied by hand into the
//     provisioned store's mutable zone, since provisionShowcaseStore's own
//     doc comment discloses that zone is present-but-empty by
//     construction — drive the real commit-to-design ritual end to end.
//   - `verdi design start` / `verdi accept`: a freshly scaffolded spec
//     (placeholder content — there is no CLI verb to author new committed
//     showcase narrative content, and Task 3.4 does not add one) is
//     designed and accepted against the real showcase git history
//     underneath it, exercising both verbs' real entry points for real.
//   - `verdi dex build`: renders a real static site whose output contains
//     spec/stale-decline's real, committed title.
//   - `verdi disposition` (spec/disposition-verb, TestCLIShowcaseDisposition):
//     driven against the real, LIVING borrower-update-mobile deviation
//     report — the only non-frozen one examples/showcase carries — exits 2
//     (operational), a genuine, disclosed fact about this specific fixture:
//     its body is hand-authored narrative prose, never produced by a real
//     align run (no toolchain: block in this store's verdi.yaml, same gap
//     `verdi align` above discloses), so it carries none of
//     align.RenderBody's mechanical per-finding bullet lines the verb must
//     locate and replace to keep body and frontmatter in agreement (dc-2).
//     The verb's fail-closed refusal, naming the finding it could not
//     safely reconcile, IS the correct behavior here — not a workaround.
//
// cli:feature is DELIBERATELY EXCLUDED from the enumerated capability set
// below (cliVerbs, coverage_test.go) — see that function's own comment and
// PLAN-V1.md ledger entry R4-I-54: `feature` is a pure deprecation alias.
// runFeatureStart shares runBuildStart with `build` (every precondition and
// side effect), differing only by a printed R4-I-6 deprecation notice on
// stderr — so `cli:build`'s coverage below already proves the one build
// code path both verb names share.
//
// cli:serve is exercised by the WHOLE Playwright suite (cmd/e2eharness/
// main.go launches the real `verdi serve --http <addr>` subprocess every
// e2e run is served from — never a fake) — mapped in coverage_test.go to
// an existing SHOWCASE.-marked spec rather than re-proving server startup
// here. cli:mcp maps to mcp_showcase_test.go (this package, Task 3.4's MCP
// axis).
package showcasealign

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/wtmanager"
)

func TestCLIShowcaseCoverage(t *testing.T) {
	t.Run("audit", func(t *testing.T) {
		root := provisionShowcaseStore(t)

		// Copy the REAL committed borrower-update-mobile/deviation-report.md
		// into place: layers.txt's own header comment discloses that this
		// file is deliberately NOT layers.txt-tracked (its sibling spec.md
		// belongs to the pre-existing "v2 fixture overlay" set), so
		// provisionShowcaseStore's fixturegit reconstruction never commits
		// it — a real, disclosed gap in this harness's construction,
		// exactly like the board/annotation files above. decisionsweep's
		// ScanSpecStale reads deviation-report.md straight off disk (never
		// through git), so supplying the genuine file's own bytes by hand
		// exercises the real spec-stale trigger the public-rollout plan's
		// Task 1.6 documents, on real content, without inventing any.
		reportData := readShowcaseFile(t, ".verdi/specs/active/borrower-update-mobile/deviation-report.md")
		writeTestFile(t, filepath.Join(root, ".verdi", "specs", "active", "borrower-update-mobile", "deviation-report.md"), reportData)

		stdout, stderr, code := runBinary(t, root, "audit")
		if code != 1 {
			t.Fatalf("verdi audit: exit %d, want 1 (a real SPEC-STALE finding on the committed borrower-update-mobile deviation report)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "SPEC-STALE") {
			t.Fatalf("verdi audit stdout missing SPEC-STALE:\n%s", stdout)
		}
		if !strings.Contains(stdout, "spec/borrower-update-mobile: SPEC-STALE") {
			t.Fatalf("verdi audit stdout missing the real borrower-update-mobile SPEC-STALE finding:\n%s", stdout)
		}
		if !strings.Contains(stdout, "audit: FLAGGED") {
			t.Fatalf("verdi audit stdout missing the FLAGGED verdict:\n%s", stdout)
		}
	})

	t.Run("dex_build", func(t *testing.T) {
		root := provisionShowcaseStore(t)
		outDir := filepath.Join(t.TempDir(), "site")
		stdout, stderr, code := runBinary(t, root, "dex", "build", "-o", outDir)
		if code != 0 {
			t.Fatalf("verdi dex build: exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}

		found := false
		walkErr := filepath.WalkDir(outDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, rerr := os.ReadFile(path)
			if rerr == nil && strings.Contains(string(data), "Stale decline handling") {
				found = true
			}
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walking dex build output %s: %v", outDir, walkErr)
		}
		if !found {
			t.Fatalf("dex build output at %s does not contain spec/stale-decline's real committed title anywhere", outDir)
		}
	})

	t.Run("board_commit", func(t *testing.T) {
		root := provisionShowcaseStore(t)
		ctx := context.Background()

		// Copy the REAL showcase board fixture + its real annotation
		// stream into place: provisionShowcaseStore's own doc comment
		// discloses the mutable zone is present-but-empty by construction
		// (helpers_test.go), so this test supplies the genuine committed
		// content by hand rather than inventing new board content.
		boardData := readShowcaseFile(t, "mutable/boards/STORY-1482.json")
		writeTestFile(t, filepath.Join(boardio.BoardsDir(root), "STORY-1482.json"), boardData)

		annoData := readShowcaseFile(t, "mutable/annotations/spec--stale-decline.jsonl")
		annoFile := boardio.AnnotationFileForBoard(store.RefSlug("STORY-1482"))
		writeTestFile(t, filepath.Join(boardio.AnnotationsDir(root), annoFile), annoData)

		if err := gitx.CheckoutNewBranch(ctx, root, "design/showcase-board-commit-check"); err != nil {
			t.Fatalf("checking out a design branch for board commit: %v", err)
		}

		stdout, stderr, code := runBinary(t, root, "board", "commit", "STORY-1482",
			"--name", "showcase-board-commit-check", "--story-ref", "jira:LOAN-1482")
		if code != 0 {
			t.Fatalf("verdi board commit: exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}

		specPath := filepath.Join(root, ".verdi", "specs", "active", "showcase-board-commit-check", "spec.md")
		data, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatalf("reading board-committed spec: %v", err)
		}
		content := string(data)
		if !strings.Contains(content, "spec/stale-decline@f6dd4c4df724c0b16cae435e96f7e34ac94026c9") {
			t.Fatalf("board-committed spec missing the real showcase board's pinned context ref:\n%s", content)
		}
		if !strings.Contains(content, "a-01J8Z0K3AAAAAAAAAAAAAAAAAA") {
			t.Fatalf("board-committed spec missing the real showcase board's sticky dispositions:\n%s", content)
		}
	})

	t.Run("rollup_publish", func(t *testing.T) {
		root := provisionShowcaseStore(t)

		// Patch verdi.yaml (working-tree only — nothing committed to
		// examples/showcase itself) to select the round-6 hermetic fake
		// jira provider instead of the real adapter examples/showcase's
		// own verdi.yaml configures: `rollup --publish` ALWAYS calls
		// PublishRollup regardless of eligibility (unlike close, which
		// fails closed before ever reaching it), so this store's real
		// (non-fake) providers.jira block would otherwise dial a real
		// network host — forbidden (CLAUDE.md: "No network in any
		// test"). store.Manifest's providers.jira.mode: fake is exactly
		// the config-only switch spec/close-verb dc-2 built for this.
		yamlPath := filepath.Join(root, ".verdi", "verdi.yaml")
		data, err := os.ReadFile(yamlPath)
		if err != nil {
			t.Fatalf("reading verdi.yaml: %v", err)
		}
		const marker = "rollup_field: customfield_00000\n"
		if !strings.Contains(string(data), marker) {
			t.Fatalf("verdi.yaml does not contain the expected jira provider block; cannot patch mode: fake safely:\n%s", data)
		}
		patched := strings.Replace(string(data), marker, marker+"    mode: fake\n", 1)
		if err := os.WriteFile(yamlPath, []byte(patched), 0o644); err != nil {
			t.Fatalf("writing patched verdi.yaml: %v", err)
		}

		stdout, stderr, code := runBinary(t, root, "rollup", "spec/stale-decline", "--publish", "--force-local")
		if code != 0 {
			t.Fatalf("verdi rollup --publish: exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "rollup: published jira:LOAN-1482") {
			t.Fatalf("verdi rollup stdout missing the real showcase story ref jira:LOAN-1482:\n%s", stdout)
		}
		// Matches mcp_showcase_test.go's get_matrix finding on the same
		// real spec/stale-decline content, from an independent code path:
		// ac-4's real, active waiver aside, nothing else here carries
		// derived evidence under this harness's provisioning.
		if !strings.Contains(stdout, "eligible=false") {
			t.Fatalf("verdi rollup stdout: want eligible=false (ac-1..ac-3 carry no derived evidence under this harness's provisioning), got:\n%s", stdout)
		}
	})

	t.Run("design_start_then_accept", func(t *testing.T) {
		root := provisionShowcaseStore(t)

		stdout, stderr, code := runBinary(t, root, "design", "start", "--kind", "feature", "--name", "showcase-lifecycle-check")
		if code != 0 {
			t.Fatalf("verdi design start: exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
		specPath := filepath.Join(root, ".verdi", "specs", "active", "showcase-lifecycle-check", "spec.md")
		draft, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatalf("reading freshly designed spec: %v", err)
		}
		if !strings.Contains(string(draft), "status: draft") {
			t.Fatalf("freshly designed spec is not status: draft:\n%s", draft)
		}

		stdout2, stderr2, code2 := runBinary(t, root, "accept", "spec/showcase-lifecycle-check")
		if code2 != 0 {
			t.Fatalf("verdi accept: exit %d\nstdout:\n%s\nstderr:\n%s", code2, stdout2, stderr2)
		}
		accepted, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatalf("reading accepted spec: %v", err)
		}
		if !strings.Contains(string(accepted), "status: accepted-pending-build") {
			t.Fatalf("accepted spec did not flip to accepted-pending-build:\n%s", accepted)
		}
		if !strings.Contains(string(accepted), "frozen: {") {
			t.Fatalf("accepted spec missing its frozen stamp:\n%s", accepted)
		}
	})

	t.Run("build_start_then_align_then_gate", func(t *testing.T) {
		root := provisionShowcaseStore(t)
		t.Setenv("CI_DEFAULT_BRANCH", "main")

		stdout, stderr, code := runBinary(t, root, "build", "start", "spec/borrower-update-api")
		if code != 0 {
			t.Fatalf("verdi build start: exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "feature/borrower-update-api") {
			t.Fatalf("verdi build start stdout missing the real story's build branch:\n%s", stdout)
		}

		alignOut, alignErr, alignCode := runBinary(t, root, "align")
		if alignCode != 2 {
			t.Fatalf("verdi align: exit %d, want 2 (examples/showcase's own verdi.yaml carries no toolchain: block)\nstdout:\n%s\nstderr:\n%s", alignCode, alignOut, alignErr)
		}
		if !strings.Contains(alignErr, "no toolchain configured") {
			t.Fatalf("verdi align stderr missing the disclosed no-toolchain reason:\n%s", alignErr)
		}

		gateOut, gateErr, gateCode := runBinary(t, root, "gate")
		if gateCode != 1 {
			t.Fatalf("verdi gate: exit %d, want 1\nstdout:\n%s\nstderr:\n%s", gateCode, gateOut, gateErr)
		}
		if !strings.Contains(gateOut, "[PASS] 1.") {
			t.Fatalf("gate condition 1 (accepted-pending-build on the default branch) should PASS for the real committed borrower-update-api:\n%s", gateOut)
		}
		if !strings.Contains(gateOut, "[PASS] 2.") {
			t.Fatalf("gate condition 2 (no AC violated) should PASS (no derived evidence to violate):\n%s", gateOut)
		}
		if !strings.Contains(gateOut, "[FAIL] 3.") {
			t.Fatalf("gate condition 3 (fresh alignment report) should FAIL — align never wrote one:\n%s", gateOut)
		}
		if !strings.Contains(gateOut, "[PASS] 4.") {
			t.Fatalf("gate condition 4 (rung-4 cascade) should PASS:\n%s", gateOut)
		}
		if !strings.Contains(gateOut, "gate: FAIL") {
			t.Fatalf("gate stdout missing the overall FAIL verdict:\n%s", gateOut)
		}
	})

	t.Run("close", func(t *testing.T) {
		root := provisionShowcaseStore(t)
		stdout, stderr, code := runBinary(t, root, "close", "spec/borrower-update-api", "--force-local")
		if code != 1 {
			t.Fatalf("verdi close: exit %d, want 1\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
		if !strings.Contains(stdout, "close: FAIL") {
			t.Fatalf("verdi close stdout missing the overall FAIL verdict:\n%s", stdout)
		}
		if !strings.Contains(stdout, "[FAIL] closure: 1.") {
			t.Fatalf("verdi close stdout missing the real closure-eligibility failure (no derived evidence under this harness's provisioning):\n%s", stdout)
		}
	})
}

// TestCLIShowcaseGC drives `verdi gc` (cli:gc) against a real
// examples/showcase-provisioned store carrying two managed worktrees cut
// via the real wtmanager.EnsureWorktree path (mirroring
// internal/wtmanager/gc_test.go's own cutManagedWorktree convention — no
// CLI verb exists to create a managed worktree in v0, since
// EnsureWorktree is a workbench-internal, not-yet-CLI-wired seam
// (spec/worktree-manager), so this test's SETUP calls it directly while
// still driving the VERB UNDER TEST, gc itself, through runBinary): one
// branch that is already an ancestor of main (reclaim-eligible) and one
// that carries a real, unmerged commit (kept). Both outcomes are asserted
// against the real, disclosed `gc: scope` line (spec/worktree-manager
// ac-5) every run prints.
func TestCLIShowcaseGC(t *testing.T) {
	root := provisionShowcaseStore(t)
	ctx := context.Background()
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	// provisionShowcaseStore leaves genuine, untracked showcase-support
	// files sitting in the working tree (the loansvc service-discovery
	// fixture, writeLoansvcFixture) — gitx.Checkout's own branch-switch
	// guard (05 §Workbench) refuses to switch branches while ANYTHING is
	// uncommitted, untracked files included (gitx.StatusDirty runs `git
	// status --porcelain` unfiltered). Committing them on main once, up
	// front, is what a real user would do before cutting throwaway
	// branches too — not a workaround, just real git hygiene — and every
	// branch cut below inherits this same clean, real content unchanged.
	if err := gitx.AddAll(ctx, root); err != nil {
		t.Fatalf("staging provisioned fixtures on main: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, root, "test setup: commit provisioned untracked fixtures"); err != nil {
		t.Fatalf("committing provisioned fixtures on main: %v", err)
	}

	mergedBranch := "design/gc-showcase-merged"
	if err := gitx.CheckoutNewBranch(ctx, root, mergedBranch); err != nil {
		t.Fatalf("cutting %s: %v", mergedBranch, err)
	}
	if err := gitx.Checkout(ctx, root, "main"); err != nil {
		t.Fatalf("returning to main: %v", err)
	}

	activeBranch := "design/gc-showcase-active"
	if err := gitx.CheckoutNewBranch(ctx, root, activeBranch); err != nil {
		t.Fatalf("cutting %s: %v", activeBranch, err)
	}
	writeTestFile(t, filepath.Join(root, "gc-showcase-check.txt"), "in progress\n")
	if err := gitx.AddAll(ctx, root); err != nil {
		t.Fatalf("staging the active branch's own commit: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, root, "wip: gc showcase check"); err != nil {
		t.Fatalf("committing the active branch: %v", err)
	}
	if err := gitx.Checkout(ctx, root, "main"); err != nil {
		t.Fatalf("returning to main: %v", err)
	}

	mergedPath, err := wtmanager.EnsureWorktree(ctx, root, mergedBranch)
	if err != nil {
		t.Fatalf("EnsureWorktree(%s): %v", mergedBranch, err)
	}
	activePath, err := wtmanager.EnsureWorktree(ctx, root, activeBranch)
	if err != nil {
		t.Fatalf("EnsureWorktree(%s): %v", activeBranch, err)
	}

	stdout, stderr, code := runBinary(t, root, "gc")
	if code != 0 {
		t.Fatalf("verdi gc: exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "reclaimed: gc-showcase-merged") {
		t.Fatalf("verdi gc stdout missing the reclaimed, merged worktree:\n%s", stdout)
	}
	if !strings.Contains(stdout, "kept: not eligible") || !strings.Contains(stdout, "gc-showcase-active") {
		t.Fatalf("verdi gc stdout missing the kept, still-active worktree:\n%s", stdout)
	}
	if !strings.Contains(stdout, "gc: scope") {
		t.Fatalf("verdi gc stdout missing the mandatory dc-5 scope disclosure:\n%s", stdout)
	}

	if _, err := os.Stat(mergedPath); !os.IsNotExist(err) {
		t.Fatalf("reclaimed worktree %s still exists on disk (err=%v)", mergedPath, err)
	}
	if _, err := os.Stat(activePath); err != nil {
		t.Fatalf("kept worktree %s no longer exists on disk: %v", activePath, err)
	}
}

// TestCLIShowcaseAttest (cli:attest, spec/attest-helper) drives `verdi
// attest` against a real (story, AC) pair from examples/showcase:
// spec/borrower-update-api (class: story, story: jira:LOAN-1482, its one
// declared ac-1) carries no attestation file at its fold path today — the
// showcase corpus's only committed attestation under that same story-ref
// slug, jira-loan-1482/ac-2.md, is for ac-2 (which borrower-update-api does
// not declare at all; it verifies spec/stale-decline instead, the
// class: feature spec sharing the same story ref — outside VL-022's
// story-scoped subject (Controller adjudication ADJ-51), skipped rather
// than refused, needing no baseline map). The spec-ref form is used
// deliberately, never the
// scheme-prefixed jira:LOAN-1482 form: that scheme-prefixed ref resolves to
// spec/stale-decline instead (storyresolve.Resolve's own matchStoryRef is
// permanently feature-class-only), exactly the two-form-contract nuance
// spec/attest-helper's classifyPair (cmd/verdi/attest.go) documents and
// resolveBuildTarget (cmd/verdi/buildstart.go) already solves.
func TestCLIShowcaseAttest(t *testing.T) {
	root := provisionShowcaseStore(t)

	stdout, stderr, code := runBinary(t, root, "attest", "spec/borrower-update-api", "ac-1")
	if code != 0 {
		t.Fatalf("verdi attest: exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	path := filepath.Join(root, ".verdi", "attestations", "jira-loan-1482", "ac-1.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading scaffolded attestation at the real fold path %s: %v", path, err)
	}
	content := string(data)

	if !strings.Contains(content, "id: attestation/jira-loan-1482--ac-1") {
		t.Fatalf("scaffold id wrong:\n%s", content)
	}
	if !strings.Contains(content, `ref: "spec/borrower-update-api"`) {
		t.Fatalf("scaffold verifies edge wrong:\n%s", content)
	}
	if !strings.Contains(content, `owners: ["platform-team"]`) {
		t.Fatalf("scaffold owners not copied verbatim from the real, committed story spec:\n%s", content)
	}
	if !strings.Contains(content, "<!-- verdi:attestation-unauthored -->") {
		t.Fatalf("scaffold missing the unauthored marker (parent spec/closure-ergonomics dc-2):\n%s", content)
	}
	if !strings.Contains(stdout, path) {
		t.Fatalf("verdi attest stdout missing the scaffolded path:\n%s", stdout)
	}

	// The already-exists refusal, against the SAME real showcase path this
	// story's own AC-2 targets: a second attest call for the exact same
	// (story, AC) refuses (exit 1, verdict) rather than overwriting the
	// scaffold just written.
	stdout2, stderr2, code2 := runBinary(t, root, "attest", "spec/borrower-update-api", "ac-1")
	if code2 != 1 {
		t.Fatalf("verdi attest (already exists): exit %d, want 1\nstdout:\n%s\nstderr:\n%s", code2, stdout2, stderr2)
	}
	if !strings.Contains(stderr2, path) {
		t.Fatalf("verdi attest (already exists) stderr missing the offending path:\n%s", stderr2)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s after the refused second call: %v", path, err)
	}
	if string(after) != content {
		t.Fatalf("the scaffolded attestation's bytes changed after a refused second call — dc-2 forbids overwriting a human-ownable record")
	}
}

// TestCLIShowcaseDisposition drives `verdi disposition` (cli:disposition,
// spec/disposition-verb) against the REAL committed
// borrower-update-mobile/deviation-report.md — the only LIVING (non-frozen)
// deviation report examples/showcase carries (its two archived siblings,
// refi-rate-check-2024 and loan-refi-2023, are both frozen, so co-3 refuses
// them unconditionally, a weaker proof than exercising the real reconciliation
// logic below).
//
// That file's body is genuinely hand-authored narrative prose (predating
// this story — illustrative showcase content, never produced by a real
// `verdi align` run: examples/showcase's own committed verdi.yaml carries no
// toolchain: block, so align.Compute cannot run against this store at all,
// per this file's own package doc comment above), not align.RenderBody's
// mechanical per-finding bullet form dc-2 requires the verb to locate and
// replace so the frontmatter write and the human-legible body stay in
// agreement. The real, disclosed, and CORRECT outcome driving the verb
// against it is therefore a fail-closed verdict refusal naming the
// finding it could not safely reconcile — never a silent, structurally
// unsound body edit — exactly the same "real, disclosed fact about the
// showcase store" pattern this file's own doc comment already uses for
// `verdi align`/`verdi close` above. This proves the verb's core safety
// property (dc-2: never write a body that falls out of agreement with the
// frontmatter) against genuine content, not a synthetic fixture built to
// make it succeed; disposition-verb's ac-1/ac-2/ac-3 obligations are proven
// in full, on real align-generated reports, by cmd/verdi/disposition_test.go.
//
// Exit code: body/frontmatter desync is a VERDICT (exit 1), not an
// operational error — ADJ-53 j-5 (shipped behavior, cmd/verdi/disposition.go)
// reclassified it, and this assertion is aligned to that shipped behavior
// under ADJ-55 (a cross-story correction: PR #115 landed the reclassification
// without updating this showcase test, leaving origin/main red).
func TestCLIShowcaseDisposition(t *testing.T) {
	root := provisionShowcaseStore(t)

	reportData := readShowcaseFile(t, ".verdi/specs/active/borrower-update-mobile/deviation-report.md")
	reportPath := filepath.Join(root, ".verdi", "specs", "active", "borrower-update-mobile", "deviation-report.md")
	writeTestFile(t, reportPath, reportData)

	stdout, stderr, code := runBinary(t, root, "disposition", "spec/borrower-update-mobile", "f-2", "accepted-deviation", "--rationale", "confirmed by the showcase coverage check", "--amend")
	if code != 1 {
		t.Fatalf("verdi disposition: exit %d, want 1 (verdict — body/frontmatter desync is a verdict per ADJ-53 j-5; the real committed report's body is hand-authored prose, not align.RenderBody's mechanical bullet form)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "f-2") {
		t.Fatalf("stderr = %q, want it to name the finding (f-2) it could not safely reconcile", stderr)
	}

	after, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("reading report after refusal: %v", err)
	}
	if string(after) != reportData {
		t.Fatalf("the real committed report was written to despite a refusal (no partial write allowed):\n--- before ---\n%s\n--- after ---\n%s", reportData, after)
	}
}

// TestCLIShowcaseModel drives `verdi model check` (cli:model,
// extensibility phase 1, spec/model-schema ac-3) against the real
// provisioned examples/showcase store. examples/showcase carries no
// .verdi/model.yaml of its own (grep-verified: this corpus predates
// verdi.model/v1 entirely) — a genuine, disclosed fact about the showcase
// store as committed, exactly like the align/close/rollup notes in this
// file's own package doc comment above — so this exercises the real
// absent-model.yaml path (store.Open resolving to model.Canonical()) end
// to end, over real showcase-sourced store content, never a synthetic
// fixture: exit 0, with an OK line naming the schema and canonical's own
// class/transition counts and digest.
func TestCLIShowcaseModel(t *testing.T) {
	root := provisionShowcaseStore(t)

	if _, err := os.Stat(filepath.Join(root, ".verdi", "model.yaml")); !os.IsNotExist(err) {
		t.Fatalf("test setup: provisioned showcase store unexpectedly carries a .verdi/model.yaml (stat err=%v) — this test's whole premise is the absent-file path", err)
	}

	stdout, stderr, code := runBinary(t, root, "model", "check")
	if code != 0 {
		t.Fatalf("verdi model check: exit %d, want 0 (examples/showcase carries no model.yaml, so this resolves to the embedded canonical default)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "model: OK — verdi.model/v1, ") {
		t.Fatalf("stdout = %q, want it to start with the OK line", stdout)
	}
	wantDigest, err := model.Canonical().Digest()
	if err != nil {
		t.Fatalf("model.Canonical().Digest(): %v", err)
	}
	if !strings.Contains(stdout, wantDigest) {
		t.Fatalf("stdout = %q, want it to contain the canonical model's own digest %q", stdout, wantDigest)
	}
	if !strings.Contains(stdout, "2 classes") || !strings.Contains(stdout, "4 transitions") {
		t.Fatalf("stdout = %q, want it to name canonical's 2 classes / 4 transitions", stdout)
	}
}
