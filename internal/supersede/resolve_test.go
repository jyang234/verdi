package supersede

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// resolveTestManifestYAML is the minimal verdi.yaml every Resolve fixture
// repo needs — no providers/toolchain required, since Resolve never
// touches either.
const resolveTestManifestYAML = `schema: verdi.layout/v1
forge: gitlab
`

// buildResolveRepo builds a one-layer fixturegit repo carrying verdi.yaml
// plus one spec at path with content, all landed directly on `main` (the
// merge-signaled "accepted" shape: no frozen: stamp, exact bytes already on
// the default branch — docs/superpowers/specs/2026-08-01-merge-signals-
// spec-acceptance-design.md).
func buildResolveRepo(t *testing.T, path, content string) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml": resolveTestManifestYAML,
				path:                content,
			},
			Message: "seed accepted predecessor",
		},
	})
}

// TestResolve_AcceptedFeaturePredecessor_Succeeds is the RED-first proof:
// a feature spec sitting directly on `main` with no frozen:/status: stamp
// at all (the merge-signaled statusless shape) resolves as
// accepted-pending-build, exactly the one precondition Resolve accepts.
func TestResolve_AcceptedFeaturePredecessor_Succeeds(t *testing.T) {
	// specstate.ResolveDefaultBranch's fallback only probes REMOTE-tracking
	// refs (refs/remotes/origin/main|master) — a fixturegit repo has no
	// origin remote at all, so effective-status resolution needs
	// CI_DEFAULT_BRANCH set explicitly, exactly like every other test in
	// this module that drives real specstate resolution against a
	// fixturegit repo (e.g. cmd/verdi's own
	// TestRunDesignStart_BasesOnDefaultBranch_NotHEAD).
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := buildResolveRepo(t, ".verdi/specs/active/widget/spec.md", minimalPredecessor)
	ctx := context.Background()

	got, err := Resolve(ctx, repo.Dir, "widget", nil)
	if err != nil {
		t.Fatalf("Resolve = %v, want no error", err)
	}
	if got.Name != "widget" {
		t.Errorf("Name = %q, want widget", got.Name)
	}
	if got.Status != "accepted-pending-build" {
		t.Errorf("Status = %q, want accepted-pending-build", got.Status)
	}
	if got.Spec == nil || got.Spec.Class != artifact.ClassFeature {
		t.Errorf("Spec.Class = %v, want feature", got.Spec)
	}
	if !bytes.Equal(got.Raw, []byte(minimalPredecessor)) {
		t.Errorf("Raw = %q, want the exact predecessor bytes", got.Raw)
	}
}

// buildBareResolveRepo builds a one-layer fixturegit repo carrying only
// verdi.yaml — no spec at all — the starting point for tests that then
// grow their own branch/commit shape (the draft-never-merged case below).
func buildBareResolveRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{".verdi/verdi.yaml": resolveTestManifestYAML}, Message: "init store"},
	})
}

// writeAndCommitSpec writes content at path under dir and commits it —
// a small helper for tests that build a SECOND commit on top of a
// fixturegit-built repo (e.g. onto a fresh branch), where fixturegit's own
// declarative Layer shape does not apply.
func writeAndCommitSpec(t *testing.T, ctx context.Context, dir, path, content, message string) {
	t.Helper()
	full := dir + "/" + path
	if err := os.MkdirAll(full[:strings.LastIndex(full, "/")], 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := gitx.AddAll(ctx, dir); err != nil {
		t.Fatalf("AddAll: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, dir, message); err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}
}

// componentPredecessor is a class: component spec — supersession is
// feature-only (02 §Kind registry) — used to prove Resolve's own
// ReasonWrongClass refusal for the "component predecessor" case.
const componentPredecessor = `---
id: spec/some-component
kind: spec
class: component
title: "Some component"
owners: [platform-team]
status: active
---
# Some component

Body.
`

// TestResolve_Negative is Resolve's own refusal table (dispatch contract,
// part A): "A story or component predecessor, a draft, a superseded/closed
// one, a missing one: each a distinct, tested refusal message." Superseded
// and closed are covered by their own dedicated tests below (each needs
// more real-git setup than a single table row reads well).
func TestResolve_Negative(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		t.Setenv("CI_DEFAULT_BRANCH", "main")
		repo := buildResolveRepo(t, ".verdi/specs/active/widget/spec.md", minimalPredecessor)
		ctx := context.Background()
		_, err := Resolve(ctx, repo.Dir, "does-not-exist", nil)
		var rerr *ResolveError
		if !errors.As(err, &rerr) || rerr.Reason != ReasonNotFound {
			t.Fatalf("Resolve = %v, want a ReasonNotFound *ResolveError", err)
		}
	})

	t.Run("not decodable", func(t *testing.T) {
		t.Setenv("CI_DEFAULT_BRANCH", "main")
		repo := buildResolveRepo(t, ".verdi/specs/active/broken/spec.md", "not even frontmatter\n")
		ctx := context.Background()
		_, err := Resolve(ctx, repo.Dir, "broken", nil)
		var rerr *ResolveError
		if !errors.As(err, &rerr) || rerr.Reason != ReasonNotDecodable {
			t.Fatalf("Resolve = %v, want a ReasonNotDecodable *ResolveError", err)
		}
	})

	t.Run("wrong class: story", func(t *testing.T) {
		t.Setenv("CI_DEFAULT_BRANCH", "main")
		repo := buildResolveRepo(t, ".verdi/specs/active/some-story/spec.md", storyPredecessor)
		ctx := context.Background()
		_, err := Resolve(ctx, repo.Dir, "some-story", nil)
		var rerr *ResolveError
		if !errors.As(err, &rerr) || rerr.Reason != ReasonWrongClass {
			t.Fatalf("Resolve = %v, want a ReasonWrongClass *ResolveError", err)
		}
		if !strings.Contains(rerr.Detail, "story") {
			t.Errorf("Detail = %q, want it to name the observed class", rerr.Detail)
		}
	})

	t.Run("wrong class: component", func(t *testing.T) {
		t.Setenv("CI_DEFAULT_BRANCH", "main")
		repo := buildResolveRepo(t, ".verdi/specs/active/some-component/spec.md", componentPredecessor)
		ctx := context.Background()
		_, err := Resolve(ctx, repo.Dir, "some-component", nil)
		var rerr *ResolveError
		if !errors.As(err, &rerr) || rerr.Reason != ReasonWrongClass {
			t.Fatalf("Resolve = %v, want a ReasonWrongClass *ResolveError", err)
		}
	})

	t.Run("wrong status: draft, never merged", func(t *testing.T) {
		t.Setenv("CI_DEFAULT_BRANCH", "main")
		repo := buildBareResolveRepo(t)
		ctx := context.Background()
		if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "design/widget"); err != nil {
			t.Fatalf("CheckoutNewBranch: %v", err)
		}
		writeAndCommitSpec(t, ctx, repo.Dir, ".verdi/specs/active/widget/spec.md", minimalPredecessor, "draft widget")

		_, err := Resolve(ctx, repo.Dir, "widget", nil)
		var rerr *ResolveError
		if !errors.As(err, &rerr) || rerr.Reason != ReasonWrongStatus {
			t.Fatalf("Resolve = %v, want a ReasonWrongStatus *ResolveError", err)
		}
		if !strings.Contains(rerr.Detail, "draft") {
			t.Errorf("Detail = %q, want it to name the observed status (draft)", rerr.Detail)
		}
	})
}

