package commitdesign

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/fixturegit"
	"github.com/OWNER/verdi/internal/gitx"
)

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
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
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

	res, err := Run(ctx, Input{Root: repo.Dir, BoardKey: "STORY-1482", SpecName: "my-new-feature", StoryRef: "jira:LOAN-1482"})
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
	if fb.Provenance.Generator != "commit-to-design" {
		t.Errorf("Provenance.Generator = %q", fb.Provenance.Generator)
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

	res, err := Run(ctx, Input{Root: repo.Dir, BoardKey: "jira:LOAN-2000", SpecName: "no-story-flag"})
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
		if _, err := Run(ctx, Input{Root: repo.Dir, BoardKey: "STORY-1482", SpecName: "dup", StoryRef: "jira:LOAN-1482"}); err != nil {
			t.Fatalf("first Run: %v", err)
		}
		if _, err := Run(ctx, Input{Root: repo.Dir, BoardKey: "STORY-1482", SpecName: "dup", StoryRef: "jira:LOAN-1482"}); err == nil {
			t.Fatal("expected an error the second time (spec dir already exists)")
		}
	})
}
