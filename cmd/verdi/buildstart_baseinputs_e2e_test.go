package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
)

// baseInputsRepo builds a repository whose main carries each of layers in
// turn, with a local bare origin whose default branch resolves to main's
// tip, then checks out elsewhere at main~from (0: main itself) and commits
// head's files there, beside elsewhere.txt. It returns the
// repository and origin/main's commit, build start's base.
func baseInputsRepo(t *testing.T, layers []map[string]string, from int, head map[string]string) (dir, base string) {
	t.Helper()
	fl := make([]fixturegit.Layer, len(layers))
	for i, files := range layers {
		fl[i] = fixturegit.Layer{Files: files, Message: "main layer"}
	}
	repo := fixturegit.Build(t, fl)
	git := func(args ...string) string { return gitTestOutput(t, repo.Dir, args...) }
	bare := t.TempDir()
	gitTestOutput(t, bare, "init", "-q", "--bare", "--initial-branch=main")
	git("remote", "add", "origin", bare)
	git("push", "-q", "origin", "main")
	git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	base = strings.TrimSpace(git("rev-parse", "origin/main"))
	if from == 0 && head == nil {
		return repo.Dir, base
	}
	git("checkout", "-q", "-b", "elsewhere", fmt.Sprintf("main~%d", from))
	head = withFile(head, "elsewhere.txt", "only on elsewhere\n")
	for rel, content := range head {
		full := filepath.Join(repo.Dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		git("add", "--", rel)
	}
	git("commit", "-q", "-m", "elsewhere")
	return repo.Dir, base
}

// withFile returns files plus rel, never changing the caller's map.
func withFile(files map[string]string, rel, content string) map[string]string {
	out := make(map[string]string, len(files)+1)
	for k, v := range files {
		out[k] = v
	}
	out[rel] = content
	return out
}

// TestBuildStart_JudgesPreconditionsOnlyAgainstItsBase is ledger SI-334
// (2), as amended, through the built binary (the R4-A2 probe P7 and its
// converse): build start resolves its base and checks the branch name
// first, then judges its preconditions — the cascade check, the
// obligation-quality check, and the conflict gate — only when the governed
// inputs (the spec's directory, its obligations, the policy store, the
// cascade check's inputs, the store manifest, and the instruction-
// projection files) equal the base commit's tree, both in the working tree
// and in HEAD's commit tree. Otherwise it refuses with exit 2 before any
// effect, naming every differing path. UAT-023 still holds: a tree that
// differs only outside those paths cuts at the base.
func TestBuildStart_JudgesPreconditionsOnlyAgainstItsBase(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	story := buildStartStoreFiles()
	policy := contextPolicyStoreFiles(t)
	var policyPaths []string
	for rel := range policy {
		policyPaths = append(policyPaths, rel)
	}
	obligation := ".verdi/obligations/widget-story/ac-1--static.md"
	edited := buildQualityObligationDocument("widget-story", "ac-1", artifact.EvidenceStatic,
		strings.Replace(buildQualityBlock(), "claim: claim", "claim: a claim edited only on HEAD's branch", 1))
	adoptedWithProjection := withFile(policy, "AGENTS.md", "# the managed projection, as the base carries it\n")
	tests := []struct {
		name       string
		layers     []map[string]string
		from       int
		head       map[string]string
		dirty      map[string]string // uncommitted working-tree writes
		branch     bool              // feature/widget-story exists before the run
		wantExit   int
		wantNamed  []string // stderr names each
		wantAbsent []string // stderr names none
	}{
		{name: "a pre-adoption HEAD against an adopted base (P7)", layers: []map[string]string{story, policy}, from: 1,
			wantExit: 2, wantNamed: append([]string{"differs from the base"}, policyPaths...)},
		{name: "an adopted HEAD against a pre-adoption base", layers: []map[string]string{story}, head: policy,
			wantExit: 2, wantNamed: append([]string{"differs from the base"}, policyPaths...)},
		{name: "an obligation edited on HEAD's branch only", layers: []map[string]string{story}, head: map[string]string{obligation: edited},
			wantExit: 2, wantNamed: []string{"differs from the base", obligation}},
		{name: "another active spec on HEAD's branch, read by the cascade check", layers: []map[string]string{story},
			head:     map[string]string{".verdi/specs/active/other/spec.md": recoverE2ESpecMD},
			wantExit: 2, wantNamed: []string{"differs from the base", ".verdi/specs/active/other/spec.md"}},
		{name: "an uncommitted obligation edit at the base", layers: []map[string]string{story}, dirty: map[string]string{obligation: edited},
			wantExit: 2, wantNamed: []string{"differs from the base", "in the working tree at " + obligation}},
		{name: "HEAD's commit edits an obligation the working tree restores to the base", layers: []map[string]string{story},
			head: map[string]string{obligation: edited}, dirty: map[string]string{obligation: story[obligation]},
			wantExit: 2, wantNamed: []string{"differs from the base", "in HEAD's commit at " + obligation}},
		{name: "a differing store manifest", layers: []map[string]string{story},
			head:     map[string]string{".verdi/verdi.yaml": story[".verdi/verdi.yaml"] + "# edited only on HEAD's branch\n"},
			wantExit: 2, wantNamed: []string{"differs from the base", ".verdi/verdi.yaml"}},
		{name: "a differing store model (ledger SI-336)", layers: []map[string]string{story},
			head:     map[string]string{".verdi/model.yaml": vocabModelYAML(t)},
			wantExit: 2, wantNamed: []string{"differs from the base", ".verdi/model.yaml"}},
		{name: "a differing instruction-projection file", layers: []map[string]string{story, adoptedWithProjection},
			head:     map[string]string{"AGENTS.md": "# edited only on HEAD's branch\n"},
			wantExit: 2, wantNamed: []string{"differs from the base", "AGENTS.md"}},
		{name: "a tree that differs only outside the governed inputs (UAT-023)", layers: []map[string]string{story},
			head:     map[string]string{".verdi/specs/active/other/board.json": "{}\n", "docs/notes.md": "notes\n"},
			wantExit: 0},
		{name: "an adopted base and HEAD still meet the conflict gate", layers: []map[string]string{story, policy},
			wantExit: 2, wantNamed: []string{"--context-request is required after constitution adoption"}},
		{name: "the collision check runs before the conflict gate", layers: []map[string]string{story, policy}, branch: true,
			wantExit: 2, wantNamed: []string{"feature/widget-story already exists as refs/heads/feature/widget-story"}},
		// Re-review RR-A4: ledger SI-334 (2) runs the collision check before
		// the governed-input check, so with both failing the collision is
		// the refusal.
		{name: "a collision and a governed difference: the collision refuses", layers: []map[string]string{story}, branch: true,
			dirty: map[string]string{obligation: edited}, wantExit: 2,
			wantNamed:  []string{"feature/widget-story already exists as refs/heads/feature/widget-story"},
			wantAbsent: []string{"differs from the base"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir, base := baseInputsRepo(t, tt.layers, tt.from, tt.head)
			git := func(args ...string) string { return gitTestOutput(t, dir, args...) }
			for rel, content := range tt.dirty {
				if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tt.branch {
				git("branch", "feature/widget-story", "main")
			}
			beforeRefs := git("for-each-ref", "--format=%(refname) %(objectname)")
			beforeHead := git("rev-parse", "--symbolic-full-name", "HEAD")
			beforeStatus := git("status", "--porcelain=v1", "--untracked-files=all")

			stdout, stderr, code := runVerdiBinary(t, bin, dir, []string{"CI_DEFAULT_BRANCH="}, "build", "start", "spec/widget-story")
			if code != tt.wantExit {
				t.Fatalf("build start: exit %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tt.wantExit, stdout, stderr)
			}
			for _, want := range tt.wantNamed {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr = %q, want it to name %q", stderr, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(stderr, absent) {
					t.Errorf("stderr = %q, want no %q", stderr, absent)
				}
			}
			if code == 0 {
				if tip := strings.TrimSpace(git("rev-parse", "refs/heads/feature/widget-story")); tip != base {
					t.Fatalf("feature/widget-story = %s, want the base %s (UAT-023)", tip, base)
				}
				return
			}
			if after := git("for-each-ref", "--format=%(refname) %(objectname)"); after != beforeRefs {
				t.Errorf("the refusal changed refs:\nbefore %s\nafter  %s", beforeRefs, after)
			}
			if after := git("rev-parse", "--symbolic-full-name", "HEAD"); after != beforeHead {
				t.Errorf("the refusal moved HEAD from %s to %s", beforeHead, after)
			}
			if after := git("status", "--porcelain=v1", "--untracked-files=all"); after != beforeStatus {
				t.Errorf("the refusal changed the working tree or index:\nbefore %s\nafter  %s", beforeStatus, after)
			}
		})
	}
}
