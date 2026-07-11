package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/fixturegit"
)

// corpusTestdataDir is testdata/corpus relative to this package, the same
// fixture internal/corpus's own tests build (the shared "committed zone
// gets fixturegit-built, mutable/derived gets copied onto disk verbatim"
// pattern).
const corpusTestdataDir = "../../testdata/corpus"

// buildCorpusRepo builds testdata/corpus's committed zone into a real
// fixturegit repo (layers.txt-driven, same as internal/corpus's own
// buildFixtureRepo), writes a minimal verdi.yaml so store.FindRoot can
// find it, and copies testdata/corpus/derived/ onto disk under
// .verdi/data/derived/ — mirroring the real store's derived tree, using
// the corpus's own commit dir names, which are themselves real
// fixturegit-built commit SHAs (layers 2 and 3), so gitx.IsAncestor
// resolves against real history rather than needing a fake.
func buildCorpusRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()

	order, files := parseCorpusLayers(t)
	var layers []fixturegit.Layer
	for _, n := range order {
		layerFiles := map[string]string{}
		for _, rel := range files[n] {
			data, err := os.ReadFile(filepath.Join(corpusTestdataDir, rel))
			if err != nil {
				t.Fatalf("reading corpus file %s (layer %d): %v", rel, n, err)
			}
			layerFiles[rel] = string(data)
		}
		layers = append(layers, fixturegit.Layer{Files: layerFiles, Message: fmt.Sprintf("layer %d", n)})
	}
	repo := fixturegit.Build(t, layers)

	// verdi.yaml is not part of the corpus's own committed-zone fixture
	// (testdata/corpus predates it needing one); write a minimal one
	// directly to disk — store.FindRoot only requires the file to exist,
	// not be git-tracked.
	if err := os.MkdirAll(filepath.Join(repo.Dir, ".verdi"), 0o755); err != nil {
		t.Fatalf("mkdir .verdi: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi", "verdi.yaml"), []byte("schema: verdi.layout/v1\nforge: gitlab\n"), 0o644); err != nil {
		t.Fatalf("writing verdi.yaml: %v", err)
	}

	// Derived data lives in data/ (gitignored, never fixturegit-tracked);
	// copy testdata/corpus/derived/ verbatim onto the built repo's own
	// data/derived/ tree, preserving the corpus's commit-named
	// subdirectories.
	copyDerivedTree(t, filepath.Join(corpusTestdataDir, "derived"), filepath.Join(repo.Dir, ".verdi", "data", "derived"))

	return repo
}

func copyDerivedTree(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("reading %s: %v", src, err)
	}
	for _, refEntry := range entries {
		refSrc := filepath.Join(src, refEntry.Name())
		commitEntries, err := os.ReadDir(refSrc)
		if err != nil {
			t.Fatalf("reading %s: %v", refSrc, err)
		}
		for _, commitEntry := range commitEntries {
			commitSrc := filepath.Join(refSrc, commitEntry.Name())
			verdictsSrc := filepath.Join(commitSrc, "verdicts.json")
			data, err := os.ReadFile(verdictsSrc)
			if err != nil {
				t.Fatalf("reading %s: %v", verdictsSrc, err)
			}
			dstDir := filepath.Join(dst, refEntry.Name(), commitEntry.Name())
			if err := os.MkdirAll(dstDir, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", dstDir, err)
			}
			if err := os.WriteFile(filepath.Join(dstDir, "verdicts.json"), data, 0o644); err != nil {
				t.Fatalf("writing %s: %v", filepath.Join(dstDir, "verdicts.json"), err)
			}
		}
	}
}

