package dex

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

// corpusDir and svcfixDir are examples/showcase and testdata/svcfix relative
// to this package — the same committed, deterministic fixtures
// internal/corpus and internal/index already build on (PLAN.md §4).
const (
	corpusDir = "../../examples/showcase"
	svcfixDir = "../../testdata/svcfix"
)

// corpusGoldenHeads mirrors internal/corpus's own golden SHA constants:
// layers 1-4 here are byte-identical to examples/showcase/layers.txt's own
// layers, so they reproduce the exact same commit SHAs the corpus fixture
// files' own frozen stamps and pinned refs already bake in.
var corpusGoldenHeads = []string{
	"2f230011b192c5ac1c0ed5442be76fc401c4cbca", // layer 1
	"6a0c563e4f688acdb225fcbc5e6942a7431b05bf", // layer 2
	"5507c6d963bd78d9eabed2324c3d380e678f891e", // layer 3
	"7b2ae03f6d5ec8a23cccca4521d7f20553d4df0a", // layer 4
}

// parseCorpusLayers reads examples/showcase/layers.txt (the same format
// internal/corpus's own parseLayers reads).
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

// readTreeFiles reads every regular file under dir, returning a map of
// dir-relative slash paths to content — used to fold testdata/svcfix
// wholesale into one fixturegit layer, at repo path "svcfix/...".
func readTreeFiles(t *testing.T, dir, destPrefix string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[destPrefix+"/"+filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("reading tree %s: %v", dir, err)
	}
	return out
}

// buildDexFixtureRepo builds a git repo whose first four commits are
// byte-identical to examples/showcase's own four layers.txt layers
// (reproducing corpusGoldenHeads exactly — every frozen stamp and pinned
// ref inside those corpus files stays honest; layer 4 is the former
// testdata/dexoverlay content — a spec-stale living report for
// borrower-update-mobile, a round-four archived quartet, and the
// supersession-chain surface fixtures — folded into layers.txt directly by
// Task 1.2, see examples/showcase/OVERLAY-NOTES.md), plus a fifth commit
// that folds in testdata/svcfix wholesale at repo path "svcfix/" — giving
// dex's by-service axis and svc/... external refs something real to
// discover, without touching the first four commits' tree contents (and so
// their SHAs).
//
// V1-P8 appends one more layer, still leaving layers 1-4's SHAs untouched:
// the v2 fixture-overlay corpus files layers.txt never listed (the
// accepted-pending-build cluster, the loan-workflow supersession pair, the
// outcome attestation, the reaffirmation — everything on disk under
// examples/showcase/.verdi/ beyond the layers.txt layers) — so the built
// store matches what cmd/e2eharness provisions and the
// feature-lens/ladder/by-story pages have their fixtures.
func buildDexFixtureRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	order, files := parseCorpusLayers(t)

	var layers []fixturegit.Layer
	inV0 := map[string]bool{}
	for _, n := range order {
		layerFiles := map[string]string{}
		for _, rel := range files[n] {
			data, err := os.ReadFile(filepath.Join(corpusDir, rel))
			if err != nil {
				t.Fatalf("reading corpus file %s (layer %d): %v", rel, n, err)
			}
			layerFiles[rel] = string(data)
			inV0[rel] = true
		}
		layers = append(layers, fixturegit.Layer{Files: layerFiles, Message: fmt.Sprintf("layer %d", n)})
	}

	svcFiles := readTreeFiles(t, svcfixDir, "svcfix")
	layers = append(layers, fixturegit.Layer{Files: svcFiles, Message: "add svcfix service"})

	v2Files := map[string]string{}
	for rel, content := range readTreeFiles(t, filepath.Join(corpusDir, ".verdi"), ".verdi") {
		if !inV0[rel] {
			v2Files[rel] = content
		}
	}
	layers = append(layers, fixturegit.Layer{Files: v2Files, Message: "v2 fixture overlay"})

	repo := fixturegit.Build(t, layers)

	for i, want := range corpusGoldenHeads {
		if repo.Heads[i] != want {
			t.Fatalf("layer %d SHA = %s, want golden %s (corpus layers 1-4 must stay byte-identical to examples/showcase's own fixture)", i+1, repo.Heads[i], want)
		}
	}
	return repo
}
