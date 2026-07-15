package index

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

const corpusDir = "../../examples/showcase"
const svcfixDir = "../../testdata/svcfix"

// parseCorpusLayers reads examples/showcase/layers.txt (the same manifest
// internal/corpus's own tests use) and returns, in ascending layer order,
// each layer's corpus-relative file paths.
func parseCorpusLayers(t testing.TB) []fixturegit.Layer {
	t.Helper()
	f, err := os.Open(filepath.Join(corpusDir, "layers.txt"))
	if err != nil {
		t.Fatalf("opening layers.txt: %v", err)
	}
	defer func() { _ = f.Close() }()

	filesByLayer := map[int][]string{}
	var order []int
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
			t.Fatalf("layers.txt: bad layer number in %q: %v", line, err)
		}
		rel := strings.TrimSpace(parts[1])
		filesByLayer[n] = append(filesByLayer[n], rel)
		if !seen[n] {
			order = append(order, n)
			seen[n] = true
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanning layers.txt: %v", err)
	}
	sort.Ints(order)

	layers := make([]fixturegit.Layer, 0, len(order))
	for _, n := range order {
		files := map[string]string{}
		for _, rel := range filesByLayer[n] {
			data, err := os.ReadFile(filepath.Join(corpusDir, rel))
			if err != nil {
				t.Fatalf("reading corpus file %s (layer %d): %v", rel, n, err)
			}
			files[rel] = string(data)
		}
		layers = append(layers, fixturegit.Layer{Files: files, Message: fmt.Sprintf("layer %d", n)})
	}
	return layers
}

// copyTree recursively copies every regular file under src to dst,
// preserving relative paths (used to overlay testdata/svcfix's on-disk
// service root onto the fixturegit-built repo — svcfix need not be
// git-tracked for store.DiscoverServices or gitx.HashObject to see it).
func copyTree(t testing.TB, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copyTree(%s -> %s): %v", src, dst, err)
	}
}

// buildGoldenRepo builds examples/showcase via fixturegit (stable, golden
// SHAs) and overlays testdata/svcfix as a real service root at
// <repo>/svcfix/, matching PLAN.md phase 3's golden test: "the
// fixturegit-built corpus + svcfix".
func buildGoldenRepo(t testing.TB) string {
	t.Helper()
	repo := fixturegit.Build(t, parseCorpusLayers(t))
	copyTree(t, svcfixDir, filepath.Join(repo.Dir, "svcfix"))
	return repo.Dir
}

// wantCommittedRefs is every ref examples/showcase's committed zone must
// produce (every kind and status the corpus exercises, per PLAN.md §4).
var wantCommittedRefs = []string{
	"spec/store-layout-notes",
	"spec/legacy-cache-policy",
	"spec/new-feature-x",
	"spec/stale-decline",
	"spec/loan-refi-2023",
	"diagram/loansvc-topology",
	"adr/0001-outbox-events",
	"adr/0002-outbox-events",
	"adr/0003-retry-policy",
	"attestation/jira-loan-1482--ac-2",
	"waiver/jira-loan-1482--ac-3",
	"waiver/jira-loan-1482--ac-4",
	"conflict/stale-decline-incident",
	"conflict/legacy-cache-dispute",
	"conflict/false-alarm",
	"spec/escrow-notify",
	"spec/escrow-notify-v2",
	"spec/rate-lock",
	"spec/rate-lock-v2",
	"spec/refi-rate-check-2024",
}

var wantExternalRefs = []string{
	"svc/svcfix/boundary-contract",
	"svc/svcfix/obligations/audit-before-publish",
	"svc/svcfix/api",
}

