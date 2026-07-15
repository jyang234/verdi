// Package corpus builds the examples/showcase fixture into a deterministic
// git repository via internal/fixturegit and decodes every file in it
// through internal/artifact, proving the whole fixture corpus is both
// git-real (stable, golden SHAs) and contract-valid (every file decodes
// strictly). PLAN.md §4 / phase 2 deliverable 3.
package corpus

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/store"
)

// corpusDir is examples/showcase relative to this package.
const corpusDir = "../../examples/showcase"

// goldenHeads are the fixturegit commit SHAs for layers 1 through 5,
// baked in once per PLAN.md §4 ("Pins inside corpus files must be the
// literal deterministic SHAs (build once, bake in, test forever)") and
// reproduced by every corpus file's frozen stamps and pinned refs.
// Layer 5 (public-rollout-plan Task 1.6: adr/0004, adr/0005) is nothing's
// own predecessor citation, so nothing in the corpus cites its own head —
// it appears here only to satisfy TestFixtureRepo_MatchesGoldenSHAs' final
// Head check.
var goldenHeads = []string{
	"78e3161594fb31fdad17f2ea8a96b52f33dbf0f3", // layer 1
	"f6dd4c4df724c0b16cae435e96f7e34ac94026c9", // layer 2
	"16219044c9d6d41de9a0de9464ed24d49283b40c", // layer 3
	"38cc28c9f7bdf4098bccc724caddd0acdc2d17f6", // layer 4
	"2fa709f0136849286c834a7de4777c47a752d731", // layer 5
}

// goldenHeadsV2 are the v1-P1 rung-4 supersession pair's own, separate
// fixturegit history (internal/artifact/v2fixture_test.go's
// TestV2SupersessionRepo_MatchesGoldenSHAs builds and re-verifies this same
// history) — a second, independent repo rather than a fourth layer on the
// v0 corpus's history, since nothing about the v2 overlay needs to
// interleave with v0's existing golden commits. examples/showcase/'s v2
// fixtures (loan-workflow, loan-workflow-v2, and the reaffirmation that
// pins loan-workflow-v2's commit) cite these SHAs, so this walk test's
// accepted-token set grows to include them.
//
// public-rollout-plan Task 1.5 extends the same dedicated history with two
// more layers (chained after the first two) for rate-lock/rate-lock-v2 —
// the feature-rung supersession pair whose v2 now carries a real
// `supersession:` block (VL-015): a predecessor's object manifest is read
// via `git show <pred.frozen.commit>:<pred.RelPath>`, which only succeeds
// if the predecessor already exists, with matching content, at that exact
// commit — impossible under the layers.txt main corpus's own "layer N
// cites layer N-1" convention for a file introduced fresh in layer N. Both
// pairs solve it the same way: a draft-then-frozen two-commit sub-history.
var goldenHeadsV2 = []string{
	"b5117ecc69b6779ad75cde60d4aec206ece0950b", // v2 layer 1 (loan-workflow v1 draft)
	"06a3f4cabb226fe9344e1645e27c344493b6b62b", // v2 layer 2 (loan-workflow v1 frozen + loan-workflow-v2 draft)
	"620ade86bbd810b440a0d995859745d4402d7be8", // v2 layer 3 (rate-lock v1 draft)
	"87c65ef5e70024c112b12e275d550f1ca8584df3", // v2 layer 4 (rate-lock v1 frozen + rate-lock-v2 draft)
}

// parseLayers reads layers.txt and returns, for each layer number in
// ascending order, the ordered list of corpus-relative file paths
// belonging to it.
func parseLayers(t *testing.T) (order []int, files map[int][]string) {
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

// buildFixtureRepo builds the git repo described by layers.txt via
// fixturegit, reading each layer's file content straight off disk.
func buildFixtureRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	order, files := parseLayers(t)

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
		layers = append(layers, fixturegit.Layer{
			Files:   layerFiles,
			Message: fmt.Sprintf("layer %d", n),
		})
	}

	return fixturegit.Build(t, layers)
}

// TestFixtureRepo_MatchesGoldenSHAs proves the corpus's git history is
// exactly the golden SHAs baked into every corpus file's pins and frozen
// stamps — the "build once, bake in, test forever" contract.
func TestFixtureRepo_MatchesGoldenSHAs(t *testing.T) {
	repo := buildFixtureRepo(t)

	if len(repo.Heads) != len(goldenHeads) {
		t.Fatalf("built %d layers, want %d (layers.txt vs goldenHeads out of sync)", len(repo.Heads), len(goldenHeads))
	}
	for i, want := range goldenHeads {
		if repo.Heads[i] != want {
			t.Fatalf("layer %d SHA = %s, want golden %s", i+1, repo.Heads[i], want)
		}
	}
	if repo.Head != goldenHeads[len(goldenHeads)-1] {
		t.Fatalf("Head = %s, want final golden %s", repo.Head, goldenHeads[len(goldenHeads)-1])
	}
}

