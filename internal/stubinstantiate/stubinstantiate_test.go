package stubinstantiate

import (
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// acceptedFeatureSpec is a class-feature, accepted-pending-build wall
// carrying one plain and one spike stub — mirroring internal/workbench's
// own scopingAcceptedSpec fixture (scopingcanvas_test.go), the exact
// shape stub-instantiate's board action has always driven against, so
// this package's own tests exercise the identical fixture contract the
// extraction must stay behavior-preserving for.
const acceptedFeatureSpec = `---
id: spec/scoping-accepted
kind: spec
class: feature
title: "Scoping accepted"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "ac one", evidence: [attestation], anchor: "#ac-1" }
open_questions:
  - { id: oq-1, text: "oq one", anchor: "#oq-1" }
stubs:
  - { slug: borrower-update-api, acceptance_criteria: [ac-1] }
  - { slug: retry-strategy-spike, spike: true, resolves: [oq-1] }
frozen: { at: 2026-07-12, commit: 6400db382876f416ed943f6b6e22954f9666fde3 }
---
# Scoping accepted

## Problem

Prose.

## Outcome

Prose.

## ac-1

Prose.

## oq-1

Prose.
`

const acceptedFeatureName = "scoping-accepted"

var acceptedFeatureStubs = []artifact.Stub{
	{Slug: "borrower-update-api", AcceptanceCriteria: []string{"ac-1"}},
	{Slug: "retry-strategy-spike", Spike: true, Resolves: []string{"oq-1"}},
}

func newAcceptedFeatureFixture(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + acceptedFeatureName + "/spec.md": acceptedFeatureSpec,
			".verdi/.gitignore": "data/\n",
			".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
		},
		Message: "seed accepted feature fixture",
	}})
}

// TestInstantiate_Plain proves the plain-stub path: a fresh design/<slug>
// branch, forked from the prior HEAD, carrying a self-validated story
// spec with the stub's real implements edge — and the calling checkout's
// HEAD/branch/working tree are never touched (spec/scoping-canvas ac-6,
// inherited by this shared core).
func TestInstantiate_Plain(t *testing.T) {
	repo := newAcceptedFeatureFixture(t)
	root := repo.Dir
	ctx := context.Background()

	beforeBranch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		t.Fatal(err)
	}

	result, err := Instantiate(ctx, root, acceptedFeatureName, artifact.ClassFeature, "accepted-pending-build", acceptedFeatureStubs, "borrower-update-api", nil)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if result.Branch != "design/borrower-update-api" {
		t.Fatalf("Branch = %q, want design/borrower-update-api", result.Branch)
	}

	afterBranch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if afterBranch != beforeBranch {
		t.Fatalf("current branch moved from %q to %q", beforeBranch, afterBranch)
	}
	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if head != repo.Head {
		t.Fatalf("HEAD moved to %s, want unchanged %s", head, repo.Head)
	}
	dirty, err := gitx.StatusDirty(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Fatal("Instantiate left the working tree dirty")
	}

	parent, err := gitx.RevParse(ctx, root, "design/borrower-update-api^")
	if err != nil {
		t.Fatal(err)
	}
	if parent != repo.Head {
		t.Fatalf("new branch's parent = %s, want %s", parent, repo.Head)
	}

	fm, _, err := artifact.SplitFrontmatter([]byte(result.Content))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if spec.Class != artifact.ClassStory {
		t.Fatalf("Class = %q, want story", spec.Class)
	}
	if spec.Spike {
		t.Fatal("Spike = true, want false")
	}
	var foundImplements bool
	for _, l := range spec.Links {
		if l.Type == artifact.LinkImplements && l.Ref == "spec/"+acceptedFeatureName+"#ac-1" {
			foundImplements = true
		}
	}
	if !foundImplements {
		t.Fatalf("links = %+v, want an implements edge to spec/%s#ac-1", spec.Links, acceptedFeatureName)
	}

	// The committed blob on the new branch matches the returned Content
	// exactly — Result.Content is not a second, independently-rendered
	// copy that could drift from what actually landed in git.
	blob, err := gitx.Show(ctx, root, "design/borrower-update-api", ".verdi/specs/active/borrower-update-api/spec.md")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if string(blob) != result.Content {
		t.Fatalf("committed blob does not match Result.Content:\nblob:\n%s\ncontent:\n%s", blob, result.Content)
	}
}

