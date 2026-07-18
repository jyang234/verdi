package commitdesign

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// fixtureModelYAML is internal/model/testdata/vocab-rename.yaml's own
// content verbatim (already proven frontier-legal by that package's own
// tests): structurally identical to the embedded canonical model, but with
// vocabulary renames and different per-class template filenames — the
// frontier's two named exceptions — so its Digest() differs from
// model.Canonical().Digest() while still resolving cleanly through
// store.Open. Used to prove (spec/model-digest ac-1) that a frozen board's
// stamped model digest tracks the ACTUAL resolved model, not a constant
// that happens to match the one model every other test fixture uses.
const fixtureModelYAML = `schema: verdi.model/v1

classes:
  feature:
    display: Feature
    decomposes: stubs
    template: custom-feature.md
  story:
    display: Story
    parent: feature
    template: custom-story.md

lifecycle:
  feature:
    states: [draft, accepted-pending-build, closed, superseded]
    terminal: [closed, superseded]
    transitions:
      - verb: accept
        from: draft
        to: accepted-pending-build
        obligations:
          - { scheme: attestation, kind: author-vouch }
      - verb: close
        from: accepted-pending-build
        to: closed
        obligations:
          - { scheme: attestation, kind: countersign, count: 1 }
          - { scheme: behavioral, kind: fold-green }
  story:
    states: [draft, accepted-pending-build, closed, superseded]
    terminal: [closed, superseded]
    transitions:
      - verb: accept
        from: draft
        to: accepted-pending-build
        obligations:
          - { scheme: attestation, kind: author-vouch }
      - verb: close
        from: accepted-pending-build
        to: closed
        obligations:
          - { scheme: attestation, kind: countersign, count: 1 }
          - { scheme: behavioral, kind: fold-green }

vocabulary:
  verbs:
    accept: "Sign off"
  states:
    accepted-pending-build: "Ready to build"
  classes:
    feature: "Initiative"
`

// testModelDigest resolves root's operating model digest exactly the way
// Run's real callers (cmd/verdi/board.go, internal/workbench's
// boardCommitHandler) do — via store.Open — so Input.ModelDigest carries a
// real, StampProvenance-accepted value in test.
func testModelDigest(t *testing.T, root string) string {
	t.Helper()
	cfg, err := store.Open(root)
	if err != nil {
		t.Fatalf("store.Open(%s): %v", root, err)
	}
	digest, err := cfg.Model.Digest()
	if err != nil {
		t.Fatalf("Model.Digest(): %v", err)
	}
	return digest
}

const testManifestYAML = `schema: verdi.layout/v1
forge: gitlab
providers:
  jira:
    base_url: https://example.atlassian.net
    rollup_field: customfield_00000
lint:
  gated_generated: []
services:
  discovery: flowmap
`

const testGitAttributes = `.verdi/specs/*/*/board.json          gitlab-generated
.verdi/specs/*/*/rollup.json         gitlab-generated
.verdi/specs/*/*/deviation-report.md gitlab-generated
`

// testOtherSpecYAML is a real, committed component spec (kind "spec",
// class "component" — no story, no ACs) that seedBoard's board.json pins
// as context, so VL-003 (link/pin resolution) has something real to
// resolve against instead of a fabricated all-zero commit.
const testOtherSpecYAML = `---
id: spec/other
kind: spec
class: component
title: "Other"
status: active
owners: [platform-team]
---
# Other

Fixture context spec, pinned by commit-to-design test boards.
`

// buildRepo builds a one-layer fixturegit repo carrying verdi.yaml,
// .gitattributes (VL-012), and one real component spec (spec/other) for
// board pins to resolve against — the common starting point for
// commit-to-design tests.
func buildRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                 testManifestYAML,
				".verdi/.gitignore":                 "data/\n",
				".gitattributes":                    testGitAttributes,
				".verdi/specs/active/other/spec.md": testOtherSpecYAML,
			},
			Message: "init store",
		},
	})
}

