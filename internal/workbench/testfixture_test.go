package workbench

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// corpusDir is examples/showcase relative to this package — the same
// committed, deterministic fixture internal/dex, internal/index, and
// internal/lint's own tests build on (PLAN.md §4), and this phase's own
// assignment names explicitly: "examples/showcase (derived records at two
// commits — the verdict viewer's fixture)".
const corpusDir = "../../examples/showcase"

// corpusGoldenHeads mirrors internal/dex's own golden SHA constants:
// layers 1-4 reproduce examples/showcase's own four layers byte-identically,
// so every frozen stamp and pinned ref those files carry stays honest, and
// the derived/spec--stale-decline/<commit>/verdicts.json directories (keyed
// by these exact SHAs) line up with the built repo's real history.
var corpusGoldenHeads = []string{
	"66588948af8b36c02c8fb8f423645afa0a58dbe4", // layer 1
	"d70cb19fa17ced67d27b8f9a63b47b3bf280b7d1", // layer 2
	"faf8d8c412c9df35b5a445146a5fe0e8309caa71", // layer 3
	"a02dd7dd74cf087aa5ce91a2b49447147dc2132e", // layer 4
}

func parseCorpusLayers(t *testing.T) (order []int, files map[int][]string) {
	t.Helper()
	f, err := os.Open(filepath.Join(corpusDir, "layers.txt"))
	if err != nil {
		t.Fatalf("opening layers.txt: %v", err)
	}
	defer func() { _ = f.Close() }()

	files = map[int][]string{}
	seen := map[int]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			t.Fatalf("layers.txt: malformed line %q", line)
		}
		var n int
		if _, err := fmt.Sscanf(parts[0], "%d", &n); err != nil {
			t.Fatalf("layers.txt: bad layer number in line %q: %v", line, err)
		}
		rel := strings.TrimSpace(parts[1])
		files[n] = append(files[n], rel)
		if !seen[n] {
			order = append(order, n)
			seen[n] = true
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanning layers.txt: %v", err)
	}
	sort.Ints(order)
	return order, files
}

// buildWorkbenchFixtureRepo builds a git repo whose first three commits
// are byte-identical to examples/showcase's own three layers (reproducing
// corpusGoldenHeads exactly), then overlays the corpus's mutable/ and
// derived/ trees UNTRACKED (VL-013: nothing under data/ is ever
// git-tracked — the real store's mutable/derived zones are filesystem
// state, not git history) at data/mutable/ and data/derived/ under
// .verdi/, matching 01 §Directory layout.
func buildWorkbenchFixtureRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	order, files := parseCorpusLayers(t)

	var layers []fixturegit.Layer
	for _, n := range order {
		layerFiles := map[string]string{}
		for _, rel := range files[n] {
			data, err := os.ReadFile(filepath.Join(corpusDir, rel))
			if err != nil {
				t.Fatalf("reading corpus file %s (layer %d): %v", rel, n, err)
			}
			layerFiles[rel] = string(data)
		}
		layers = append(layers, fixturegit.Layer{Files: layerFiles, Message: fmt.Sprintf("layer %d", n)})
	}

	repo := fixturegit.Build(t, layers)

	for i, want := range corpusGoldenHeads {
		if repo.Heads[i] != want {
			t.Fatalf("layer %d SHA = %s, want golden %s (corpus layers 1-3 must stay byte-identical to examples/showcase's own fixture)", i+1, repo.Heads[i], want)
		}
	}

	copyTreeUntracked(t, filepath.Join(corpusDir, "mutable"), filepath.Join(repo.Dir, ".verdi", "data", "mutable"))
	copyTreeUntracked(t, filepath.Join(corpusDir, "derived"), filepath.Join(repo.Dir, ".verdi", "data", "derived"))

	// The corpus fixture predates the mutable zone and carries no
	// .verdi/.gitignore of its own; write one directly (untracked is
	// enough — git respects a working-tree .gitignore regardless of
	// whether it is itself tracked) so any test that exercises a real
	// `git add -A` (commit-to-design's write path) never sweeps the
	// mutable/derived overlay above into a commit (VL-013).
	if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi", ".gitignore"), []byte("data/\n"), 0o644); err != nil {
		t.Fatalf("writing .verdi/.gitignore: %v", err)
	}

	return repo
}

// copyTreeUntracked recursively copies every regular file under src to
// dst, creating directories as needed. Used for the mutable/derived
// overlay above (a plain filesystem copy — never routed through git, so
// nothing under it is ever tracked).
func copyTreeUntracked(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, path)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if merr := os.MkdirAll(filepath.Dir(target), 0o755); merr != nil {
			return merr
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copying %s -> %s: %v", src, dst, err)
	}
}
