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
// inputs are the spec's directory, its obligations, the policy store, and
// the store manifest and model the conflict gate's store load reads (ledger
// SI-336), plus what the cascade check
// reads — every active spec's spec.md and the story's re-affirmations —
// only when the spec implements a feature, the one case the cascade check
// reads anything. The instruction-projection files join them in a second
// phase (projectionInputs), once the policy store is known to agree.
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
			[]string{".verdi/specs/active/widget-story", ".verdi/obligations/widget-story", ".verdi/policy", ".verdi/verdi.yaml", ".verdi/model.yaml", ".verdi/reaffirmations/jira-widget-1"},
			[]string{".verdi/specs/active/*/spec.md"}},
		{"a spec implementing nothing", standalone,
			[]string{".verdi/specs/active/widget-story", ".verdi/obligations/widget-story", ".verdi/policy", ".verdi/verdi.yaml", ".verdi/model.yaml"}, nil},
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

// TestDifferingGovernedInputs: the governed paths' content, both in the
// working tree and in HEAD's commit tree, is compared with the base
// commit's tree — a changed, added, untracked, ignored, or deleted file is
// named, by its store-relative path, under the side it differs on; a
// change outside them is not; a nested store is compared under its own
// prefix; and a base that does not resolve is an error.
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
		// Symlinks under a governed path (re-review RR-A2): the
		// preconditions read through a link, so it is never skipped.
		{"an untracked symlink under a governed path is named", "", func(t *testing.T, root string) {
			symlink(t, "../../../elsewhere.txt", filepath.Join(root, ".verdi", "obligations", "s", "ac-2--static.md"))
		}, []string{".verdi/obligations/s/ac-2--static.md"}},
		{"a symlinked directory under a governed path differs", "", func(t *testing.T, root string) {
			other := t.TempDir()
			write(t, other, "x.md", "x\n")
			symlink(t, other, filepath.Join(root, ".verdi", "policy", "overlays"))
		}, []string{".verdi/policy/overlays"}},
		{"a tracked file replaced by a symlink whose target differs", "", func(t *testing.T, root string) {
			p := filepath.Join(root, ".verdi", "obligations", "s", "ac-1--static.md")
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
			symlink(t, "../../../elsewhere.txt", p)
		}, []string{".verdi/obligations/s/ac-1--static.md"}},
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
			// HEAD is the base here, so only the working tree can differ.
			if !slices.Equal(got.worktree, tt.want) || got.head != nil {
				t.Fatalf("differingGovernedInputs = %+v, want working tree %v and HEAD nil", got, tt.want)
			}
		})
	}

	edited := ".verdi/obligations/s/ac-1--static.md"
	for _, tt := range []struct {
		name         string
		restore      bool // the working tree restores the base's content
		wantWorktree []string
	}{
		{"HEAD's commit differs, and the working tree with it", false, []string{edited}},
		{"HEAD's commit differs while the working tree restores the base", true, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := fixturegit.Build(t, []fixturegit.Layer{
				{Files: committed, Message: "base"},
				{Files: map[string]string{edited: "edited on HEAD's branch\n"}, Message: "HEAD's branch"},
			})
			if tt.restore {
				write(t, repo.Dir, edited, committed[edited])
			}
			base := strings.TrimSpace(gitTestOutput(t, repo.Dir, "rev-parse", "HEAD~1"))
			got, err := differingGovernedInputs(ctx, repo.Dir, base, g)
			if err != nil {
				t.Fatalf("differingGovernedInputs: %v", err)
			}
			if !slices.Equal(got.worktree, tt.wantWorktree) || !slices.Equal(got.head, []string{edited}) {
				t.Fatalf("differingGovernedInputs = %+v, want working tree %v and HEAD [%s]", got, tt.wantWorktree, edited)
			}
		})
	}

	// Pinned as it stands, and left to BL-142: a symlink the base tracks
	// under a governed path always differs, since the working tree's side
	// hashes the link's target while the base's blob is the link itself,
	// so build start refuses with exit 2 (re-review s4).
	t.Run("a base that tracks a symlink under a governed path always differs (BL-142)", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{Files: committed, Message: "base"}})
		symlink(t, "constitution.md", filepath.Join(repo.Dir, ".verdi", "policy", "alias.md"))
		gitTestOutput(t, repo.Dir, "add", "--", ".verdi/policy/alias.md")
		gitTestOutput(t, repo.Dir, "commit", "-q", "-m", "a tracked symlink")
		base := strings.TrimSpace(gitTestOutput(t, repo.Dir, "rev-parse", "HEAD"))
		got, err := differingGovernedInputs(ctx, repo.Dir, base, g)
		if err != nil {
			t.Fatalf("differingGovernedInputs: %v", err)
		}
		if !slices.Equal(got.worktree, []string{".verdi/policy/alias.md"}) || got.head != nil {
			t.Fatalf("differingGovernedInputs = %+v, want the tracked symlink named in the working tree only", got)
		}
	})

	// A store root whose own path holds a glob metacharacter is never read
	// as part of the pattern (re-review RR-A3, probe P-A5).
	for _, tt := range []struct {
		name   string
		change func(t *testing.T, root string)
		want   []string
	}{
		{"a root holding [x], identical with a second active spec", func(*testing.T, string) {}, nil},
		{"a root holding [x], an untracked active spec added", func(t *testing.T, root string) {
			write(t, root, ".verdi/specs/active/u/spec.md", "an untracked superseding spec\n")
		}, []string{".verdi/specs/active/u/spec.md"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string]string{".verdi/specs/active/t/spec.md": "t\n"}
			for rel, content := range committed {
				files[rel] = content
			}
			repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "base"}})
			root := filepath.Join(t.TempDir(), "br[x]ket")
			if err := os.Rename(repo.Dir, root); err != nil {
				t.Fatal(err)
			}
			tt.change(t, root)
			got, err := differingGovernedInputs(ctx, root, repo.Head, g)
			if err != nil {
				t.Fatalf("differingGovernedInputs: %v", err)
			}
			if !slices.Equal(got.worktree, tt.want) || got.head != nil {
				t.Fatalf("differingGovernedInputs = %+v, want working tree %v and HEAD nil", got, tt.want)
			}
		})
	}

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

// TestProjectionInputs: once the policy store agrees, an adopted store's
// governed inputs gain every adapter's managed instruction-projection file
// (the conflict gate's projection check reads them); a store that has not
// adopted a constitution gains none; a store that cannot load is an error.
func TestProjectionInputs(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		want    []string
		wantErr bool
	}{
		{"not adopted", map[string]string{".verdi/verdi.yaml": minimalManifestYAML}, nil, false},
		{"adopted", contextPolicyStoreFiles(t), []string{"AGENTS.md"}, false},
		{"a policy store that does not load", map[string]string{".verdi/policy/constitution.md": "not a constitution\n"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := fixturegit.Build(t, []fixturegit.Layer{{Files: tt.files, Message: "store"}})
			got, err := projectionInputs(repo.Dir)
			if (err != nil) != tt.wantErr {
				t.Fatalf("projectionInputs = %+v, %v; want error %v", got, err, tt.wantErr)
			}
			if !slices.Equal(got.trees, tt.want) || got.globs != nil {
				t.Fatalf("projectionInputs = %+v, want trees %v", got, tt.want)
			}
		})
	}
}

// symlink creates link pointing at target, failing the test on error.
func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}