// parseCorpusLayers reads testdata/corpus/layers.txt, the same format
// internal/corpus/corpus_test.go's own parseLayers reads (duplicated here
// rather than exported cross-package, since it is test-only plumbing).
func parseCorpusLayers(t *testing.T) (order []int, files map[int][]string) {
	t.Helper()
	f, err := os.Open(filepath.Join(corpusTestdataDir, "layers.txt"))
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

// TestCmdMatrix_Golden runs `verdi matrix jira:LOAN-1482` (I-30: a
// scheme-prefixed story ref, matched against the feature spec's story:
// field) against the real fixturegit-built corpus and checks the result
// byte-for-byte: ac-1 (static, one ci pass record) is evidenced; ac-2
// (static+behavioral, only a behavioral ci pass record — no static record
// at all) is pending; ac-3 (behavioral, only a source:local abstain
// record, excluded from the authoritative fold) is no-signal; ac-4
// (runtime) is waived by an active waiver.
//
// Note on ac-4: the fold consults waivers/<slug>/ keyed by the story's own
// ref slug — store.RefSlug("jira:LOAN-1482") = "jira-loan-1482" (I-31's
// canonical <story> path segment). The corpus carries an active waiver at
// waivers/jira-loan-1482/ac-4.md under exactly that segment, so the waiver
// is reachable and ac-4 folds to waived. Story: not violated, but not
// eligible either — ac-2 (pending) and ac-3 (no-signal) keep it short of
// the all-evidenced-or-waived bar.
func TestCmdMatrix_Golden(t *testing.T) {
	repo := buildCorpusRepo(t)
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runMatrixForTest(t, []string{"jira:LOAN-1482"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdMatrix exit = %d, want 0 (matrix reports, never gates); stderr=%q", got, stderr.String())
	}

	want := `story: jira:LOAN-1482
spec:  spec/stale-decline

AC    STATUS     EVIDENCE                      TEXT
ac-1  evidenced  static:pass                   static obligation holds for the retry path
ac-2  pending    static:none; behavioral:pass  static and behavioral: charge API retried on stale decline
ac-3  no-signal  behavioral:none               behavioral: golden flow for partial refunds
ac-4  waived     runtime:awaited               runtime: post-deploy decline-rate check

story.violated: false
story.eligible: false
`
	if stdout.String() != want {
		t.Fatalf("matrix output mismatch:\n--- got ---\n%s\n--- want ---\n%s", stdout.String(), want)
	}
}

// TestCmdMatrix_Preview_DiffersExactlyByAdvisoryRecords proves --preview's
// output differs from the authoritative run only in what the advisory
// (source: local) ac-3 record changes: ac-3 goes from no-signal to
// pending (the local abstain record now counts as behavioral signal), and
// nothing else about the table changes.
func TestCmdMatrix_Preview_DiffersExactlyByAdvisoryRecords(t *testing.T) {
	repo := buildCorpusRepo(t)
	t.Chdir(repo.Dir)

	var authoritative, preview bytes.Buffer
	if got := runMatrixForTest(t, []string{"jira:LOAN-1482"}, &authoritative, &bytes.Buffer{}); got != 0 {
		t.Fatalf("authoritative run exit = %d, want 0", got)
	}
	if got := runMatrixForTest(t, []string{"jira:LOAN-1482", "--preview"}, &preview, &bytes.Buffer{}); got != 0 {
		t.Fatalf("preview run exit = %d, want 0", got)
	}

	wantPreview := `story: jira:LOAN-1482
spec:  spec/stale-decline
PREVIEW: advisory (source: local) evidence included alongside authoritative (source: ci)

AC    STATUS     EVIDENCE                      TEXT
ac-1  evidenced  static:pass                   static obligation holds for the retry path
ac-2  pending    static:none; behavioral:pass  static and behavioral: charge API retried on stale decline
ac-3  pending    behavioral:abstain            behavioral: golden flow for partial refunds
ac-4  waived     runtime:awaited               runtime: post-deploy decline-rate check

story.violated: false
story.eligible: false
`
	if preview.String() != wantPreview {
		t.Fatalf("preview output mismatch:\n--- got ---\n%s\n--- want ---\n%s", preview.String(), wantPreview)
	}

	// The only content difference between the two runs is the PREVIEW
	// banner line and ac-3's row (no-signal -> pending, evidence
	// none -> abstain) — every other line is byte-identical.
	authLines := strings.Split(strings.TrimRight(authoritative.String(), "\n"), "\n")
	previewLines := strings.Split(strings.TrimRight(preview.String(), "\n"), "\n")
	previewLinesNoBanner := make([]string, 0, len(previewLines))
	for _, l := range previewLines {
		if strings.HasPrefix(l, "PREVIEW:") {
			continue
		}
		previewLinesNoBanner = append(previewLinesNoBanner, l)
	}
	if len(authLines) != len(previewLinesNoBanner) {
		t.Fatalf("line count differs beyond the PREVIEW banner: authoritative=%d preview=%d", len(authLines), len(previewLinesNoBanner))
	}
	diffCount := 0
	for i := range authLines {
		if authLines[i] != previewLinesNoBanner[i] {
			diffCount++
			if !strings.HasPrefix(authLines[i], "ac-3") {
				t.Fatalf("unexpected diff outside ac-3's row: authoritative=%q preview=%q", authLines[i], previewLinesNoBanner[i])
			}
		}
	}
	if diffCount != 1 {
		t.Fatalf("expected exactly 1 differing row (ac-3), got %d", diffCount)
	}
}

// TestRun_MatrixDispatchesToRealVerb proves dispatch.go routes "matrix" to
// the real implementation, matching the equivalent lint/sync tests.
func TestRun_MatrixDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())

	var stderr bytes.Buffer
	got := run([]string{"matrix", "STORY-1482"}, &stderr)
	if got != 2 {
		t.Fatalf("run([matrix, STORY-1482]) outside a store = %d, want 2 (operational)", got)
	}
	if strings.Contains(stderr.String(), "usage") || strings.Contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}