// hexTokenRe matches a bare, word-bounded 40-character lowercase hex
// string — the shape of every git sha used in this fixture. \b correctly
// excludes 40-char substrings of the longer 64-char sha256: digests
// elsewhere in the corpus, since those digests have no word boundary at
// offset 40 (more hex characters follow immediately).
var hexTokenRe = regexp.MustCompile(`\b[0-9a-f]{40}\b`)

// TestFixtureCorpus_PinsNameGoldenCommits scans every corpus file (both
// the git-layered committed-zone files and the standalone mutable/derived
// fixtures) for 40-hex-character tokens and asserts each one is one of
// the golden layer SHAs — i.e. every pinned ref, frozen stamp, and
// evidence/rollup/deviation commit field in the corpus names a real,
// reproducible commit (PLAN.md §4 deliverable 3).
func TestFixtureCorpus_PinsNameGoldenCommits(t *testing.T) {
	golden := map[string]bool{}
	for _, h := range goldenHeads {
		golden[h] = true
	}
	for _, h := range goldenHeadsV2 {
		golden[h] = true
	}

	var checked int
	err := filepath.Walk(corpusDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Base(path) == "layers.txt" || filepath.Base(path) == "README.md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, tok := range hexTokenRe.FindAllString(string(data), -1) {
			checked++
			if !golden[tok] {
				t.Errorf("%s: hex commit token %q is not one of the golden SHAs %v", path, tok, goldenHeads)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking corpus: %v", err)
	}
	if checked == 0 {
		t.Fatal("no 40-hex commit tokens found in the corpus at all — test is vacuous, fixture design changed?")
	}
}

// splitJSONL splits JSONL content into individual non-empty lines.
func splitJSONL(data []byte) [][]byte {
	var lines [][]byte
	for _, l := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		lines = append(lines, l)
	}
	return lines
}

// decodeCommittedFile dispatches a single committed-zone corpus file
// (identified by its corpus-relative path) to the right internal/artifact
// decoder, per the kind directories 01 §Directory layout defines.
func decodeCommittedFile(t *testing.T, rel string, data []byte) {
	t.Helper()

	switch {
	case rel == ".verdi/verdi.yaml":
		if _, err := store.DecodeManifest(data); err != nil {
			t.Fatalf("%s: DecodeManifest: %v", rel, err)
		}

	case strings.HasSuffix(rel, "/layout.json"):
		if _, err := artifact.DecodeBoardLayout(data); err != nil {
			t.Fatalf("%s: DecodeBoardLayout: %v", rel, err)
		}

	case strings.HasSuffix(rel, "/spec.md"):
		fm, body, err := artifact.SplitFrontmatter(data)
		if err != nil {
			t.Fatalf("%s: SplitFrontmatter: %v", rel, err)
		}
		if _, err := artifact.DecodeSpec(fm); err != nil {
			t.Fatalf("%s: DecodeSpec: %v", rel, err)
		}
		_ = body

	case strings.HasSuffix(rel, "/board.json"):
		if _, err := artifact.DecodeBoard(data); err != nil {
			t.Fatalf("%s: DecodeBoard: %v", rel, err)
		}

	case strings.HasSuffix(rel, "/rollup.json"):
		if _, err := artifact.DecodeRollup(data); err != nil {
			t.Fatalf("%s: DecodeRollup: %v", rel, err)
		}

	case strings.HasSuffix(rel, "/deviation-report.md"):
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			t.Fatalf("%s: SplitFrontmatter: %v", rel, err)
		}
		if _, err := artifact.DecodeDeviation(fm); err != nil {
			t.Fatalf("%s: DecodeDeviation: %v", rel, err)
		}

	case strings.HasPrefix(rel, ".verdi/adr/"):
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			t.Fatalf("%s: SplitFrontmatter: %v", rel, err)
		}
		if _, err := artifact.DecodeADR(fm); err != nil {
			t.Fatalf("%s: DecodeADR: %v", rel, err)
		}

	case strings.HasPrefix(rel, ".verdi/diagrams/"):
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			t.Fatalf("%s: SplitFrontmatter: %v", rel, err)
		}
		if _, err := artifact.DecodeDiagram(fm); err != nil {
			t.Fatalf("%s: DecodeDiagram: %v", rel, err)
		}

	case strings.HasPrefix(rel, ".verdi/attestations/"):
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			t.Fatalf("%s: SplitFrontmatter: %v", rel, err)
		}
		if _, err := artifact.DecodeAttestation(fm); err != nil {
			t.Fatalf("%s: DecodeAttestation: %v", rel, err)
		}

	case strings.HasPrefix(rel, ".verdi/waivers/"):
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			t.Fatalf("%s: SplitFrontmatter: %v", rel, err)
		}
		if _, err := artifact.DecodeWaiver(fm); err != nil {
			t.Fatalf("%s: DecodeWaiver: %v", rel, err)
		}

	case strings.HasPrefix(rel, ".verdi/conflicts/"):
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			t.Fatalf("%s: SplitFrontmatter: %v", rel, err)
		}
		if _, err := artifact.DecodeConflict(fm); err != nil {
			t.Fatalf("%s: DecodeConflict: %v", rel, err)
		}

	case strings.HasPrefix(rel, ".verdi/obligations/"):
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			t.Fatalf("%s: SplitFrontmatter: %v", rel, err)
		}
		if _, err := artifact.DecodeObligation(fm); err != nil {
			t.Fatalf("%s: DecodeObligation: %v", rel, err)
		}

	default:
		t.Fatalf("%s: no decoder dispatch rule matches this path (update decodeCommittedFile)", rel)
	}
}

