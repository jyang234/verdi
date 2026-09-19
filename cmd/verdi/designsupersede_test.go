package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/supersede"
)

// supersedeManifestYAML is the minimal verdi.yaml every designsupersede
// fixture repo needs — no toolchain: block, so baseline regeneration
// best-effort skips (baseline.go treats a nil Runner as "skip
// gracefully", matching this file's own designDeps{Runner: nil}
// convention elsewhere in this package).
const supersedeManifestYAML = `schema: verdi.layout/v1
forge: gitlab
`

// lockboxPredecessor is the rich accepted-feature fixture Contract C asks
// for: two ACs, one constraint, one decision, one open question, one plain
// stub and one spike stub with resolves, a fragment (non-supersedes) link,
// and a non-default frontmatter field (impacts) — so a single fixture
// exercises verbatim copying AND stays lint-clean (every AC declares
// attestation, 03 §The feature fold's outcome floor; every anchor resolves
// against an exact body heading, R4-I-15/VL-006).
const lockboxPredecessor = `---
id: spec/lockbox
kind: spec
class: feature
title: "Lockbox (fixture)"
owners: [platform-team]
impacts: [loansvc]
problem: { text: "borrowers cannot secure a shared document", anchor: "#problem" }
outcome: { text: "borrowers can secure a shared document", anchor: "#outcome" }
links:
  - { type: exempts, ref: "adr/0099-lockbox-exempt" }
acceptance_criteria:
  - { id: ac-1, text: "a borrower can lock a document", evidence: [static, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "a borrower can unlock a document", evidence: [behavioral, attestation], anchor: "#ac-2" }
constraints:
  - { id: co-1, text: "a lock expires after 24 hours", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "use a signed token for the lock", anchor: "#dc-1" }
open_questions:
  - { id: oq-1, text: "should a lock be transferable", anchor: "#oq-1" }
stubs:
  - { slug: lockbox-api, acceptance_criteria: [ac-1, ac-2] }
  - { slug: lockbox-spike, spike: true, resolves: [oq-1] }
---
# Lockbox (fixture)

## Problem

Borrowers cannot secure a shared document.

## Outcome

Borrowers can secure a shared document.

## AC-1

A borrower can lock a document.

## AC-2

A borrower can unlock a document.

## CO-1

A lock expires after 24 hours.

## DC-1

Use a signed token for the lock.

## OQ-1

Should a lock be transferable.
`

// lockboxExemptADR resolves the predecessor/successor's own fragment
// (non-supersedes) "exempts" link — VL-003 fails closed on a dangling ref,
// so a fixture wanting a real lint-clean run needs a real target, not a
// bare unresolved name.
const lockboxExemptADR = `---
id: adr/0099-lockbox-exempt
kind: adr
title: "Lockbox exempt"
status: proposed
owners: [platform-team]
---
body
`

// behindCheckoutFixtureSpec is a minimal, strict-decodable feature spec used
// only as UAT-031's own "a name already landed on main" collision probe
// (TestRunDesignStartSupersede_Negative's "behind checkout" case) — its
// content is never inspected, only its presence at the target path, but it
// must still strict-decode: supersede.Resolve's predecessor-status
// projection corpus-scans every spec on the default branch (both zones)
// looking for successors, and a malformed spec anywhere in that scan fails
// the scan closed (disclosed-unproven) rather than merely skipping it.
const behindCheckoutFixtureSpec = `---
id: spec/taken-on-main
kind: spec
class: feature
title: "Taken on main (fixture)"
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a", evidence: [static], anchor: "#ac-1" }
---
# Taken on main (fixture)

## Problem

p

## Outcome

o

## AC-1

a
`

// buildSupersedeRepo builds a one-layer fixturegit repo carrying verdi.yaml
// plus the lockbox predecessor, landed directly on main (the merge-signaled
// "accepted" shape: no frozen:/status:, exact bytes already on the default
// branch).
func buildSupersedeRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                   supersedeManifestYAML,
				".verdi/specs/active/lockbox/spec.md": lockboxPredecessor,
				".verdi/adr/0099-lockbox-exempt.md":   lockboxExemptADR,
			},
			Message: "lockbox lands",
		},
	})
}