func TestGolden_EveryCorpusArtifactIndexed(t *testing.T) {
	root := buildGoldenRepo(t)
	ix, err := Build(root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	for _, ref := range wantCommittedRefs {
		if _, ok := ix.Get(ref); !ok {
			t.Errorf("committed artifact %q not indexed", ref)
		}
	}

	wantTotal := len(wantCommittedRefs) + len(wantExternalRefs)
	if ix.Len() != wantTotal {
		got := make([]string, 0, ix.Len())
		for _, e := range ix.All() {
			got = append(got, e.Ref)
		}
		t.Fatalf("Len() = %d, want %d\nindexed refs: %v", ix.Len(), wantTotal, got)
	}
}

func TestGolden_ExternalRefsMinted(t *testing.T) {
	root := buildGoldenRepo(t)
	ix, err := Build(root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	for _, ref := range wantExternalRefs {
		e, ok := ix.Get(ref)
		if !ok {
			t.Errorf("external ref %q not minted", ref)
			continue
		}
		if e.Kind != "external" {
			t.Errorf("external ref %q has Kind %q, want %q", ref, e.Kind, "external")
		}
	}
}

// TestGolden_BacklinksCoverEveryExercisedInverseType asserts at least one
// backlink of every 02 §Link taxonomy inverse type the corpus exercises
// (every forward link type in examples/showcase except `story`, which has no
// inverse — verified against a full grep of the corpus in code review).
func TestGolden_BacklinksCoverEveryExercisedInverseType(t *testing.T) {
	root := buildGoldenRepo(t)
	ix, err := Build(root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	cases := []struct {
		inverseType string
		targetRef   string
		wantFrom    string
	}{
		{"implemented-by", "adr/0002-outbox-events", "spec/stale-decline"},
		{"superseded-by", "adr/0001-outbox-events", "adr/0002-outbox-events"},
		{"superseded-by", "spec/legacy-cache-policy", "spec/store-layout-notes"},
		{"verified-by", "spec/stale-decline", "attestation/jira-loan-1482--ac-2"},
		{"source-of", "spec/store-layout-notes", "diagram/loansvc-topology"},
		{"annotated-by", "spec/stale-decline", "conflict/false-alarm"},
		{"depended-on-by", "adr/0002-outbox-events", "adr/0003-retry-policy"},
		// impacted-by exercises the dangling-target case deliberately: the
		// corpus links to svc/loansvc/boundary-contract, but the only
		// discovered service in this fixture is svcfix — the backlink must
		// still be recorded even though svc/loansvc/boundary-contract is
		// not itself an indexed entry (lint, not the index, owns
		// resolution — VL-003).
		{"impacted-by", "svc/loansvc/boundary-contract", "spec/stale-decline"},
		{"challenged-by", "spec/legacy-cache-policy", "conflict/legacy-cache-dispute"},
		{"challenged-by", "spec/stale-decline", "conflict/false-alarm"},
		{"challenged-by", "spec/stale-decline", "conflict/stale-decline-incident"},
	}

	for _, tc := range cases {
		t.Run(tc.inverseType+" on "+tc.targetRef+" from "+tc.wantFrom, func(t *testing.T) {
			bl := ix.Backlinks(tc.targetRef)
			var found bool
			for _, b := range bl {
				if b.Type == tc.inverseType && b.From == tc.wantFrom {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("Backlinks(%q) = %+v, want one {From: %q, Type: %q}", tc.targetRef, bl, tc.wantFrom, tc.inverseType)
			}
		})
	}
}

func TestGolden_Search(t *testing.T) {
	root := buildGoldenRepo(t)
	ix, err := Build(root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// "outbox" is a known term across both outbox ADRs' titles/bodies.
	results := ix.Search("outbox")
	if len(results) == 0 {
		t.Fatal("Search(outbox): want at least one hit, got none")
	}
	foundADR2 := false
	for _, r := range results {
		if r.Ref == "adr/0002-outbox-events" {
			foundADR2 = true
		}
	}
	if !foundADR2 {
		t.Fatalf("Search(outbox) = %+v, want adr/0002-outbox-events among the hits", results)
	}

	// A bogus term must miss entirely.
	if results := ix.Search("zzznonexistentbogustermzzz"); len(results) != 0 {
		t.Fatalf("Search(bogus term) = %+v, want none", results)
	}
}