// TestInstantiate_Spike proves the spike-stub path: spike: true, a
// resolves edge to the stub's open question, no implements edge.
func TestInstantiate_Spike(t *testing.T) {
	repo := newAcceptedFeatureFixture(t)
	ctx := context.Background()

	result, err := Instantiate(ctx, repo.Dir, acceptedFeatureName, artifact.ClassFeature, "accepted-pending-build", acceptedFeatureStubs, "retry-strategy-spike", nil)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	spec, err := artifact.DecodeSpec(mustFrontmatter(t, result.Content))
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if !spec.Spike {
		t.Fatal("Spike = false, want true")
	}
	var foundResolves bool
	for _, l := range spec.Links {
		if l.Type == artifact.LinkImplements {
			t.Fatalf("spike-instantiated spec carries an implements edge: %+v", l)
		}
		if l.Type == artifact.LinkResolves && l.Ref == "spec/"+acceptedFeatureName+"#oq-1" {
			foundResolves = true
		}
	}
	if !foundResolves {
		t.Fatalf("links = %+v, want a resolves edge to spec/%s#oq-1", spec.Links, acceptedFeatureName)
	}
}

func mustFrontmatter(t *testing.T, content string) []byte {
	t.Helper()
	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	return fm
}

// TestInstantiate_Negative covers the guard: empty slug, unknown slug,
// wrong class, wrong status, and an already-existing branch — each fails
// closed and never mutates the repository.
func TestInstantiate_Negative(t *testing.T) {
	t.Run("empty slug", func(t *testing.T) {
		repo := newAcceptedFeatureFixture(t)
		_, err := Instantiate(context.Background(), repo.Dir, acceptedFeatureName, artifact.ClassFeature, "accepted-pending-build", acceptedFeatureStubs, "", nil)
		if err == nil {
			t.Fatal("Instantiate(empty slug) = nil error, want a refusal")
		}
	})

	t.Run("unknown slug", func(t *testing.T) {
		repo := newAcceptedFeatureFixture(t)
		_, err := Instantiate(context.Background(), repo.Dir, acceptedFeatureName, artifact.ClassFeature, "accepted-pending-build", acceptedFeatureStubs, "no-such-stub", nil)
		if err == nil {
			t.Fatal("Instantiate(unknown slug) = nil error, want a refusal")
		}
		if !strings.Contains(err.Error(), "no-such-stub") {
			t.Fatalf("error = %v, want it to name the unknown slug", err)
		}
	})

	t.Run("wrong status (draft feature)", func(t *testing.T) {
		repo := newAcceptedFeatureFixture(t)
		_, err := Instantiate(context.Background(), repo.Dir, acceptedFeatureName, artifact.ClassFeature, "draft", acceptedFeatureStubs, "borrower-update-api", nil)
		if err == nil {
			t.Fatal("Instantiate(draft wall) = nil error, want a refusal")
		}
	})

	t.Run("wrong class (story)", func(t *testing.T) {
		repo := newAcceptedFeatureFixture(t)
		_, err := Instantiate(context.Background(), repo.Dir, acceptedFeatureName, artifact.ClassStory, "accepted-pending-build", acceptedFeatureStubs, "borrower-update-api", nil)
		if err == nil {
			t.Fatal("Instantiate(story class) = nil error, want a refusal")
		}
	})

	t.Run("branch already exists", func(t *testing.T) {
		repo := newAcceptedFeatureFixture(t)
		ctx := context.Background()
		if err := gitx.UpdateRef(ctx, repo.Dir, "refs/heads/design/borrower-update-api", repo.Head); err != nil {
			t.Fatalf("pre-creating the branch: %v", err)
		}
		_, err := Instantiate(ctx, repo.Dir, acceptedFeatureName, artifact.ClassFeature, "accepted-pending-build", acceptedFeatureStubs, "borrower-update-api", nil)
		if err == nil {
			t.Fatal("Instantiate(branch exists) = nil error, want a refusal")
		}
		if !strings.Contains(err.Error(), "design/borrower-update-api already exists") {
			t.Fatalf("error = %v, want it to name the branch as already existing", err)
		}
	})
}

// TestSealedFeatureWallGuard_NilModel proves the guard is nil-receiver-safe
// (model.DisplayClass/DisplayState's own contract) — a caller with no
// resolved model (an operational fallback, never expected in production)
// still gets a legible, bare-id refusal rather than a panic.
func TestSealedFeatureWallGuard_NilModel(t *testing.T) {
	if err := SealedFeatureWallGuard(artifact.ClassStory, "accepted-pending-build", "stub-instantiate", nil); err == nil {
		t.Fatal("SealedFeatureWallGuard(wrong class, nil model) = nil error, want a refusal")
	}
	if err := SealedFeatureWallGuard(artifact.ClassFeature, "draft", "stub-instantiate", nil); err == nil {
		t.Fatal("SealedFeatureWallGuard(wrong status, nil model) = nil error, want a refusal")
	}
	if err := SealedFeatureWallGuard(artifact.ClassFeature, "accepted-pending-build", "stub-instantiate", nil); err != nil {
		t.Fatalf("SealedFeatureWallGuard(valid) = %v, want nil", err)
	}
}