// TestResolve_Superseded_Refuses is the "superseded" refusal case: once a
// validated successor lands on the default branch naming widget as its
// sole whole-spec predecessor, widget's OWN effective status is COMPUTED
// as superseded (specstate's successor-corpus scan — vl015.go's own doc
// comment: "once a successor lands, the predecessor itself projects
// Superseded") even though widget's own bytes never change. Uses this
// package's own Compose to build a valid successor, so this test does not
// hand-author a second fixture that could silently drift from what Compose
// actually produces.
func TestResolve_Superseded_Refuses(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	composed, err := Compose(ComposeInput{
		PredecessorName: "widget",
		PredecessorRaw:  []byte(minimalPredecessor),
		SuccessorName:   "widget-v2",
	})
	if err != nil {
		t.Fatalf("Compose (test setup): %v", err)
	}

	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                  resolveTestManifestYAML,
				".verdi/specs/active/widget/spec.md": minimalPredecessor,
			},
			Message: "widget lands",
		},
		{
			Files: map[string]string{
				".verdi/specs/active/widget-v2/spec.md": string(composed.Content),
			},
			Message: "widget-v2 supersedes widget",
		},
	})
	ctx := context.Background()

	_, err = Resolve(ctx, repo.Dir, "widget", nil)
	var rerr *ResolveError
	if !errors.As(err, &rerr) || rerr.Reason != ReasonWrongStatus {
		t.Fatalf("Resolve = %v, want a ReasonWrongStatus *ResolveError", err)
	}
	if !strings.Contains(rerr.Detail, "superseded") {
		t.Errorf("Detail = %q, want it to name the observed status (superseded)", rerr.Detail)
	}
}

// TestResolve_ClosedInActiveZone_Refuses is the "closed" refusal case: a
// predecessor still readable at the active-zone path (Resolve's own read
// contract — store.ActiveSpecPath in the current checkout) but carrying an
// explicit legacy `status: closed` field, its own exact bytes landed on the
// default branch. A predecessor actually MOVED to specs/archive/ (the
// ordinary way a spec closes) is not reachable through this read contract
// at all and surfaces as ReasonNotFound instead — a disclosed consequence
// of Resolve's own documented read path, not a gap: see this lane's final
// report.
func TestResolve_ClosedInActiveZone_Refuses(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	closedSpec := strings.Replace(minimalPredecessor, "owners: [platform-team]\n", "owners: [platform-team]\nstatus: closed\nfrozen: { at: 2026-01-01, commit: 0123456789abcdef0123456789abcdef01234567 }\n", 1)
	repo := buildResolveRepo(t, ".verdi/specs/active/widget/spec.md", closedSpec)
	ctx := context.Background()

	_, err := Resolve(ctx, repo.Dir, "widget", nil)
	var rerr *ResolveError
	if !errors.As(err, &rerr) || rerr.Reason != ReasonWrongStatus {
		t.Fatalf("Resolve = %v, want a ReasonWrongStatus *ResolveError", err)
	}
	if !strings.Contains(rerr.Detail, "closed") {
		t.Errorf("Detail = %q, want it to name the observed status (closed)", rerr.Detail)
	}
}
