package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/workbench"
)

// fromStubFeatureSpec is an accepted-pending-build feature carrying one
// plain and one spike stub — the exact shape `--from-stub` and the
// board's own stub-instantiate action both scaffold against.
const fromStubFeatureSpec = `---
id: spec/fromstub-feature
kind: spec
class: feature
title: "Fromstub Feature"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "ac one", evidence: [attestation], anchor: "#ac-1" }
open_questions:
  - { id: oq-1, text: "oq one", anchor: "#oq-1" }
stubs:
  - { slug: fromstub-story, acceptance_criteria: [ac-1] }
  - { slug: fromstub-spike, spike: true, resolves: [oq-1] }
frozen: { at: 2026-07-12, commit: 6400db382876f416ed943f6b6e22954f9666fde3 }
---
# Fromstub Feature

## Problem

Prose.

## Outcome

Prose.

## ac-1

Prose.

## oq-1

Prose.
`

const fromStubFeatureName = "fromstub-feature"

func buildFromStubRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + fromStubFeatureName + "/spec.md": fromStubFeatureSpec,
			".verdi/verdi.yaml": "schema: verdi.layout/v1\n",
		},
		Message: "seed --from-stub fixture",
	}})
}

// TestRunDesignStartFromStub_Plain proves the plain-stub path end to end:
// a fresh design/<slug> branch, forked from the prior HEAD, carrying a
// self-validated story spec with the stub's real implements edge — and
// the calling checkout's HEAD/branch/working tree are never touched.
func TestRunDesignStartFromStub_Plain(t *testing.T) {
	repo := buildFromStubRepo(t)
	ctx := context.Background()

	beforeBranch, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr strings.Builder
	got := runDesignStartFromStub(ctx, repo.Dir, fromStubFeatureName, "fromstub-story", model.Canonical(), &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStartFromStub = %d, want 0; stderr=%s", got, stderr.String())
	}
	if !contains(stdout.String(), "design/fromstub-story") {
		t.Fatalf("stdout = %q, want it to name the new branch", stdout.String())
	}

	afterBranch, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if afterBranch != beforeBranch {
		t.Fatalf("current branch moved from %q to %q", beforeBranch, afterBranch)
	}
	head, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if head != repo.Head {
		t.Fatalf("HEAD moved to %s, want unchanged %s", head, repo.Head)
	}

	blob, err := gitx.Show(ctx, repo.Dir, "design/fromstub-story", ".verdi/specs/active/fromstub-story/spec.md")
	if err != nil {
		t.Fatalf("Show new spec: %v", err)
	}
	fm, _, err := artifact.SplitFrontmatter(blob)
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
	var foundImplements bool
	for _, l := range spec.Links {
		if l.Type == artifact.LinkImplements && l.Ref == "spec/"+fromStubFeatureName+"#ac-1" {
			foundImplements = true
		}
	}
	if !foundImplements {
		t.Fatalf("links = %+v, want an implements edge to spec/%s#ac-1", spec.Links, fromStubFeatureName)
	}
}

// TestRunDesignStartFromStub_Spike proves the spike-stub path.
func TestRunDesignStartFromStub_Spike(t *testing.T) {
	repo := buildFromStubRepo(t)
	ctx := context.Background()

	var stdout, stderr strings.Builder
	got := runDesignStartFromStub(ctx, repo.Dir, fromStubFeatureName, "fromstub-spike", model.Canonical(), &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStartFromStub = %d, want 0; stderr=%s", got, stderr.String())
	}

	blob, err := gitx.Show(ctx, repo.Dir, "design/fromstub-spike", ".verdi/specs/active/fromstub-spike/spec.md")
	if err != nil {
		t.Fatalf("Show new spec: %v", err)
	}
	fm, _, err := artifact.SplitFrontmatter(blob)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if !spec.Spike {
		t.Fatal("Spike = false, want true")
	}
	var foundResolves bool
	for _, l := range spec.Links {
		if l.Type == artifact.LinkResolves && l.Ref == "spec/"+fromStubFeatureName+"#oq-1" {
			foundResolves = true
		}
	}
	if !foundResolves {
		t.Fatalf("links = %+v, want a resolves edge to spec/%s#oq-1", spec.Links, fromStubFeatureName)
	}
}