// writeBoard writes a mutable board state file directly to root's working
// tree (untracked — VL-013 forbids git-tracking anything under
// data/mutable/, so the fixture writes it the same way the real workbench
// autosave would: filesystem only).
func writeBoard(t *testing.T, root, key string, board *artifact.Board) {
	t.Helper()
	path := filepath.Join(root, ".verdi", "data", "mutable", "boards", key+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(board)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeAnnotation appends one board-only annotation record to root's
// mutable annotations stream, untracked.
func writeAnnotation(t *testing.T, root, fileName string, a map[string]any) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "data", "mutable", "annotations")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, fileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func seedBoard(t *testing.T, repo *fixturegit.Repo) {
	t.Helper()
	root := repo.Dir
	writeBoard(t, root, "STORY-1482", &artifact.Board{
		Schema: "verdi.board/v1",
		Pins:   []artifact.Pin{{Ref: "spec/other@" + repo.Head, X: 1, Y: 1}},
		Stickies: []artifact.Sticky{
			{ID: "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", X: 10, Y: 10},
			{ID: "a-01J8Z0K4BBBBBBBBBBBBBBBBBB", X: 20, Y: 20},
		},
		Yarn: []artifact.Yarn{{From: "pin:x", To: "sticky:a-01J8Z0K3AAAAAAAAAAAAAAAAAA", Label: "relates"}},
	})
	writeAnnotation(t, root, "board--story-1482.jsonl", map[string]any{
		"id": "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", "ts": "2026-05-10T14:02:11Z", "author": "john",
		"board": map[string]any{"story": "STORY-1482", "x": 10, "y": 10},
		"type":  "comment", "body": "note one", "status": "open",
	})
	writeAnnotation(t, root, "board--story-1482.jsonl", map[string]any{
		"id": "a-01J8Z0K4BBBBBBBBBBBBBBBBBB", "ts": "2026-05-10T14:03:00Z", "author": "jane",
		"board": map[string]any{"story": "STORY-1482", "x": 20, "y": 20},
		"type":  "question", "body": "note two", "status": "open",
	})
}

func TestRun_Happy(t *testing.T) {
	repo := buildRepo(t)
	seedBoard(t, repo)
	ctx := context.Background()

	res, err := Run(ctx, Input{Root: repo.Dir, BoardKey: "STORY-1482", SpecName: "my-new-feature", StoryRef: "jira:LOAN-1482", ModelDigest: testModelDigest(t, repo.Dir)})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.SpecRef != "spec/my-new-feature" {
		t.Errorf("SpecRef = %q", res.SpecRef)
	}
	if len(res.Dispositions) != 2 {
		t.Fatalf("Dispositions = %+v, want 2", res.Dispositions)
	}
	for _, d := range res.Dispositions {
		if d.Disposition != artifact.DispositionOpenQuestion {
			t.Errorf("disposition %+v, want open-question", d)
		}
	}

	// The spec.md written to disk decodes and validates cleanly.
	specPath := filepath.Join(repo.Dir, res.SpecRelPath)
	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("reading %s: %v", specPath, err)
	}
	fm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if spec.Story != "jira:LOAN-1482" {
		t.Errorf("spec.Story = %q", spec.Story)
	}
	if spec.Status != "draft" {
		t.Errorf("spec.Status = %q, want draft", spec.Status)
	}
	if len(spec.Context) != 1 || spec.Context[0] != "spec/other@"+repo.Head {
		t.Errorf("spec.Context = %+v, want the board's one pin", spec.Context)
	}
	if len(spec.Dispositions) != 2 {
		t.Errorf("spec.Dispositions = %+v, want 2", spec.Dispositions)
	}

	// The frozen board.json decodes, carries Frozen+Provenance, and its
	// digest is recomputable (self-consistency: DecodeBoard already
	// validates shape; here we just check both fields are present).
	boardPath := filepath.Join(repo.Dir, res.BoardRelPath)
	braw, err := os.ReadFile(boardPath)
	if err != nil {
		t.Fatalf("reading %s: %v", boardPath, err)
	}
	fb, err := artifact.DecodeBoard(braw)
	if err != nil {
		t.Fatalf("DecodeBoard: %v", err)
	}
	if fb.Frozen == nil || fb.Provenance == nil {
		t.Fatalf("frozen board.json missing Frozen/Provenance: %+v", fb)
	}
	// judged-ac5-board-freeze-wallclock: frozen.at is the covering (pre-commit)
	// commit's own committer date — here fixturegit's fixed 2024-01-01, never
	// today's wall clock — and pairs with Frozen.Commit (preCommit == repo.Head,
	// since seedBoard writes only untracked mutable files, adding no commit).
	wantAt, err := gitx.CommitDateOnly(ctx, repo.Dir, repo.Head)
	if err != nil {
		t.Fatalf("CommitDateOnly(%s): %v", repo.Head, err)
	}
	if fb.Frozen.At != wantAt {
		t.Errorf("frozen board At = %q, want the covering commit's date %q (commit-derived, never wall clock)", fb.Frozen.At, wantAt)
	}
	if fb.Frozen.Commit != repo.Head {
		t.Errorf("frozen board Commit = %q, want the covering commit %q", fb.Frozen.Commit, repo.Head)
	}
	if fb.Provenance.Generator != "commit-to-design" {
		t.Errorf("Provenance.Generator = %q", fb.Provenance.Generator)
	}
	// spec/model-digest ac-1: the frozen board's provenance.model equals
	// the resolved model's own Digest() (canonical here — no model.yaml).
	wantDigest := testModelDigest(t, repo.Dir)
	if fb.Provenance.Model != wantDigest {
		t.Errorf("Provenance.Model = %q, want %q (the resolved canonical model's digest)", fb.Provenance.Model, wantDigest)
	}
	if len(fb.Stickies) != 2 {
		t.Errorf("frozen board stickies = %+v, want 2", fb.Stickies)
	}

	// A new commit was created.
	head, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if head != res.Commit || head == repo.Head {
		t.Fatalf("expected a new HEAD commit; got %s (was %s)", head, repo.Head)
	}

	// Both stickies graduated in the mutable stream.
	annPath := filepath.Join(repo.Dir, ".verdi", "data", "mutable", "annotations", "board--story-1482.jsonl")
	annRaw, err := os.ReadFile(annPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(annRaw), `"status":"graduated"`) != 2 {
		t.Fatalf("expected both stickies graduated in %s:\n%s", annPath, annRaw)
	}
}

