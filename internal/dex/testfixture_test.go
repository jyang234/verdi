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

// corpusDir and svcfixDir are testdata/corpus and testdata/svcfix relative
// to this package — the same committed, deterministic fixtures
// internal/corpus and internal/index already build on (PLAN.md §4).
const (
	corpusDir = "../../testdata/corpus"
	svcfixDir = "../../testdata/svcfix"
)

// corpusGoldenHeads mirrors internal/corpus's own golden SHA constants:
// layers 1-3 here are byte-identical to testdata/corpus/layers.txt's own
// layers, so they reproduce the exact same commit SHAs the corpus fixture
// files' own frozen stamps and pinned refs already bake in.
var corpusGoldenHeads = []string{
	"c5e360a9ee5e9eb6089e54b772fa16959ada4662", // layer 1
	"7176513ece8b608ab0911000691bb697ee7e75ec", // layer 2
	"93ddc5bbbb398cf747151e1c466afb83114398df", // layer 3
}

// parseCorpusLayers reads testdata/corpus/layers.txt (the same format
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

// dexOverlayDir is testdata/dexoverlay — V1-P8's dex-only fixture overlay
// (a spec-stale living report, a round-four archived quartet, and the
// open-MR supersession candidate; see its README.md).
const dexOverlayDir = "../../testdata/dexoverlay"

// buildDexFixtureRepo builds a git repo whose first three commits are
// byte-identical to testdata/corpus's own three layers (reproducing
// corpusGoldenHeads exactly — every frozen stamp and pinned ref inside
// those corpus files stays honest), plus a fourth commit that folds in
// testdata/svcfix wholesale at repo path "svcfix/" — giving dex's by-service
// axis and svc/... external refs something real to discover, without
// touching the first three commits' tree contents (and so their SHAs).
//
// V1-P8 appends two more layers, still leaving layers 1-3's SHAs
// untouched: the v2 fixture-overlay corpus files layers.txt never listed
// (the accepted-pending-build cluster, the loan-workflow supersession
// pair, the outcome attestation, the reaffirmation — everything on disk
// under testdata/corpus/.verdi/ beyond the v0 layers), then
// testdata/dexoverlay/'s dex-only files — so the built store matches what
// cmd/e2eharness provisions and the feature-lens/ladder/by-story pages
// have their fixtures.
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

	overlay := readTreeFiles(t, filepath.Join(dexOverlayDir, ".verdi"), ".verdi")
	layers = append(layers, fixturegit.Layer{Files: overlay, Message: "dex overlay: spec-stale report + round-four quartet"})

	repo := fixturegit.Build(t, layers)

	for i, want := range corpusGoldenHeads {
		if repo.Heads[i] != want {
			t.Fatalf("layer %d SHA = %s, want golden %s (corpus layers 1-3 must stay byte-identical to testdata/corpus's own fixture)", i+1, repo.Heads[i], want)
		}
	}
	return repo
}