// TestRunDesignStartFromStub_Negative covers the refusal paths: unknown
// slug, an already-existing branch, and a feature spec that cannot be
// read at all — every case operational (exit 2), design start's own
// established local exit-code convention.
func TestRunDesignStartFromStub_Negative(t *testing.T) {
	t.Run("unknown slug", func(t *testing.T) {
		repo := buildFromStubRepo(t)
		var stdout, stderr strings.Builder
		got := runDesignStartFromStub(context.Background(), repo.Dir, fromStubFeatureName, "no-such-stub", model.Canonical(), &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runDesignStartFromStub(unknown slug) = %d, want 2", got)
		}
		if !contains(stderr.String(), "no-such-stub") {
			t.Fatalf("stderr = %q, want it to name the unknown slug", stderr.String())
		}
	})

	t.Run("unknown feature", func(t *testing.T) {
		repo := buildFromStubRepo(t)
		var stdout, stderr strings.Builder
		got := runDesignStartFromStub(context.Background(), repo.Dir, "no-such-feature", "fromstub-story", model.Canonical(), &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runDesignStartFromStub(unknown feature) = %d, want 2", got)
		}
	})

	t.Run("branch already exists", func(t *testing.T) {
		repo := buildFromStubRepo(t)
		ctx := context.Background()
		if err := gitx.UpdateRef(ctx, repo.Dir, "refs/heads/design/fromstub-story", repo.Head); err != nil {
			t.Fatalf("pre-creating the branch: %v", err)
		}
		var stdout, stderr strings.Builder
		got := runDesignStartFromStub(ctx, repo.Dir, fromStubFeatureName, "fromstub-story", model.Canonical(), &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runDesignStartFromStub(branch exists) = %d, want 2", got)
		}
		if !contains(stderr.String(), "design/fromstub-story already exists") {
			t.Fatalf("stderr = %q, want it to name the branch as already existing", stderr.String())
		}
	})
}

// TestCmdDesignStartFromStub_UsageArgs proves the argument-count guard at
// the real entry point: exactly two positional args (feature, stub).
func TestCmdDesignStartFromStub_UsageArgs(t *testing.T) {
	cases := [][]string{nil, {"one-arg"}, {"one", "two", "three"}}
	for _, args := range cases {
		var stdout, stderr strings.Builder
		got := cmdDesignStartFromStub(args, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdDesignStartFromStub(%v) = %d, want 2", args, got)
		}
		if !contains(stderr.String(), "usage") {
			t.Fatalf("stderr = %q, want a usage message", stderr.String())
		}
	}
}

// TestRun_DesignStartFromStub_Dispatch proves `design start --from-stub`
// reaches this path through the real verb dispatch (run -> runDesignVerb
// -> cmdDesignStart -> cmdDesignStartFromStub), not only when the
// testable core is called directly.
func TestRun_DesignStartFromStub_Dispatch(t *testing.T) {
	repo := buildFromStubRepo(t)
	t.Chdir(repo.Dir)

	var stderr strings.Builder
	got := run([]string{"design", "start", "--from-stub", fromStubFeatureName, "fromstub-story"}, &stderr)
	if got != 0 {
		t.Fatalf("run(design start --from-stub ...) = %d, want 0; stderr=%s", got, stderr.String())
	}

	branch, err := gitx.CurrentBranch(context.Background(), repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	// The CLI invocation never checked the new branch out — it was cut
	// via pure git plumbing exactly like the board's own action (spec/
	// scoping-canvas ac-6, inherited through the shared core).
	if branch == "design/fromstub-story" {
		t.Fatal("design start --from-stub checked out the new branch; it must stay pure git plumbing")
	}
}

// TestDesignStartFromStub_ParityWithBoardAction is spec/cli-creation
// ac-3's own parity proof: given the identical feature and stub, `design
// start --from-stub`'s rendered spec content is asserted equal to the
// board's own stub-instantiate action's rendered content — the property
// that holds ONLY because both now call the one shared
// internal/stubinstantiate.Instantiate core (extracted out of
// internal/workbench/boardspecapi.go), never two independent
// implementations that happen to agree today.
func TestDesignStartFromStub_ParityWithBoardAction(t *testing.T) {
	boardRepo := buildFromStubRepo(t)
	cliRepo := buildFromStubRepo(t)

	// fixturegit's deterministic authorship/dates (CLAUDE.md: "fixturegit
	// stable SHAs") mean two independent builds of the IDENTICAL layer
	// list land on the identical HEAD commit — the two repos start from
	// the same content, which is what makes an equality assertion between
	// them meaningful rather than coincidental.
	if boardRepo.Head != cliRepo.Head {
		t.Fatalf("fixture repos diverged before either path ran: board HEAD=%s cli HEAD=%s", boardRepo.Head, cliRepo.Head)
	}

	h := workbench.NewHandler(boardRepo.Dir)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/board/spec/"+fromStubFeatureName+"/api/stub-instantiate", strings.NewReader(`{"id":"fromstub-story"}`))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("board stub-instantiate = %d\n%s", rec.Code, rec.Body.String())
	}

	var stdout, stderr strings.Builder
	if got := runDesignStartFromStub(context.Background(), cliRepo.Dir, fromStubFeatureName, "fromstub-story", model.Canonical(), &stdout, &stderr); got != 0 {
		t.Fatalf("runDesignStartFromStub = %d, want 0; stderr=%s", got, stderr.String())
	}

	ctx := context.Background()
	boardBlob, err := gitx.Show(ctx, boardRepo.Dir, "design/fromstub-story", ".verdi/specs/active/fromstub-story/spec.md")
	if err != nil {
		t.Fatalf("Show (board): %v", err)
	}
	cliBlob, err := gitx.Show(ctx, cliRepo.Dir, "design/fromstub-story", ".verdi/specs/active/fromstub-story/spec.md")
	if err != nil {
		t.Fatalf("Show (cli): %v", err)
	}
	if string(boardBlob) != string(cliBlob) {
		t.Fatalf("board and CLI --from-stub rendered different content:\nboard:\n%s\ncli:\n%s", boardBlob, cliBlob)
	}
}
