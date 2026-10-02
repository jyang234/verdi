package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
)

// TestBuildGovernedInputs (ledger SI-334 (2)): build start's governed
// inputs are the spec's directory, its obligations, and the policy store,
// plus what the cascade check reads — every active spec's spec.md and the
// story's re-affirmations — only when the spec implements a feature, the
// one case the cascade check reads anything.
func TestBuildGovernedInputs(t *testing.T) {
	implementing := &artifact.SpecFrontmatter{
		Base:  artifact.Base{Links: []artifact.Link{{Type: artifact.LinkImplements, Ref: "spec/some-feature#ac-1"}}},
		Story: "jira:WIDGET-1",
	}
	standalone := &artifact.SpecFrontmatter{Story: "jira:WIDGET-1"}
	tests := []struct {
		name      string
		spec      *artifact.SpecFrontmatter
		wantTrees []string
		wantGlobs []string
	}{
		{"a story implementing a feature", implementing,
			[]string{".verdi/specs/active/widget-story", ".verdi/obligations/widget-story", ".verdi/policy", ".verdi/reaffirmations/jira-widget-1"},
			[]string{".verdi/specs/active/*/spec.md"}},
		{"a spec implementing nothing", standalone,
			[]string{".verdi/specs/active/widget-story", ".verdi/obligations/widget-story", ".verdi/policy"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildGovernedInputs("widget-story", tt.spec)
			if !slices.Equal(got.trees, tt.wantTrees) || !slices.Equal(got.globs, tt.wantGlobs) {
				t.Fatalf("buildGovernedInputs = trees %v globs %v, want %v and %v", got.trees, got.globs, tt.wantTrees, tt.wantGlobs)
			}
		})
	}
}

// TestGovernedInputs_Covers: a tree covers itself and everything under it,
// never a sibling sharing its prefix; a glob covers what path.Match does.
func TestGovernedInputs_Covers(t *testing.T) {
	g := governedInputs{trees: []string{".verdi/specs/active/widget-story", ".verdi/policy"}, globs: []string{".verdi/specs/active/*/spec.md"}}
	for rel, want := range map[string]bool{
		".verdi/specs/active/widget-story/spec.md":    true,
		".verdi/specs/active/widget-story/board.json": true,
		".verdi/specs/active/other/spec.md":           true,
		".verdi/specs/active/other/board.json":        false,
		".verdi/specs/active/widget-story-2/spec.md":  true,
		".verdi/specs/active/widget-story-2/plan.md":  false,
		".verdi/policy":                   true,
		".verdi/policy/constitution.md":   true,
		".verdi/policyx/constitution.md":  false,
		".verdi/specs/active/a/b/spec.md": false,
		"elsewhere.txt":                   false,
	} {
		if got := g.covers(rel); got != want {
			t.Errorf("covers(%q) = %v, want %v", rel, got, want)
		}
	}
}

// TestDifferingGovernedInputs: the working tree's content at the governed
// paths is compared with the base commit's tree — a changed, added,
// untracked, ignored, or deleted file is named, by its store-relative
// path; a change outside them is not; a nested store is compared under its
// own prefix; and a base that does not resolve is an error.
func TestDifferingGovernedInputs(t *testing.T) {
	ctx := context.Background()
	g := governedInputs{trees: []string{".verdi/obligations/s", ".verdi/policy"}, globs: []string{".verdi/specs/active/*/spec.md"}}
	committed := map[string]string{
		".verdi/obligations/s/ac-1--static.md": "obligation\n",
		".verdi/policy/constitution.md":        "constitution\n",
		".verdi/specs/active/s/spec.md":        "spec\n",
		".verdi/specs/active/s/board.json":     "{}\n",
		"elsewhere.txt":                        "outside\n",
	}
	write := func(t *testing.T, root, rel, content string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name   string
		prefix string // the store root's directory inside the repository
		change func(t *testing.T, root string)
		want   []string
	}{
		{"identical", "", func(*testing.T, string) {}, nil},
		{"a change outside the governed paths", "", func(t *testing.T, root string) {
			write(t, root, "elsewhere.txt", "edited\n")
			write(t, root, ".verdi/specs/active/s/board.json", "{\"edited\":true}\n")
			write(t, root, "new.txt", "untracked\n")
		}, nil},
		{"an edited obligation", "", func(t *testing.T, root string) {
			write(t, root, ".verdi/obligations/s/ac-1--static.md", "edited\n")
		}, []string{".verdi/obligations/s/ac-1--static.md"}},
		{"an untracked policy file and a new active spec", "", func(t *testing.T, root string) {
			write(t, root, ".verdi/policy/overlays/o.md", "overlay\n")
			write(t, root, ".verdi/specs/active/t/spec.md", "another spec\n")
		}, []string{".verdi/policy/overlays/o.md", ".verdi/specs/active/t/spec.md"}},
		{"an ignored governed file is still read", "", func(t *testing.T, root string) {
			write(t, root, ".gitignore", ".verdi/policy/ignored.md\n")
			write(t, root, ".verdi/policy/ignored.md", "ignored but read\n")
		}, []string{".verdi/policy/ignored.md"}},
		{"a deleted governed file", "", func(t *testing.T, root string) {
			if err := os.RemoveAll(filepath.Join(root, ".verdi", "policy")); err != nil {
				t.Fatal(err)
			}
		}, []string{".verdi/policy/constitution.md"}},
		{"a nested store, identical", "store", func(*testing.T, string) {}, nil},
		{"a nested store, an edited spec", "store", func(t *testing.T, root string) {
			write(t, root, ".verdi/specs/active/s/spec.md", "edited\n")
		}, []string{".verdi/specs/active/s/spec.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := make(map[string]string, len(committed))
			for rel, content := range committed {
				files[filepath.ToSlash(filepath.Join(tt.prefix, rel))] = content
			}
			// The repository root carries a governed-looking path of its
			// own, outside a nested store, which must never be compared.
			if tt.prefix != "" {
				files[".verdi/policy/constitution.md"] = "the repository's, not the store's\n"
			}
			repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "base"}})
			root := filepath.Join(repo.Dir, tt.prefix)
			tt.change(t, root)
			got, err := differingGovernedInputs(ctx, root, repo.Head, g)
			if err != nil {
				t.Fatalf("differingGovernedInputs: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("differingGovernedInputs = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("a base commit whose tree differs from the working tree", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{
			{Files: committed, Message: "base"},
			{Files: map[string]string{".verdi/obligations/s/ac-1--static.md": "edited on HEAD's branch\n"}, Message: "HEAD's branch"},
		})
		base := strings.TrimSpace(gitTestOutput(t, repo.Dir, "rev-parse", "HEAD~1"))
		got, err := differingGovernedInputs(ctx, repo.Dir, base, g)
		if err != nil {
			t.Fatalf("differingGovernedInputs: %v", err)
		}
		if want := []string{".verdi/obligations/s/ac-1--static.md"}; !slices.Equal(got, want) {
			t.Fatalf("differingGovernedInputs = %v, want %v", got, want)
		}
	})

	t.Run("an unresolvable base is an error", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{Files: committed, Message: "base"}})
		if got, err := differingGovernedInputs(ctx, repo.Dir, strings.Repeat("0", 40), g); err == nil {
			t.Fatalf("differingGovernedInputs = %v, nil; want an error", got)
		}
	})

	t.Run("outside a repository is an error", func(t *testing.T) {
		if got, err := differingGovernedInputs(ctx, t.TempDir(), "HEAD", g); err == nil {
			t.Fatalf("differingGovernedInputs = %v, nil; want an error", got)
		}
	})
}