// TestFixtureCorpus_CommittedFilesDecode proves every committed-zone
// corpus file (every file named in layers.txt) decodes strictly through
// internal/artifact.
func TestFixtureCorpus_CommittedFilesDecode(t *testing.T) {
	_, files := parseLayers(t)
	for _, layerFiles := range files {
		for _, rel := range layerFiles {
			rel := rel
			t.Run(rel, func(t *testing.T) {
				data, err := os.ReadFile(filepath.Join(corpusDir, rel))
				if err != nil {
					t.Fatalf("reading %s: %v", rel, err)
				}
				decodeCommittedFile(t, rel, data)
			})
		}
	}
}

// TestFixtureCorpus_MutableAndDerivedFilesDecode proves every
// mutable-zone and derived-zone fixture (never git-tracked in the real
// store, per VL-013) decodes strictly through internal/artifact.
func TestFixtureCorpus_MutableAndDerivedFilesDecode(t *testing.T) {
	t.Run("annotations jsonl", func(t *testing.T) {
		path := filepath.Join(corpusDir, "mutable/annotations/spec--stale-decline.jsonl")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		lines := splitJSONL(data)
		if len(lines) != 3 {
			t.Fatalf("got %d annotation records, want 3 (targeted, board-only, agent-task)", len(lines))
		}
		var sawTargeted, sawBoardOnly, sawAgentTask bool
		for i, line := range lines {
			a, err := artifact.DecodeAnnotation(line)
			if err != nil {
				t.Fatalf("line %d: DecodeAnnotation: %v", i, err)
			}
			if a.Target != nil {
				sawTargeted = true
			}
			if a.Target == nil && a.Board != nil {
				sawBoardOnly = true
			}
			if a.Type == artifact.AnnotationAgentTask {
				sawAgentTask = true
			}
		}
		if !sawTargeted || !sawBoardOnly || !sawAgentTask {
			t.Fatalf("annotations fixture missing required variety: targeted=%v boardOnly=%v agentTask=%v", sawTargeted, sawBoardOnly, sawAgentTask)
		}
	})

	t.Run("live board json", func(t *testing.T) {
		path := filepath.Join(corpusDir, "mutable/boards/STORY-1482.json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		b, err := artifact.DecodeBoard(data)
		if err != nil {
			t.Fatalf("DecodeBoard: %v", err)
		}
		if b.Frozen != nil {
			t.Fatal("live board fixture must not carry a frozen stamp")
		}
	})

	derivedDirs := []struct {
		dir        string
		wantSource artifact.ProvenanceSource
	}{
		{"derived/spec--stale-decline/f6dd4c4df724c0b16cae435e96f7e34ac94026c9", artifact.SourceCI},
		{"derived/spec--stale-decline/16219044c9d6d41de9a0de9464ed24d49283b40c", artifact.SourceLocal},
	}
	for _, dd := range derivedDirs {
		dd := dd
		t.Run(dd.dir, func(t *testing.T) {
			path := filepath.Join(corpusDir, dd.dir, "verdicts.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			var raw []json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("unmarshaling verdicts array: %v", err)
			}
			if len(raw) == 0 {
				t.Fatal("verdicts.json has no records")
			}
			for i, rec := range raw {
				ev, err := artifact.DecodeEvidence(rec)
				if err != nil {
					t.Fatalf("record %d: DecodeEvidence: %v", i, err)
				}
				if ev.Provenance.Source != dd.wantSource {
					t.Fatalf("record %d: provenance.source = %q, want %q", i, ev.Provenance.Source, dd.wantSource)
				}
			}
		})
	}
}
