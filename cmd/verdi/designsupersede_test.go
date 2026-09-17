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
	got := runDesignStartSupersede(ctx, repo.Dir, "lockbox", "lockbox-v2", nil, fakeGoTest{}, &stdout, &stderr)
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
		got := runDesignStartSupersede(ctx, repo.Dir, "lockbox", "lockbox-v2", nil, fakeGoTest{}, &stdout, &stderr)
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
		got := runDesignStartSupersede(ctx, repo.Dir, "lockbox", "Not_A_Valid_Name", nil, fakeGoTest{}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runDesignStartSupersede = %d, want 2", got)
		}
	})

	t.Run("predecessor not found", func(t *testing.T) {
		repo := buildSupersedeRepo(t)
		ctx := context.Background()
		var stdout, stderr bytes.Buffer
		got := runDesignStartSupersede(ctx, repo.Dir, "does-not-exist", "does-not-exist-v2", nil, fakeGoTest{}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runDesignStartSupersede = %d, want 2", got)
		}
		if !strings.Contains(stderr.String(), "not found") {
			t.Fatalf("stderr = %q, want the Resolve not-found refusal forwarded", stderr.String())
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
// test above — a real `verdi lint` afterward proving VL-015 raises no
// finding for the successor and the successor introduces no OTHER finding
// either.
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
		// Only a genuine per-file Finding line (internal/lint's Finding.String():
		// "RULE path: message", e.g. "VL-015 .verdi/specs/.../spec.md: ...")
		// counts here — a "disclosed-unproven [lint:VL-017]: ..." NOTICE
		// (disclosure.Render's own distinct format) is printed, not a
		// verdict failure (lint.go's own doc comment: "a run with no other
		// findings still exits 0"), and may legitimately NAME the successor's
		// path in its prose (e.g. an infrastructure-absence notice covering
		// every spec in the checkout) without that being a finding AGAINST
		// the successor's own content.
		if strings.HasPrefix(line, "VL-") && strings.Contains(line, successorRelPath) {
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