func TestRun_Happy_StoryRefDefaultsFromBoardKey(t *testing.T) {
	repo := buildRepo(t)
	writeBoard(t, repo.Dir, "jira:LOAN-2000", &artifact.Board{Schema: "verdi.board/v1"})
	ctx := context.Background()

	res, err := Run(ctx, Input{Root: repo.Dir, BoardKey: "jira:LOAN-2000", SpecName: "no-story-flag", ModelDigest: testModelDigest(t, repo.Dir)})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	specPath := filepath.Join(repo.Dir, res.SpecRelPath)
	raw, _ := os.ReadFile(specPath)
	fm, _, _ := artifact.SplitFrontmatter(raw)
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if spec.Story != "jira:LOAN-2000" {
		t.Errorf("spec.Story = %q, want the board key used verbatim", spec.Story)
	}
}

func TestRun_Negative(t *testing.T) {
	ctx := context.Background()

	t.Run("empty root", func(t *testing.T) {
		if _, err := Run(ctx, Input{BoardKey: "S", SpecName: "x", StoryRef: "jira:X-1"}); err == nil {
			t.Fatal("expected an error for an empty root")
		}
	})
	t.Run("invalid board key", func(t *testing.T) {
		repo := buildRepo(t)
		if _, err := Run(ctx, Input{Root: repo.Dir, BoardKey: "../escape", SpecName: "x", StoryRef: "jira:X-1"}); err == nil {
			t.Fatal("expected an error for a path-traversal board key")
		}
	})
	t.Run("invalid spec name", func(t *testing.T) {
		repo := buildRepo(t)
		seedBoard(t, repo)
		if _, err := Run(ctx, Input{Root: repo.Dir, BoardKey: "STORY-1482", SpecName: "Not_Valid", StoryRef: "jira:LOAN-1482"}); err == nil {
			t.Fatal("expected an error for a non-kebab-case spec name")
		}
	})
	t.Run("no story ref, and board key is not scheme:key shaped", func(t *testing.T) {
		repo := buildRepo(t)
		seedBoard(t, repo)
		if _, err := Run(ctx, Input{Root: repo.Dir, BoardKey: "STORY-1482", SpecName: "x"}); err == nil {
			t.Fatal("expected an error demanding an explicit StoryRef")
		}
	})
	t.Run("malformed story ref", func(t *testing.T) {
		repo := buildRepo(t)
		seedBoard(t, repo)
		if _, err := Run(ctx, Input{Root: repo.Dir, BoardKey: "STORY-1482", SpecName: "x", StoryRef: "not-a-ref"}); err == nil {
			t.Fatal("expected an error for a malformed story ref")
		}
	})
	t.Run("spec already exists", func(t *testing.T) {
		repo := buildRepo(t)
		seedBoard(t, repo)
		if _, err := Run(ctx, Input{Root: repo.Dir, BoardKey: "STORY-1482", SpecName: "dup", StoryRef: "jira:LOAN-1482", ModelDigest: testModelDigest(t, repo.Dir)}); err != nil {
			t.Fatalf("first Run: %v", err)
		}
		if _, err := Run(ctx, Input{Root: repo.Dir, BoardKey: "STORY-1482", SpecName: "dup", StoryRef: "jira:LOAN-1482", ModelDigest: testModelDigest(t, repo.Dir)}); err == nil {
			t.Fatal("expected an error the second time (spec dir already exists)")
		}
	})
}