// TestRunDesignStartSupersede_Happy is the unit-level proof (runs the
// testable core directly, mirroring design_test.go's own
// TestRunDesignStart_Happy): exit 0, the new branch checked out, the
// successor's on-disk bytes are EXACTLY what supersede.Compose itself
// produces from the same predecessor bytes (never a second, drifting
// rendering), and the extra "supersedes: N objects carried" disclosure
// line names the right count.
func TestRunDesignStartSupersede_Happy(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := buildSupersedeRepo(t)
	ctx := context.Background()

	predRaw, err := os.ReadFile(filepath.Join(repo.Dir, ".verdi", "specs", "active", "lockbox", "spec.md"))
	if err != nil {
		t.Fatalf("reading predecessor fixture: %v", err)
	}
	wantComposed, err := supersede.Compose(supersede.ComposeInput{
		PredecessorName: "lockbox",
		PredecessorRaw:  predRaw,
		SuccessorName:   "lockbox-v2",
	})
	if err != nil {
		t.Fatalf("supersede.Compose (test oracle): %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := runDesignStartSupersede(ctx, repo.Dir, "lockbox", "lockbox-v2", phase7Model(t), nil, fakeGoTest{}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStartSupersede = %d, want 0; stderr=%s", got, stderr.String())
	}

	branch, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "design/lockbox-v2" {
		t.Fatalf("CurrentBranch = %q, want design/lockbox-v2", branch)
	}

	gotContent, err := os.ReadFile(filepath.Join(repo.Dir, ".verdi", "specs", "active", "lockbox-v2", "spec.md"))
	if err != nil {
		t.Fatalf("reading successor: %v", err)
	}
	if !bytes.Equal(gotContent, wantComposed.Content) {
		t.Fatalf("successor bytes differ from supersede.Compose's own output:\ngot:\n%s\nwant:\n%s", gotContent, wantComposed.Content)
	}

	commits, err := gitx.Log(ctx, repo.Dir, "HEAD")
	if err != nil || len(commits) == 0 {
		t.Fatalf("gitx.Log: %v", err)
	}
	if want := "design start: supersede spec/lockbox as spec/lockbox-v2"; commits[0].Subject != want {
		t.Fatalf("HEAD commit subject = %q, want %q", commits[0].Subject, want)
	}

	out := stdout.String()
	for _, want := range []string{
		"design start: base ",
		"design start: switched checkout from ",
		"design start: created branch design/lockbox-v2",
		"design start: scaffolded spec/lockbox-v2 (kind: feature, state: proposed (derived until merge))",
		"design start: board: http://",
		"design start: supersedes spec/lockbox: 5 objects carried, 0 amended, 0 removed",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q; got:\n%s", want, out)
		}
	}
}