// runMatrixForTest calls cmdMatrix directly — the same entry point
// dispatch.go's run() calls for the "matrix" verb.
func runMatrixForTest(t *testing.T, args []string, stdout, stderr io.Writer) int {
	t.Helper()
	return cmdMatrix(args, stdout, stderr)
}

// TestCmdMatrix_Negative covers cmdMatrix's own operational-error paths
// that don't need a real store: missing story argument, an unexpected
// extra positional argument, and no findable store root.
func TestCmdMatrix_Negative(t *testing.T) {
	t.Run("no story argument", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdMatrix(nil, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdMatrix(no args) = %d, want 2", got)
		}
		if stderr.Len() == 0 {
			t.Fatal("stderr empty, want a usage message")
		}
	})

	t.Run("extra positional argument", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdMatrix([]string{"STORY-1482", "spec/other"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdMatrix(two positional args) = %d, want 2", got)
		}
	})

	t.Run("no store root", func(t *testing.T) {
		t.Chdir(t.TempDir())
		var stdout, stderr bytes.Buffer
		got := cmdMatrix([]string{"jira:LOAN-1482"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdMatrix(no store root) = %d, want 2", got)
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty on an operational error", stdout.String())
		}
	})
}

// TestCmdMatrix_RefForms drives cmdMatrix end-to-end against the real
// corpus to pin I-30's strict ref contract: a bare tracker key is an
// operational error naming both accepted forms; a well-formed but unknown
// scheme-prefixed story ref is an operational error naming no matching
// spec; and the spec-ref path still folds and prints the same story.
func TestCmdMatrix_RefForms(t *testing.T) {
	repo := buildCorpusRepo(t)
	t.Chdir(repo.Dir)

	t.Run("bare tracker key exits 2 naming both accepted forms", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdMatrix([]string{"STORY-1482"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdMatrix(STORY-1482) = %d, want 2 (operational error); stderr=%q", got, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty on an operational error", stdout.String())
		}
		msg := stderr.String()
		if !strings.Contains(msg, "jira:LOAN-1482") || !strings.Contains(msg, "spec/") {
			t.Fatalf("stderr %q must name both accepted forms (a scheme-prefixed story ref and a spec ref)", msg)
		}
	})

	t.Run("unknown scheme-prefixed story ref exits 2 naming no matching spec", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdMatrix([]string{"jira:NOPE-1"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdMatrix(jira:NOPE-1) = %d, want 2 (operational error); stderr=%q", got, stderr.String())
		}
		if !strings.Contains(stderr.String(), "jira:NOPE-1") {
			t.Fatalf("stderr %q should name the unmatched story ref", stderr.String())
		}
	})

	t.Run("spec ref path still folds", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdMatrix([]string{"spec/stale-decline"}, &stdout, &stderr)
		if got != 0 {
			t.Fatalf("cmdMatrix(spec/stale-decline) = %d, want 0; stderr=%q", got, stderr.String())
		}
		if !strings.HasPrefix(stdout.String(), "story: jira:LOAN-1482\nspec:  spec/stale-decline\n") {
			t.Fatalf("spec-ref output header mismatch:\n%s", stdout.String())
		}
	})
}