// TestRun_ModelDigestTracksFixtureModel is spec/model-digest ac-1's
// distinguishing case: with a store `.verdi/model.yaml` that resolves to a
// DIFFERENT model than the embedded canonical, the frozen board's
// provenance.model must equal THAT model's own digest — proving the
// stamped value tracks the actual resolved model rather than a constant
// that happens to match the one model every other test fixture uses.
func TestRun_ModelDigestTracksFixtureModel(t *testing.T) {
	repo := buildRepo(t)
	seedBoard(t, repo)
	writeFixtureModelYAML(t, repo.Dir)
	ctx := context.Background()

	fixtureDigest := testModelDigest(t, repo.Dir)

	// A second, model.yaml-less repo resolves to the embedded canonical —
	// the baseline this fixture's digest must differ from.
	plainRepo := buildRepo(t)
	canonicalDigest := testModelDigest(t, plainRepo.Dir)
	if fixtureDigest == canonicalDigest {
		t.Fatalf("fixture model.yaml's digest %q equals the canonical digest — the fixture is not actually distinct", fixtureDigest)
	}

	res, err := Run(ctx, Input{
		Root: repo.Dir, BoardKey: "STORY-1482", SpecName: "tracks-fixture-model", StoryRef: "jira:LOAN-1482",
		ModelDigest: fixtureDigest,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	boardPath := filepath.Join(repo.Dir, res.BoardRelPath)
	braw, err := os.ReadFile(boardPath)
	if err != nil {
		t.Fatalf("reading %s: %v", boardPath, err)
	}
	fb, err := artifact.DecodeBoard(braw)
	if err != nil {
		t.Fatalf("DecodeBoard: %v", err)
	}
	if fb.Provenance == nil || fb.Provenance.Model != fixtureDigest {
		t.Fatalf("Provenance.Model = %+v, want %q (the fixture model's own digest)", fb.Provenance, fixtureDigest)
	}
	if fb.Provenance.Model == canonicalDigest {
		t.Fatalf("Provenance.Model %q equals the canonical digest — expected the distinct fixture model's digest", fb.Provenance.Model)
	}
}

// writeFixtureModelYAML writes fixtureModelYAML to root's .verdi/model.yaml
// — a store override, resolved by store.Open in preference to the embedded
// canonical default.
func writeFixtureModelYAML(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, ".verdi", "model.yaml")
	if err := os.WriteFile(path, []byte(fixtureModelYAML), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestFreezeBoard_ModelDigestDeterministic is ac-1's "identical across
// repeated runs" property, proven at freezeBoard's own level (a pure
// function of its arguments — no wall clock, no git I/O) rather than
// through two full Run calls, which would need two independently-built
// fixturegit repos and could only match byte-for-byte if their commit SHAs
// and today's wall-clock date happened to coincide: two calls with
// identical inputs must produce byte-identical frozen boards, including
// the new Model field.
func TestFreezeBoard_ModelDigestDeterministic(t *testing.T) {
	board := &artifact.Board{
		Schema: "verdi.board/v1",
		Pins:   []artifact.Pin{{Ref: "spec/other@abc1234", X: 1, Y: 1}},
	}
	modelDigest := "sha256:" + strings.Repeat("ab", 32)

	first, err := freezeBoard(board, ".verdi/data/mutable/boards/x.json", "abc1234", "2026-07-17", modelDigest)
	if err != nil {
		t.Fatalf("freezeBoard (first): %v", err)
	}
	second, err := freezeBoard(board, ".verdi/data/mutable/boards/x.json", "abc1234", "2026-07-17", modelDigest)
	if err != nil {
		t.Fatalf("freezeBoard (second): %v", err)
	}

	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("freezeBoard not byte-identical across calls with identical inputs:\n--- first ---\n%s\n--- second ---\n%s", firstJSON, secondJSON)
	}
	if first.Provenance.Model != modelDigest {
		t.Fatalf("Provenance.Model = %q, want %q", first.Provenance.Model, modelDigest)
	}
}