// TestRunDesignStartSupersede_Negative covers the unit-level operational
// refusals runDesignStartSupersede itself is responsible for (successor
// dir collision, invalid successor name, predecessor refusal propagation —
// internal/supersede's own resolve_test.go already proves every Resolve
// refusal reason individually, so this only proves the CLI forwards it).
func TestRunDesignStartSupersede_Negative(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	t.Run("successor dir already exists", func(t *testing.T) {
		repo := buildSupersedeRepo(t)
		ctx := context.Background()
		if err := os.MkdirAll(filepath.Join(repo.Dir, ".verdi", "specs", "active", "lockbox-v2"), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		var stdout, stderr bytes.Buffer
		got := runDesignStartSupersede(ctx, repo.Dir, "lockbox", "lockbox-v2", phase7Model(t), nil, fakeGoTest{}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runDesignStartSupersede = %d, want 2", got)
		}
		if !strings.Contains(stderr.String(), "already exists") {
			t.Fatalf("stderr = %q, want it to name the existing-dir refusal", stderr.String())
		}
	})

	t.Run("invalid successor name", func(t *testing.T) {
		repo := buildSupersedeRepo(t)
		ctx := context.Background()
		var stdout, stderr bytes.Buffer
		got := runDesignStartSupersede(ctx, repo.Dir, "lockbox", "Not_A_Valid_Name", phase7Model(t), nil, fakeGoTest{}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runDesignStartSupersede = %d, want 2", got)
		}
	})

	t.Run("predecessor not found", func(t *testing.T) {
		repo := buildSupersedeRepo(t)
		ctx := context.Background()
		var stdout, stderr bytes.Buffer
		got := runDesignStartSupersede(ctx, repo.Dir, "does-not-exist", "does-not-exist-v2", phase7Model(t), nil, fakeGoTest{}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runDesignStartSupersede = %d, want 2", got)
		}
		if !strings.Contains(stderr.String(), "not found") {
			t.Fatalf("stderr = %q, want the Resolve not-found refusal forwarded", stderr.String())
		}
	})

	// -- UAT-030/031/032 fix coverage ---------------------------------------

	// "fragment successor name" is UAT-030's own witness on --supersedes: a
	// "#fragment" suffix decorates a REFERENCE (02 §Identity and
	// references), never a spec's own name, but used to inherit
	// artifact.ParseRef's tolerance via the old 2-arg
	// supersede.ValidateSuccessorName.
	t.Run("fragment successor name", func(t *testing.T) {
		repo := buildSupersedeRepo(t)
		ctx := context.Background()
		var stdout, stderr bytes.Buffer
		got := runDesignStartSupersede(ctx, repo.Dir, "lockbox", "lockbox-v2#dc-1", phase7Model(t), nil, fakeGoTest{}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runDesignStartSupersede(fragment name) = %d, want 2; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "lockbox-v2#dc-1") {
			t.Fatalf("stderr = %q, want it to name the rejected successor name", stderr.String())
		}
		if strings.Contains(strings.ToLower(stderr.String()), "internal error") {
			t.Fatalf("stderr = %q, want operator-facing wording, never \"internal error\"", stderr.String())
		}
	})

	// "pinned successor name" is UAT-030's other half, and the one that
	// closes this lane's fourth item: before this fix, a pinned --name
	// DID already fail closed, but only deep inside supersede.Compose's
	// own self-validation (artifact.DecodeSpec refusing a pinned id:
	// field), surfacing as "internal error: composed successor failed
	// self-validation" — blaming the tool for operator input. The shared
	// predicate now catches it first, before Compose ever runs.
	t.Run("pinned successor name", func(t *testing.T) {
		repo := buildSupersedeRepo(t)
		ctx := context.Background()
		var stdout, stderr bytes.Buffer
		got := runDesignStartSupersede(ctx, repo.Dir, "lockbox", "lockbox-v2@abc1234", phase7Model(t), nil, fakeGoTest{}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runDesignStartSupersede(pinned name) = %d, want 2; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
		}
		if strings.Contains(strings.ToLower(stderr.String()), "internal error") {
			t.Fatalf("stderr = %q, want operator-facing wording, never \"internal error\" (the old bug this lane closes)", stderr.String())
		}
		if !strings.Contains(stderr.String(), "lockbox-v2@abc1234") {
			t.Fatalf("stderr = %q, want it to name the rejected successor name", stderr.String())
		}
	})

	// "archived successor name exists" is UAT-032's own witness: the CLI
	// checked only the active zone (supersede.ValidateSuccessorName's own
	// half), never the archive zone the board already checked.
	t.Run("archived successor name exists", func(t *testing.T) {
		repo := buildSupersedeRepo(t)
		ctx := context.Background()
		if err := os.MkdirAll(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "retired"), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		var stdout, stderr bytes.Buffer
		got := runDesignStartSupersede(ctx, repo.Dir, "lockbox", "retired", phase7Model(t), nil, fakeGoTest{}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runDesignStartSupersede(archived name) = %d, want 2; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "specs/archive/") {
			t.Fatalf("stderr = %q, want it to name the archive collision (guide 6.1)", stderr.String())
		}
	})

	// "behind checkout" is UAT-031's own witness on --supersedes: the
	// collision check used to stat only the current checkout's working
	// tree, while the new branch is cut from the resolved default branch.
	t.Run("successor name present on main, absent from behind checkout", func(t *testing.T) {
		repo := buildSupersedeRepo(t)
		ctx := context.Background()
		if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "side"); err != nil {
			t.Fatalf("CheckoutNewBranch(side): %v", err)
		}
		if err := gitx.CheckoutExisting(ctx, repo.Dir, "main"); err != nil {
			t.Fatalf("CheckoutExisting(main): %v", err)
		}
		specDir := filepath.Join(repo.Dir, ".verdi", "specs", "active", "taken-on-main")
		if err := os.MkdirAll(specDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// A real (strict-decodable) spec, not a one-line placeholder: this
		// lands on main's ACTIVE zone, which supersede.Resolve's own
		// predecessor-status projection also corpus-scans (internal/
		// specstate's successorCorpus, both zones) to check whether
		// "lockbox" itself is superseded by anything — a malformed spec
		// anywhere in that scan makes the corpus scan fail closed
		// (disclosed-unproven), which would refuse this call for an
		// UNRELATED reason before ever reaching the name check this test
		// means to exercise.
		if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(behindCheckoutFixtureSpec), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := gitx.AddAll(ctx, repo.Dir); err != nil {
			t.Fatalf("AddAll: %v", err)
		}
		if _, err := gitx.CreateCommit(ctx, repo.Dir, "land taken-on-main on main"); err != nil {
			t.Fatalf("CreateCommit: %v", err)
		}
		if err := gitx.CheckoutExisting(ctx, repo.Dir, "side"); err != nil {
			t.Fatalf("CheckoutExisting(side): %v", err)
		}

		var stdout, stderr bytes.Buffer
		got := runDesignStartSupersede(ctx, repo.Dir, "lockbox", "taken-on-main", phase7Model(t), nil, fakeGoTest{}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runDesignStartSupersede(name on main, absent from behind checkout) = %d, want 2; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "main") {
			t.Fatalf("stderr = %q, want it to name the base ref the checkout is behind", stderr.String())
		}
		branch, err := gitx.CurrentBranch(ctx, repo.Dir)
		if err != nil {
			t.Fatal(err)
		}
		if branch != "side" {
			t.Fatalf("current branch = %q, want the checkout left on side (preparation refusal, no switch)", branch)
		}
	})
}

// TestExtractSupersedeFlags covers extractSupersedeFlags' own parsing
// contract: every flag in any position, the incompatible-flag list, and
// leftover positional arguments.
func TestExtractSupersedeFlags(t *testing.T) {
	t.Run("happy: canonical order", func(t *testing.T) {
		ref, kind, name, incompat, rest, err := extractSupersedeFlags([]string{"--supersedes", "spec/lockbox", "--name", "lockbox-v2"})
		if err != nil {
			t.Fatalf("extractSupersedeFlags = %v, want no error", err)
		}
		if ref != "spec/lockbox" || name != "lockbox-v2" || kind != "" || len(incompat) != 0 || len(rest) != 0 {
			t.Fatalf("got (%q,%q,%q,%v,%v)", ref, kind, name, incompat, rest)
		}
	})

	t.Run("happy: reordered with explicit --kind", func(t *testing.T) {
		ref, kind, name, incompat, rest, err := extractSupersedeFlags([]string{"--name", "lockbox-v2", "--kind", "feature", "--supersedes", "spec/lockbox"})
		if err != nil {
			t.Fatalf("extractSupersedeFlags = %v, want no error", err)
		}
		if ref != "spec/lockbox" || name != "lockbox-v2" || kind != "feature" || len(incompat) != 0 || len(rest) != 0 {
			t.Fatalf("got (%q,%q,%q,%v,%v)", ref, kind, name, incompat, rest)
		}
	})

	t.Run("incompatible flags collected", func(t *testing.T) {
		_, _, _, incompat, _, err := extractSupersedeFlags([]string{"--supersedes", "spec/lockbox", "--name", "x", "--problem", "p", "--outcome", "o", "--defer-statements"})
		if err != nil {
			t.Fatalf("extractSupersedeFlags = %v, want no error", err)
		}
		want := []string{"--problem", "--outcome", "--defer-statements"}
		if len(incompat) != len(want) {
			t.Fatalf("incompatible = %v, want %v", incompat, want)
		}
		for i := range want {
			if incompat[i] != want[i] {
				t.Fatalf("incompatible = %v, want %v", incompat, want)
			}
		}
	})

	t.Run("duplicate flag errors", func(t *testing.T) {
		_, _, _, _, _, err := extractSupersedeFlags([]string{"--supersedes", "spec/a", "--supersedes", "spec/b"})
		if err == nil {
			t.Fatal("extractSupersedeFlags = nil error, want a duplicate-flag refusal")
		}
	})

	t.Run("happy: every flag in its --flag=value form", func(t *testing.T) {
		ref, kind, name, incompat, rest, err := extractSupersedeFlags([]string{"--supersedes=spec/lockbox", "--name=lockbox-v2", "--kind=feature"})
		if err != nil {
			t.Fatalf("extractSupersedeFlags = %v, want no error", err)
		}
		if ref != "spec/lockbox" || name != "lockbox-v2" || kind != "feature" || len(incompat) != 0 || len(rest) != 0 {
			t.Fatalf("got (%q,%q,%q,%v,%v)", ref, kind, name, incompat, rest)
		}
	})

	t.Run("happy: the two forms mix freely", func(t *testing.T) {
		ref, kind, name, incompat, rest, err := extractSupersedeFlags([]string{"--name=lockbox-v2", "--supersedes", "spec/lockbox", "--kind=feature"})
		if err != nil {
			t.Fatalf("extractSupersedeFlags = %v, want no error", err)
		}
		if ref != "spec/lockbox" || name != "lockbox-v2" || kind != "feature" || len(incompat) != 0 || len(rest) != 0 {
			t.Fatalf("got (%q,%q,%q,%v,%v)", ref, kind, name, incompat, rest)
		}
	})

	for _, dup := range [][]string{
		{"--supersedes=spec/a", "--supersedes=spec/b"},
		{"--supersedes", "spec/a", "--supersedes=spec/b"},
		{"--name=a", "--name", "b"},
		{"--kind=feature", "--kind=feature"},
	} {
		t.Run("duplicate errors: "+strings.Join(dup, " "), func(t *testing.T) {
			if _, _, _, _, _, err := extractSupersedeFlags(dup); err == nil {
				t.Fatal("extractSupersedeFlags = nil error, want a duplicate-flag refusal")
			}
		})
	}

	t.Run("leftover positional captured in rest", func(t *testing.T) {
		_, _, _, _, rest, err := extractSupersedeFlags([]string{"--supersedes", "spec/a", "--name", "b", "extra-token"})
		if err != nil {
			t.Fatalf("extractSupersedeFlags = %v, want no error", err)
		}
		if len(rest) != 1 || rest[0] != "extra-token" {
			t.Fatalf("rest = %v, want [extra-token]", rest)
		}
	})
}

// buildVerdiBinary/runVerdiBinary are shared cmd/verdi test helpers
// (serve_integration_test.go / obligationseam_e2e_test.go).

// TestDesignStartSupersedeE2E_Happy is Contract C's built-binary proof: a
// real fixturegit repo, the real compiled verdi binary, exit 0, the new
// branch checked out, successor bytes matching supersede.Compose's own
// output, the right commit subject, the right stdout lines, and — the
// point of running through the real binary rather than only the unit-level
// test above — a real `verdi lint` afterward, asserting that NO `VL-`
// finding line names the successor's own path.
//
// That assertion is deliberately scoped to the successor's path and claims
// nothing about the run's overall verdict: this fixture repo carries a
// pre-existing VL-012 `.gitattributes` finding of its own (the store
// declares no generated-file attributes), so `verdi lint` exits 1 here
// whether or not this verb ever ran. What is proven is the contract
// ac-11 states — the untouched scaffold raises no finding AGAINST itself,
// VL-015 included.
func TestDesignStartSupersedeE2E_Happy(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildSupersedeRepo(t)
	env := []string{"CI_DEFAULT_BRANCH=main"}

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "design", "start", "--supersedes", "spec/lockbox", "--name", "lockbox-v2")
	if code != 0 {
		t.Fatalf("design start --supersedes exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	branch, err := gitx.CurrentBranch(context.Background(), repo.Dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "design/lockbox-v2" {
		t.Fatalf("CurrentBranch = %q, want design/lockbox-v2", branch)
	}

	predRaw := []byte(lockboxPredecessor)
	wantComposed, err := supersede.Compose(supersede.ComposeInput{PredecessorName: "lockbox", PredecessorRaw: predRaw, SuccessorName: "lockbox-v2"})
	if err != nil {
		t.Fatalf("supersede.Compose (test oracle): %v", err)
	}
	gotContent, err := os.ReadFile(filepath.Join(repo.Dir, ".verdi", "specs", "active", "lockbox-v2", "spec.md"))
	if err != nil {
		t.Fatalf("reading successor: %v", err)
	}
	if !bytes.Equal(gotContent, wantComposed.Content) {
		t.Fatalf("successor bytes differ from supersede.Compose's own output:\ngot:\n%s\nwant:\n%s", gotContent, wantComposed.Content)
	}

	commits, err := gitx.Log(context.Background(), repo.Dir, "HEAD")
	if err != nil || len(commits) == 0 {
		t.Fatalf("gitx.Log: %v", err)
	}
	if want := "design start: supersede spec/lockbox as spec/lockbox-v2"; commits[0].Subject != want {
		t.Fatalf("HEAD commit subject = %q, want %q", commits[0].Subject, want)
	}
	if !strings.Contains(stdout, "design start: supersedes spec/lockbox: 5 objects carried, 0 amended, 0 removed") {
		t.Fatalf("stdout missing the supersedes summary line:\n%s", stdout)
	}

	lintStdout, lintStderr, lintCode := runVerdiBinary(t, bin, repo.Dir, env, "lint")
	t.Logf("verdi lint exit=%d\nstdout:\n%s\nstderr:\n%s", lintCode, lintStdout, lintStderr)
	successorRelPath := store.ActiveSpecRelPath("lockbox-v2")
	for _, line := range strings.Split(lintStdout, "\n") {
		// Only a genuine per-file Finding line (internal/lint's Finding.String(),
		// R-W4-5's sentence-first grammar: "message (path) [RULE]", e.g.
		// "... (.verdi/specs/.../spec.md) [VL-015]") counts here — a
		// "disclosed-unproven [lint:VL-017]: ..." NOTICE (disclosure.Render's
		// own distinct format) is printed, not a verdict failure (lint.go's
		// own doc comment: "a run with no other findings still exits 0"), and
		// may legitimately NAME the successor's path in its prose (e.g. an
		// infrastructure-absence notice covering every spec in the checkout)
		// without that being a finding AGAINST the successor's own content.
		// The bracketed "[VL-...]" tail (never present on a disclosure line,
		// which brackets "lint:VL-...") is what distinguishes the two.
		if strings.HasSuffix(line, "]") && strings.Contains(line, "[VL-") && strings.Contains(line, successorRelPath) {
			t.Errorf("verdi lint raised a finding against the untouched successor scaffold (must lint clean under VL-015, ac-11): %s", line)
		}
	}
}

// storyPredecessorForCLI is a class: story spec fixture for the CLI's own
// built-binary negative case ("predecessor story") — supersession is
// feature-only.
const storyPredecessorForCLI = `---
id: spec/some-story
kind: spec
class: story
title: "Some story"
owners: [platform-team]
story: jira:LOAN-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "a", evidence: [static], anchor: "#ac-1" }
---
# Some story

## Problem

p.

## Outcome

o.

## AC-1

a.
`

// TestDesignStartSupersedeE2E_Negative is Contract C's built-binary
// negative-case table: predecessor draft (not on main), predecessor story,
// an existing successor dir, --supersedes combined with --from-stub, and a
// malformed --supersedes ref. Every case: exit 2, no branch/spec-dir left
// behind beyond what the case itself set up.
func TestDesignStartSupersedeE2E_Negative(t *testing.T) {
	bin := buildVerdiBinary(t)
	env := []string{"CI_DEFAULT_BRANCH=main"}

	t.Run("predecessor draft (not on main)", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{
			{Files: map[string]string{".verdi/verdi.yaml": supersedeManifestYAML}, Message: "init store"},
		})
		ctx := context.Background()
		if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "design/lockbox"); err != nil {
			t.Fatalf("CheckoutNewBranch: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(repo.Dir, ".verdi", "specs", "active", "lockbox"), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi", "specs", "active", "lockbox", "spec.md"), []byte(lockboxPredecessor), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if err := gitx.AddAll(ctx, repo.Dir); err != nil {
			t.Fatalf("AddAll: %v", err)
		}
		if _, err := gitx.CreateCommit(ctx, repo.Dir, "draft lockbox"); err != nil {
			t.Fatalf("CreateCommit: %v", err)
		}

		stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "design", "start", "--supersedes", "spec/lockbox", "--name", "lockbox-v2")
		if code != 2 {
			t.Fatalf("exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
		if !strings.Contains(stderr, "draft") {
			t.Fatalf("stderr = %q, want it to name the draft status", stderr)
		}
	})

	t.Run("predecessor is a story", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{
			{
				Files: map[string]string{
					".verdi/verdi.yaml":                      supersedeManifestYAML,
					".verdi/specs/active/some-story/spec.md": storyPredecessorForCLI,
				},
				Message: "story lands",
			},
		})
		stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "design", "start", "--supersedes", "spec/some-story", "--name", "some-story-v2")
		if code != 2 {
			t.Fatalf("exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
		if !strings.Contains(stderr, "story") {
			t.Fatalf("stderr = %q, want it to name the observed class", stderr)
		}
	})

	t.Run("existing successor dir", func(t *testing.T) {
		repo := buildSupersedeRepo(t)
		if err := os.MkdirAll(filepath.Join(repo.Dir, ".verdi", "specs", "active", "lockbox-v2"), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "design", "start", "--supersedes", "spec/lockbox", "--name", "lockbox-v2")
		if code != 2 {
			t.Fatalf("exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
		if !strings.Contains(stderr, "already exists") {
			t.Fatalf("stderr = %q, want the existing-dir refusal", stderr)
		}
	})

	t.Run("--supersedes with --from-stub", func(t *testing.T) {
		repo := buildSupersedeRepo(t)
		stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "design", "start", "--supersedes", "spec/lockbox", "--name", "lockbox-v2", "--from-stub")
		if code != 2 {
			t.Fatalf("exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
		if !strings.Contains(stderr, "--from-stub") || !strings.Contains(stderr, "--supersedes") {
			t.Fatalf("stderr = %q, want it to name both incompatible flags", stderr)
		}
	})

	t.Run("malformed --supersedes ref", func(t *testing.T) {
		repo := buildSupersedeRepo(t)
		stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "design", "start", "--supersedes", "not-a-ref-at-all", "--name", "lockbox-v2")
		if code != 2 {
			t.Fatalf("exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
		}
	})
}

// composeBreakingPredecessor decodes cleanly and resolves as an accepted
// predecessor, but breaks supersede.Compose: its title is a multi-line
// double-quoted scalar whose continuation line sits at column 0 and reads
// exactly like a top-level key, the one shape this lane's report discloses
// the line-level frontmatter splitter cannot see through. It is used here
// as a REAL predecessor that reaches Compose and fails there — the only
// way to prove, through the built binary, what happens to the operator's
// checkout when composition fails.
const composeBreakingPredecessor = `---
id: spec/dedent
kind: spec
class: feature
title: "Dedent fixture
status: this continuation line is inside the title scalar"
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a", evidence: [attestation], anchor: "#ac-1" }
---
# Dedent fixture

Body.
`

// TestDesignStartSupersedeE2E_ComposeFailureLeavesCheckoutUntouched proves
// the preparation boundary this verb documents actually holds through the
// real binary: when composition fails, the operator is left on the branch
// they started on, with no design/<new> ref and no successor directory —
// not stranded on an empty new branch they never asked to be on. It also
// pins the refusal to ONE "internal error" prefix: the CLI relays
// Compose's own classified message rather than prefixing it a second time.
func TestDesignStartSupersedeE2E_ComposeFailureLeavesCheckoutUntouched(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                  supersedeManifestYAML,
				".verdi/specs/active/dedent/spec.md": composeBreakingPredecessor,
			},
			Message: "dedent lands",
		},
	})
	ctx := context.Background()
	env := []string{"CI_DEFAULT_BRANCH=main"}

	before, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "design", "start", "--supersedes", "spec/dedent", "--name", "dedent-v2")
	if code != 2 {
		t.Fatalf("exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	after, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if after != before {
		t.Errorf("checkout moved from %q to %q; a composition failure must leave the checkout where it was", before, after)
	}
	if sha, err := gitx.RevParse(ctx, repo.Dir, "refs/heads/design/dedent-v2"); err == nil {
		t.Errorf("design/dedent-v2 exists at %s; a composition failure must cut no branch", sha)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "active", "dedent-v2")); err == nil {
		t.Error("the successor directory exists; a composition failure must write nothing")
	}
	if strings.Contains(stdout, "switched checkout") {
		t.Errorf("stdout discloses a checkout switch that must never have happened:\n%s", stdout)
	}
	if n := strings.Count(stderr, "internal error"); n != 1 {
		t.Errorf("stderr says %q %d times, want exactly 1:\n%s", "internal error", n, stderr)
	}
}

// TestDesignStartSupersedeE2E_EqualsFlagSpellings proves the whole
// invocation works through the built binary when every flag is written in
// the --flag=value form design start's own --kind/--name grammar has always
// accepted — including the dispatch itself, which matched only the bare
// `--supersedes` token and so routed `--supersedes=spec/lockbox` into the
// plain --kind/--name path, where it surfaced as an unrelated story-ref
// complaint.
func TestDesignStartSupersedeE2E_EqualsFlagSpellings(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildSupersedeRepo(t)
	env := []string{"CI_DEFAULT_BRANCH=main"}

	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "design", "start", "--supersedes=spec/lockbox", "--name=lockbox-v2", "--kind=feature")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "design start: supersedes spec/lockbox: 5 objects carried, 0 amended, 0 removed") {
		t.Fatalf("stdout missing the supersedes summary line:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "active", "lockbox-v2", "spec.md")); err != nil {
		t.Fatalf("successor spec.md: %v", err)
	}
}

// TestRunDesignStartSupersede_ScaffoldCommitStagesOnlySpecDir is UAT-033's
// own witness for the --supersedes path (design_test.go's
// TestRunDesignStart_ScaffoldCommitStagesOnlySpecDir proves the identical
// invariant for the plain --kind/--name path this file's runDesignStartSupersede
// used to reuse gitx.AddAll from by parity). Plants the same three shapes of
// working-tree noise the round spec PR's real scaffold commit carried — an
// untracked file at the repo root, an untracked file in a nested docs
// directory, and a modified tracked file (here the tracked verdi.yaml,
// never the predecessor spec.md this ritual reads to compose the successor)
// — then proves the successor's scaffold commit contains exactly the one
// path this ritual itself wrote, every planted file rides untouched in the
// working tree afterward, and the verb's own success disclosure is
// unchanged.
func TestRunDesignStartSupersede_ScaffoldCommitStagesOnlySpecDir(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := buildSupersedeRepo(t)
	ctx := context.Background()

	// (a) untracked file at the repo root.
	if err := os.WriteFile(filepath.Join(repo.Dir, ".DS_Store"), []byte("junk"), 0o644); err != nil {
		t.Fatalf("planting root untracked file: %v", err)
	}
	// (b) untracked file in a nested docs directory.
	nestedDir := filepath.Join(repo.Dir, "docs", "superpowers", "specs")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll nested docs dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "lens.sqlite3"), []byte("junk"), 0o644); err != nil {
		t.Fatalf("planting nested untracked file: %v", err)
	}
	// (c) a modified tracked file — the manifest, never the predecessor
	// spec.md this ritual reads verbatim to compose the successor.
	manifestPath := filepath.Join(repo.Dir, ".verdi", "verdi.yaml")
	original, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("reading tracked manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, append(original, []byte("\n# local edit\n")...), 0o644); err != nil {
		t.Fatalf("modifying tracked manifest: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := runDesignStartSupersede(ctx, repo.Dir, "lockbox", "lockbox-v2", phase7Model(t), nil, fakeGoTest{}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStartSupersede = %d, want 0; stderr=%s", got, stderr.String())
	}

	head, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD): %v", err)
	}
	entries, err := gitx.DiffNameStatus(ctx, repo.Dir, repo.Head, head)
	if err != nil {
		t.Fatalf("DiffNameStatus: %v", err)
	}
	wantPath := ".verdi/specs/active/lockbox-v2/spec.md"
	if len(entries) != 1 || entries[0].Path != wantPath {
		t.Fatalf("scaffold commit's changed paths = %+v, want exactly [%s] (never the planted working-tree noise)", entries, wantPath)
	}

	plants := []string{".DS_Store", "docs/superpowers/specs/lens.sqlite3", ".verdi/verdi.yaml"}

	changed, err := gitx.WorktreeChangedPaths(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("WorktreeChangedPaths: %v", err)
	}
	changedSet := map[string]bool{}
	for _, p := range changed {
		changedSet[p] = true
	}
	for _, want := range plants {
		if !changedSet[want] {
			t.Errorf("WorktreeChangedPaths = %v, want %q still present (an unresolved working-tree change: untracked or modified)", changed, want)
		}
	}
	if changedSet[wantPath] {
		t.Errorf("WorktreeChangedPaths = %v, want %q absent (it was committed, so the working tree is clean at that path)", changed, wantPath)
	}

	// WorktreeChangedPaths alone does not prove "never staged": `git status
	// --porcelain` reports an index-staged-but-uncommitted path too (as
	// "A "/"M " rather than "??"/" M"), so it would stay green even if a
	// regression left one of the plants sitting staged in the index (e.g. a
	// future rewrite that commits via a pathspec scoped to specDir, which
	// would leave any OTHER staged path uncommitted rather than absent).
	// gitx.StagedPaths reports exactly the paths whose index entry differs
	// from HEAD — the precise check for "never staged by this ritual".
	staged, err := gitx.StagedPaths(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("StagedPaths: %v", err)
	}
	stagedSet := map[string]bool{}
	for _, p := range staged {
		stagedSet[p] = true
	}
	for _, want := range plants {
		if stagedSet[want] {
			t.Errorf("StagedPaths = %v, want %q absent (this ritual must never stage it, even transiently)", staged, want)
		}
	}

	if !strings.Contains(stdout.String(), "board:") {
		t.Fatalf("stdout = %q, want a board URL placeholder line (the verb's own stdout is unchanged by this fix)", stdout.String())
	}
}

// TestDesignSupersedeGo_NoAddAll is a source-text witness mirroring
// design_test.go's TestDesignGo_NoAddAll: designsupersede.go's scaffold
// commit must stage exactly the successor spec directory via
// gitx.AddPaths (UAT-033), never gitx.AddAll's blanket `git add -A` sweep
// of the rest of the working tree.
func TestDesignSupersedeGo_NoAddAll(t *testing.T) {
	data, err := os.ReadFile("designsupersede.go")
	if err != nil {
		t.Fatalf("reading designsupersede.go: %v", err)
	}
	if strings.Contains(string(data), "AddAll(") {
		t.Error("designsupersede.go calls gitx.AddAll — the scaffold commit must stage exactly the spec directory via gitx.AddPaths instead (UAT-033)")
	}
}
